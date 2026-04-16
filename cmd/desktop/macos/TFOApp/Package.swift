// swift-tools-version: 5.9
import PackageDescription

let package = Package(
    name: "TFOApp",
    platforms: [
        .macOS(.v13)
    ],
    targets: [
        .executableTarget(
            name: "TFOApp",
            path: "Sources",
            resources: []
        )
    ]
)
