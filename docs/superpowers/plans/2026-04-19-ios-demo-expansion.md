# iOS Demo Expansion Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build milestone 1 of the iOS demo expansion: native Files/Web/SSH tabs, polished file receive, functional Web tunnel browser, SSH scaffold, intent-aware QR payloads, and repeatable live tests.

**Architecture:** Keep transport and tunnel behavior in Go through `pkg/derpholemobile`; keep Swift focused on native UI state, scanning, persistence, browser presentation, and later terminal integration. Use one shared QR payload parser/classifier, one reusable mobile tunnel object, and small tab-specific Swift state models.

**Tech Stack:** Go, gomobile, SwiftUI, AVFoundation, WKWebView, XCTest/XCUITest, `mise`, `derphole`, `derptun`, existing session direct/relay transport.

---

## Scope Check

The approved spec covers Files, Web, and SSH. Keep this as one milestone plan because the first implementation needs shared QR payloads, shared mobile bridge APIs, shared scanner presentation, and shared token persistence. SSH terminal function is explicitly milestone 2; milestone 1 only builds its UI scaffold and bridge boundary.

## Subagent Execution Model

Use subagents during implementation, with one primary owner per lane and main-session integration reviews after each task.

- Go/mobile bridge lane: Tasks 1, 2, 3, and the Go half of Task 8.
- iOS UI lane: Tasks 4, 5, 6, 7, and Swift tests.
- Live verification lane: Task 8 and Task 9, using real CLI commands, simulator, and physical device when available.

Do not let two workers edit the same file at the same time. If a task needs a shared file, complete and review that task before starting another task that touches it.

## File Structure

Create or modify these files:

- Modify `pkg/derphole/qrpayload/payload.go`: replace receive-only payload helpers with typed file/web/tcp payload helpers.
- Modify `pkg/derphole/qrpayload/payload_test.go`: add typed payload tests.
- Modify `pkg/derphole/ui.go`: emit `derphole://file` payloads for file QR sends.
- Modify `pkg/derpholemobile/mobile.go`: expose typed payload parsing and a long-lived tunnel client using `session.DerptunOpen`.
- Modify `pkg/derpholemobile/mobile_test.go`: cover parse/classify behavior and tunnel client seams.
- Modify `cmd/derptun/serve.go`: add `--qr` and `--web` flags and QR output path.
- Modify `cmd/derptun/command_test.go`: cover new serve flags and QR behavior.
- Create `apple/Derphole/Derphole/AppMode.swift`: tab enum and debug/test mode helpers.
- Create `apple/Derphole/Derphole/PayloadModels.swift`: Swift wrappers for Go-classified payloads.
- Create `apple/Derphole/Derphole/TokenStore.swift`: Web/SSH token persistence and fingerprint helpers.
- Create `apple/Derphole/Derphole/ScannerSheet.swift`: modal scanner wrapper around existing `QRScannerView`.
- Modify `apple/Derphole/Derphole/QRScannerView.swift`: remove embedded-resting-screen assumptions and keep camera controller reusable.
- Replace `apple/Derphole/Derphole/ContentView.swift`: top-level `TabView` and tab navigation stacks.
- Create `apple/Derphole/Derphole/Files/FilesReceiveState.swift`: file receive state model split from current `TransferState`.
- Create `apple/Derphole/Derphole/Files/FilesTabView.swift`: zero state, receive state, completion actions.
- Create `apple/Derphole/Derphole/Shared/TransferFormatting.swift`: byte, MiB, MiB/s, and route display helpers.
- Create `apple/Derphole/Derphole/Web/WebTunnelState.swift`: Web tunnel lifecycle and persistence.
- Create `apple/Derphole/Derphole/Web/WebTabView.swift`: scan/reconnect/connect UI.
- Create `apple/Derphole/Derphole/Web/WebBrowserView.swift`: pushed `WKWebView` browser with controls.
- Create `apple/Derphole/Derphole/SSH/SSHTunnelState.swift`: SSH token state and credential prompt state.
- Create `apple/Derphole/Derphole/SSH/SSHTabView.swift`: SSH scaffold UI and credential prompt.
- Keep `apple/Derphole/Derphole/TransferUIUpdatePump.swift`: reuse for file/tunnel status coalescing.
- Keep `apple/Derphole/Derphole/DocumentExporter.swift`: relabel UI calls as Save File.
- Add or update Swift tests under `apple/Derphole/DerpholeTests`.
- Modify `apple/Derphole/DerpholeUITests/DerpholeUITests.swift`: new tab UI and injected payload flows.
- Modify `.mise.toml`: add `apple:web-tunnel`; keep existing Apple tasks.
- Create `scripts/apple-web-tunnel.sh`: live Web tunnel harness with runtime-generated token.
- Create `scripts/apple_web_tunnel_script_test.go`: static harness safety tests.

## Task 1: Typed QR Payloads

