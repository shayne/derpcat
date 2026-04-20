import XCTest
@testable import Derphole

final class FilesReceiveStateTests: XCTestCase {
    @MainActor
    func testDiscardDeletesCompletedFileAndResets() throws {
        let state = FilesReceiveState()
        let dir = FileManager.default.temporaryDirectory
            .appendingPathComponent("FilesReceiveStateTests-\(UUID().uuidString)", isDirectory: true)
        try FileManager.default.createDirectory(at: dir, withIntermediateDirectories: true)
        let file = dir.appendingPathComponent("received.bin")
        try Data([1, 2, 3]).write(to: file)

        state.markCompletedForTesting(fileURL: file)
        XCTAssertTrue(FileManager.default.fileExists(atPath: file.path))

        state.discardReceivedFile()

        XCTAssertEqual(state.phase, .idle)
        XCTAssertNil(state.completedFileURL)
        XCTAssertFalse(FileManager.default.fileExists(atPath: dir.path))
        XCTAssertEqual(state.statusText, "Ready.")
    }

    @MainActor
    func testSpeedUpdatesFromProgressSnapshots() {
        let clock = ManualClock()
        let state = FilesReceiveState(now: { clock.now })

        state.recordProgress(current: 0, total: 4_194_304)
        clock.advance(by: 1)
        state.recordProgress(current: 2_097_152, total: 4_194_304)

        XCTAssertEqual(state.progressText, "2.0 MiB / 4.0 MiB")
        XCTAssertEqual(state.speedText, "2.0 MiB/s")
    }

    @MainActor
    func testRouteLabelsMapTransportStatuses() {
        let state = FilesReceiveState()

        state.recordStatusForTesting("connected-relay")
        XCTAssertEqual(state.route, .relay)
        XCTAssertEqual(state.route.label, "Relay")

        state.recordStatusForTesting("connected-direct")
        XCTAssertEqual(state.route, .direct)
        XCTAssertEqual(state.route.label, "Direct")
    }
}

private final class ManualClock {
    private(set) var now: Date = Date(timeIntervalSince1970: 0)

    func advance(by seconds: TimeInterval) {
        now = now.addingTimeInterval(seconds)
    }
}
