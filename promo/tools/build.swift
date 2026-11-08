// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Henrik Hellpap. Part of Orbis: https://github.com/Henkek221/orbis

// Renders the "city builds itself" shot: tiles land in export order, splats fade in last.
// usage: build <model.usdz> <outdir> <frames> <tilesEndFrame> <splatStart> <splatEnd> [startDeg sweepDeg elevDeg dist]
import AppKit
import SceneKit

let a = CommandLine.arguments
let frames = Int(a[3])!, tilesEnd = Double(a[4])!, splatStart = Double(a[5])!, splatEnd = Double(a[6])!
let startDeg = a.count > 7 ? Double(a[7])! : 20
let sweepDeg = a.count > 8 ? Double(a[8])! : 70
let elevDeg = a.count > 9 ? Double(a[9])! : 34
let distFactor = a.count > 10 ? Double(a[10])! : 1.25

let scene = try SCNScene(url: URL(fileURLWithPath: a[1]), options: nil)
scene.background.contents = NSColor.clear
var tiles: [(Int, SCNNode)] = []
var splats: [SCNNode] = []
scene.rootNode.enumerateHierarchy { node, _ in
    guard let name = node.name else { return }
    node.geometry?.materials.forEach { $0.lightingModel = .constant; $0.isDoubleSided = true }
    if name.hasPrefix("tile_"), node.geometry != nil {
        let n = Int(name.split(separator: "_")[1]) ?? 0
        tiles.append((n, node))
    } else if name.hasPrefix("splats_") {
        splats.append(node)
    }
}
tiles.sort { $0.0 < $1.0 }
let tileIDs = Array(Set(tiles.map { $0.0 })).sorted()
let rank = Dictionary(uniqueKeysWithValues: tileIDs.enumerated().map { ($1, $0) })
print("tiles", tileIDs.count, "meshes", tiles.count, "splat nodes", splats.count)

let (minB, maxB) = scene.rootNode.boundingBox
let span = Double(max(maxB.x - minB.x, maxB.z - minB.z))
let target = SCNVector3((minB.x + maxB.x) / 2, 8, (minB.z + maxB.z) / 2)
let camera = SCNNode()
camera.camera = SCNCamera()
camera.camera!.fieldOfView = 38
camera.camera!.zNear = 1
camera.camera!.zFar = span * 20
scene.rootNode.addChildNode(camera)
let renderer = SCNRenderer(device: MTLCreateSystemDefaultDevice(), options: nil)
renderer.scene = scene
renderer.pointOfView = camera
try FileManager.default.createDirectory(atPath: a[2], withIntermediateDirectories: true)

func easeOutCubic(_ t: Double) -> Double { 1 - pow(1 - t, 3) }
func easeInOut(_ t: Double) -> Double { t < 0.5 ? 4 * t * t * t : 1 - pow(-2 * t + 2, 3) / 2 }
let base = Dictionary(uniqueKeysWithValues: tiles.map { (ObjectIdentifier($0.1), $0.1.position) })
let count = Double(max(tileIDs.count, 1))
for f in 0..<frames {
    let fd = Double(f)
    for (n, node) in tiles {
        // Tiles land evenly over [0, tilesEnd] in export order; each drops 10 m in 6 frames.
        let appear = tilesEnd * Double(rank[n]!) / count
        let p = base[ObjectIdentifier(node)]!
        if fd < appear {
            node.isHidden = true
        } else {
            node.isHidden = false
            let t = min(1, (fd - appear) / 6)
            node.position = SCNVector3(p.x, p.y + CGFloat(10 * (1 - easeOutCubic(t))), p.z)
        }
    }
    let s = max(0, min(1, (fd - splatStart) / max(splatEnd - splatStart, 1)))
    for node in splats { node.isHidden = s <= 0; node.opacity = CGFloat(easeInOut(s)) }

    let t = fd / Double(max(frames - 1, 1))
    let az = (startDeg + sweepDeg * t) * .pi / 180
    let el = (elevDeg - 4 * t) * .pi / 180
    let d = span * distFactor * (1.1 - 0.18 * easeInOut(t))
    camera.position = SCNVector3(target.x + CGFloat(d * cos(el) * cos(az)), target.y + CGFloat(d * sin(el)), target.z + CGFloat(d * cos(el) * sin(az)))
    camera.look(at: target)
    let image = renderer.snapshot(atTime: 0, with: CGSize(width: 1920, height: 1080), antialiasingMode: .multisampling4X)
    let rep = NSBitmapImageRep(data: image.tiffRepresentation!)!
    try rep.representation(using: .png, properties: [:])!.write(to: URL(fileURLWithPath: String(format: "%@/f%04d.png", a[2], f)))
}
print("rendered", frames)