**Files:**
- Modify: `pkg/derphole/qrpayload/payload.go`
- Modify: `pkg/derphole/qrpayload/payload_test.go`
- Modify: `pkg/derphole/ui.go`
- Modify: `apple/Derphole/DerpholeTests/DerpholeTests.swift`

- [ ] **Step 1: Write failing tests for typed payloads**

Replace or extend `pkg/derphole/qrpayload/payload_test.go` with tests that name all three modes:

```go
func TestEncodeAndParseFilePayload(t *testing.T) {
	got, err := Encode(Payload{Kind: KindFile, Token: "file-token"})
	if err != nil {
		t.Fatalf("Encode() error = %v", err)
	}
	if got != "derphole://file?v=1&token=file-token" {
		t.Fatalf("Encode() = %q", got)
	}
	parsed, err := Parse(got)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	if parsed.Kind != KindFile || parsed.Token != "file-token" {
		t.Fatalf("Parse() = %#v", parsed)
	}
}

func TestEncodeAndParseWebPayload(t *testing.T) {
	got, err := Encode(Payload{Kind: KindWeb, Token: "dtc1_test", Scheme: "http", Path: "/admin"})
	if err != nil {
		t.Fatalf("Encode() error = %v", err)
	}
	if got != "derphole://web?path=%2Fadmin&scheme=http&token=dtc1_test&v=1" {
		t.Fatalf("Encode() = %q", got)
	}
	parsed, err := Parse(got)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	if parsed.Kind != KindWeb || parsed.Token != "dtc1_test" || parsed.Scheme != "http" || parsed.Path != "/admin" {
		t.Fatalf("Parse() = %#v", parsed)
	}
}

func TestEncodeAndParseTCPPayload(t *testing.T) {
	got, err := Encode(Payload{Kind: KindTCP, Token: "dtc1_test"})
	if err != nil {
		t.Fatalf("Encode() error = %v", err)
	}
	if got != "derphole://tcp?v=1&token=dtc1_test" {
		t.Fatalf("Encode() = %q", got)
	}
	parsed, err := Parse(got)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	if parsed.Kind != KindTCP || parsed.Token != "dtc1_test" {
		t.Fatalf("Parse() = %#v", parsed)
	}
}

func TestParseLegacyReceivePayloadAsFile(t *testing.T) {
	parsed, err := Parse("derphole://receive?v=1&token=legacy-token")
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	if parsed.Kind != KindFile || parsed.Token != "legacy-token" {
		t.Fatalf("Parse() = %#v", parsed)
	}
}

func TestParseRawTokenDefaultsToFile(t *testing.T) {
	parsed, err := Parse(" raw-token ")
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	if parsed.Kind != KindFile || parsed.Token != "raw-token" {
		t.Fatalf("Parse() = %#v", parsed)
	}
}
```

- [ ] **Step 2: Run QR payload tests and verify red**

Run: `go test ./pkg/derphole/qrpayload -count=1`

Expected: fails with undefined `Encode`, `Payload`, `KindFile`, `KindWeb`, `KindTCP`, or old receive-host expectations.

- [ ] **Step 3: Implement typed payload helpers**

Update `pkg/derphole/qrpayload/payload.go` with this public shape:

```go
type Kind string

const (
	Scheme         = "derphole"
	Version        = "1"
	KindFile Kind = "file"
	KindWeb  Kind = "web"
	KindTCP  Kind = "tcp"
)

type Payload struct {
	Kind   Kind
	Token  string
	Scheme string
	Path   string
}

func Encode(payload Payload) (string, error)
func EncodeFileToken(token string) (string, error)
func EncodeWebToken(token, scheme, path string) (string, error)
func EncodeTCPToken(token string) (string, error)
func Parse(raw string) (Payload, error)
func ParseReceivePayload(raw string) (string, error)
```

Implementation details:

- trim token and reject empty with `ErrMissingToken`
- default web scheme to `http`
- default web path to `/`
- sort query values by using `url.Values.Encode()`
- accept legacy `derphole://receive` as `KindFile`
- keep `ParseReceivePayload` as compatibility wrapper that requires file kind and returns token

- [ ] **Step 4: Migrate file QR output to `derphole://file`**

Update `pkg/derphole/ui.go` so the QR path calls `qrpayload.EncodeFileToken(token)` instead of `EncodeReceiveToken`.

Update Swift test data in `apple/Derphole/DerpholeTests/DerpholeTests.swift` from:

```swift
state.pastedPayload = "derphole://receive?v=1&token=token-123"
```

to:

```swift
state.pastedPayload = "derphole://file?v=1&token=token-123"
```

- [ ] **Step 5: Verify and commit Task 1**

Run:

```bash
go test ./pkg/derphole/qrpayload ./pkg/derpholemobile -count=1
go test ./pkg/derphole -run 'Test.*QR|Test.*Payload' -count=1
```

Expected: all selected tests pass.

Commit:

```bash
git add pkg/derphole/qrpayload pkg/derphole/ui.go pkg/derpholemobile apple/Derphole/DerpholeTests/DerpholeTests.swift
git commit -m "feat: add typed derphole QR payloads"
```

