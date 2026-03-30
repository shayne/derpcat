# derpcat Direct Upgrade Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace derpcat's one-shot direct probing with a long-lived Tailscale-style transport manager that can start sessions on DERP relay, keep discovering paths in the background, and upgrade active sessions to direct UDP when the network allows it.

**Architecture:** Introduce `pkg/transport` as the owner of endpoint state, disco pinging, `CallMeMaybe` signaling, and best-path selection. Refactor `pkg/session` to bring up the overlay as soon as any path works and keep `pkg/wg` focused on carrying packets over whichever path `pkg/transport` currently considers best.

**Tech Stack:** Go, public Tailscale DERP map/bootstrap, userspace WireGuard via `wireguard-go`, DERP control messages, STUN endpoint gathering, `mise`, live SSH verification against `hetz` and `pve1`.

---

## File Structure

### Create

- `pkg/transport/manager.go` — long-lived peer transport manager API and lifecycle
- `pkg/transport/state.go` — endpoint registry, path state, best-path tracking, trust windows
- `pkg/transport/disco.go` — disco ping scheduling and `CallMeMaybe` handling
- `pkg/transport/control.go` — DERP control message types used by the transport manager
- `pkg/transport/manager_test.go` — unit tests for relay-first, upgrade, and fallback behavior
- `pkg/transport/fake_test.go` — fake packet conn / fake DERP helpers for transport tests

### Modify

- `pkg/session/external.go` — stdio sender/listener should use the new transport manager
- `pkg/session/external_share.go` — share/open DERP-backed sessions should use the new transport manager
- `pkg/session/types.go` — add runtime states for upgrades/fallback transitions if needed
- `pkg/wg/bind.go` — remove bind-owned direct truth and delegate active path to transport
- `pkg/wg/device.go` — wire `wg.Node` to a transport-aware bind/path selector
- `pkg/traversal/candidates.go` — keep endpoint gathering only, remove one-shot upgrade ownership
- `pkg/traversal/prober.go` — either delete entirely or reduce to transport-internal probe helpers
- `pkg/session/session_test.go` — update end-to-end tests for relay-first-then-direct behavior
- `scripts/promotion-test.sh` — capture path transitions during large transfers
- `scripts/smoke-remote.sh` — verify stdio sessions can upgrade during long runs
- `scripts/smoke-remote-share.sh` — verify share/open can upgrade during long-lived forwarding
- `README.md` — document relay-first, upgrade-later semantics and verbose status transitions
- `AGENTS.md` — update contributor guidance for the new transport package and verification flow

### Remove Or Collapse

- `pkg/traversal/prober.go` if its logic is fully absorbed into `pkg/transport`

## Task 1: Add Failing Transport Manager Tests

**Files:**
- Create: `pkg/transport/manager_test.go`
- Create: `pkg/transport/fake_test.go`
- Modify: `pkg/session/session_test.go`

- [ ] **Step 1: Write focused failing tests for relay-first upgrade and truthful path state**

