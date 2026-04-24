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
}
