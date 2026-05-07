# Direct UDP Runtime Guardrails Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make direct UDP avoid the Mac to `canlxc` four/eight-lane stall while preserving aggressive profiles on paths that prove they can use them.

**Architecture:** Keep the existing DERP rendezvous and relay-prefix handoff. Fix the data-plane profile selection so no-probe relay-prefix upgrades start with a two-lane safe profile, make `--parallel=N` cap active direct UDP lanes, and feed sender rate control with committed receiver progress rather than merely accepted out-of-order packets.

**Tech Stack:** Go, existing `pkg/session` direct UDP planner, existing `pkg/probe` blast stream transport, `mise` test/build tasks, existing smoke and promotion benchmark scripts.

---

## File Structure

- Modify `pkg/session/external_direct_udp.go`: direct UDP plan lane budgeting, `--parallel` cap application, verbose lane diagnostics.
- Modify `pkg/session/external_direct_udp_test.go`: unit coverage for no-probe two-lane budget and fixed parallel caps.
- Modify `pkg/probe/session.go`: committed-progress stats for blast stream receive and active-lane cap support in parallel sender scheduling.
- Modify `pkg/probe/blast_rate.go`: explicit stalled-commit response that reduces adaptive rate when sender bytes advance but committed receiver bytes do not.
- Modify `pkg/probe/blast_rate_test.go`, `pkg/probe/session_test.go`, or `pkg/probe/blast_control_test.go`: focused coverage for committed-progress stats and rate downgrade.
- Update the uncommitted report with live `canlxc` findings after verification.

## Task 1: Plan Commit

**Files:**
- Create: `docs/superpowers/plans/2026-05-07-direct-udp-runtime-guardrails.md`

- [ ] **Step 1: Save this plan**

- [ ] **Step 2: Verify clean formatting**

Run:

```bash
git diff -- docs/superpowers/plans/2026-05-07-direct-udp-runtime-guardrails.md
```

Expected: the plan contains concrete files, tasks, commands, and no placeholder sections.

- [ ] **Step 3: Commit and push**

```bash
git add docs/superpowers/plans/2026-05-07-direct-udp-runtime-guardrails.md
PATH="$(dirname "$(mise which go)"):$PATH" git commit -m "docs: plan direct udp runtime guardrails"
git push origin main
```

## Task 2: Safe No-Probe Direct UDP Profile

**Files:**
- Modify: `pkg/session/external_direct_udp.go`
- Modify: `pkg/session/external_direct_udp_test.go`

- [ ] **Step 1: Add a failing test**

Extend `TestExternalPrepareDirectUDPSendSkipsBlockingRateProbesForRelayPrefixUpgrade` so the relay-prefix/no-probe plan with multiple available sockets expects:

```go
if got, want := len(result.plan.probeConns), externalDirectUDPActiveLanesForRate(externalDirectUDPActiveLaneTwoMaxMbps, externalDirectUDPParallelism); got != want {
    t.Fatalf("relay-prefix no-probe active lanes = %d, want %d", got, want)
}
if got, want := result.plan.sendCfg.RateMbps, externalDirectUDPActiveLaneTwoMaxMbps; got != want {
    t.Fatalf("relay-prefix no-probe start rate = %d, want %d", got, want)
}
```

Use eight local UDP sockets in the test so it catches the current four-lane inference.

- [ ] **Step 2: Run the failing test**

```bash
mise exec -- go test ./pkg/session -run TestExternalPrepareDirectUDPSendSkipsBlockingRateProbesForRelayPrefixUpgrade -count=1
```

Expected: fail because the no-probe upgrade retains more than two active lanes.

- [ ] **Step 3: Implement the safe profile**

In `externalPrepareDirectUDPSend`, after the existing retained-lane calculation, clamp `retainedLanes` for `cfg.skipDirectUDPRateProbes` to the active lane count for `activeRateMbps`. Keep the socket pool creation unchanged; only the selected data lanes should shrink.

- [ ] **Step 4: Run targeted tests**

```bash
mise exec -- go test ./pkg/session -run 'TestExternalPrepareDirectUDPSendSkipsBlockingRateProbesForRelayPrefixUpgrade|TestExternalDirectUDPActiveLanesForRateScalesDownSlowPaths|TestExternalDirectUDPRetainedLanesForRateKeepsStripedHeadroom' -count=1
```

Expected: pass.

- [ ] **Step 5: Commit and push**

```bash
git add pkg/session/external_direct_udp.go pkg/session/external_direct_udp_test.go
PATH="$(dirname "$(mise which go)"):$PATH" git commit -m "session: guard no-probe direct udp lanes"
git push origin main
```

## Task 3: Honor Fixed Parallel Caps

**Files:**
- Modify: `pkg/session/external_direct_udp.go`
- Modify: `pkg/session/external_direct_udp_test.go`
- Modify: `pkg/probe/session.go`
- Modify: `pkg/probe/session_test.go`

- [ ] **Step 1: Add cap tests**

Add a session test that calls `externalPrepareDirectUDPSend` with eight sockets and `SendConfig{ParallelPolicy: FixedParallelPolicy(1)}` and expects one selected active data lane. Add a probe test that calls `SendBlastParallel` with two lanes, `StripedBlast: true`, and a new active-lane cap of one, then verifies the transfer completes and reports the cap.

- [ ] **Step 2: Run the failing tests**

```bash
mise exec -- go test ./pkg/session ./pkg/probe -run 'FixedParallel|ActiveLaneCap' -count=1
```

Expected: fail until cap plumbing exists.

- [ ] **Step 3: Add `probe.SendConfig.MaxActiveLanes`**