```go
package transport

import (
    "context"
    "testing"
    "time"
)

func TestManagerStartsRelayAndLaterUpgradesDirect(t *testing.T) {
    ctx, cancel := context.WithCancel(context.Background())
    defer cancel()

    fx := newFakeNetwork()
    m := NewManager(ManagerConfig{
        SessionID:      "sess-upgrade",
        LocalDERP:      fx.LocalDERP,
        PeerDERP:       fx.PeerDERP,
        PacketConn:     fx.LocalConn,
        CandidateSource: fx.CandidateSource,
        Clock:          fx.Clock,
    })

    if err := m.Start(ctx); err != nil {
        t.Fatalf("Start() error = %v", err)
    }
    if got := m.ActivePath(); got != PathRelay {
        t.Fatalf("ActivePath() = %v, want %v", got, PathRelay)
    }

    fx.EnableDirectAfter(3 * time.Second)
    fx.Advance(5 * time.Second)

    if got := m.ActivePath(); got != PathDirect {
        t.Fatalf("ActivePath() after upgrade = %v, want %v", got, PathDirect)
    }
}

func TestManagerFallsBackToRelayWhenDirectBreaks(t *testing.T) {
    ctx, cancel := context.WithCancel(context.Background())
    defer cancel()

    fx := newFakeNetwork()
    fx.EnableDirectImmediately()

    m := NewManager(ManagerConfig{
        SessionID:      "sess-fallback",
        LocalDERP:      fx.LocalDERP,
        PeerDERP:       fx.PeerDERP,
        PacketConn:     fx.LocalConn,
        CandidateSource: fx.CandidateSource,
        Clock:          fx.Clock,
    })

    if err := m.Start(ctx); err != nil {
        t.Fatalf("Start() error = %v", err)
    }
    if got := m.ActivePath(); got != PathDirect {
        t.Fatalf("ActivePath() = %v, want %v", got, PathDirect)
    }

    fx.DisableDirect()
    fx.Advance(2 * time.Second)

    if got := m.ActivePath(); got != PathRelay {
        t.Fatalf("ActivePath() after fallback = %v, want %v", got, PathRelay)
    }
}

func TestManagerDoesNotReportDirectForUnownedUDPNoise(t *testing.T) {
    ctx, cancel := context.WithCancel(context.Background())
    defer cancel()

    fx := newFakeNetwork()
    m := NewManager(ManagerConfig{
        SessionID:      "sess-noise",
        LocalDERP:      fx.LocalDERP,
        PeerDERP:       fx.PeerDERP,
        PacketConn:     fx.LocalConn,
        CandidateSource: fx.CandidateSource,
        Clock:          fx.Clock,
    })

    if err := m.Start(ctx); err != nil {
        t.Fatalf("Start() error = %v", err)
    }

    fx.InjectUnattributedUDP()
    if got := m.ActivePath(); got != PathRelay {
        t.Fatalf("ActivePath() after UDP noise = %v, want %v", got, PathRelay)
    }
}
```

- [ ] **Step 2: Add a failing session-level regression test for long-lived relay-first upgrade**

```go
func TestExternalListenSendCanUpgradeAfterRelayStart(t *testing.T) {
    t.Setenv("DERPCAT_FAKE_TRANSPORT", "1")

    result := runExternalRoundTrip(t, roundTripConfig{
        payload:           []byte("late-upgrade"),
        enableDirectAfter: 4 * time.Second,
        keepAlive:         6 * time.Second,
    })

    if !result.SeenRelay {
        t.Fatalf("SeenRelay = false, want true")
    }
    if !result.SeenDirect {
        t.Fatalf("SeenDirect = false, want true")
    }
}
```

- [ ] **Step 3: Run the focused tests to verify they fail for the current implementation**

Run: `go test ./pkg/transport ./pkg/session -run 'TestManager|TestExternalListenSendCanUpgradeAfterRelayStart' -count=1`

Expected: FAIL with errors such as `undefined: NewManager`, `undefined: PathRelay`, or the session test staying `connected-relay` only.

- [ ] **Step 4: Commit the failing-test scaffold**

```bash
git add pkg/transport/manager_test.go pkg/transport/fake_test.go pkg/session/session_test.go
git commit -m "test: add failing direct-upgrade transport coverage"
```

## Task 2: Introduce pkg/transport And Path State Ownership

**Files:**
- Create: `pkg/transport/manager.go`
- Create: `pkg/transport/state.go`
- Create: `pkg/transport/control.go`
- Test: `pkg/transport/manager_test.go`

- [ ] **Step 1: Add the minimal transport manager types used by the tests**

