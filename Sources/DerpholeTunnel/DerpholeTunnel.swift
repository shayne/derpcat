import DerpholeMobile
import Foundation

public struct DerptunInvite: Equatable, Sendable {
    public let rawValue: String

    public init(_ rawValue: String) throws {
        try self.init(rawValue: rawValue)
    }

    public init(rawValue: String) throws {
        let trimmed = rawValue.trimmingCharacters(in: .whitespacesAndNewlines)
        var error: NSError?
        guard let parsed = DerpholemobileParsePayload(trimmed, &error) else {
            throw DerpholeTunnelError.invalidInvite(error?.localizedDescription ?? "Payload could not be parsed.")
        }
        if let error {
            throw DerpholeTunnelError.invalidInvite(error.localizedDescription)
        }
        guard parsed.kind() == "tcp" else {
            throw DerpholeTunnelError.unsupportedInviteKind(parsed.kind())
        }
        self.rawValue = trimmed
    }
}

public struct DerptunEndpoint: Equatable, Sendable {
    public let boundAddress: String
    public let host: String
    public let port: Int
    public let websocketURL: URL

    public init(boundAddress: String) throws {
        let trimmed = boundAddress.trimmingCharacters(in: .whitespacesAndNewlines)
        guard let colon = trimmed.lastIndex(of: ":") else {
            throw DerpholeTunnelError.invalidBoundAddress(trimmed)
        }

        var host = String(trimmed[..<colon])
        if host.hasPrefix("[") && host.hasSuffix("]") {
            host.removeFirst()
            host.removeLast()
        }

        let portText = String(trimmed[trimmed.index(after: colon)...])
        guard !host.isEmpty else {
            throw DerpholeTunnelError.invalidBoundAddress(trimmed)
        }
        guard let port = Int(portText), (1...65_535).contains(port) else {
            throw DerpholeTunnelError.invalidBoundAddress(portText)
        }

        let urlHost = host.contains(":") ? "[\(host)]" : host
        guard let websocketURL = URL(string: "ws://\(urlHost):\(port)/") else {
            throw DerpholeTunnelError.invalidBoundAddress(trimmed)
        }

        self.boundAddress = trimmed
        self.host = host
        self.port = port
        self.websocketURL = websocketURL
    }
}

public enum DerpholeRoute: Equatable, Sendable {
    case relay
    case direct
}

public enum DerpholeTunnelEvent: Equatable, Sendable {
    case route(DerpholeRoute)
    case trace(String)
}

public enum DerpholeTunnelError: Error, Equatable, LocalizedError, Sendable {
    case invalidInvite(String)
    case unsupportedInviteKind(String)
    case mobileClientUnavailable
    case missingBoundAddress
    case invalidBoundAddress(String)
    case openFailed(String)

    public var errorDescription: String? {
        switch self {
        case .invalidInvite(let message):
            return "Derptun invite is invalid: \(message)"
        case .unsupportedInviteKind(let kind):
            return "Derptun invite kind is unsupported: \(kind)"
        case .mobileClientUnavailable:
            return "DerpholeMobile could not create a tunnel client."
        case .missingBoundAddress:
            return "DerpholeMobile did not report a bound tunnel address."
        case .invalidBoundAddress(let address):
            return "DerpholeMobile reported an invalid bound address: \(address)"
        case .openFailed(let message):
            return "Derptun tunnel failed to open: \(message)"
        }
    }
}

public final class DerptunTunnelClient: @unchecked Sendable {
    private let client: DerpholemobileTunnelClient
    private let lock = NSLock()
    private var activeAdapter: CallbackAdapter?

    public init() throws {
        guard let client = DerpholemobileNewTunnelClient() else {
            throw DerpholeTunnelError.mobileClientUnavailable
        }
        self.client = client
    }

    public func open(
        invite: DerptunInvite,
        onEvent: @escaping @Sendable (DerpholeTunnelEvent) -> Void = { _ in }
    ) async throws -> DerptunEndpoint {
        let adapter = CallbackAdapter(onEvent: onEvent)
        setActiveAdapter(adapter)

        do {
            let endpoint = try await withTaskCancellationHandler(operation: {
                try Task.checkCancellation()
                return try await Task.detached(priority: .userInitiated) { [client] in
                    do {
                        try client.openInvite(
                            invite.rawValue,
                            listenAddr: "127.0.0.1:0",
                            callbacks: adapter
                        )
                    } catch is CancellationError {
                        throw CancellationError()
                    } catch {
                        if Task.isCancelled {
                            throw CancellationError()
                        }
                        throw DerpholeTunnelError.openFailed(error.localizedDescription)
                    }

                    try Task.checkCancellation()
                    guard let boundAddress = adapter.boundAddress else {
                        throw DerpholeTunnelError.missingBoundAddress
                    }
                    return try DerptunEndpoint(boundAddress: boundAddress)
                }.value
            }, onCancel: {
                self.client.cancel()
            })
            return endpoint
        } catch {
            clearActiveAdapter(adapter)
            client.cancel()
            throw error
        }
    }

    public func open(
        _ invite: DerptunInvite,
        onEvent: @escaping @Sendable (DerpholeTunnelEvent) -> Void = { _ in }
    ) async throws -> DerptunEndpoint {
        try await open(invite: invite, onEvent: onEvent)
    }

    public func cancel() {
        lock.lock()
        activeAdapter = nil
        lock.unlock()
        client.cancel()
    }

    deinit {
        cancel()
    }

    private func setActiveAdapter(_ adapter: CallbackAdapter) {
        lock.lock()
        activeAdapter = adapter
        lock.unlock()
    }

    private func clearActiveAdapter(_ adapter: CallbackAdapter) {
        lock.lock()
        if activeAdapter === adapter {
            activeAdapter = nil
        }
        lock.unlock()
    }
}

nonisolated final class CallbackAdapter: NSObject, DerpholemobileTunnelCallbacksProtocol, @unchecked Sendable {
    private let lock = NSLock()
    private var boundAddressValue: String?
    private let onEvent: @Sendable (DerpholeTunnelEvent) -> Void

    init(onEvent: @escaping @Sendable (DerpholeTunnelEvent) -> Void) {
        self.onEvent = onEvent
    }

    var boundAddress: String? {
        lock.lock()
        defer { lock.unlock() }
        return boundAddressValue
    }

    func boundAddr(_ addr: String?) {
        let trimmed = (addr ?? "").trimmingCharacters(in: .whitespacesAndNewlines)
        guard !trimmed.isEmpty else { return }
        lock.lock()
        boundAddressValue = trimmed
        lock.unlock()
    }

    func status(_ status: String?) {
        let trimmed = (status ?? "").trimmingCharacters(in: .whitespacesAndNewlines)
        switch trimmed {
        case "connected-relay":
            onEvent(.route(.relay))
        case "connected-direct":
            onEvent(.route(.direct))
        default:
            break
        }
    }

    func trace(_ trace: String?) {
        let trimmed = (trace ?? "").trimmingCharacters(in: .whitespacesAndNewlines)
        guard !trimmed.isEmpty else { return }
        onEvent(.trace(trimmed))
    }
}
