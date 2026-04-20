import Combine
import DerpholeMobile
import Foundation

nonisolated protocol SSHLocalTunnelConnecting {
    func connect(token: String, username: String, password: String) async throws
    func disconnect()
}

enum SSHConnectionError: LocalizedError {
    case invalidPayload
    case nonTCPPayload
    case missingRememberedToken
    case missingCredentials
    case terminalIntegrationPending

    var errorDescription: String? {
        switch self {
        case .invalidPayload:
            return "Payload could not be parsed."
        case .nonTCPPayload:
            return "Payload is not a TCP tunnel."
        case .missingRememberedToken:
            return "Scan an SSH tunnel QR code first."
        case .missingCredentials:
            return "Enter a username and password."
        case .terminalIntegrationPending:
            return "Terminal integration pending."
        }
    }
}

@MainActor
final class SSHTunnelState: ObservableObject {
    @Published var isScannerPresented = false
    @Published var isCredentialPromptPresented = false
    @Published var username = ""
    @Published var password = ""
    @Published private(set) var statusText = "Ready."
    @Published private(set) var errorText: String?
    @Published private(set) var isConnecting = false
    @Published private(set) var isConnected = false

    private let tokenStore: TokenStore
    private let connectorFactory: () -> SSHLocalTunnelConnecting
    private var activeConnector: SSHLocalTunnelConnecting?
    private var connectionID = UUID()
    private var ignoresNextCredentialPromptDismissal = false

    init(
        tokenStore: TokenStore,
        connectorFactory: @escaping () -> SSHLocalTunnelConnecting = { PlaceholderSSHConnector() }
    ) {
        self.tokenStore = tokenStore
        self.connectorFactory = connectorFactory
    }

    var rememberedToken: String? {
        tokenStore.tcpToken
    }

    var rememberedTokenFingerprint: String? {
        guard let token = tokenStore.tcpToken else { return nil }
        return TransferFormatting.fingerprint(token)
    }

    func scanStarted() {
        guard !isConnecting else { return }
        errorText = nil
        statusText = "Scanning for SSH tunnel QR code."
        isScannerPresented = true
    }

    func scannerDismissed() {
        isScannerPresented = false
        if !isConnecting, !isConnected, !isCredentialPromptPresented {
            statusText = "Ready."
        }
    }

    func acceptScannedPayload(_ payload: String) {
        isScannerPresented = false
        do {
            let token = try parseTCPPayload(payload)
            tokenStore.tcpToken = token
            presentCredentialPrompt(status: "TCP tunnel QR code scanned. Enter SSH credentials.")
        } catch SSHConnectionError.nonTCPPayload {
            failBeforeConnect(status: "Scanned code was not an SSH tunnel.", error: "Scan a Derphole TCP tunnel QR code.")
        } catch {
            failBeforeConnect(status: "Scanned code was invalid.", error: error.localizedDescription)
        }
    }

    func reconnect() {
        guard let token = tokenStore.tcpToken?.trimmingCharacters(in: .whitespacesAndNewlines), !token.isEmpty else {
            failBeforeConnect(status: "No remembered SSH tunnel.", error: SSHConnectionError.missingRememberedToken.localizedDescription)
            return
        }
        presentCredentialPrompt(status: "Enter SSH credentials.")
    }

    func cancelCredentialPrompt() {
        isCredentialPromptPresented = false
        errorText = nil
        clearCredentials()
        if !isConnecting, !isConnected {
            statusText = "Ready."
        }
    }

    func credentialPromptDismissed() {
        if ignoresNextCredentialPromptDismissal {
            ignoresNextCredentialPromptDismissal = false
            return
        }
        cancelCredentialPrompt()
    }

    func submitCredentials() async {
        let trimmedUsername = username.trimmingCharacters(in: .whitespacesAndNewlines)
        let submittedPassword = password
        guard let token = tokenStore.tcpToken?.trimmingCharacters(in: .whitespacesAndNewlines), !token.isEmpty else {
            failBeforeConnect(status: "No remembered SSH tunnel.", error: SSHConnectionError.missingRememberedToken.localizedDescription)
            clearCredentials()
            return
        }
        guard !trimmedUsername.isEmpty, !submittedPassword.isEmpty else {
            errorText = SSHConnectionError.missingCredentials.localizedDescription
            statusText = "Credentials required."
            return
        }

        let connector = connectorFactory()
        let currentConnectionID = UUID()
        connectionID = currentConnectionID
        activeConnector = connector
        ignoresNextCredentialPromptDismissal = true
        isCredentialPromptPresented = false
        isConnecting = true
        isConnected = false
        errorText = nil
        statusText = "Opening SSH tunnel..."

        do {
            try await connector.connect(token: token, username: trimmedUsername, password: submittedPassword)
            guard connectionID == currentConnectionID else { return }
            isConnecting = false
            isConnected = true
            statusText = "SSH tunnel connected."
            errorText = nil
            clearCredentials()
        } catch {
            guard connectionID == currentConnectionID else { return }
            activeConnector = nil
            isConnecting = false
            isConnected = false
            let message = error.localizedDescription
            statusText = message
            errorText = message
            clearCredentials()
        }
    }

    func disconnect() {
        activeConnector?.disconnect()
        activeConnector = nil
        connectionID = UUID()
        isCredentialPromptPresented = false
        isConnecting = false
        isConnected = false
        errorText = nil
        statusText = "Disconnected."
        clearCredentials()
    }

    private func presentCredentialPrompt(status: String) {
        activeConnector?.disconnect()
        activeConnector = nil
        connectionID = UUID()
        clearCredentials()
        isConnecting = false
        isConnected = false
        errorText = nil
        statusText = status
        isCredentialPromptPresented = true
    }

    private func parseTCPPayload(_ raw: String) throws -> String {
        var error: NSError?
        guard let parsed = DerpholemobileParsePayload(raw, &error) else {
            throw error ?? SSHConnectionError.invalidPayload
        }
        if let error {
            throw error
        }
        guard parsed.kind() == "tcp" else {
            throw SSHConnectionError.nonTCPPayload
        }
        return parsed.token()
    }

    private func failBeforeConnect(status: String, error: String) {
        activeConnector?.disconnect()
        activeConnector = nil
        connectionID = UUID()
        isCredentialPromptPresented = false
        isConnecting = false
        isConnected = false
        statusText = status
        errorText = error
        clearCredentials()
    }

    private func clearCredentials() {
        username = ""
        password = ""
    }
}

nonisolated private struct PlaceholderSSHConnector: SSHLocalTunnelConnecting {
    func connect(token: String, username: String, password: String) async throws {
        throw SSHConnectionError.terminalIntegrationPending
    }

    func disconnect() {}
}