```go
package transport

import (
    "context"
    "net"
    "sync"
    "time"
)

type Path string

const (
    PathRelay   Path = "relay"
    PathDirect  Path = "direct"
    PathUnknown Path = "unknown"
)

type ManagerConfig struct {
    SessionID       string
    LocalDERP       string
    PeerDERP        string
    PacketConn      net.PacketConn
    CandidateSource func(context.Context) []string
    SendControl     func(context.Context, ControlMessage) error
    ReceiveControl  func(context.Context) (ControlMessage, error)
    Clock           Clock
}

type Clock interface {
    Now() time.Time
    AfterFunc(time.Duration, func()) Timer
}

type Timer interface {
    Stop() bool
}

type Manager struct {
    mu         sync.RWMutex
    cfg        ManagerConfig
    state      pathState
    activePath Path
    closed     bool
}

func NewManager(cfg ManagerConfig) *Manager {
    return &Manager{cfg: cfg, activePath: PathRelay}
}

func (m *Manager) Start(context.Context) error { return nil }

func (m *Manager) ActivePath() Path {
    m.mu.RLock()
    defer m.mu.RUnlock()
    return m.activePath
}
```

- [ ] **Step 2: Add explicit state transitions instead of bind-owned direct truth**

```go
package transport

import "time"

type pathState struct {
    Active           Path
    LastRelayAt      time.Time
    LastDirectAt     time.Time
    Upgrades         int
    Fallbacks        int
    BestEndpoint     string
    LastEndpointsAt  time.Time
}

func (s *pathState) noteRelay(now time.Time) {
    s.Active = PathRelay
    s.LastRelayAt = now
}

func (s *pathState) noteDirect(now time.Time, endpoint string) {
    if s.Active != PathDirect {
        s.Upgrades++
    }
    s.Active = PathDirect
    s.LastDirectAt = now
    s.BestEndpoint = endpoint
}

func (s *pathState) noteFallback(now time.Time) {
    if s.Active == PathDirect {
        s.Fallbacks++
    }
    s.Active = PathRelay
    s.LastRelayAt = now
}
```

- [ ] **Step 3: Implement just enough manager logic to make the initial tests pass**

```go
func (m *Manager) Start(ctx context.Context) error {
    if m.cfg.Clock == nil {
        m.cfg.Clock = realClock{}
    }
    m.mu.Lock()
    m.activePath = PathRelay
    m.mu.Unlock()
    return nil
}

type realClock struct{}

func (realClock) Now() time.Time { return time.Now() }

func (realClock) AfterFunc(d time.Duration, fn func()) Timer {
    return time.AfterFunc(d, fn)
}
```

- [ ] **Step 4: Run the transport tests and make sure the compile-time failures are gone**

Run: `go test ./pkg/transport -run TestManager -count=1`

Expected: still FAIL on upgrade behavior assertions, but no longer fail for missing types.

- [ ] **Step 5: Commit the transport skeleton**

```bash
git add pkg/transport/manager.go pkg/transport/state.go pkg/transport/control.go pkg/transport/manager_test.go pkg/transport/fake_test.go
git commit -m "feat: add transport manager skeleton"
```

## Task 3: Port Ongoing Disco And CallMeMaybe Behavior Into pkg/transport

**Files:**
- Create: `pkg/transport/disco.go`
- Modify: `pkg/transport/manager.go`
- Modify: `pkg/transport/state.go`
- Test: `pkg/transport/manager_test.go`

- [ ] **Step 1: Add transport control messages for endpoint refresh and call-me-maybe signaling**

```go
package transport

type ControlType string

const (
    ControlCandidates  ControlType = "candidates"
    ControlCallMeMaybe ControlType = "call-me-maybe"
)

type ControlMessage struct {
    Type       ControlType `json:"type"`
    Candidates []string    `json:"candidates,omitempty"`
}
```

- [ ] **Step 2: Add a long-lived disco loop that keeps retrying while the session is relayed or stale**

```go
const (
    discoTickInterval      = 2 * time.Second
    endpointRefreshInterval = 15 * time.Second
)

func (m *Manager) runDiscovery(ctx context.Context) {
    timer := m.cfg.Clock.AfterFunc(discoTickInterval, func() { m.discoveryTick(ctx) })
    _ = timer
}

func (m *Manager) discoveryTick(ctx context.Context) {
    if ctx.Err() != nil {
        return
    }
    m.sendDiscoPings(ctx)
    m.maybeSendCallMeMaybe(ctx)
    m.scheduleNextDiscovery(ctx)
}
```

