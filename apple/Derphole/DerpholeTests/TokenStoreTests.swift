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

    func testClearsTokensWhenSetToNil() {
        let suiteName = "TokenStoreTests-\(UUID().uuidString)"
        let defaults = UserDefaults(suiteName: suiteName)!
        defer { defaults.removePersistentDomain(forName: suiteName) }
        let store = TokenStore(defaults: defaults)

        store.webToken = "dtc1_web_token"
        store.tcpToken = "dtc1_tcp_token"
        store.webToken = nil
        store.tcpToken = nil

        XCTAssertNil(TokenStore(defaults: defaults).webToken)
        XCTAssertNil(TokenStore(defaults: defaults).tcpToken)
    }
}
