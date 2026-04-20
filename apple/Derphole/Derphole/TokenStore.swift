import Combine
import Foundation

final class TokenStore: ObservableObject {
    @Published var webToken: String? {
        didSet {
            store(webToken, forKey: Self.webTokenKey)
        }
    }

    @Published var tcpToken: String? {
        didSet {
            store(tcpToken, forKey: Self.tcpTokenKey)
        }
    }

    private static let webTokenKey = "webToken"
    private static let tcpTokenKey = "tcpToken"

    private let defaults: UserDefaults

    init(defaults: UserDefaults = .standard) {
        self.defaults = defaults
        self.webToken = defaults.string(forKey: Self.webTokenKey)
        self.tcpToken = defaults.string(forKey: Self.tcpTokenKey)
    }

    private func store(_ value: String?, forKey key: String) {
        if let value {
            defaults.set(value, forKey: key)
        } else {
            defaults.removeObject(forKey: key)
        }
    }
}
