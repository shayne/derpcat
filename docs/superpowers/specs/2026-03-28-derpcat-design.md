# derpcat Design

Date: 2026-03-28

## Summary

`derpcat` is a standalone Go CLI that creates a one-shot, bearer-token-authenticated, point-to-point byte stream between two machines using the public Tailscale DERP network for bootstrap and relay fallback, and a just-in-time userspace WireGuard connection for encrypted transport.

The user experience is intentionally netcat-like:

- `derpcat listen` creates a one-shot receive side and prints an opaque token.
- `derpcat send <token>` claims that session and sends or receives a single bidirectional byte stream.
- The session attempts direct UDP connectivity first and falls back to DERP relay if direct traversal fails.
- The transport is fully standalone: no Tailscale login, no `tailscaled`, no tailnet membership, no external control plane.

## Background

This design is based on the DERP documentation in:

- `/Users/shayne/code/derpcat/docs/derp/README.md`
- `/Users/shayne/code/derpcat/docs/derp/architecture.md`
- `/Users/shayne/code/derpcat/docs/derp/protocol.md`
- `/Users/shayne/code/derpcat/docs/derp/derp-map-control-plane.md`
- `/Users/shayne/code/derpcat/docs/derp/client-runtime.md`
- `/Users/shayne/code/derpcat/docs/derp/server-runtime.md`
- `/Users/shayne/code/derpcat/docs/derp/operations.md`
- `/Users/shayne/code/derpcat/docs/derp/testing.md`

The key conclusion from those references is that public DERP is usable without a Tailscale account, but the hard problem is not DERP bootstrap itself. The hard problem is reliable NAT traversal and promotion from relay to direct UDP. That is why the design aims to stay as close as practical to Tailscale’s native traversal behavior instead of inventing a new rendezvous protocol for v1.

## Goals

- Build a new Go binary at `cmd/derpcat`.
- Use the public Tailscale DERP constellation as the bootstrap and relay network.
- Require no login, no `tailscaled`, and no Tailscale control plane.
- Use a self-contained opaque bearer token produced by `listen`.
- Support exactly one successful claimant per token.
- Establish a userspace WireGuard session using `wireguard-go`.
- Attempt direct UDP connectivity and prefer it when available.
- Fall back to DERP relay when direct connectivity fails.
- Support both `stdio` streaming and local TCP bridge modes.
- Keep the data plane clean so status output never contaminates streamed bytes.
- Be testable locally and against real remote Linux hosts.

## Non-Goals

- Multi-use or reusable tokens.
- Multiple concurrent senders.
- Multiplexed streams.
- Persistent node identity.
- Full tailnet semantics.
- General VPN behavior.
- Full parity with all `magicsock` behaviors in Tailscale.
- Non-TCP user payloads for v1.
- File-transfer-specific features beyond what a single byte stream already enables.

## Chosen Approach

### Selected

Use a standalone “native-style” architecture:

- custom and tiny session control plane
- token-driven, one-shot authorization
- public DERP bootstrap
- just-in-time ephemeral identities
- reuse of proven Tailscale DERP/STUN/traversal behavior where practical
- userspace WireGuard transport

### Rejected Alternative: DERP-only transport

This would reduce complexity but would fail the core requirement of upgrading to direct peer-to-peer connectivity when possible.

### Rejected Alternative: Bespoke rendezvous protocol

This looks simpler architecturally but carries high operational risk. NAT traversal is the part most likely to fail in the field. Reusing more battle-tested traversal logic is more important than keeping the control plane “pure.”

### Design Principle

`derpcat` should be independent at the session and CLI layer while remaining conservative and opportunistic at the networking layer. The control plane is custom. The traversal behavior should be as native as practical.

## Session Model

`derpcat listen` creates a one-off session with fresh ephemeral state:

- WireGuard keypair for transport
- DERP routing keypair so peers can address each other through DERP
- rendezvous/discovery keypair for traversal coordination
- random bearer secret
- session ID
- expiry
- mode capability bits

The token is the full capability for claiming that session once.

### Token Contents

The listener token contains:

- protocol version
- expiry timestamp
- session ID
- bootstrap DERP region or node hint
- listener DERP routing public key
- listener WireGuard public key
- listener rendezvous/discovery public key
- bearer secret
- feature bits for compatibility such as `stdio` and `tcp`

The token should be serialized as a compact binary payload with integrity protection and encoded in a paste-friendly textual form such as base64url or base58.

### Token Trust Boundary

The token is a bearer capability. Possession is sufficient for one successful session claim.

The token must not include:

- local private IP addresses
- local interface details
- hostnames
- persistent identities

The sender initially learns only:

