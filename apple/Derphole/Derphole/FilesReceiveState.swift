import Combine
import DerpholeMobile
import Foundation

final class FilesReceiveState: ObservableObject {
    enum Phase: Equatable {
        case idle
        case scanning
        case receiving
        case received
        case failed
        case canceled
    }

    enum Route: Equatable {
        case unknown
        case relay
        case direct

        var label: String {
            switch self {
            case .unknown:
                return "Negotiating"
            case .relay:
                return "Relay"
            case .direct:
                return "Direct"
            }
        }
    }

    @Published var pastedPayload = ""
    @Published var isScannerPresented = false
    @Published var isSaveFilePresented = false
    @Published private(set) var phase: Phase = .idle
    @Published private(set) var statusText = "Ready."
    @Published private(set) var traceText = ""
    @Published private(set) var route: Route = .unknown
    @Published private(set) var progressCurrent: Int64 = 0
    @Published private(set) var progressTotal: Int64 = 0
    @Published private(set) var speedBytesPerSecond: Double = 0
    @Published private(set) var validatedToken = ""
    @Published private(set) var completedFileURL: URL?
    @Published private(set) var errorText: String?

    let showsDebugPayloadControls: Bool

    private let now: () -> Date
    private var activeReceiver: DerpholemobileReceiver?
    private var callbackPump: TransferUIUpdatePump?
    private var transferID = UUID()
    private var cancelRequested = false
    private var lastScannedPayload = ""
    private var lastProgressSample: (bytes: Int64, date: Date)?
    #if DEBUG
    private var runtimeInjectedReceiveStarted = false
    #endif

    init(
        showsDebugPayloadControls: Bool = AppLaunchMode.showsDebugPayloadControls,
        now: @escaping () -> Date = Date.init
    ) {
        self.showsDebugPayloadControls = showsDebugPayloadControls
        self.now = now
    }

    var isReceiving: Bool {
        phase == .receiving
    }

    var isScanning: Bool {
        phase == .scanning
    }

    var canValidatePayload: Bool {
        !pastedPayload.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty && !isReceiving && !canSave
    }

    var canStartReceive: Bool {
        canValidatePayload && !canSave
    }

    var canStartScan: Bool {
        !isReceiving && !canSave
    }

    var canSave: Bool {
        completedFileURL != nil && !isReceiving
    }

    var canExport: Bool {
        canSave
    }

    var progressFraction: Double? {
        guard progressTotal > 0 else { return nil }
        return min(max(Double(progressCurrent) / Double(progressTotal), 0), 1)
    }

    var progressText: String {
        guard progressTotal > 0 else {
            return TransferFormatting.mib(progressCurrent)
        }
        return "\(TransferFormatting.mib(progressCurrent)) / \(TransferFormatting.mib(progressTotal))"
    }

    var speedText: String {
        TransferFormatting.speed(bytesPerSecond: speedBytesPerSecond)
    }

    var statusSummary: String {
        switch phase {
        case .idle:
            return "Ready to receive"
        case .scanning:
            return "Scanning"
        case .receiving:
            return "Receiving"
        case .received:
            return "Receive complete"
        case .failed:
            return "Receive failed"
        case .canceled:
            return "Receive canceled"
        }
    }

    func validatePastedPayload() {
        do {
            let token = try parsePayload(pastedPayload)
            validatedToken = token
            errorText = nil
            phase = .idle
            statusText = "Payload looks valid."
        } catch {
            validatedToken = ""
            errorText = error.localizedDescription
            statusText = "Payload validation failed."
        }
    }

    func scanStarted() {
        guard canStartScan else { return }
        phase = .scanning
        isScannerPresented = true
        resetTransferProgress()
        validatedToken = ""
        completedFileURL = nil
        traceText = ""
        statusText = "Scanning for QR code."
        errorText = nil
        cancelRequested = false
        lastScannedPayload = ""
    }

    func scannerDismissed() {
        guard phase == .scanning else { return }
        isScannerPresented = false
        phase = .idle
        statusText = "Ready."
    }

    func receivePastedPayload() {
        startReceive(from: pastedPayload, source: .manual)
    }

    func receiveScannedPayload(_ payload: String) {
        let trimmed = payload.trimmingCharacters(in: .whitespacesAndNewlines)
        guard !trimmed.isEmpty, canStartScan, trimmed != lastScannedPayload else {
            return
        }
        pastedPayload = trimmed
        startReceive(from: trimmed, source: .scanner)
    }

    func notePastedPayloadEdited() {
        if !isReceiving {
            validatedToken = ""
            errorText = nil
            if phase == .failed || phase == .canceled {
                phase = .idle
                statusText = "Ready."
            }
        }
    }

