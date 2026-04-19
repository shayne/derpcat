# iOS QR Transfer Design

## Purpose

Derphole should have a proof-of-concept iOS app that receives a file from a terminal QR code. The intended user flow is simple: run `derphole send --qr <file>` on the sending machine, scan the QR code with the iPhone app, receive the file through the existing Derphole transport stack, and choose where to save it on iOS.

The important requirement is direct transfer. The iOS app must reuse the existing Go code as a native library and must have the same direct-preferred transport behavior as the CLI. Relay fallback remains available for reliability, but a relay-only successful transfer does not satisfy the proof-of-concept goal.

## Current State

The repository already has a starter SwiftUI app under `apple/Derphole`. The app currently displays placeholder content and has no Derphole integration.

The CLI's structured transfer path lives in `cmd/derphole/send.go`, `cmd/derphole/receive.go`, and `pkg/derphole/transfer.go`. `derphole send` creates an offer and prints receive instructions through `WriteSendInstruction`. `pkg/derphole.Receive` already supports `token.CapabilityWebFile` tokens through `receiveViaWebRelay`.

The web-file receive path already has a direct transport seam. `receiveViaWebRelay` calls `webrelay.ReceiveWithOptions` and, when `ForceRelay` is false, passes `webrtcdirect.New()` as the direct transport. This mirrors the direct-preferred, relay-fallback behavior needed by the iOS app.

The repository has `mise` tasks for Go build/test and Apple build/test. The mobile toolchain and generated Go framework should be added to that same task surface.

## External References

Go's mobile toolchain supports using a Go package in an iOS application by generating Objective-C bindings from a Go package with `gomobile bind`. The package exported to Swift must expose a small cross-language API because gomobile supports only a subset of Go types.

The terminal QR dependency should be `github.com/mdp/qrterminal/v3`. It renders QR codes directly to an `io.Writer`, supports compact half-block output, and fits the CLI use case without adding image-generation plumbing.

iOS QR scanning should use AVFoundation. The app must include a camera usage description.

iOS local network access requires a clear `NSLocalNetworkUsageDescription` when an app uses the local network directly or indirectly. Direct unicast local-host communication is covered by this privacy model. If physical-device testing shows Pion ICE needs IP multicast or broadcast on iOS, the app must also add the multicast networking entitlement path because Apple requires that entitlement for IP multicast or broadcast on iOS.

## Goals

- Add `derphole send --qr <file>` for terminal-to-iPhone file transfer.
- Encode a versioned app payload in the QR code: `derphole://receive?v=1&token=<token>`.
- Keep normal `derphole send` output unchanged when `--qr` is not set.
- Reuse existing Go receive and WebRTC direct transport code inside the iOS app through native gomobile bindings.
- Prefer WebRTC direct and fall back to relay in the same way the CLI does.
- Surface transfer status, progress, and final path in the app so direct success is visible.
- Let the user export the received file through the iOS document picker after transfer.
- Add simulator-safe payload injection for tests without changing the QR-first user flow.
- Drive tool installation, mobile framework generation, Apple build, and Apple tests through `mise`.

## Non-Goals

- Reimplementing Derphole transport in Swift.
- Using WASM in the iOS app.
- Making relay-only success acceptable for the proof of concept.
- Supporting text, stdin, or directory QR sends in the first version.
- Adding background transfer support.
- Building an App Store-ready product shell, account system, sharing history, or persistent inbox.

## QR Payload

The QR payload is a versioned URL:

```text
derphole://receive?v=1&token=<url-escaped-token>
```

The payload format is intentionally small and self-describing. The scheme tells the app this is a Derphole receive payload. The `v=1` parameter gives the scanner a compatibility gate. The `token` parameter carries the existing Derphole token without changing token encoding or transport semantics.

A shared Go helper should encode and parse this payload. It should accept the versioned URL and, for support/test convenience, raw Derphole tokens. Invalid schemes, unsupported versions, missing tokens, and malformed URLs should fail before transfer starts.

## CLI Design

`sendFlags` gains:

```go
QR bool `flag:"qr" help:"Render a QR code for the receive token"`
```

When `--qr` is false, the CLI prints the current `npx -y derphole@latest receive <token>` instruction exactly as it does today.

When `--qr` is true, the CLI prints a short instruction for the iOS app and renders a compact terminal QR code containing the versioned payload. The QR renderer should write to stderr, matching the current instruction output. The transfer itself should still stream normally after the receiver claims the offer.

The first implementation should only allow `--qr` for file sends. If `what` is empty, text, stdin, or a directory, the command should return a clear error. This keeps iOS saving and verification focused on the POC's primary goal.

## Mobile Go Bridge

Add `pkg/derpholemobile` as the only gomobile-exported package. It is an adapter, not a transport package. It should expose simple Swift-friendly APIs using strings, integers, and small exported interfaces.

The bridge should provide:

- payload parsing and validation
- receive start
- cancellation
- callback hooks for status, progress, trace, completion, and failure

Internally, receive must call existing Go Derphole code. The bridge should call `pkg/derphole.Receive` with `ForceRelay` false and a temp output directory. It must not set a mobile-specific relay-only mode. That preserves the same direct transport decision as the CLI because `pkg/derphole.Receive` already wires `webrtcdirect.New()` for web-file tokens when relay is not forced.

The bridge should not add a mobile-specific throughput cap, chunk size cap, alternate transport, or relay-only mode. If gomobile or iOS forces a platform-specific workaround, the workaround must be documented and verified against the direct-performance requirement.

## iOS App Design