## Task 2: derptun QR CLI

**Files:**
- Modify: `cmd/derptun/serve.go`
- Modify: `cmd/derptun/command_test.go`
- Modify: `cmd/derptun/root.go`
- Test: `cmd/derptun/command_test.go`

- [ ] **Step 1: Write failing tests for `serve --qr` and `serve --web --qr`**

Add tests to `cmd/derptun/command_test.go`:

```go
func TestRunServeQRPrintsTCPPayload(t *testing.T) {
	oldServe := derptunServe
	defer func() { derptunServe = oldServe }()
	derptunServe = func(ctx context.Context, cfg session.DerptunServeConfig) error {
		if cfg.ServerToken != "dts1_test" || cfg.TargetAddr != "127.0.0.1:22" {
			t.Fatalf("cfg = %#v", cfg)
		}
		return nil
	}

	var stderr bytes.Buffer
	code := run([]string{"serve", "--token", "dts1_test", "--tcp", "127.0.0.1:22", "--qr"}, strings.NewReader(""), &bytes.Buffer{}, &stderr)
	if code != 0 {
		t.Fatalf("code = %d stderr=%s", code, stderr.String())
	}
	if !strings.Contains(stderr.String(), "derphole://tcp?v=1&token=") {
		t.Fatalf("stderr = %q, want tcp QR payload", stderr.String())
	}
}

func TestRunServeWebQRPrintsWebPayload(t *testing.T) {
	oldServe := derptunServe
	defer func() { derptunServe = oldServe }()
	derptunServe = func(ctx context.Context, cfg session.DerptunServeConfig) error { return nil }

	var stderr bytes.Buffer
	code := run([]string{"serve", "--token", "dts1_test", "--tcp", "127.0.0.1:3000", "--web", "--qr"}, strings.NewReader(""), &bytes.Buffer{}, &stderr)
	if code != 0 {
		t.Fatalf("code = %d stderr=%s", code, stderr.String())
	}
	if !strings.Contains(stderr.String(), "derphole://web?") || !strings.Contains(stderr.String(), "scheme=http") {
		t.Fatalf("stderr = %q, want web QR payload", stderr.String())
	}
}

func TestRunServeRejectsWebWithoutQR(t *testing.T) {
	var stderr bytes.Buffer
	code := run([]string{"serve", "--token", "dts1_test", "--tcp", "127.0.0.1:3000", "--web"}, strings.NewReader(""), &bytes.Buffer{}, &stderr)
	if code != 2 {
		t.Fatalf("code = %d, want 2", code)
	}
	if !strings.Contains(stderr.String(), "--web requires --qr") {
		t.Fatalf("stderr = %q", stderr.String())
	}
}
```

- [ ] **Step 2: Run derptun tests and verify red**

Run: `go test ./cmd/derptun -run 'TestRunServe.*QR|TestRunServeRejectsWebWithoutQR' -count=1`

Expected: fails because `--qr` and `--web` are unknown or not emitted.

- [ ] **Step 3: Add serve flags and QR output**

Update `serveFlags` in `cmd/derptun/serve.go`:

```go
QR  bool `flag:"qr" help:"Render a QR code for the Derphole iOS app"`
Web bool `flag:"web" help:"Mark the QR payload as an HTTP web tunnel"`
```

After resolving the server token, before calling `derptunServe`, generate the client token for QR output:

```go
if parsed.SubCommandFlags.Web && !parsed.SubCommandFlags.QR {
	fmt.Fprintln(stderr, "--web requires --qr")
	fmt.Fprint(stderr, serveHelpText())
	return 2
}
if parsed.SubCommandFlags.QR {
	clientToken, err := derptun.GenerateClientToken(derptun.ClientTokenOptions{ServerToken: token})
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	payload, err := qrpayload.EncodeTCPToken(clientToken)
	if parsed.SubCommandFlags.Web {
		payload, err = qrpayload.EncodeWebToken(clientToken, "http", "/")
	}
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	fmt.Fprintln(stderr, payload)
}
```

Import `github.com/shayne/derphole/pkg/derphole/qrpayload` and `github.com/shayne/derphole/pkg/derptun`.

- [ ] **Step 4: Verify and commit Task 2**

Run:

```bash
go test ./cmd/derptun ./pkg/derphole/qrpayload -count=1
```

Expected: tests pass.

Commit:

```bash
git add cmd/derptun pkg/derphole/qrpayload
git commit -m "feat: add derptun QR tunnel payloads"
```

## Task 3: Mobile Bridge Parser And Tunnel Client

**Files:**
- Modify: `pkg/derpholemobile/mobile.go`
- Modify: `pkg/derpholemobile/mobile_test.go`
- Modify: `pkg/session/derptun.go` only if an exported seam is needed

- [ ] **Step 1: Write failing bridge tests for payload classification**

Add tests to `pkg/derpholemobile/mobile_test.go`:

```go
func TestParsePayloadClassifiesModes(t *testing.T) {
	for _, tt := range []struct {
		name string
		raw  string
		kind string
	}{
		{name: "file", raw: "derphole://file?v=1&token=file-token", kind: "file"},
		{name: "web", raw: "derphole://web?path=%2F&scheme=http&token=dtc1_test&v=1", kind: "web"},
		{name: "tcp", raw: "derphole://tcp?v=1&token=dtc1_test", kind: "tcp"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			parsed, err := ParsePayload(tt.raw)
			if err != nil {
				t.Fatalf("ParsePayload() error = %v", err)
			}
			if parsed.Kind() != tt.kind {
				t.Fatalf("Kind() = %q, want %q", parsed.Kind(), tt.kind)
			}
		})
	}
}
```

- [ ] **Step 2: Run mobile tests and verify red**

Run: `go test ./pkg/derpholemobile -run TestParsePayloadClassifiesModes -count=1`

Expected: fails because `ParsePayload` returns `string` today instead of an object.

- [ ] **Step 3: Introduce mobile parsed payload object**

In `pkg/derpholemobile/mobile.go`, replace `ParsePayload(payload string) (string, error)` with:

```go
type ParsedPayload struct {
	kind   string
	token  string
	scheme string
	path   string
}

func (p *ParsedPayload) Kind() string { return p.kind }
func (p *ParsedPayload) Token() string { return p.token }
func (p *ParsedPayload) Scheme() string { return p.scheme }
func (p *ParsedPayload) Path() string { return p.path }

func ParsePayload(payload string) (*ParsedPayload, error) {
	parsed, err := qrpayload.Parse(payload)
	if err != nil {
		return nil, err
	}
	return &ParsedPayload{
		kind:   string(parsed.Kind),
		token:  parsed.Token,
		scheme: parsed.Scheme,
		path:   parsed.Path,
	}, nil
}

func ParseFileToken(payload string) (string, error) {
	parsed, err := qrpayload.Parse(payload)
	if err != nil {
		return "", err
	}
	if parsed.Kind != qrpayload.KindFile {
		return "", qrpayload.ErrUnsupportedPayload
	}
	return parsed.Token, nil
}
```

Update `Receiver.Receive` and Swift-facing code paths to use `ParseFileToken`.

- [ ] **Step 4: Write failing tunnel client seam test**

Add package-level seam:

```go
var derptunOpen = session.DerptunOpen
```

Add test:

```go
func TestTunnelClientOpenUsesDerptunOpen(t *testing.T) {
	oldOpen := derptunOpen
	defer func() { derptunOpen = oldOpen }()
	called := false
	derptunOpen = func(ctx context.Context, cfg session.DerptunOpenConfig) error {
		called = true
		if cfg.ClientToken != "dtc1_test" {
			t.Fatalf("ClientToken = %q", cfg.ClientToken)
		}
		if cfg.ListenAddr != "127.0.0.1:0" {
			t.Fatalf("ListenAddr = %q", cfg.ListenAddr)
		}
		cfg.BindAddrSink <- "127.0.0.1:54321"
		<-ctx.Done()
		return nil
	}

	client := NewTunnelClient()
	callbacks := &recordingTunnelCallbacks{}
	if err := client.Open("dtc1_test", "127.0.0.1:0", callbacks); err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	if !called {
		t.Fatal("derptunOpen was not called")
	}
	if callbacks.boundAddr != "127.0.0.1:54321" {
		t.Fatalf("boundAddr = %q", callbacks.boundAddr)
	}
	client.Cancel()
}
```

- [ ] **Step 5: Implement `TunnelClient`**

Add:

```go
type TunnelCallbacks interface {
	Status(status string)
	Trace(trace string)
	BoundAddr(addr string)
}

type TunnelClient struct {
	mu     sync.Mutex
	cancel context.CancelFunc
}

func NewTunnelClient() *TunnelClient { return &TunnelClient{} }

func (c *TunnelClient) Open(token, listenAddr string, callbacks TunnelCallbacks) error
func (c *TunnelClient) Cancel()
```

`Open` should start `derptunOpen` in a goroutine, wait for the first bind address or first error, and keep the context alive until `Cancel`. Use `telemetry.New(statusWriter, telemetry.LevelDefault)` like the receiver bridge.

- [ ] **Step 6: Verify and commit Task 3**

Run:

```bash
go test ./pkg/derpholemobile ./pkg/session -count=1
mise run apple:mobile-framework
```

Expected: Go tests pass and the mobile framework builds.

Commit:

```bash
git add pkg/derpholemobile pkg/session
git commit -m "feat: expose mobile tunnel bridge"
```

## Task 4: Swift App Shell And Shared Services

**Files:**
- Create: `apple/Derphole/Derphole/AppMode.swift`
- Create: `apple/Derphole/Derphole/PayloadModels.swift`
- Create: `apple/Derphole/Derphole/TokenStore.swift`
- Create: `apple/Derphole/Derphole/ScannerSheet.swift`
- Create: `apple/Derphole/Derphole/Shared/TransferFormatting.swift`
- Modify: `apple/Derphole/Derphole/ContentView.swift`
- Modify: `apple/Derphole/Derphole/QRScannerView.swift`
- Add tests under `apple/Derphole/DerpholeTests`