- [ ] **Step 3: Add endpoint registry updates and direct promotion based on validated pings**

```go
func (m *Manager) noteValidatedDirect(now time.Time, endpoint string) {
    m.mu.Lock()
    defer m.mu.Unlock()
    m.activePath = PathDirect
    m.state.noteDirect(now, endpoint)
}

func (m *Manager) noteRelayOnly(now time.Time) {
    m.mu.Lock()
    defer m.mu.Unlock()
    m.activePath = PathRelay
    m.state.noteRelay(now)
}
```

- [ ] **Step 4: Update tests so they simulate late direct enablement via fake CallMeMaybe and confirm upgrade**

```go
func TestManagerStartsRelayAndLaterUpgradesDirect(t *testing.T) {
    ctx, cancel := context.WithCancel(context.Background())
    defer cancel()

    fx := newFakeNetwork()
    m := NewManager(ManagerConfig{
        SessionID:        "sess-upgrade",
        LocalDERP:        fx.LocalDERP,
        PeerDERP:         fx.PeerDERP,
        PacketConn:       fx.LocalConn,
        CandidateSource:  fx.CandidateSource,
        Clock:            fx.Clock,
        SendControl:      fx.SendControl,
        ReceiveControl:   fx.ReceiveControl,
    })

    if err := m.Start(ctx); err != nil {
        t.Fatalf("Start() error = %v", err)
    }

    fx.EnableDirectAfter(3 * time.Second)
    fx.DeliverCallMeMaybeAfter(3 * time.Second)
    fx.Advance(6 * time.Second)

    if got := m.ActivePath(); got != PathDirect {
        t.Fatalf("ActivePath() = %v, want %v", got, PathDirect)
    }
}
```

- [ ] **Step 5: Run the focused tests until upgrade and fallback tests pass**

Run: `go test ./pkg/transport -run TestManager -count=1`

Expected: PASS

- [ ] **Step 6: Commit the ongoing-discovery transport behavior**

```bash
git add pkg/transport/manager.go pkg/transport/state.go pkg/transport/control.go pkg/transport/disco.go pkg/transport/manager_test.go pkg/transport/fake_test.go
git commit -m "feat: add ongoing direct-upgrade discovery"
```

## Task 4: Make pkg/wg Follow Transport Truth Instead Of Bind Heuristics

**Files:**
- Modify: `pkg/wg/bind.go`
- Modify: `pkg/wg/device.go`
- Test: `pkg/wg/wg_test.go`
- Test: `pkg/transport/manager_test.go`

- [ ] **Step 1: Add a transport-owned active-endpoint interface for the bind to consume**

```go
package wg

type PathSelector interface {
    ActiveDirectEndpoint() string
    DirectActive() bool
}

type BindConfig struct {
    PacketConn     net.PacketConn
    DERPClient     *derpbind.Client
    PeerDERP       key.NodePublic
    PathSelector   PathSelector
}
```

- [ ] **Step 2: Remove bind-owned direct truth from inbound UDP reads**

```go
func (s *bindState) readUDP() {
    buf := make([]byte, 64<<10)
    for {
        n, addr, err := s.conn.ReadFrom(buf)
        if err != nil {
            if s.closeCtx.Err() != nil {
                return
            }
            s.reportErr(err)
            return
        }
        udpAddr, ok := addr.(*net.UDPAddr)
        if !ok {
            continue
        }
        if ip, ok := netip.AddrFromSlice(udpAddr.IP); ok {
            s.parent.received.Add(1)
            s.deliver(inboundPacket{
                payload: append([]byte(nil), buf[:n]...),
                ep:      &Endpoint{dst: udpAddr.String(), ip: ip},
            })
        }
    }
}
```

- [ ] **Step 3: Make outbound sends consult the current transport-selected direct endpoint first, then DERP**

