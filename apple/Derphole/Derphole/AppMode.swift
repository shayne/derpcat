import Foundation

enum AppTab: Hashable {
    case files
    case web
    case ssh
}

enum AppLaunchMode {
    static var showsDebugPayloadControls: Bool {
        ProcessInfo.processInfo.arguments.contains("--derphole-debug-payload-controls")
            || ProcessInfo.processInfo.environment["DERPHOLE_DEBUG_PAYLOAD_CONTROLS"] == "1"
    }
}