- [ ] **Step 1: Write failing Swift tests for token store and formatting**

Create `apple/Derphole/DerpholeTests/TokenStoreTests.swift`:

```swift
import XCTest
@testable import Derphole

final class TokenStoreTests: XCTestCase {
    func testPersistsWebAndTCPButNotCredentials() {
        let suiteName = "TokenStoreTests-\(UUID().uuidString)"
        let defaults = UserDefaults(suiteName: suiteName)!
        defer { defaults.removePersistentDomain(forName: suiteName) }
        let store = TokenStore(defaults: defaults)

        store.webToken = "dtc1_web_token"
        store.tcpToken = "dtc1_tcp_token"

        XCTAssertEqual(TokenStore(defaults: defaults).webToken, "dtc1_web_token")
        XCTAssertEqual(TokenStore(defaults: defaults).tcpToken, "dtc1_tcp_token")
        XCTAssertNil(defaults.string(forKey: "sshUsername"))
        XCTAssertNil(defaults.string(forKey: "sshPassword"))
    }
}
```

Create `apple/Derphole/DerpholeTests/TransferFormattingTests.swift`:

```swift
func testFormatsMiBAndSpeed() {
    XCTAssertEqual(TransferFormatting.mib(1_048_576), "1.0 MiB")
    XCTAssertEqual(TransferFormatting.speed(bytesPerSecond: 2_097_152), "2.0 MiB/s")
    let fingerprint = TransferFormatting.fingerprint("dtc1_abcdefghijklmnopqrstuvwxyz")
    XCTAssertLessThan(fingerprint.count, "dtc1_abcdefghijklmnopqrstuvwxyz".count)
    XCTAssertTrue(fingerprint.hasPrefix("dtc1_"))
}
```

- [ ] **Step 2: Run Swift tests and verify red**

Run: `mise run apple:test`

Expected: fails because `TokenStore` and `TransferFormatting` do not exist.

- [ ] **Step 3: Implement shared services**

Create `TokenStore` with:

```swift
final class TokenStore: ObservableObject {
    @Published var webToken: String? { didSet { defaults.set(webToken, forKey: "webToken") } }
    @Published var tcpToken: String? { didSet { defaults.set(tcpToken, forKey: "tcpToken") } }
    private let defaults: UserDefaults

    init(defaults: UserDefaults = .standard) {
        self.defaults = defaults
        self.webToken = defaults.string(forKey: "webToken")
        self.tcpToken = defaults.string(forKey: "tcpToken")
    }
}
```

Create `TransferFormatting` with static helpers for MiB, MiB/s, and token fingerprint. Use base 1024 and one decimal place.

Create `AppMode`:

```swift
enum AppTab: Hashable {
    case files
    case web
    case ssh
}

enum AppLaunchMode {
    static var showsDebugPayloadControls: Bool {
        ProcessInfo.processInfo.arguments.contains("--derphole-debug-payload-controls")
        || ProcessInfo.processInfo.environment["DERPHOLE_DEBUG_PAYLOAD_CONTROLS"] == "1"
    }
}
```

- [ ] **Step 4: Replace top-level content with tabs**

Update `ContentView` to own a shared `TokenStore` and render:

```swift
TabView {
    NavigationStack { FilesTabView() }
        .tabItem { Label("Files", systemImage: "doc") }
    NavigationStack { WebTabView(tokenStore: tokenStore) }
        .tabItem { Label("Web", systemImage: "safari") }
    NavigationStack { SSHTabView(tokenStore: tokenStore) }
        .tabItem { Label("SSH", systemImage: "terminal") }
}
```

- [ ] **Step 5: Verify and commit Task 4**

Run:

```bash
mise run apple:test
```

Expected: Swift tests pass; UI tests may fail until Task 5 updates identifiers. If UI tests fail only because old receive screen identifiers changed, record that and continue to Task 5 before committing.

Commit after tests are green in Task 5 if needed; otherwise:

```bash
git add apple/Derphole/Derphole apple/Derphole/DerpholeTests
git commit -m "apple: add tab shell and shared services"
```

## Task 5: Files Tab Polish

**Files:**
- Create: `apple/Derphole/Derphole/Files/FilesReceiveState.swift`
- Create: `apple/Derphole/Derphole/Files/FilesTabView.swift`
- Modify or remove old usage of `apple/Derphole/Derphole/TransferState.swift`
- Modify: `apple/Derphole/Derphole/DocumentExporter.swift`
- Modify: `apple/Derphole/DerpholeUITests/DerpholeUITests.swift`

- [ ] **Step 1: Write failing state tests for Files completion actions**

Create `apple/Derphole/DerpholeTests/FilesReceiveStateTests.swift`:

```swift
@MainActor
func testDiscardDeletesCompletedFileAndResets() throws {
    let state = FilesReceiveState()
    let dir = FileManager.default.temporaryDirectory.appendingPathComponent(UUID().uuidString, isDirectory: true)
    try FileManager.default.createDirectory(at: dir, withIntermediateDirectories: true)
    let file = dir.appendingPathComponent("received.bin")
    try Data([1, 2, 3]).write(to: file)

    state.markCompletedForTesting(fileURL: file)
    XCTAssertTrue(FileManager.default.fileExists(atPath: file.path))

    state.discardReceivedFile()

    XCTAssertEqual(state.phase, .idle)
    XCTAssertNil(state.completedFileURL)
    XCTAssertFalse(FileManager.default.fileExists(atPath: dir.path))
}

@MainActor
func testSpeedUpdatesFromProgressSnapshots() {
    let clock = ManualClock()
    let state = FilesReceiveState(now: { clock.now })

    state.recordProgress(current: 0, total: 4_194_304)
    clock.advance(by: 1)
    state.recordProgress(current: 2_097_152, total: 4_194_304)

    XCTAssertEqual(state.speedText, "2.0 MiB/s")
}

private final class ManualClock {
    private(set) var now: Date = Date(timeIntervalSince1970: 0)

    func advance(by seconds: TimeInterval) {
        now = now.addingTimeInterval(seconds)
    }
}
```

- [ ] **Step 2: Run Files tests and verify red**

Run: `mise run apple:test`

Expected: fails because `FilesReceiveState` does not exist.

- [ ] **Step 3: Implement `FilesReceiveState`**

Move file receive logic out of `TransferState` into `FilesReceiveState`. Preserve:

- receiver creation through `DerpholemobileNewReceiver`
- callback pump
- runtime injected receive for test mode
- route parsing for `connected-relay` and `connected-direct`

Add:

- `showsDebugPayloadControls`
- scanner modal state
- `speedText`
- `saveFile()` presentation flag
- `discardReceivedFile()`
- test helpers under `#if DEBUG`

- [ ] **Step 4: Implement `FilesTabView`**

Build the UI around:

- zero state with one primary `Scan QR Code` button
- `.fullScreenCover` or `.sheet` for `ScannerSheet`
- receive progress surface
- `Save File` and `Discard` buttons on completion
- debug payload controls only when `AppLaunchMode.showsDebugPayloadControls` is true

Use accessibility identifiers:

- `filesTab`
- `filesScanQRCodeButton`
- `filesScannerSheet`
- `filesTransferProgress`
- `filesRouteBadge`
- `filesSaveFileButton`
- `filesDiscardButton`
- `filesDebugPayloadField`

- [ ] **Step 5: Update UI tests**

In `DerpholeUITests.swift`, update launch test expectations:

```swift
XCTAssertTrue(app.tabBars.buttons["Files"].exists)
XCTAssertTrue(app.tabBars.buttons["Web"].exists)
XCTAssertTrue(app.tabBars.buttons["SSH"].exists)
XCTAssertTrue(app.buttons["filesScanQRCodeButton"].exists)
XCTAssertFalse(app.textFields["filesDebugPayloadField"].exists)
```

Add a test launched with `--derphole-debug-payload-controls` that asserts `filesDebugPayloadField` exists.

- [ ] **Step 6: Verify and commit Task 5**

Run:

```bash
mise run apple:test
APPLE_PHYSICAL_TRANSFER_MIB=2 mise run apple:physical-transfer
```

Expected: simulator tests pass; physical transfer completes with `route: connected-direct`.

Commit:

```bash
git add apple/Derphole/Derphole apple/Derphole/DerpholeTests apple/Derphole/DerpholeUITests
git commit -m "apple: polish file receive tab"
```

## Task 6: Web Tunnel Tab And Browser

**Files:**
- Create: `apple/Derphole/Derphole/Web/WebTunnelState.swift`
- Create: `apple/Derphole/Derphole/Web/WebTabView.swift`
- Create: `apple/Derphole/Derphole/Web/WebBrowserView.swift`
- Create: `apple/Derphole/Derphole/Web/WebViewRepresentable.swift`
- Add tests under `apple/Derphole/DerpholeTests`

- [ ] **Step 1: Write failing Web state tests**

Create `apple/Derphole/DerpholeTests/WebTunnelStateTests.swift`:

```swift
@MainActor
func testWebScanPersistsTokenAndBuildsBrowserURL() throws {
    let defaults = UserDefaults(suiteName: "WebTunnelStateTests-\(UUID().uuidString)")!
    let store = TokenStore(defaults: defaults)
    let state = WebTunnelState(tokenStore: store)

    try state.acceptPayloadForTesting(kind: "web", token: "dtc1_web", scheme: "http", path: "/admin")
    state.markBoundForTesting("127.0.0.1:54321")

    XCTAssertEqual(store.webToken, "dtc1_web")
    XCTAssertEqual(state.browserURL, URL(string: "http://127.0.0.1:54321/admin"))
}

@MainActor
func testDisconnectKeepsRememberedToken() {
    let defaults = UserDefaults(suiteName: "WebDisconnect-\(UUID().uuidString)")!
    let store = TokenStore(defaults: defaults)
    store.webToken = "dtc1_web"
    let state = WebTunnelState(tokenStore: store)

    state.disconnect()

    XCTAssertEqual(store.webToken, "dtc1_web")
    XCTAssertFalse(state.isConnected)
}
```

