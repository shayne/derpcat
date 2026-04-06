# Unified WireGuard Transport Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace derpcat's public QUIC and public-native-TCP transport paths with one `pkg/wg`-backed public transport that uses DERP for rendezvous and relay fallback and direct UDP for the fast path.

**Architecture:** Reuse `pkg/traversal` and `pkg/transport.Manager` for NAT traversal and direct promotion, but move all public data-plane traffic onto `pkg/wg`. Public sessions will establish one WireGuard overlay and run `send/listen` and `share/open` over overlay TCP streams, with DERP carrying the same encrypted packets only when direct UDP is unavailable.

**Tech Stack:** Go, `wireguard-go`, existing `pkg/wg`, existing `pkg/transport`, existing DERP session control in `pkg/session`, SSH live-host validation against `ktzlxc`, `canlxc`, `uklxc`, and `orange-india.exe.xyz`.

---

## File Structure

**Create:**

- `pkg/session/external_wg.go` — public session tunnel creation, overlay dial/listen helpers, and shared setup code for all public modes.
- `pkg/session/external_wg_test.go` — focused tests for public WireGuard tunnel setup and relay/direct behavior.

**Modify:**

- `pkg/session/external.go` — remove QUIC/TCP mode negotiation and switch `sendExternal` / `listenExternal` to the shared WireGuard tunnel.
- `pkg/session/external_share.go` — switch `shareExternal` / `openExternal` from QUIC streams to overlay TCP streams.
- `pkg/session/send.go` — keep external entrypoint behavior but expect the public path to be unified.
- `pkg/session/listen.go` — same as above.
- `pkg/session/open.go` — same as above.
- `pkg/session/share.go` — same as above.
- `pkg/token/token.go` — remove public-native-TCP bootstrap fields and helpers after callers are gone.
- `pkg/token/token_test.go` — update coverage for removed bootstrap fields.
- `pkg/session/session_test.go` — add regression coverage for the unified public path.
- `pkg/wg/bind.go` — add any small hooks needed for public-session direct promotion and relay reporting without changing the packet model.
- `README.md` — update transport wording if user-facing docs still mention the old public path behavior.

**Delete:**

- `pkg/session/external_native_quic.go`
- `pkg/session/external_native_quic_test.go`
- `pkg/session/external_native_tcp.go`
- `pkg/session/external_native_tcp_test.go`
- `pkg/session/external_bootstrap.go`
- `pkg/quicpath/config.go`
- `pkg/quicpath/integration_test.go`
- `pkg/quicpath/metrics_tracer.go`

## Tasks

### Task 1: Add a shared public WireGuard tunnel surface

**Files:**
- Create: `pkg/session/external_wg.go`
- Test: `pkg/session/external_wg_test.go`

- [ ] **Step 1: Write the failing test**

```go
func TestExternalWGTunnelUsesDirectAndDERPFallback(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	tunnel, err := newExternalWGTunnel(ctx, externalWGTunnelConfig{
		LocalAddr:      netip.MustParseAddr("192.168.4.28"),
		PeerAddr:       netip.MustParseAddr("192.168.4.29"),
		DirectConn:     mustListenPacket(t),
		DERPClient:     newStubDERPClient(t),
		PeerDERP:       mustNodePublic(t),
		PathSelector:   newStubPathSelector(),
		TransportLabel: "test",
	})
	if err != nil {
		t.Fatalf("newExternalWGTunnel() error = %v", err)
	}
	defer tunnel.Close()

	if tunnel.node == nil {
		t.Fatal("newExternalWGTunnel() returned nil node")
	}
	if tunnel.listenPort == 0 {
		t.Fatal("newExternalWGTunnel() did not allocate overlay listen port")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./pkg/session -run TestExternalWGTunnelUsesDirectAndDERPFallback -count=1`
Expected: FAIL with `undefined: newExternalWGTunnel`

- [ ] **Step 3: Write minimal implementation**

