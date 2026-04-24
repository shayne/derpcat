import XCTest
@testable import DerpholeTunnel

final class DerpholeTunnelTests: XCTestCase {
    func testEndpointParsesBoundAddressAndBuildsWebsocketURL() throws {
        let endpoint = try DerptunEndpoint(boundAddress: "127.0.0.1:54321")

        XCTAssertEqual(endpoint.host, "127.0.0.1")
        XCTAssertEqual(endpoint.port, 54321)
        XCTAssertEqual(endpoint.websocketURL.absoluteString, "ws://127.0.0.1:54321/")
    }

    func testEndpointRejectsMalformedBoundAddress() {
        XCTAssertThrowsError(try DerptunEndpoint(boundAddress: "not-a-port")) { error in
            XCTAssertEqual(error as? DerpholeTunnelError, .invalidBoundAddress("not-a-port"))
        }
    }

    func testEndpointRejectsURLLikeHost() {
        XCTAssertThrowsError(try DerptunEndpoint(boundAddress: "example.com/path:80"))
    }

    func testEndpointRejectsEmptyBracketedHost() {
        XCTAssertThrowsError(try DerptunEndpoint(boundAddress: "[]:1234"))
    }

    func testEndpointParsesBracketedIPv6Address() throws {
        let endpoint = try DerptunEndpoint(boundAddress: "[::1]:54321")

        XCTAssertEqual(endpoint.host, "::1")
        XCTAssertEqual(endpoint.port, 54321)
        XCTAssertEqual(endpoint.websocketURL.absoluteString, "ws://[::1]:54321/")
    }

    func testCallbackAdapterEmitsRouteStatusAndTraceEvents() {
        let recorder = EventRecorder()
        let adapter = CallbackAdapter { event in
            recorder.append(event)
        }

        adapter.status("connected-relay")
        adapter.status("connected-direct")
        adapter.status("negotiating")
        adapter.status("  ")
        adapter.trace(" relay trace ")
        adapter.trace("")

        XCTAssertEqual(recorder.events, [
            .route(.relay),
            .route(.direct),
            .status("negotiating"),
            .trace("relay trace")
        ])
    }

    func testOpenStateTracksExplicitCancelByGeneration() {
        var state = TunnelOpenState()
        let first = state.begin(adapter: CallbackAdapter { _ in })

        XCTAssertFalse(state.isCanceled(first))

        state.cancelActive()

        XCTAssertTrue(state.isCanceled(first))

        let second = state.begin(adapter: CallbackAdapter { _ in })

        XCTAssertFalse(state.isCanceled(second))
        XCTAssertFalse(state.isCanceled(first))
    }
}

private final class EventRecorder: @unchecked Sendable {
    private let lock = NSLock()
    private var values: [DerpholeTunnelEvent] = []

    var events: [DerpholeTunnelEvent] {
        lock.lock()
        defer { lock.unlock() }
        return values
    }

    func append(_ event: DerpholeTunnelEvent) {
        lock.lock()
        values.append(event)
        lock.unlock()
    }
}