- [ ] **Step 2: Run Swift tests and verify red**

Run: `mise run apple:test`

Expected: fails because Web state/view files do not exist.

- [ ] **Step 3: Implement Web state**

`WebTunnelState` should:

- parse accepted payloads through the mobile bridge
- reject non-web payloads with user-readable error text
- persist `webToken`
- call `DerpholemobileNewTunnelClient`
- hold `boundAddr`, `route`, `statusText`, and `browserURL`
- cancel on disconnect

Use `TunnelCallbacks` equivalent in Swift to receive `BoundAddr`, `Status`, and `Trace`.

- [ ] **Step 4: Implement Web UI**

`WebTabView`:

- zero/connect state
- remembered token row
- `Scan QR Code`
- `Reconnect`
- error display
- navigation push to `WebBrowserView` when `browserURL` is non-nil

`WebBrowserView`:

- `WKWebView`
- back/forward/reload buttons
- compact address display
- route badge
- Disconnect button

Accessibility identifiers:

- `webTab`
- `webScanQRCodeButton`
- `webRememberedToken`
- `webReconnectButton`
- `webBrowserView`
- `webDisconnectButton`

- [ ] **Step 5: Verify and commit Task 6**

Run:

```bash
mise run apple:test
```

Expected: Swift and UI tests pass for Web zero/reconnect states. Live browser verification waits for Task 8.

Commit:

```bash
git add apple/Derphole/Derphole/Web apple/Derphole/DerpholeTests apple/Derphole/DerpholeUITests
git commit -m "apple: add web tunnel browser tab"
```

## Task 7: SSH Tab Scaffold

**Files:**
- Create: `apple/Derphole/Derphole/SSH/SSHTunnelState.swift`
- Create: `apple/Derphole/Derphole/SSH/SSHTabView.swift`
- Create: `apple/Derphole/Derphole/SSH/SSHCredentialPrompt.swift`
- Add tests under `apple/Derphole/DerpholeTests`

- [ ] **Step 1: Write failing SSH persistence and credential tests**

Create `apple/Derphole/DerpholeTests/SSHTunnelStateTests.swift`:

```swift
@MainActor
func testTCPScanPersistsTokenButNotCredentials() throws {
    let defaults = UserDefaults(suiteName: "SSHTunnelStateTests-\(UUID().uuidString)")!
    let store = TokenStore(defaults: defaults)
    let state = SSHTunnelState(tokenStore: store)

    try state.acceptPayloadForTesting(kind: "tcp", token: "dtc1_tcp")
    state.username = "shayne"
    state.password = "secret"
    state.disconnect()

    XCTAssertEqual(store.tcpToken, "dtc1_tcp")
    XCTAssertNil(defaults.string(forKey: "sshUsername"))
    XCTAssertNil(defaults.string(forKey: "sshPassword"))
}
```

- [ ] **Step 2: Run Swift tests and verify red**

Run: `mise run apple:test`

Expected: fails because SSH state/view files do not exist.

- [ ] **Step 3: Implement SSH scaffold**

`SSHTunnelState` should:

- persist only TCP token
- reject non-tcp payloads
- expose `isCredentialPromptPresented`
- hold transient `username` and `password`
- clear credentials on disconnect and after failed attempts
- define future integration point:

```swift
protocol SSHLocalTunnelConnecting {
    func connect(token: String, username: String, password: String) async throws
    func disconnect()
}
```

Do not copy vvterm code in milestone 1.

- [ ] **Step 4: Implement SSH UI**

`SSHTabView`:

- scan QR
- remembered TCP token row
- reconnect button
- credential prompt sheet
- terminal integration boundary message when credentials are submitted

Use identifiers:

- `sshTab`
- `sshScanQRCodeButton`
- `sshRememberedToken`
- `sshReconnectButton`
- `sshCredentialPrompt`

- [ ] **Step 5: Verify and commit Task 7**

Run:

```bash
mise run apple:test
```

Expected: tests pass and SSH UI scaffold is visible.

Commit:

```bash
git add apple/Derphole/Derphole/SSH apple/Derphole/DerpholeTests apple/Derphole/DerpholeUITests
git commit -m "apple: scaffold ssh tunnel tab"
```

## Task 8: Web Live Harness And Mise Task

**Files:**
- Modify: `.mise.toml`
- Create: `scripts/apple-web-tunnel.sh`
- Create: `scripts/apple_web_tunnel_script_test.go`
- Modify: `apple/Derphole/Derphole/LiveReceiveLaunchConfiguration.swift` or create `LiveLaunchPayloadConfiguration.swift`

