// swift-tools-version: 5.10
import PackageDescription

let package = Package(
    name: "derphole",
    platforms: [
        .iOS(.v17)
    ],
    products: [
        .library(name: "DerpholeTunnel", targets: ["DerpholeTunnel"])
    ],
    targets: [
        .binaryTarget(
            name: "DerpholeMobile",
            url: "https://github.com/shayne/derphole/releases/download/v0.12.0/DerpholeMobile.xcframework.zip",
            checksum: "c7e2a05f0d2083250d580a98627f762697f8bc6f751a396a5dc55848d0a14850"
        ),
        .target(
            name: "DerpholeTunnel",
            dependencies: ["DerpholeMobile"],
            path: "Sources/DerpholeTunnel"
        ),
        .testTarget(
            name: "DerpholeTunnelTests",
            dependencies: ["DerpholeTunnel"],
            path: "Tests/DerpholeTunnelTests"
        )
    ]
)
