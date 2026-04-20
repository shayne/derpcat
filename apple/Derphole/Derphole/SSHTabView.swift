import SwiftUI

struct SSHTabView: View {
    @StateObject private var state: SSHTunnelState

    init(tokenStore: TokenStore) {
        _state = StateObject(wrappedValue: SSHTunnelState(tokenStore: tokenStore))
    }

    var body: some View {
        ScrollView {
            VStack(alignment: .leading, spacing: 20) {
                connectionSection

                if let errorText = state.errorText {
                    Text(errorText)
                        .font(.footnote)
                        .foregroundStyle(.red)
                }
            }
            .frame(maxWidth: .infinity, alignment: .leading)
            .padding(20)
        }
        .accessibilityIdentifier("sshTab")
        .navigationTitle("SSH")
        .fullScreenCover(isPresented: $state.isScannerPresented, onDismiss: state.scannerDismissed) {
            ScannerSheet(
                accessibilityIdentifier: "sshScannerSheet",
                onPayload: state.acceptScannedPayload,
                onCancel: state.scannerDismissed
            )
        }
        .sheet(isPresented: $state.isCredentialPromptPresented, onDismiss: state.credentialPromptDismissed) {
            SSHCredentialPrompt(
                username: $state.username,
                password: $state.password,
                onCancel: state.cancelCredentialPrompt,
                onConnect: {
                    Task {
                        await state.submitCredentials()
                    }
                }
            )
        }
    }

    @ViewBuilder
    private var connectionSection: some View {
        if state.isConnecting {
            connectingView
        } else if state.isConnected {
            connectedView
        } else {
            zeroStateView
        }
    }

    private var zeroStateView: some View {
        VStack(alignment: .leading, spacing: 16) {
            Button {
                state.scanStarted()
            } label: {
                Label("Scan QR Code", systemImage: "qrcode.viewfinder")
                    .font(.headline)
                    .frame(maxWidth: .infinity)
            }
            .buttonStyle(.borderedProminent)
            .controlSize(.large)
            .accessibilityIdentifier("sshScanQRCodeButton")

            if let fingerprint = state.rememberedTokenFingerprint {
                VStack(alignment: .leading, spacing: 10) {
                    Label {
                        Text(fingerprint)
                            .font(.caption.monospaced())
                    } icon: {
                        Image(systemName: "key.horizontal")
                    }
                    .foregroundStyle(.secondary)
                    .accessibilityIdentifier("sshRememberedToken")

                    Button {
                        state.reconnect()
                    } label: {
                        Label("Reconnect", systemImage: "arrow.clockwise")
                            .frame(maxWidth: .infinity)
                    }
                    .buttonStyle(.bordered)
                    .controlSize(.large)
                    .accessibilityIdentifier("sshReconnectButton")
                }
            }

            Text(state.statusText)
                .font(.callout)
                .foregroundStyle(.secondary)
        }
    }

    private var connectingView: some View {
        VStack(alignment: .leading, spacing: 14) {
            HStack(alignment: .firstTextBaseline) {
                Text("Opening tunnel")
                    .font(.headline)
                Spacer()
                Image(systemName: "terminal")
                    .foregroundStyle(.secondary)
            }

            ProgressView()

            Text(state.statusText)
                .font(.callout)
                .foregroundStyle(.secondary)

            Button("Disconnect", role: .destructive) {
                state.disconnect()
            }
            .buttonStyle(.bordered)
            .accessibilityIdentifier("sshDisconnectButton")
        }
        .padding(16)
        .background(.regularMaterial, in: RoundedRectangle(cornerRadius: 8))
    }

    private var connectedView: some View {
        VStack(alignment: .leading, spacing: 16) {
            Label("Connected", systemImage: "checkmark.circle.fill")
                .font(.headline)
                .foregroundStyle(.green)

            Text(state.statusText)
                .font(.callout)
                .foregroundStyle(.secondary)

            Button(role: .destructive) {
                state.disconnect()
            } label: {
                Label("Disconnect", systemImage: "xmark.circle")
                    .frame(maxWidth: .infinity)
            }
            .buttonStyle(.bordered)
            .controlSize(.large)
            .accessibilityIdentifier("sshDisconnectButton")
        }
        .padding(16)
        .background(.regularMaterial, in: RoundedRectangle(cornerRadius: 8))
    }
}

#Preview {
    NavigationStack {
        SSHTabView(tokenStore: TokenStore())
    }
}