```go
func (b *Bind) activeDirectAddr() *net.UDPAddr {
    if b.selector == nil {
        return nil
    }
    ep := b.selector.ActiveDirectEndpoint()
    if ep == "" {
        return nil
    }
    addr, err := net.ResolveUDPAddr("udp", ep)
    if err != nil {
        return nil
    }
    return addr
}
```

- [ ] **Step 4: Add a regression test proving stray UDP no longer flips the bind to direct**

```go
func TestBindDoesNotInferDirectFromArbitraryInboundUDP(t *testing.T) {
    sel := fakeSelector{direct: false}
    b := NewBind(BindConfig{PacketConn: newLoopPacketConn(t), PathSelector: sel})

    if got := b.activeDirectAddr(); got != nil {
        t.Fatalf("activeDirectAddr() = %v, want nil", got)
    }
}
```

- [ ] **Step 5: Run the WG-focused tests**

Run: `go test ./pkg/wg -count=1`

Expected: PASS

- [ ] **Step 6: Commit the transport-aware bind behavior**

```bash
git add pkg/wg/bind.go pkg/wg/device.go pkg/wg/wg_test.go
git commit -m "feat: make wireguard bind follow transport path state"
```

## Task 5: Migrate listen/send To The Long-Lived Transport Manager

**Files:**
- Modify: `pkg/session/external.go`
- Modify: `pkg/session/types.go`
- Test: `pkg/session/session_test.go`
- Test: `cmd/derpcat/listen_test.go`
- Test: `cmd/derpcat/send_test.go`

- [ ] **Step 1: Replace one-shot direct probing with transport manager creation in sender flow**

```go
transportManager := transport.NewManager(transport.ManagerConfig{
    SessionID:       tok.SessionID,
    LocalDERP:       derpClient.PublicKey().ShortString(),
    PeerDERP:        listenerDERP.ShortString(),
    PacketConn:      probeConn,
    CandidateSource: func(ctx context.Context) []string { return publicProbeCandidates(ctx, probeConn, dm) },
    SendControl:     func(ctx context.Context, msg transport.ControlMessage) error { return sendTransportControl(ctx, derpClient, listenerDERP, msg) },
    ReceiveControl:  func(ctx context.Context) (transport.ControlMessage, error) { return receiveTransportControl(ctx, derpClient, listenerDERP) },
})
if err := transportManager.Start(ctx); err != nil {
    return err
}
defer transportManager.Close()
```

- [ ] **Step 2: Replace one-shot direct probing with transport manager creation in listener flow**

```go
transportManager := transport.NewManager(transport.ManagerConfig{
    SessionID:       session.token.SessionID,
    LocalDERP:       session.derp.PublicKey().ShortString(),
    PeerDERP:        peerDERP.ShortString(),
    PacketConn:      session.probeConn,
    CandidateSource: func(ctx context.Context) []string { return publicProbeCandidates(ctx, session.probeConn, session.derpMap) },
    SendControl:     func(ctx context.Context, msg transport.ControlMessage) error { return sendTransportControl(ctx, session.derp, peerDERP, msg) },
    ReceiveControl:  func(ctx context.Context) (transport.ControlMessage, error) { return receiveTransportControl(ctx, session.derp, peerDERP) },
})
if err := transportManager.Start(ctx); err != nil {
    return tok, err
}
defer transportManager.Close()
```

- [ ] **Step 3: Emit path transitions from the transport manager instead of a final one-shot transportState call**

```go
for update := range transportManager.Updates() {
    switch update.Path {
    case transport.PathDirect:
        emitStatus(cfg.Emitter, StateDirect)
    case transport.PathRelay:
        emitStatus(cfg.Emitter, StateRelay)
    }
}
```

- [ ] **Step 4: Update the stdio regression tests to require truthful relay-first-then-direct behavior**

