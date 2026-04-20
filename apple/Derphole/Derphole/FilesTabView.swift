import SwiftUI

struct FilesTabView: View {
    @StateObject private var state: FilesReceiveState

    init(state: @autoclosure @escaping () -> FilesReceiveState = FilesReceiveState()) {
        _state = StateObject(wrappedValue: state())
    }

    var body: some View {
        ScrollView {
            VStack(alignment: .leading, spacing: 20) {
                switch state.phase {
                case .receiving:
                    transferProgressView
                case .received:
                    receiveCompleteView
                default:
                    zeroStateView
                }

                if let errorText = state.errorText {
                    Text(errorText)
                        .font(.footnote)
                        .foregroundStyle(.red)
                        .accessibilityIdentifier("filesErrorText")
                }

                if state.showsDebugPayloadControls {
                    debugPayloadSection
                }
            }
            .frame(maxWidth: .infinity, alignment: .leading)
            .padding(20)
        }
        .accessibilityIdentifier("filesTab")
        .navigationTitle("Files")
        .fullScreenCover(isPresented: $state.isScannerPresented, onDismiss: state.scannerDismissed) {
            ScannerSheet(
                accessibilityIdentifier: "filesScannerSheet",
                onPayload: state.receiveScannedPayload,
                onCancel: state.scannerDismissed
            )
        }
        .sheet(isPresented: $state.isSaveFilePresented) {
            if let fileURL = state.completedFileURL {
                DocumentExporter(fileURL: fileURL) {
                    state.saveFinished(saved: $0)
                }
            }
        }
        .onAppear {
            #if DEBUG
            state.receiveRuntimeInjectedPayloadIfConfigured()
            #endif
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
            .disabled(!state.canStartScan)
            .accessibilityIdentifier("filesScanQRCodeButton")

            if state.phase == .canceled || state.phase == .failed {
                Text(state.statusText)
                    .font(.callout)
                    .foregroundStyle(.secondary)
            }
        }
    }

    private var transferProgressView: some View {
        VStack(alignment: .leading, spacing: 14) {
            HStack(alignment: .firstTextBaseline) {
                Text(state.statusSummary)
                    .font(.headline)
                Spacer()
                routeBadge
            }

            Text(state.statusText)
                .font(.body)
                .foregroundStyle(.secondary)

            if let fraction = state.progressFraction {
                ProgressView(value: fraction)
                    .progressViewStyle(.linear)
            } else {
                ProgressView()
            }

            HStack {
                Text(state.progressText)
                    .font(.subheadline.monospacedDigit())
                Spacer()
                Text(state.speedText)
                    .font(.subheadline.monospacedDigit().weight(.semibold))
            }
            .foregroundStyle(.secondary)

            if !state.traceText.isEmpty {
                Text(state.traceText)
                    .font(.caption.monospaced())
                    .foregroundStyle(.secondary)
                    .lineLimit(4)
                    .textSelection(.enabled)
            }

            Button("Cancel Receive") {
                state.cancelReceive()
            }
            .buttonStyle(.bordered)
        }
        .padding(16)
        .background(.regularMaterial, in: RoundedRectangle(cornerRadius: 8))
        .accessibilityIdentifier("filesTransferProgress")
    }

    private var receiveCompleteView: some View {
        VStack(alignment: .leading, spacing: 16) {
            HStack(alignment: .firstTextBaseline) {
                Label("Receive Complete", systemImage: "checkmark.circle.fill")
                    .font(.headline)
                    .foregroundStyle(.green)
                Spacer()
                routeBadge
            }

            if let fileURL = state.completedFileURL {
                Text(fileURL.lastPathComponent)
                    .font(.body.weight(.semibold))
                    .lineLimit(2)
            }

            HStack(spacing: 12) {
                Button("Save File") {
                    state.saveFile()
                }
                .buttonStyle(.borderedProminent)
                .accessibilityIdentifier("filesSaveFileButton")

                Button("Discard", role: .destructive) {
                    state.discardReceivedFile()
                }
                .buttonStyle(.bordered)
                .accessibilityIdentifier("filesDiscardButton")
            }
        }
        .padding(16)
        .background(.regularMaterial, in: RoundedRectangle(cornerRadius: 8))
    }

    private var routeBadge: some View {
        Text(state.route.label)
            .font(.caption.weight(.semibold))
            .padding(.horizontal, 10)
            .padding(.vertical, 6)
            .background(routeBackground)
            .clipShape(Capsule())
            .accessibilityIdentifier("filesRouteBadge")
    }

    private var debugPayloadSection: some View {
        VStack(alignment: .leading, spacing: 12) {
            Text("Debug Payload")
                .font(.headline)

            TextField("Paste a Derphole file payload or raw token", text: $state.pastedPayload)
                .textInputAutocapitalization(.never)
                .disableAutocorrection(true)
                .textFieldStyle(.roundedBorder)
                .accessibilityIdentifier("filesDebugPayloadField")
                .onChange(of: state.pastedPayload) { _, _ in
                    state.notePastedPayloadEdited()
                }

            HStack(spacing: 12) {
                Button("Validate") {
                    state.validatePastedPayload()
                }
                .buttonStyle(.bordered)
                .disabled(!state.canValidatePayload)

                Button("Receive") {
                    state.receivePastedPayload()
                }
                .buttonStyle(.borderedProminent)
                .disabled(!state.canStartReceive)
            }
        }
        .padding(16)
        .background(Color(.secondarySystemBackground), in: RoundedRectangle(cornerRadius: 8))
    }

    private var routeBackground: Color {
        switch state.route {
        case .unknown:
            return .gray.opacity(0.16)
        case .relay:
            return .orange.opacity(0.18)
        case .direct:
            return .green.opacity(0.18)
        }
    }
}

#Preview {
    NavigationStack {
        FilesTabView()
    }
}
