import SwiftUI

struct SSHCredentialPrompt: View {
    @Binding var username: String
    @Binding var password: String

    let onCancel: () -> Void
    let onConnect: () -> Void

    var body: some View {
        NavigationStack {
            Form {
                Section {
                    TextField("Username", text: $username)
                        .textContentType(.username)
                        .textInputAutocapitalization(.never)
                        .autocorrectionDisabled()
                        .accessibilityIdentifier("sshUsernameField")

                    SecureField("Password", text: $password)
                        .textContentType(.password)
                        .accessibilityIdentifier("sshPasswordField")
                }
            }
            .navigationTitle("SSH Credentials")
            .navigationBarTitleDisplayMode(.inline)
            .toolbar {
                ToolbarItem(placement: .cancellationAction) {
                    Button("Cancel", action: onCancel)
                        .accessibilityIdentifier("sshCredentialCancelButton")
                }

                ToolbarItem(placement: .confirmationAction) {
                    Button("Connect", action: onConnect)
                        .disabled(!canConnect)
                        .accessibilityIdentifier("sshCredentialConnectButton")
                }
            }
        }
        .presentationDetents([.medium])
        .accessibilityIdentifier("sshCredentialPrompt")
    }

    private var canConnect: Bool {
        !username.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty && !password.isEmpty
    }
}

#Preview {
    SSHCredentialPrompt(
        username: .constant(""),
        password: .constant(""),
        onCancel: {},
        onConnect: {}
    )
}
