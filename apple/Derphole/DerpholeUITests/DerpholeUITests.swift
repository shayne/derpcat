import XCTest

final class DerpholeUITests: XCTestCase {
    override func setUpWithError() throws {
        continueAfterFailure = false
    }

    @MainActor
    func testLaunchShowsNativeTabsAndMinimalFilesUI() throws {
        let app = XCUIApplication()
        app.launch()

        XCTAssertTrue(app.tabBars.buttons["Files"].waitForExistence(timeout: 5))
        XCTAssertTrue(app.tabBars.buttons["Web"].exists)
        XCTAssertTrue(app.tabBars.buttons["SSH"].exists)
        XCTAssertTrue(app.descendants(matching: .any)["filesTab"].waitForExistence(timeout: 5))
        XCTAssertTrue(app.buttons["filesScanQRCodeButton"].waitForExistence(timeout: 5))
        XCTAssertFalse(app.textFields["filesDebugPayloadField"].exists)

        app.tabBars.buttons["Web"].tap()
        XCTAssertTrue(app.descendants(matching: .any)["webTab"].waitForExistence(timeout: 5))

        app.tabBars.buttons["SSH"].tap()
        XCTAssertTrue(app.descendants(matching: .any)["sshTab"].waitForExistence(timeout: 5))
    }

    @MainActor
    func testDebugPayloadControlsAreHiddenUnlessLaunchModeRequestsThem() throws {
        let app = XCUIApplication()
        app.launchArguments.append("--derphole-debug-payload-controls")
        app.launch()

        XCTAssertTrue(app.descendants(matching: .any)["filesTab"].waitForExistence(timeout: 5))
        XCTAssertTrue(app.textFields["filesDebugPayloadField"].waitForExistence(timeout: 5))
    }

    @MainActor
    func testScanButtonPresentsModalScanner() throws {
        let app = XCUIApplication()
        addUIInterruptionMonitor(withDescription: "Camera permission") { alert in
            let button = alert.buttons.element(boundBy: 0)
            if button.exists {
                button.tap()
                return true
            }
            return false
        }
        app.launch()
        app.tap()

        let scanButton = app.buttons["filesScanQRCodeButton"]
        XCTAssertTrue(scanButton.waitForExistence(timeout: 5))
        scanButton.tap()

        XCTAssertTrue(app.otherElements["filesScannerSheet"].waitForExistence(timeout: 10))
    }
}