- where to bootstrap through DERP
- what public keys to expect
- what bearer secret authorizes the claim
- when the token expires

## Rendezvous And Traversal Flow

### Listener Side

1. Load the public Tailscale DERP map.
2. Run a small STUN or netcheck probe to gather likely public endpoint candidates.
3. Choose a bootstrap DERP region or node.
4. Open a DERP client using a fresh ephemeral DERP identity.
5. Open a UDP socket that will later carry direct probe traffic and WireGuard packets.
6. Emit the token and wait for a claim.

### Sender Side

1. Decode and validate the token.
2. Open its own DERP client with its own fresh ephemeral DERP identity.
3. Open its own UDP socket.
4. Run STUN or netcheck to gather current endpoint candidates.
5. Send a claim message over DERP to the listener’s DERP routing public key.

### Claim Protocol

The sender’s claim message includes:

- session ID
- proof of bearer secret possession
- sender DERP routing public key
- sender WireGuard public key
- sender rendezvous/discovery public key
- sender endpoint candidates
- sender capability bits

The listener validates:

- token expiry
- session ID
- bearer secret
- version compatibility
- one-shot claim state

The listener then returns either:

- `accept`, with listener candidates and current traversal instructions
- `reject`, with a deterministic reason such as expired, already claimed, or incompatible

### Traversal Phase

After a valid claim:

- both sides exchange endpoint candidates over DERP
- both sides send authenticated discovery probes over UDP to all viable candidates
- DERP remains live for coordination and fallback
- direct UDP is preferred as soon as an authenticated path succeeds
- if no direct path succeeds within the configured window, the session continues through DERP relay if available

### Path Promotion Rule

`derpcat` should begin with DERP available, probe direct aggressively, and promote to direct as soon as the session has a validated direct path. Direct connectivity is preferred but not required for session success.

## Transport And Data Plane

Each session carries exactly one bidirectional byte stream.

This is the central v1 simplification. `derpcat` is not a mini VPN and not a general multi-port forwarder. It is one session, one claimant, one bridged stream, then exit.

### Userspace Transport

The data plane should use:

- `wireguard-go` for WireGuard device behavior
- userspace bind plumbing instead of kernel TUN requirements
- a userspace netstack for the overlay TCP listener and dialer

Both peers derive a deterministic session-scoped overlay prefix from the session ID, then assign fixed role addresses:

- listener = derived prefix + host ID 1
- sender = derived prefix + host ID 2

This avoids placing overlay addresses in the token and lets both sides derive the same values independently.

### Overlay Stream Model

Once the session transport is ready:

- the listener exposes a single TCP listener on its overlay address
- the sender dials that overlay listener once connectivity is established
- the resulting overlay TCP connection becomes the single in-process byte stream

Everything local adapts to that one stream.

## CLI Model

### Core Commands

- `derpcat listen`
- `derpcat send <token>`

### Default Attachment

The default local attachment for both commands is `stdio`.

Examples:

- `derpcat listen > out.txt`
- `cat file | derpcat send <token>`

### Local TCP Bridge Modes

Each side may optionally use one local TCP adapter:

- `--tcp-listen <addr>`: accept exactly one local TCP connection and bridge it into the session
- `--tcp-connect <addr>`: dial one local TCP service and bridge the session into it

Examples:

- `derpcat listen --tcp-connect 127.0.0.1:9000`
- `derpcat send <token> --tcp-listen 127.0.0.1:7000`

### Output Controls

- default: minimal human-readable status on `stderr`
- `-v`, `--verbose`: extra bootstrap, DERP, endpoint, and close details
- `-q`, `--quiet`: suppress non-error status
- `-s`, `--silent`: suppress everything except fatal startup or usage errors
- `--print-token-only`: print only the token on `stdout`

The default output should be sparse. Typical status lines:

- `waiting-for-claim`
- `probing-direct`
- `connected-direct`
- `connected-relay`
- `stream-complete`

## Failure Handling

### User-Visible Failure Classes

- malformed or unsupported token
- expired token
- already claimed token
- DERP bootstrap failure
- STUN or endpoint discovery failure
- direct traversal timeout
- relay unavailability
- local adapter failure
- peer disconnected unexpectedly

### Required Behaviors

- If direct traversal fails but DERP relay is healthy, continue and report `connected-relay`.
- If both direct and relay paths fail, exit non-zero with a specific error.
- If the token expires before claim, the listener exits non-zero.
- If a second sender attempts to claim an already claimed session, it gets a deterministic rejection.
- If one side closes the stream cleanly, propagate EOF and exit successfully after draining.
- If the transport breaks unexpectedly, exit non-zero.

### Exit Codes

The CLI should define stable exit code categories for:

- usage error
- token validation error
- authorization or claim rejection
- bootstrap failure
- traversal failure
- relay failure
- local I/O failure
- remote disconnect

The exact numeric mapping can be chosen during implementation, but the categories must remain stable.

## Package Layout

The code should be organized under `pkg/`, not `internal/`, so the reusable pieces can be imported or tested independently if needed later.

Proposed layout:

- `cmd/derpcat`
  - CLI parsing, subcommands, logging, exit codes
- `pkg/token`
  - token schema, encoding, decoding, versioning, expiry validation
- `pkg/session`
  - high-level orchestration and state machine
- `pkg/derpbind`
  - DERP map loading, region selection, DERP client lifecycle, control-message loops
- `pkg/rendezvous`
  - claim, accept, reject, session authorization, one-shot claim state
- `pkg/traversal`
  - STUN or netcheck, endpoint gathering, punch scheduling, direct-path promotion, relay fallback state
- `pkg/wg`
  - `wireguard-go` device lifecycle, peer config, userspace bind integration, overlay addressing
- `pkg/stream`
  - overlay TCP handling, `stdio`, local TCP adapters
- `pkg/telemetry`
  - human-readable status events and verbosity filtering

### Boundary Rules

- `pkg/rendezvous` and `pkg/traversal` must not know about `stdio` or local sockets.
- `pkg/stream` must not know about DERP message details.
- `pkg/session` is the only place that coordinates the full lifecycle end to end.
- Any imported Tailscale internals should sit behind narrow adapters rather than leaking across the codebase.

## Dependencies And Reuse Strategy

### Required Tooling

- Use `mise` for development tooling and task execution.
- Use `go get` to add any required Go dependencies.

### Networking Reuse Direction

The preferred reuse strategy is:

- public DERP map handling from Tailscale-compatible code paths
- DERP client or transport behavior where practical
- STUN or netcheck style endpoint discovery
- traversal and probe behavior that reduces bespoke NAT logic

The design does not require full direct reuse of `magicsock`, but it explicitly prefers native-compatible patterns over inventing a new traversal stack.

## Observability

Observability is intentionally lightweight:

- sparse human-readable status by default
- richer debug logs in verbose mode
- deterministic path outcome reporting: `direct` or `relay`

There is no metrics backend or daemon-level observability in v1.

## Security Model

- all key material is ephemeral per session
- no persistent registry of peers
- no user accounts
- no token reuse
- bearer token is sufficient to claim once
- session token must be treated as sensitive secret material

The token should have a short but usable expiry. The exact default duration can be finalized in implementation planning, but it must be finite and enforced on both sides.

## Testing Strategy

`derpcat` should be verified at three levels.

### Unit Tests

- token encode and decode
- token integrity and expiry validation
- rendezvous message validation
- one-shot claim state transitions
- overlay prefix derivation
- CLI flag compatibility and validation

### Integration Tests

- in-process listener and sender with fake or simulated control transport
- one-stream bridging over `stdio`
- `--tcp-listen` bridging
- `--tcp-connect` bridging
- forced relay mode
- claim races and rejection of second claimant
- clean EOF propagation
- non-clean disconnect handling

### Live Validation

Real-network validation must include:

- local macOS host to `ssh root@hetz`
- `ssh root@hetz` to local macOS host
- same-LAN validation against `ssh root@pve1`
- as many different-network cases as practical to confirm both direct and relayed outcomes

Success criteria:

- one-shot transfer works with no login and no `tailscaled`
- direct UDP path is achieved in at least some real environments
- DERP relay fallback works when direct traversal fails
- `stdio` mode works end to end
- local TCP bridge mode works end to end
- binaries run on macOS locally and Linux x86_64 remotely

### Automation

Add scripted smoke tests using `mise` so the project can repeatedly:

- build locally
- run unit and integration tests
- cross-compile Linux x86_64 binaries
- copy binaries to `hetz` and `pve1`
- run basic transfer checks without hand-driven command setup each time

## Implementation Notes For Planning

The design is intentionally shaped so implementation can proceed incrementally:

1. token and CLI scaffolding
2. DERP bootstrap and claim protocol
3. userspace transport bring-up
4. relay-only end-to-end stream
5. direct traversal promotion
6. TCP bridge modes
7. live validation and hardening

This sequence preserves the requirement that the final tool supports direct promotion, while still giving intermediate checkpoints that can be tested.

## Open Decisions Intentionally Deferred To Planning

These are intentionally left for the planning stage rather than the design stage:

- exact token binary encoding format
- exact exit code numbers
- exact token expiry default
- exact set of reused Tailscale packages versus copied or wrapped logic
- precise `mise` task names

These are implementation-shaping details, not architectural uncertainties.