```go
func TestExternalRoundTripReportsUpgradeWhenDirectArrivesLate(t *testing.T) {
    t.Setenv("DERPCAT_FAKE_TRANSPORT", "1")
    result := runExternalRoundTrip(t, roundTripConfig{
        payload:           []byte("upgrade-me"),
        enableDirectAfter: 4 * time.Second,
        keepAlive:         6 * time.Second,
    })

    if !result.SeenRelay || !result.SeenDirect {
        t.Fatalf("SeenRelay=%v SeenDirect=%v, want both true", result.SeenRelay, result.SeenDirect)
    }
}
```

- [ ] **Step 5: Run session and CLI tests for stdio mode**

Run: `go test ./pkg/session ./cmd/derpcat -run 'TestExternal|TestListen|TestSend' -count=1`

Expected: PASS

- [ ] **Step 6: Commit the stdio migration**

```bash
git add pkg/session/external.go pkg/session/types.go pkg/session/session_test.go cmd/derpcat/listen_test.go cmd/derpcat/send_test.go
git commit -m "feat: migrate stdio sessions to transport manager"
```

## Task 6: Migrate share/open To The Long-Lived Transport Manager

**Files:**
- Modify: `pkg/session/external_share.go`
- Modify: `pkg/session/share.go`
- Modify: `pkg/session/open.go`
- Test: `pkg/session/session_test.go`
- Test: `cmd/derpcat/share_test.go`
- Test: `cmd/derpcat/open_test.go`

- [ ] **Step 1: Replace share/open one-shot direct probing with the same transport manager pattern**

```go
transportManager := transport.NewManager(transport.ManagerConfig{
    SessionID:       session.token.SessionID,
    LocalDERP:       session.derp.PublicKey().ShortString(),
    PeerDERP:        peerDERP.ShortString(),
    PacketConn:      session.probeConn,
    CandidateSource: func(ctx context.Context) []string { return publicProbeCandidates(ctx, session.probeConn, session.derpMap) },
    SendControl:     func(ctx context.Context, msg transport.ControlMessage) error { return sendTransportControl(ctx, session.derp, peerDERP, msg) },
    ReceiveControl:  func(ctx context.Context) (transport.ControlMessage, error) { return receiveTransportControl(ctx, session.derp, peerDERP) },
})
```

- [ ] **Step 2: Add a regression test proving a long-lived share/open session can start relayed and later upgrade while serving connections**

```go
func TestShareOpenCanUpgradeDuringLongLivedSession(t *testing.T) {
    t.Setenv("DERPCAT_FAKE_TRANSPORT", "1")

    result := runShareOpenRoundTrip(t, shareOpenConfig{
        targetResponse:     []byte("ok"),
        enableDirectAfter:  5 * time.Second,
        requestCount:       3,
        delayBetweenReqs:   3 * time.Second,
    })

    if !result.SeenRelay {
        t.Fatalf("SeenRelay = false, want true")
    }
    if !result.SeenDirect {
        t.Fatalf("SeenDirect = false, want true")
    }
}
```

- [ ] **Step 3: Preserve one-token-one-claimer behavior while allowing many forwarded TCP connections over the same upgraded transport**

```go
func TestShareTokenStillAllowsExactlyOneOpen(t *testing.T) {
    result := runShareOpenClaimRace(t)
    if result.SecondClaimErr == nil {
        t.Fatal("SecondClaimErr = nil, want ErrSessionClaimed")
    }
}
```

- [ ] **Step 4: Run share/open package and CLI tests**

Run: `go test ./pkg/session ./cmd/derpcat -run 'TestShare|TestOpen' -count=1`

Expected: PASS

- [ ] **Step 5: Commit the share/open migration**

```bash
git add pkg/session/external_share.go pkg/session/share.go pkg/session/open.go pkg/session/session_test.go cmd/derpcat/share_test.go cmd/derpcat/open_test.go
git commit -m "feat: migrate share and open to transport manager"
```

## Task 7: Update Live Verification Scripts And Documentation

**Files:**
- Modify: `scripts/smoke-remote.sh`
- Modify: `scripts/smoke-remote-share.sh`
- Modify: `scripts/promotion-test.sh`
- Modify: `README.md`
- Modify: `AGENTS.md`

