# Direct UDP Runtime Guardrails Design

Date: 2026-05-07

## Summary

Add runtime guardrails to the direct UDP stream path so aggressive rate and lane choices cannot freeze a transfer.

The current path can establish direct connectivity, choose eight UDP lanes and a high start rate, then keep burning sender CPU while the receiver commits almost no output. This contradicts the intended dynamic/scalable behavior. The fix is not to make direct UDP conservative by default. The fix is to keep aggressive startup, then continuously validate real receiver progress and react quickly when the selected profile is wrong.

The sender should remain free to start with a high-throughput plan from probes. Once data starts, the receiver reports committed progress over the authenticated control plane. The sender uses those reports, plus local replay and send-pressure metrics, to classify the path as healthy, degraded, or stalled. Degraded paths step down rate and lanes without restarting. Stalled paths try one safe direct profile, then fall back to relay and finish rather than hanging.

## Observed Failure

Live checks between this Mac and `canlxc` showed:

- both endpoints reported `direct-friendly` from `derphole netcheck`
- `smoke-remote.sh`, `smoke-remote-share.sh`, and `smoke-remote-derptun.sh` all promoted from relay to direct successfully
- raw direct UDP from Mac to `canlxc` worked with one or two lanes
- raw direct UDP stalled with four or eight lanes
- derphole `listen/pipe` from Mac to `canlxc` selected `udp-lanes=8` and `udp-rate-start-mbps=700`
- the receiver committed only about `2.4-2.8 MiB` after tens of seconds on 16 MiB and 64 MiB runs
- a 1 GiB run committed only about `2.7 MiB` after roughly six minutes
- the reverse direction, `canlxc` to Mac, completed a 16 MiB direct transfer at about `220 Mbps` sender goodput

This means NAT traversal and basic direct UDP are viable. The bad region is the Mac to `canlxc` high-lane direct UDP stream profile.

## Goals

- Keep aggressive direct UDP startup behavior.
- Detect stalled receiver commit progress quickly.
- Recover by stepping down rate and active lanes without restarting the session.
- Fall back to relay if reduced direct UDP does not resume progress.
- Finish transfers instead of spinning with active sender CPU and flat receiver output.
- Preserve high-throughput behavior on healthy paths.
- Emit enough diagnostics to explain selected profiles, downgrades, recovery, and fallback.
- Make `canlxc` Mac-to-host a regression case.

## Non-Goals

- Do not replace the direct UDP stream path with a new transport.
- Do not make relay the preferred steady-state path.
- Do not make default direct UDP permanently low-lane or low-rate.
- Do not redesign the DERP rendezvous protocol beyond the minimum progress-control messages.
- Do not solve native QUIC/TCP auto-growth in this work, except where existing `--parallel` semantics need to stop being misleading for direct UDP.

## Approach

Use a reactive progress controller.

The initial direct UDP setup remains probe-driven and may still choose an aggressive rate and lane count. After the transfer starts, the controller treats actual receiver commit progress as the source of truth. Packet delivery, lane connectivity, and probe results are useful signals, but they are not enough. A path is only healthy if bytes are being committed at the receiver.

When progress looks bad:

1. Reduce the sender rate.
2. Reduce active lanes, with a preferred safe profile of two lanes for paths like Mac to `canlxc`.
3. Hold the recovered profile for the rest of the transfer unless it remains healthy long enough to justify cautious regrowth.
4. If recovery does not produce committed bytes inside a short deadline, demote to relay and complete.

This keeps the peak-throughput intent while bounding the failure window.

## Components

### `directUDPProgressReport`

Add an authenticated receiver-to-sender control message over the existing DERP control plane.

Fields should include:

- committed bytes written to the output stream
- direct bytes accepted by the receiver
- relay bytes accepted by the receiver, if applicable
- receive retransmit or repair counters
- reorder or queued-byte pressure, if available
- active receive lanes
- report sequence number
- receiver monotonic timestamp or elapsed milliseconds

The sender must ignore stale, duplicate, or unauthenticated reports.

### `directUDPRuntimeController`

Add a sender-side state machine for direct UDP data sessions.

States:

- `healthy`: committed bytes are advancing at an acceptable rate
- `degraded`: progress exists but is below expectation, or replay/queue pressure indicates instability
- `stalled`: committed bytes are flat past the stall window while sender/direct activity continues
- `recovering`: a reduced rate/lane profile has been applied and is being evaluated
- `relayFallback`: direct recovery failed and the transfer should finish over relay

Inputs:

- receiver progress reports
- local send bytes and pacing rate
- replay window occupancy and replay-window waits
- retransmit counts
- elapsed time since direct start
- elapsed time since last committed-byte advance
- active lane count and configured rate

Outputs:

- new target rate
- new active lane count
- hold-regrowth decision
- direct-to-relay fallback decision
- diagnostics for verbose logs

### `probe.SendBlastParallel` Integration

The existing direct UDP sender needs a runtime tuning hook. It should either:

- accept live updates for pacing rate and active lane count, or
- expose a small control interface that lets the session layer apply those updates safely.

The implementation must not require tearing down and recreating the full session for a downgrade. Lane reduction can stop scheduling new data on high-numbered lanes while keeping sockets open for late packets and repairs.

### Receiver Metrics

Receiver progress must be based on committed output progress, not merely packet receipt.