    func cancelReceive() {
        cancel()
    }

    func cancel() {
        guard isReceiving else {
            phase = .canceled
            isScannerPresented = false
            statusText = "Receive canceled."
            errorText = nil
            return
        }
        cancelRequested = true
        statusText = "Canceling receive..."
        activeReceiver?.cancel()
    }

    func saveFile() {
        guard completedFileURL != nil else { return }
        isSaveFilePresented = true
    }

    func presentExporter() {
        saveFile()
    }

    func saveFinished(saved: Bool) {
        isSaveFilePresented = false
        guard saved else { return }
        resetToIdle(deleteReceivedFile: false)
    }

    func exporterFinished(exported: Bool) {
        saveFinished(saved: exported)
    }

    func discardReceivedFile() {
        resetToIdle(deleteReceivedFile: true)
    }

    #if DEBUG
    func receiveRuntimeInjectedPayloadIfConfigured(
        environment: [String: String] = ProcessInfo.processInfo.environment,
        arguments: [String] = ProcessInfo.processInfo.arguments
    ) {
        guard !runtimeInjectedReceiveStarted else { return }
        guard let payload = LiveReceiveLaunchConfiguration.payload(from: environment, arguments: arguments) else { return }

        runtimeInjectedReceiveStarted = true
        pastedPayload = payload
        startReceive(from: payload, source: .manual)
    }

    func markCompletedForTesting(fileURL: URL) {
        resetTransferProgress()
        phase = .received
        completedFileURL = fileURL
        statusText = "Receive complete."
    }

    func recordStatusForTesting(_ status: String) {
        handleStatus(status, transferID: transferID)
    }
    #endif

    func recordProgress(current: Int64, total: Int64) {
        handleProgress(current: current, total: total, transferID: transferID)
    }

    private enum ReceiveSource {
        case manual
        case scanner
    }

    private func startReceive(from payload: String, source: ReceiveSource) {
        guard !isReceiving, !canSave else { return }

        let token: String
        do {
            token = try parsePayload(payload)
        } catch {
            validatedToken = ""
            lastScannedPayload = ""
            isScannerPresented = false
            phase = .failed
            errorText = error.localizedDescription
            statusText = source == .scanner ? "Scanned code was invalid." : "Payload validation failed."
            return
        }

        let receiveRoot = FileManager.default.temporaryDirectory.appendingPathComponent("DerpholeReceive-\(UUID().uuidString)", isDirectory: true)
        do {
            try FileManager.default.createDirectory(at: receiveRoot, withIntermediateDirectories: true)
        } catch {
            isScannerPresented = false
            phase = .failed
            errorText = error.localizedDescription
            statusText = "Could not prepare a receive directory."
            return
        }

        guard let receiver = DerpholemobileNewReceiver() else {
            isScannerPresented = false
            phase = .failed
            errorText = "Could not create the Derphole receiver bridge."
            statusText = "Receiver initialization failed."
            return
        }

        let currentTransferID = UUID()
        transferID = currentTransferID
        activeReceiver = receiver
        if source == .scanner {
            lastScannedPayload = payload.trimmingCharacters(in: .whitespacesAndNewlines)
        }
        cancelRequested = false
        isScannerPresented = false
        phase = .receiving
        resetTransferProgress()
        validatedToken = token
        completedFileURL = nil
        errorText = nil
        traceText = ""
        statusText = source == .scanner ? "QR code scanned. Starting receive..." : "Starting receive..."

        let callbackPump = TransferUIUpdatePump { [weak self] snapshot in
            guard let self else { return }
            if let progress = snapshot.progress {
                self.handleProgress(current: progress.current, total: progress.total, transferID: currentTransferID)
            }
            if let status = snapshot.status {
                self.handleStatus(status, transferID: currentTransferID)
            }
            if let trace = snapshot.trace {
                self.handleTrace(trace, transferID: currentTransferID)
            }
        }
        self.callbackPump = callbackPump

        let callbacks = TransferCallbacks(
            onProgress: { current, total in
                callbackPump.progress(current: current, total: total)
            },
            onStatus: { status in
                callbackPump.status(status)
            },
            onTrace: { trace in
                callbackPump.trace(trace)
            }
        )

        DispatchQueue.global(qos: .userInitiated).async { [weak self] in
            do {
                let outputPath = try Self.receiveWithBridge(receiver: receiver, payload: payload, outputDir: receiveRoot.path, callbacks: callbacks)
                DispatchQueue.main.async {
                    self?.completeReceive(at: URL(fileURLWithPath: outputPath), transferID: currentTransferID)
                }
            } catch {
                DispatchQueue.main.async {
                    self?.failReceive(error, transferID: currentTransferID)
                }
            }
        }
    }

