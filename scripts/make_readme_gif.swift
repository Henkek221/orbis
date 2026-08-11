#!/usr/bin/env swift

import AVFoundation
import CoreGraphics
import Foundation
import ImageIO
import UniformTypeIdentifiers

struct Options {
    let input: URL
    let output: URL
    let width: Int
    let framesPerSecond: Double
    let maxDuration: Double
}

func fail(_ message: String) -> Never {
    FileHandle.standardError.write(Data(("Fehler: \(message)\n").utf8))
    exit(1)
}

func parseOptions() -> Options {
    let arguments = Array(CommandLine.arguments.dropFirst())
    guard arguments.count >= 2 else {
        fail("Aufruf: swift scripts/make_readme_gif.swift aufnahme.mov ausgabe.gif [breite] [fps] [max-sekunden]")
    }

    return Options(
        input: URL(fileURLWithPath: arguments[0]),
        output: URL(fileURLWithPath: arguments[1]),
        width: arguments.count > 2 ? Int(arguments[2]) ?? 900 : 900,
        framesPerSecond: arguments.count > 3 ? Double(arguments[3]) ?? 6 : 6,
        maxDuration: arguments.count > 4 ? Double(arguments[4]) ?? 35 : 35
    )
}

let options = parseOptions()
guard FileManager.default.fileExists(atPath: options.input.path) else {
    fail("Videodatei nicht gefunden: \(options.input.path)")
}
guard options.width >= 320, options.framesPerSecond > 0, options.maxDuration > 0 else {
    fail("Breite, Bildrate und Dauer müssen größer als null sein.")
}

try? FileManager.default.createDirectory(
    at: options.output.deletingLastPathComponent(),
    withIntermediateDirectories: true
)

let asset = AVURLAsset(url: options.input)
let assetDuration: CMTime
do {
    assetDuration = try await asset.load(.duration)
} catch {
    fail("Die Videodauer konnte nicht gelesen werden: \(error.localizedDescription)")
}
let duration = CMTimeGetSeconds(assetDuration)
guard duration.isFinite, duration > 0 else {
    fail("Die Videodauer konnte nicht gelesen werden.")
}

let usedDuration = min(duration, options.maxDuration)
let frameCount = max(2, Int(floor(usedDuration * options.framesPerSecond)))
let frameDelay = usedDuration / Double(frameCount)

guard let destination = CGImageDestinationCreateWithURL(
    options.output as CFURL,
    UTType.gif.identifier as CFString,
    frameCount,
    nil
) else {
    fail("GIF-Ausgabe konnte nicht angelegt werden.")
}

let gifProperties: [CFString: Any] = [
    kCGImagePropertyGIFDictionary: [
        kCGImagePropertyGIFLoopCount: 0
    ]
]
CGImageDestinationSetProperties(destination, gifProperties as CFDictionary)

let generator = AVAssetImageGenerator(asset: asset)
generator.appliesPreferredTrackTransform = true
generator.maximumSize = CGSize(width: options.width, height: options.width)
generator.requestedTimeToleranceBefore = .zero
generator.requestedTimeToleranceAfter = CMTime(seconds: 0.08, preferredTimescale: 600)

let frameProperties: [CFString: Any] = [
    kCGImagePropertyGIFDictionary: [
        kCGImagePropertyGIFDelayTime: frameDelay,
        kCGImagePropertyGIFUnclampedDelayTime: frameDelay
    ]
]

for index in 0..<frameCount {
    let seconds = min(usedDuration - 0.001, Double(index) * frameDelay)
    let time = CMTime(seconds: seconds, preferredTimescale: 600)
    do {
        let (image, _) = try await generator.image(at: time)
        CGImageDestinationAddImage(destination, image, frameProperties as CFDictionary)
    } catch {
        fail("Frame bei \(String(format: "%.2f", seconds)) s konnte nicht gelesen werden: \(error.localizedDescription)")
    }
}

guard CGImageDestinationFinalize(destination) else {
    fail("GIF konnte nicht fertig geschrieben werden.")
}

let attributes = try? FileManager.default.attributesOfItem(atPath: options.output.path)
let bytes = (attributes?[.size] as? NSNumber)?.intValue ?? 0
print("GIF erstellt: \(options.output.path) (\(frameCount) Frames, \(bytes / 1_024) KB)")