```go
type externalWGTunnel struct {
	node       *wg.Node
	listenPort uint16
}

type externalWGTunnelConfig struct {
	LocalAddr      netip.Addr
	PeerAddr       netip.Addr
	DirectConn     net.PacketConn
	DERPClient     *derpbind.Client
	PeerDERP       key.NodePublic
	PathSelector   wg.PathSelector
	TransportLabel string
}

func newExternalWGTunnel(ctx context.Context, cfg externalWGTunnelConfig) (*externalWGTunnel, error) {
	localPriv, peerPub, err := deriveExternalWGKeys()
	if err != nil {
		return nil, err
	}
	node, err := wg.NewNode(wg.Config{
		PrivateKey:    localPriv,
		PeerPublicKey: peerPub,
		LocalAddr:     cfg.LocalAddr,
		PeerAddr:      cfg.PeerAddr,
		PacketConn:    cfg.DirectConn,
		Transport:     cfg.TransportLabel,
		DERPClient:    cfg.DERPClient,
		PeerDERP:      cfg.PeerDERP,
		PathSelector:  cfg.PathSelector,
	})
	if err != nil {
		return nil, err
	}
	return &externalWGTunnel{node: node, listenPort: 1}, nil
}

func (t *externalWGTunnel) Close() error {
	if t == nil || t.node == nil {
		return nil
	}
	return t.node.Close()
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./pkg/session -run TestExternalWGTunnelUsesDirectAndDERPFallback -count=1`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add pkg/session/external_wg.go pkg/session/external_wg_test.go
git commit -m "feat: add shared external wg tunnel"
```

### Task 2: Migrate public `send/listen` off QUIC and onto the shared tunnel

**Files:**
- Modify: `pkg/session/external.go`
- Modify: `pkg/session/session_test.go`
- Test: `pkg/session/external_wg_test.go`

- [ ] **Step 1: Write the failing test**

```go
func TestSendExternalUsesOverlayTCPInsteadOfQUIC(t *testing.T) {
	cfg := SendConfig{UsePublicDERP: true, Token: mustExternalToken(t)}
	if usesExternalQUICForSend(cfg) {
		t.Fatal("public send path still depends on QUIC")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./pkg/session -run TestSendExternalUsesOverlayTCPInsteadOfQUIC -count=1`
Expected: FAIL because `usesExternalQUICForSend` is undefined or still returns true

- [ ] **Step 3: Write minimal implementation**

```go
func sendExternal(ctx context.Context, cfg SendConfig) error {
	// issue claim, create transport manager, then build externalWGTunnel
	// and stream stdin/file bytes into one or more overlay TCP conns.
	return sendExternalViaWGTunnel(ctx, cfg)
}

func listenExternal(ctx context.Context, cfg ListenConfig) (string, error) {
	// accept claim, create transport manager, then build externalWGTunnel
	// and accept overlay TCP conns into the configured sink.
	return listenExternalViaWGTunnel(ctx, cfg)
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./pkg/session -run 'Test(SendExternalUsesOverlayTCPInsteadOfQUIC|ListenExternal)' -count=1`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add pkg/session/external.go pkg/session/session_test.go pkg/session/external_wg_test.go
git commit -m "feat: move public stdio sessions onto wg tunnel"
```

### Task 3: Migrate public `share/open` onto the shared tunnel

**Files:**
- Modify: `pkg/session/external_share.go`
- Modify: `pkg/session/open.go`
- Modify: `pkg/session/share.go`
- Test: `pkg/session/session_test.go`

- [ ] **Step 1: Write the failing test**

```go
func TestShareExternalUsesOverlayTCPInsteadOfQUIC(t *testing.T) {
	cfg := ShareConfig{UsePublicDERP: true, TargetAddr: "127.0.0.1:8080"}
	if usesExternalQUICForShare(cfg) {
		t.Fatal("public share path still depends on QUIC")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./pkg/session -run TestShareExternalUsesOverlayTCPInsteadOfQUIC -count=1`
Expected: FAIL because `usesExternalQUICForShare` is undefined or still returns true

- [ ] **Step 3: Write minimal implementation**

```go
func shareExternal(ctx context.Context, cfg ShareConfig) (string, error) {
	return shareExternalViaWGTunnel(ctx, cfg)
}

func openExternal(ctx context.Context, cfg OpenConfig, tok token.Token) error {
	return openExternalViaWGTunnel(ctx, cfg, tok)
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./pkg/session -run 'Test(ShareExternalUsesOverlayTCPInsteadOfQUIC|OpenExternal)' -count=1`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add pkg/session/external_share.go pkg/session/open.go pkg/session/share.go pkg/session/session_test.go
git commit -m "feat: move public share sessions onto wg tunnel"
```

### Task 4: Delete obsolete QUIC and public-native-TCP transport code

**Files:**
- Delete: `pkg/session/external_native_quic.go`
- Delete: `pkg/session/external_native_quic_test.go`
- Delete: `pkg/session/external_native_tcp.go`
- Delete: `pkg/session/external_native_tcp_test.go`
- Delete: `pkg/session/external_bootstrap.go`
- Delete: `pkg/quicpath/config.go`
- Delete: `pkg/quicpath/integration_test.go`
- Delete: `pkg/quicpath/metrics_tracer.go`
- Modify: `pkg/token/token.go`
- Modify: `pkg/token/token_test.go`

- [ ] **Step 1: Write the failing test**

```go
func TestTokenNoLongerCarriesNativeTCPBootstrap(t *testing.T) {
	tok := Token{}
	if tok.NativeTCPBootstrapAddr() != "" {
		t.Fatal("native tcp bootstrap address should not exist")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./pkg/token -run TestTokenNoLongerCarriesNativeTCPBootstrap -count=1`
Expected: FAIL because bootstrap helpers still exist

- [ ] **Step 3: Write minimal implementation**

```go
type Token struct {
	Version      uint8
	SessionID    [16]byte
	ExpiresUnix  int64
	BootstrapRegion uint16
	DERPPublic   [32]byte
	BearerSecret [32]byte
	Capabilities uint32
}

// Delete:
// - nativeTCPBootstrapAddr field
// - SetNativeTCPBootstrapAddr
// - NativeTCPBootstrapAddr
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./pkg/token ./pkg/session -count=1`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add pkg/token/token.go pkg/token/token_test.go pkg/session
git rm pkg/session/external_native_quic.go pkg/session/external_native_quic_test.go pkg/session/external_native_tcp.go pkg/session/external_native_tcp_test.go pkg/session/external_bootstrap.go pkg/quicpath/config.go pkg/quicpath/integration_test.go pkg/quicpath/metrics_tracer.go
git commit -m "refactor: remove legacy public transport paths"
```

### Task 5: Verify package behavior and smoke tests locally

**Files:**
- Modify: `README.md`

- [ ] **Step 1: Write the failing test**

```go
func TestPublicSessionDocsDescribeUnifiedTransport(t *testing.T) {
	data, err := os.ReadFile("README.md")
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(data, []byte("QUIC direct fast path")) {
		t.Fatal("README still documents the old public transport")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./pkg/session -run TestPublicSessionDocsDescribeUnifiedTransport -count=1`
Expected: FAIL because docs or fixtures still mention old transport wording

- [ ] **Step 3: Write minimal implementation**

```md
Update README wording so public derpcat transport is described as:
- DERP for rendezvous and relay fallback
- direct UDP plus NAT traversal for the fast path
- unified encrypted transport for all public modes
```

- [ ] **Step 4: Run test to verify it passes**

Run:

```bash
go test ./pkg/session ./pkg/token -count=1
mise run test
mise run vet
mise run smoke-local
```

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add README.md
git commit -m "docs: describe unified public transport"
```

### Task 6: Prove the refactor on live hosts

**Files:**
- Modify: `README.md` if commands need updated examples

- [ ] **Step 1: Establish the live direct-path acceptance run on `ktzlxc`**

Run on `ktzlxc`:

```bash
DERPCAT_TEST_DISABLE_TAILSCALE_CANDIDATES=1 npx -y derpcat@dev listen > /tmp/ktzlxc.out
```

Run on this Mac:

```bash
cat ~/1GBFile | pv -brt | DERPCAT_TEST_DISABLE_TAILSCALE_CANDIDATES=1 ./dist/derpcat send <token>
```

Expected:
- logs show relay rendezvous then direct promotion
- no public-native-TCP bootstrap logs
- no QUIC logs
- throughput stays near the proven WAN ceiling

- [ ] **Step 2: Verify the reverse direction on `ktzlxc`**

Run the inverse send/listen pair and record throughput.

- [ ] **Step 3: Verify relay fallback**

Run a controlled case that prevents direct promotion and confirm transfer still succeeds via DERP relay using the same transport.

- [ ] **Step 4: Verify `share/open`**

Run:

```bash
python3 -m http.server 8080
DERPCAT_TEST_DISABLE_TAILSCALE_CANDIDATES=1 ./dist/derpcat share 127.0.0.1:8080
DERPCAT_TEST_DISABLE_TAILSCALE_CANDIDATES=1 ./dist/derpcat open <token>
curl http://127.0.0.1:<bind-port>/
```

Expected: overlay-backed forwarding works on the unified transport.

- [ ] **Step 5: Widen to the rest of the host matrix**

Run equivalent `send/listen` validation for:
- `canlxc`
- `uklxc`
- `orange-india.exe.xyz`

- [ ] **Step 6: Commit**

```bash
git add README.md
git commit -m "test: prove unified wg transport on live hosts"
```

### Task 7: Final verification, push, and CI confirmation

**Files:**
- Modify: none unless fixes are needed from verification

- [ ] **Step 1: Run final verification**

Run:

```bash
go test ./pkg/session ./pkg/token ./pkg/wg -count=1
mise run check
mise run release:npm-dry-run
```

Expected: PASS

- [ ] **Step 2: Push `main`**

```bash
git push origin main
```

Expected: push succeeds

- [ ] **Step 3: Confirm GitHub checks and npm dev release**

Run:

```bash
gh run list --limit 5
```

Expected: `Checks` and `Release` complete successfully for the pushed commit

## Success Criteria

- Public derpcat uses the unified `pkg/wg` transport by default.
- QUIC direct code is gone.
- Public-native-TCP bootstrap code is gone.
- Live no-open-ports transfers on this Mac and `ktzlxc` promote to direct UDP automatically.
- The direct path stays near the proven WAN ceiling.
- DERP relay fallback still works.
- `send/listen` and `share/open` both work on the unified transport.
- CI and npm dev publishing are green.
