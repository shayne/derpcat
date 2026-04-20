import SwiftUI

struct ScannerSheet: View {
    let accessibilityIdentifier: String
    let onPayload: (String) -> Void
    let onCancel: () -> Void

    var body: some View {
        NavigationStack {
            QRScannerView(isScanning: true) { payload in
                onPayload(payload)
            }
            .ignoresSafeArea()
            .navigationTitle("Scan QR Code")
            .navigationBarTitleDisplayMode(.inline)
            .toolbar {
                ToolbarItem(placement: .cancellationAction) {
                    Button("Cancel", action: onCancel)
                }
            }
        }
        .accessibilityIdentifier(accessibilityIdentifier)
    }
}
