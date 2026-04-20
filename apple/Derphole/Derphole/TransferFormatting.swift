import Foundation

enum TransferFormatting {
    private static let bytesPerMiB = 1_048_576.0

    static func mib(_ bytes: Int64) -> String {
        String(format: "%.1f MiB", Double(bytes) / bytesPerMiB)
    }

    static func speed(bytesPerSecond: Double) -> String {
        String(format: "%.1f MiB/s", max(bytesPerSecond, 0) / bytesPerMiB)
    }

    static func fingerprint(_ token: String) -> String {
        guard token.count > 18 else { return token }

        let prefix = token.prefix(10)
        let suffix = token.suffix(4)
        return "\(prefix)...\(suffix)"
    }
}