For stream mode, this means reporting bytes that have advanced the contiguous write frontier. The `canlxc` failure showed that direct packets can be present while committed output barely moves, so receive-packet counts alone would miss the bug.

## Data Flow

1. Existing rendezvous and direct probing choose an initial direct UDP plan.
2. Sender starts payload using that plan.
3. Receiver emits progress reports periodically while direct data is active.
4. Sender records direct start time and waits for committed progress.
5. If progress is healthy, sender continues with the current profile.
6. If progress is degraded, sender reduces rate and may reduce lanes.
7. If progress is stalled, sender switches to a safe direct profile.
8. If the safe direct profile recovers progress, sender continues direct and suppresses aggressive regrowth.
9. If the safe profile fails, sender demotes to relay and finishes the remaining bytes.

## Timing Defaults

Use conservative initial constants and make them easy to adjust in tests:

- progress report interval: about `250 ms`
- direct grace period after first data: about `1s`
- degraded window: about `1s` without plausible committed progress
- stall window: about `2s` with no committed-byte advance after grace
- recovery evaluation window: about `2s`
- safe recovery profile: two active lanes at `350 Mbps`

The exact numbers should be benchmarked. The key requirement is behavioral: a transfer may not sit for tens of seconds with high sender CPU and nearly flat receiver output.

## Error Handling And Fallback

Direct UDP can be slow, but it must not freeze the transfer.

Rules:

- Missing progress reports after direct start should trigger the watchdog.
- No committed-byte advance past the stall window should mark the path stalled.
- The first stalled response should reduce to a safe direct profile.
- Recovery that resumes committed progress should continue direct.
- Recovery that does not resume progress should demote to relay.
- Relay fallback must preserve payload correctness and should reuse the existing relay-prefix/handoff machinery where possible.
- If relay fallback cannot safely continue, fail explicitly with diagnostics instead of spinning.
- Cleanup must close all benchmark processes and UDP sockets on success, recovery, fallback, and failure paths.

## Diagnostics

Verbose output should include:

- `udp-runtime-controller=true`
- initial selected rate and active lanes
- progress report counts
- last committed byte offset
- degraded and stalled decisions
- downgrade target rate and active lanes
- recovery success or failure
- relay fallback decision and reason
- final direct and relay byte counts

Diagnostics should make a `canlxc`-style run explain itself without inspecting process state manually.

## CLI Semantics

The direct UDP path currently does not behave as users would infer from `--parallel`. A run with `--parallel=1` can still show `udp-lanes=8`.

This design should clarify and fix that behavior:

- `--parallel=N` caps active direct UDP lanes as well as native QUIC/TCP stripes
- direct UDP may keep a larger internal socket pool for punching and repair, but logs must distinguish available sockets from active lanes
- `--parallel=auto` should map naturally to runtime growth/shrink behavior for direct UDP in a later phase

The immediate guardrail work must prevent fixed `--parallel` from being misleading in diagnostics.

## Testing

### Unit Tests

Add tests for the runtime controller:

- healthy progress keeps the current profile
- short jitter does not cause downgrade
- degraded progress reduces rate
- stalled progress reduces lanes
- eight lanes stall, two lanes recover
- reduced direct fails and chooses relay fallback
- stale progress reports are ignored
- unauthenticated progress reports are rejected

### Package Tests

Add fake direct UDP stream tests:

- packets arrive but committed output stalls
- committed progress resumes after lane/rate reduction
- committed progress does not resume and relay fallback is selected
- cleanup runs on recovery, fallback, and explicit failure

Add tests for CLI/session policy:

- fixed `--parallel=N` caps active direct UDP lanes
- logs distinguish available lanes from active lanes
- default behavior can still choose aggressive startup when healthy

### Live Validation

Required live validation with `DERPHOLE_TEST_DISABLE_TAILSCALE_CANDIDATES=1`:

- `./scripts/smoke-remote.sh canlxc`
- `./scripts/smoke-remote-share.sh canlxc`
- `./scripts/smoke-remote-derptun.sh canlxc`
- Mac to `canlxc` direct UDP transfer, at least 16 MiB and 64 MiB
- `canlxc` to Mac direct UDP transfer, at least 16 MiB
- raw probe lane checks showing one and two lanes remain healthy

Before claiming broad success, repeat representative checks against other benchmark hosts such as `ktzlxc` and `uklxc` so the fix does not overfit `canlxc`.

## Success Criteria

- Mac to `canlxc` no longer stalls for more than the configured watchdog window.
- Mac to `canlxc` either completes direct after downgrade or completes over relay with clear diagnostics.
- `canlxc` to Mac still completes direct without regression.
- Healthy high-bandwidth hosts can still use aggressive direct UDP profiles.
- No process or UDP socket leaks after benchmark scripts.
- Payload size and SHA-256 verification still pass.

## Rollout

Phase 1:

- add progress report message and controller unit tests
- collect receiver committed progress
- emit diagnostics without changing behavior

Phase 2:

- wire sender downgrade decisions into direct UDP pacing and active lane scheduling
- honor fixed `--parallel` as an active-lane cap or clarify available-versus-active logs

Phase 3:

- add relay fallback on failed recovery
- run `canlxc` live validation

Phase 4:

- benchmark `ktzlxc`, `canlxc`, and `uklxc`
- tune watchdog windows and safe profile constants
- consider mapping `--parallel=auto` to direct UDP regrowth once shrink/recovery is proven
