// swift-tools-version: 5.9
import PackageDescription

let package = Package(
    name: "VoiceInput",
    platforms: [.macOS(.v14)],
    products: [
        .executable(
            name: "VoiceInput",
            targets: ["VoiceInput"],
            bundleIdiom: .mac
        )
    ],
    targets: [
        .executableTarget(
            name: "VoiceInput",
            dependencies: [],
            path: "Sources/VoiceInput",
            resources: [.process("Resources")]
        )
    ]
)