Add an integer cap to `probe.SendConfig`. In `SendBlastParallel`, compute active lanes by applying the cap to `parallelActiveLanesForRate`. For striped scheduling, use the active lane count when choosing the next lane for new data so inactive lanes do not receive new payload.

- [ ] **Step 4: Wire session policy into probe config**

In `externalPrepareDirectUDPSend`, normalize `cfg.ParallelPolicy`. For fixed policies, cap retained direct UDP data lanes and set `sendCfg.MaxActiveLanes` to the same value. Emit separate verbose logs for available sockets and selected active lanes.

- [ ] **Step 5: Run targeted tests**

```bash
mise exec -- go test ./pkg/session ./pkg/probe -run 'FixedParallel|ActiveLaneCap|BlastParallelStreamPreservesOrder' -count=1
```

Expected: pass.

- [ ] **Step 6: Commit and push**

```bash
git add pkg/session/external_direct_udp.go pkg/session/external_direct_udp_test.go pkg/probe/session.go pkg/probe/session_test.go
PATH="$(dirname "$(mise which go)"):$PATH" git commit -m "probe: cap direct udp active lanes"
git push origin main
```

## Task 4: Committed-Progress Feedback

**Files:**
- Modify: `pkg/probe/session.go`
- Modify: `pkg/probe/blast_rate.go`
- Modify: `pkg/probe/blast_rate_test.go`
- Modify: `pkg/probe/blast_control_test.go`

- [ ] **Step 1: Add failing committed-progress tests**

Add tests for receiver stats where out-of-order striped packets are accepted but the contiguous output frontier is flat. The stats payload must report the committed byte frontier, not total accepted payload bytes.

- [ ] **Step 2: Add rate stall test**

Add a `blastRateController` test where `SentPayloadBytes` increases while `ReceivedPayloadBytes` stays flat past the stall window. Expected result: the rate drops toward the conservative floor and holds before any increase.

- [ ] **Step 3: Implement committed progress**

Change `sendStatsFeedbackLocked` and `sendStripedStatsFeedbackLocked` so `ReceivedPayloadBytes` is committed output progress. Use `state.nextOffset` for striped stream output and `c.bytesReceived` for discard/spooled cases where the coordinator is the committed-byte source.

- [ ] **Step 4: Implement stalled-progress downgrade**

Add minimal stall state to the adaptive rate controller. When committed bytes do not advance while sent bytes continue advancing for the configured window, call the existing decrease path and hold increases. Keep the constants near the existing blast rate constants.

- [ ] **Step 5: Run targeted tests**

```bash
mise exec -- go test ./pkg/probe -run 'Committed|Stall|Stats|BlastRate' -count=1
```

Expected: pass.

- [ ] **Step 6: Commit and push**

```bash
git add pkg/probe/session.go pkg/probe/blast_rate.go pkg/probe/blast_rate_test.go pkg/probe/blast_control_test.go
PATH="$(dirname "$(mise which go)"):$PATH" git commit -m "probe: rate direct udp by committed progress"
git push origin main
```

## Task 5: Verification And Benchmarks

**Files:**
- Create or modify: uncommitted report under `docs/superpowers/reports/`

- [ ] **Step 1: Run package and full checks**

```bash
mise exec -- go test ./pkg/probe ./pkg/session ./cmd/derphole -count=1
mise run test
mise run build
```

Expected: all pass.

- [ ] **Step 2: Run `canlxc` smoke**

```bash
DERPHOLE_TEST_DISABLE_TAILSCALE_CANDIDATES=1 ./scripts/smoke-remote.sh canlxc
DERPHOLE_TEST_DISABLE_TAILSCALE_CANDIDATES=1 ./scripts/smoke-remote-share.sh canlxc
./scripts/smoke-remote-derptun.sh canlxc
```

Expected: direct promotion still succeeds.

- [ ] **Step 3: Run `canlxc` throughput**

```bash
DERPHOLE_TEST_DISABLE_TAILSCALE_CANDIDATES=1 ./scripts/promotion-test.sh canlxc 16
DERPHOLE_TEST_DISABLE_TAILSCALE_CANDIDATES=1 ./scripts/promotion-test.sh canlxc 64
DERPHOLE_TEST_DISABLE_TAILSCALE_CANDIDATES=1 ./scripts/promotion-test-reverse.sh canlxc 16
```

Expected: Mac to `canlxc` no longer freezes; payload SHA and size match; cleanup checks pass.

- [ ] **Step 4: Compare baselines**

Run `iperf3` over Tailscale and the user-provided speedtest binary on `canlxc` to record WAN/baseline limits. Add raw probe one-lane and two-lane results for context.

- [ ] **Step 5: Regression hosts**

Run at least one 16 MiB no-Tailscale promotion test against another reachable high-bandwidth host such as `ktzlxc` or `uklxc`.

- [ ] **Step 6: Record an uncommitted report**

Document commands, pass/fail status, throughput, lane counts, and final diagnosis. Leave the report uncommitted unless it contains repo-safe, non-machine-specific findings the user wants to preserve.

## Task 6: Release

**Files:**
- Version/tag only unless the repo has a source version file requiring a patch bump.

- [ ] **Step 1: Verify clean committed code**

```bash
git status --short --branch
git tag --sort=-v:refname | head -5
```

Expected: only the report may remain uncommitted; latest tag is `v0.12.1` unless another tag has appeared.

- [ ] **Step 2: Create patch tag**

```bash
git -c tag.gpgSign=false tag v0.12.2
git push origin main v0.12.2
```

Expected: main and tag push succeed.

- [ ] **Step 3: Check release workflow**

```bash
gh run list --branch main --limit 10
gh run watch --exit-status
```

Expected: build/release workflow succeeds before final response.