- [ ] **Step 1: Make the remote smoke script assert path transitions instead of a single final path**

```bash
if ! grep -Eq 'connected-relay|connected-direct' "$sender_log"; then
  echo "missing sender path state" >&2
  exit 1
fi

if grep -q 'connected-relay' "$sender_log" && ! grep -q 'upgraded-direct' "$sender_log"; then
  echo "sender stayed on relay for full run" >&2
fi
```

- [ ] **Step 2: Update the 1 GiB promotion script to capture whether the path changed mid-run**

```bash
sender_state=$(grep -E 'connected-|upgraded-direct|fell-back-relay' "$sender_log" | tr '\n' ';')
listener_state=$(grep -E 'connected-|upgraded-direct|fell-back-relay' "$listener_log" | tr '\n' ';')
echo "sender_state=$sender_state"
echo "listener_state=$listener_state"
```

- [ ] **Step 3: Document the new runtime semantics and verification commands**

```md
`derpcat` sessions may start on DERP relay and later upgrade to direct UDP.
Use `--verbose` to see transition events such as `upgraded-direct` and
`fell-back-relay` during long-lived sessions.

Recommended verification:

```bash
mise run smoke-remote
mise run smoke-remote-share
./scripts/promotion-test.sh hetz 1024
./scripts/promotion-test.sh pve1 1024
```
```

- [ ] **Step 4: Run the repo checks plus updated local smoke test**

Run: `mise run check && ./scripts/smoke-local.sh`

Expected: PASS

- [ ] **Step 5: Commit scripts and docs**

```bash
git add scripts/smoke-remote.sh scripts/smoke-remote-share.sh scripts/promotion-test.sh README.md AGENTS.md
git commit -m "docs: update transport upgrade verification guidance"
```

## Task 8: Perform Release-Blocking Live Verification

**Files:**
- Modify: none required unless failures are found
- Test: `scripts/smoke-remote.sh`
- Test: `scripts/smoke-remote-share.sh`
- Test: `scripts/promotion-test.sh`

- [ ] **Step 1: Verify local sender to hetz listener**

Run: `./scripts/smoke-remote.sh hetz`

Expected: PASS, with at least one of sender or listener logs showing `connected-direct` or `upgraded-direct`.

- [ ] **Step 2: Verify hetz sender to local listener for late upgrade**

Run: `bash ./scripts/smoke-remote.sh hetz`

Expected: PASS, and if the session begins relayed, the logs should show later direct promotion for long enough runs.

- [ ] **Step 3: Verify pve1 sender to hetz listener**

Run: `./scripts/smoke-remote.sh pve1`

Expected: PASS with truthful path reporting.

- [ ] **Step 4: Verify hetz sender to pve1 listener via share/open path**

Run: `./scripts/smoke-remote-share.sh pve1`

Expected: PASS, serving repeated HTTP requests over one long-lived session, with runtime path transitions captured in logs.

- [ ] **Step 5: Verify 1 GiB transfer on both remote paths**

Run: `./scripts/promotion-test.sh hetz 1024 && ./scripts/promotion-test.sh pve1 1024`

Expected: PASS, checksum match, and logs captured for relay-first/direct-upgrade analysis.

- [ ] **Step 6: Commit only if scripts or docs changed during debugging**

```bash
git status --short
# If output is empty, do not create a no-op commit.
# If output is non-empty because scripts/docs changed during verification:
git add scripts/smoke-remote.sh scripts/smoke-remote-share.sh scripts/promotion-test.sh README.md AGENTS.md
git commit -m "test: capture direct-upgrade verification fixes"
```

## Spec Coverage Check

- Goals to support relay-first then direct-upgrade for all session types are covered by Tasks 3 through 6 and validated in Task 8.
- Reuse of Tailscale-style traversal concepts is covered by Tasks 2 and 3.
- Truthful path reporting is covered by Tasks 4, 5, 6, and 7.
- Live validation requirements from the spec are covered by Task 8.
- No spec gaps remain.