- [ ] **Step 1: Write failing static script test**

Create `scripts/apple_web_tunnel_script_test.go`:

```go
func TestAppleWebTunnelTaskUsesRuntimePayload(t *testing.T) {
	mise := readRepoFile(t, ".mise.toml")
	script := readScriptFile(t, "apple-web-tunnel.sh")
	for _, want := range []string{
		`[tasks."apple:web-tunnel"]`,
		`bash ./scripts/apple-web-tunnel.sh`,
	} {
		if !strings.Contains(mise, want) {
			t.Fatalf(".mise.toml missing %q", want)
		}
	}
	for _, want := range []string{
		"derptun serve",
		"--web",
		"--qr",
		"derphole://web",
		"DERPHOLE_LIVE_WEB_PAYLOAD",
		"WKWebView",
	} {
		if !strings.Contains(script, want) {
			t.Fatalf("apple-web-tunnel.sh missing %q", want)
		}
	}
}
```

- [ ] **Step 2: Run script test and verify red**

Run: `go test ./scripts -run TestAppleWebTunnelTaskUsesRuntimePayload -count=1`

Expected: fails because the task and script do not exist.

- [ ] **Step 3: Add `apple:web-tunnel` task**

Add to `.mise.toml`:

```toml
[tasks."apple:web-tunnel"]
description = "Run a live derptun web tunnel test against the iOS app"
run = "bash ./scripts/apple-web-tunnel.sh"
```

- [ ] **Step 4: Implement `scripts/apple-web-tunnel.sh`**

Script flow:

1. Start local HTTP fixture on `127.0.0.1:0` with a unique marker.
2. Generate server token with `dist/derptun token server`.
3. Start `dist/derptun serve --token "$server_token" --tcp "$fixture_addr" --web --qr`.
4. Parse the live `derphole://web` payload from stderr.
5. Launch simulator or physical app with `DERPHOLE_LIVE_WEB_PAYLOAD="$payload"` and no hardcoded token.
6. Wait for UI test or app automation to load marker text through `WKWebView`.
7. Fail with preserved logs on timeout.

Use existing device-selection patterns from `scripts/apple-physical-transfer.sh`.

- [ ] **Step 5: Add app launch handling for web payload**

Create or generalize a launch config type so test launches can inject:

```swift
DERPHOLE_LIVE_WEB_PAYLOAD
DERPHOLE_LIVE_TCP_PAYLOAD
```

The injected web payload should select the Web tab, persist the token, start the tunnel, and push the browser.

- [ ] **Step 6: Verify and commit Task 8**

Run:

```bash
go test ./scripts -run 'TestApple.*Tunnel|TestApplePhysicalTransfer' -count=1
bash -n scripts/apple-web-tunnel.sh
mise run apple:web-tunnel
```

Expected: static tests pass, shell parses, and live web tunnel loads fixture content.

Commit:

```bash
git add .mise.toml scripts apple/Derphole/Derphole apple/Derphole/DerpholeUITests
git commit -m "test: add iOS web tunnel harness"
```

## Task 9: Final Verification And Integration Review

**Files:**
- No new files unless earlier verification exposes a focused fix.

- [ ] **Step 1: Run full Go tests**

Run:

```bash
mise run test
```

Expected: `go test ./...` passes.

- [ ] **Step 2: Run iOS build and simulator tests**

Run:

```bash
mise run apple:build
mise run apple:test
```

Expected: signed/simulator build succeeds and simulator tests pass. Known gomobile nullability warnings are acceptable if unchanged.

- [ ] **Step 3: Run live Files verification**

Run:

```bash
APPLE_PHYSICAL_TRANSFER_MIB=2 mise run apple:physical-transfer
```

Expected:

```text
transfer complete: 2097152 bytes
route: connected-direct
```

- [ ] **Step 4: Run live Web verification**

Run:

```bash
mise run apple:web-tunnel
```

Expected: fixture content loads through the app browser and the task exits 0.

- [ ] **Step 5: Scan for hardcoded live tokens**

Run:

```bash
rg -n 'dtc1_|dts1_|derphole://web|derphole://tcp|DERPHOLE_LIVE_.*TOKEN' apple scripts cmd pkg .mise.toml
```

Expected: only test fixture strings, parser constants, and runtime environment variable names appear. No generated live token from a run is committed.

- [ ] **Step 6: Review staged changes by task**

Run:

```bash
git status --short
git log --oneline -10
```

Expected: each task has its own commit and the worktree is clean.

- [ ] **Step 7: Push after human approval**

After review approval:

```bash
git push origin main
```

Expected: local `main` pushes to `origin/main`.

## Milestone 2 Entry Criteria

Start SSH terminal implementation only after milestone 1 passes:

- Files live physical direct transfer works.
- Web live tunnel test works.
- QR payloads are mode-aware.
- Mobile tunnel bridge is stable.
- SSH tab can persist TCP tokens and prompt for credentials without storing them.

Milestone 2 should get its own plan before copying vvterm/libghostty/libssh2 code into this app.