The app's first screen should be the transfer tool, not a landing page. The primary action is scanning a QR code. A secondary simulator/test support path can accept a payload string, but it should not compete with the QR flow in normal use.

The app states are:

- idle
- camera permission needed
- scanning
- validating payload
- receiving
- completed
- exporting
- failed
- cancelled

The receive state should show file name when available, bytes received, total bytes when known, current transport state, and final path. Transport labels should distinguish at least waiting, relay, probing direct, direct, relay fallback, and complete.

On completion, the app presents a document export flow for the received temp file. If the user cancels export, the file should remain available for another export attempt during the current app session. Temporary files should be cleaned when cancelled before completion or when superseded by a new transfer.

## iOS Permissions And Entitlements

The app must include:

- `NSCameraUsageDescription`
- `NSLocalNetworkUsageDescription`

The local-network prompt should be triggered in context, during receive startup rather than on cold launch. The app should explain the blocked state if the user denies access, because direct transfer cannot meet the POC goal without local network access.

The initial design does not assume the multicast entitlement is required. Pion ICE may be able to use unicast STUN and host candidates without multicast or broadcast. Physical-device testing decides this. If direct fails because iOS blocks multicast or broadcast behavior, add an entitlements file and the multicast networking entitlement path, then retest on device.

## Data Flow

1. Sender runs `derphole send --qr ./file`.
2. CLI creates the same offer token used by normal `derphole send`.
3. CLI renders a QR containing `derphole://receive?v=1&token=<token>`.
4. iPhone scans the QR with AVFoundation.
5. Swift validates the payload and starts the Go mobile receive bridge.
6. Go receive claims the offer over DERP.
7. Transfer starts over relay while WebRTC direct negotiation runs in parallel.
8. When direct is ready, the existing webrelay path switches to direct.
9. Swift receives status/progress callbacks and shows whether the final path was direct.
10. On completion, Swift exports the temp file through the document picker.

## Error Handling

Invalid QR payloads should fail before any network operation. Expired, unsupported, or wrong-purpose tokens should surface the Go error clearly.

Camera denial should keep the app usable through the support payload path and should tell the user how to retry QR scanning. Local-network denial should be treated as a direct-transfer blocker. The app may still receive over relay if the transport permits it, but verification must report that direct did not work.

Direct failure before switch should not abort the user transfer if relay fallback succeeds. Direct failure after switch should follow the existing Go transport behavior. The app should still record the final path and direct failure reason if available.

User cancellation should cancel the Go context, stop scanning or receive work, close partial files, and clean temporary output. Save/export cancellation should not delete a completed file until the user starts another receive or leaves the relevant session state.

## Mise Tasks

Add tasks for the mobile toolchain and framework:

- `apple:mobile-tools`: install `gomobile` and `gobind` into a repo-local tool directory under `dist`, then run `gomobile init`.
- `apple:mobile-framework`: build the gomobile XCFramework from `pkg/derpholemobile` into `dist/apple/DerpholeMobile.xcframework`.
- `apple:build`: ensure the mobile framework exists before running `xcodebuild`.
- `apple:test`: ensure the mobile framework exists before running simulator tests.

The tasks should keep generated outputs under `dist/` and should not require users to hand-install gomobile globally. They may use `go install` through the `mise` Go toolchain.

## Testing

Automated Go tests should cover:

- QR payload encode and parse.
- Raw token fallback parsing.
- Rejection of invalid scheme, unsupported version, and missing token.
- `send --qr` selecting QR output and not printing the `npx` instruction.
- File-only validation for `send --qr`.
- Mobile bridge state and callback behavior with test seams where practical.

Swift tests should cover:

- app state transitions from payload injection to receive start
- invalid payload presentation
- cancellation state
- completed transfer export availability

Simulator UI tests should use the payload injection path. They should not depend on camera access or real QR scanning.

Required local verification:

```bash
mise run test
mise run apple:mobile-framework
mise run apple:build
mise run apple:test
```

## Physical-Device Direct Verification

The POC is not complete until an actual iPhone proves direct transfer works.

The required manual verification is:

1. Build and install the app on an iPhone.
2. Run `derphole send --qr <large-file>` on the Mac.
3. Scan the terminal QR code with the iPhone.
4. Confirm the app reports WebRTC direct connection and final path direct.
5. Save the received file through the document picker.
6. Compare the sent and received bytes.
7. Record elapsed time and throughput for a file large enough to make relay-vs-direct meaningful.

Also run one direct-blocked test to prove relay fallback still works. That test is secondary. It cannot replace the direct-path verification.

## Rollout

Implementation should land in small steps:

1. QR payload helpers and tests.
2. CLI `send --qr` flag and QR rendering tests.
3. `pkg/derpholemobile` bridge and gomobile task.
4. Xcode project integration with the generated XCFramework.
5. SwiftUI scanner, receive state, callbacks, and export flow.
6. Simulator tests through payload injection.
7. Physical-device direct verification and any required iOS network permission or entitlement fixes.

## Success Criteria

- `derphole send --qr <file>` displays a scannable terminal QR containing a versioned Derphole receive payload.
- Normal `derphole send` output is unchanged without `--qr`.
- The iOS app scans the QR, receives the file through native Go code, and exports it through iOS document UI.
- The iOS app uses the same direct-preferred Go WebRTC path as the CLI.
- A physical iPhone reaches direct WebRTC transfer on a network where direct is available.
- The app visibly reports whether transfer used direct or relay fallback.
- The saved file matches the source bytes.
- `mise` owns mobile tool setup, framework generation, build, and test commands.
- Simulator tests do not depend on camera hardware.
- Relay fallback remains available, but relay-only success is not enough to complete this POC.