    private func parsePayload(_ payload: String) throws -> String {
        var error: NSError?
        let token = DerpholemobileParseFileToken(payload, &error)
        if let error {
            throw error
        }
        return token
    }

    private func handleProgress(current: Int64, total: Int64, transferID: UUID) {
        guard self.transferID == transferID else { return }

        let sampleDate = now()
        if let previous = lastProgressSample {
            let elapsed = sampleDate.timeIntervalSince(previous.date)
            let delta = current - previous.bytes
            if elapsed > 0, delta >= 0 {
                speedBytesPerSecond = Double(delta) / elapsed
            }
        }
        lastProgressSample = (bytes: current, date: sampleDate)

        progressCurrent = current
        progressTotal = total
    }

    private func handleStatus(_ status: String, transferID: UUID) {
        guard self.transferID == transferID else { return }

        let normalized = status.trimmingCharacters(in: .whitespacesAndNewlines)
        guard !normalized.isEmpty else { return }

        switch normalized {
        case "connected-relay":
            route = .relay
            statusText = "Connected through relay."
        case "connected-direct":
            route = .direct
            statusText = "Promoted to direct path."
        default:
            statusText = normalized.replacingOccurrences(of: "-", with: " ")
        }
    }

    private func handleTrace(_ trace: String, transferID: UUID) {
        guard self.transferID == transferID else { return }

        let normalized = trace.trimmingCharacters(in: .whitespacesAndNewlines)
        guard !normalized.isEmpty else { return }

        traceText = normalized
        if normalized.contains("webrelay") && route == .unknown {
            route = .relay
        }
    }

    private func completeReceive(at fileURL: URL, transferID: UUID) {
        guard self.transferID == transferID else { return }
        callbackPump?.flushPending()
        callbackPump = nil
        activeReceiver = nil
        phase = .received
        completedFileURL = fileURL
        statusText = "Receive complete."
        if progressTotal > 0 {
            progressCurrent = progressTotal
        }
        lastScannedPayload = ""
    }

    private func failReceive(_ error: Error, transferID: UUID) {
        guard self.transferID == transferID else { return }
        callbackPump?.flushPending()
        callbackPump = nil
        activeReceiver = nil
        completedFileURL = nil
        lastScannedPayload = ""

        let description = error.localizedDescription
        if cancelRequested || description.localizedCaseInsensitiveContains("canceled") {
            phase = .canceled
            statusText = "Receive canceled."
            errorText = nil
        } else {
            phase = .failed
            statusText = "Receive failed."
            errorText = description
        }
        cancelRequested = false
    }

    private func resetTransferProgress() {
        route = .unknown
        progressCurrent = 0
        progressTotal = 0
        speedBytesPerSecond = 0
        lastProgressSample = nil
    }

    private func resetToIdle(deleteReceivedFile: Bool) {
        if deleteReceivedFile, let completedFileURL {
            try? FileManager.default.removeItem(at: completedFileURL.deletingLastPathComponent())
        }

        activeReceiver?.cancel()
        callbackPump = nil
        activeReceiver = nil
        phase = .idle
        isScannerPresented = false
        isSaveFilePresented = false
        cancelRequested = false
        validatedToken = ""
        completedFileURL = nil
        traceText = ""
        errorText = nil
        lastScannedPayload = ""
        pastedPayload = ""
        statusText = "Ready."
        resetTransferProgress()
    }

    private static func receiveWithBridge(
        receiver: DerpholemobileReceiver,
        payload: String,
        outputDir: String,
        callbacks: TransferCallbacks
    ) throws -> String {
        var error: NSError?
        let outputPath = receiver.receive(payload, outputDir: outputDir, callbacks: callbacks, error: &error)
        if let error {
            throw error
        }
        return outputPath
    }
}

private final class TransferCallbacks: NSObject, DerpholemobileCallbacksProtocol, @unchecked Sendable {
    private let onProgress: @Sendable (Int64, Int64) -> Void
    private let onStatus: @Sendable (String) -> Void
    private let onTrace: @Sendable (String) -> Void

    init(
        onProgress: @escaping @Sendable (Int64, Int64) -> Void,
        onStatus: @escaping @Sendable (String) -> Void,
        onTrace: @escaping @Sendable (String) -> Void
    ) {
        self.onProgress = onProgress
        self.onStatus = onStatus
        self.onTrace = onTrace
    }

    func progress(_ current: Int64, total: Int64) {
        onProgress(current, total)
    }

    func status(_ status: String?) {
        onStatus(status ?? "")
    }

    func trace(_ trace: String?) {
        onTrace(trace ?? "")
    }
}
