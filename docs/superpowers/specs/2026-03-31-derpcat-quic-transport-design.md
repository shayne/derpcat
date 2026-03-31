# Derpcat QUIC Transport Design

## Summary

Replace the current inner payload path for `derpcat` with a QUIC-based stream transport built on top of the existing DERP bootstrap and direct-path upgrade machinery. Keep the CLI and session model unchanged:

- `listen` / `send` remain one-shot stream tools
- `share` / `open` remain long-lived forwarded-service tools
- tokens remain bearer capabilities
- relay-first and direct-upgrade behavior remain part of the transport

The goal is to remove the current userspace WireGuard + gVisor TCP overlay from the payload path, because benchmarking shows it is the dominant throughput bottleneck on macOS.

## Problem Statement

Current live measurements on pushed `main` show:

- raw LAN baseline to `pve1`: about `2350 Mbps`
- `derpcat share/open` on the same path: about `323 Mbps`
- `derpcat listen/send` on the same path: about `303 Mbps`

Micro-optimizations to buffers, batching, and netstack plumbing do not materially improve those numbers. The practical conclusion is that the current inner architecture is tapped out on this host.

## Goals

- materially improve throughput over the current `~320 Mbps` ceiling on macOS
- preserve the current CLI behavior and token model
- preserve relay-first startup and late upgrade to direct UDP
- support one-shot streams and many concurrent forwarded connections
- keep MTU at `1280`

## Non-Goals

- no control-plane or login dependency
- no tailnet identity model
- no CLI redesign as part of this work
- no attempt to preserve internal compatibility with the current WG/netstack payload path

## Recommended Approach

Use `quic-go` as the new inner stream layer.

This keeps the existing outer peer transport:

- DERP bootstrap
- claim/accept over DERP
- endpoint discovery
- relay-to-direct upgrades
- direct fallback back to relay when necessary

But changes the inner payload layer:

- remove userspace WireGuard from the payload path
- remove gVisor TCP from the payload path
- establish one QUIC connection per claimed peer session
- use QUIC streams for application payloads

## Architecture

### Outer Transport

`pkg/transport` remains the owner of:

- direct candidate management
- disco probing and retries
- relay fallback
- current best-path selection

It must present a stable peer datagram abstraction that can start on relay and later move to direct without restarting the logical session.

### New QUIC Layer

Add a new package, tentatively `pkg/quicpath`, responsible for:

- adapting the single-peer transport manager into a QUIC-capable datagram endpoint
- owning QUIC connection lifecycle
- exposing stream accept/dial APIs to `pkg/session`

QUIC must not know about DERP versus direct. It only sees one logical peer path whose underlying route can migrate.

### Session Mapping

- `listen` accepts one QUIC stream, copies it to stdout, and exits
- `send` dials one QUIC stream, copies stdin into it, and exits
- `share` accepts many QUIC streams, one per forwarded TCP connection, and bridges each to the target backend
- `open` maintains one local TCP listener and opens one QUIC stream per accepted local TCP connection

## Security Model

Continue to use ephemeral per-session credentials only.

- token remains the bearer capability
- QUIC uses ephemeral TLS material derived for the session
- peer authentication is bound to the already-validated session token / claim flow

The exact certificate shape can be implementation-defined, but the connection must reject unexpected peers even when DERP is acting as relay.

## Migration Plan

### Stage 1: QUIC for `listen/send`

First move one-shot payloads to QUIC streams while leaving `share/open` on the old path.

This gives a simpler benchmark surface:

- one connection
- one stream
- no forwarded TCP listener complexity

Success criteria for Stage 1:

- relay-first startup still works
- late direct upgrade still works during an active transfer
- 1 GiB `listen/send` throughput improves materially over the current baseline

### Stage 2: QUIC for `share/open`

Then move `share/open` to one long-lived QUIC connection with many concurrent streams.

At this point:

- remove the inner WireGuard payload path entirely
- remove gVisor netstack usage from the payload path entirely
- preserve the CLI and token UX

## Testing And Validation

### Automated

- unit tests for QUIC session establishment and stream lifecycle
- relay-first then direct-upgrade tests
- multi-stream `share/open` tests
- cancellation and teardown tests

### Live

The following matrix must pass before shipping:

- raw `iperf3` LAN baseline to `pve1`
- `listen/send` 1 GiB to `pve1`
- `share/open` iperf run to `pve1`
- `listen/send` 1 GiB to `hetz`
- `share/open` throughput run to `hetz`
- relay-first then direct-upgrade observed during active sessions

Track for each run:

- sender and receiver throughput
- path transition sequence
- CPU usage on both hosts
- stability under long-running transfer

## Success Criteria

This project is successful when:

- the new QUIC path is measurably faster than the current shipped build
- relay/direct behavior remains correct
- `share/open` remains stable under concurrent connections
- the final implementation is merged on `main`, pushed, and live-tested against both `pve1` and `hetz`

## Risks

- QUIC path migration over the custom transport adapter may need careful handling during relay/direct transitions
- direct-path upgrades must not create duplicate or wedged QUIC sessions
- a partial migration that keeps both payload stacks alive too long would increase maintenance cost

The mitigation is the staged rollout: prove `listen/send` first, then move `share/open`.
