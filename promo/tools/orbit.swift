// Renders an orbiting camera flight around a model to a transparent PNG sequence.
// usage: orbit <model> <outdir> <frames> [startDeg] [sweepDeg] [elevDeg] [dist] [width] [height]
import AppKit
import SceneKit

let a = CommandLine.arguments
let frames = Int(a[3])!
let startDeg = a.count > 4 ? Double(a[4])! : 20
let sweepDeg = a.count > 5 ? Double(a[5])! : 110
let elevDeg = a.count > 6 ? Double(a[6])! : 32
let distFactor = a.count > 7 ? Double(a[7])! : 0.95
let width = a.count > 8 ? Int(a[8])! : 1920
let height = a.count > 9 ? Int(a[9])! : 1080

let scene = try SCNScene(url: URL(fileURLWithPath: a[1]), options: nil)
scene.background.contents = NSColor.clear
let hideSplats = ProcessInfo.processInfo.environment["HIDE_SPLATS"] != nil
scene.rootNode.enumerateHierarchy { node, _ in
    node.geometry?.materials.forEach { $0.lightingModel = .constant; $0.isDoubleSided = true }
    if hideSplats, node.name?.hasPrefix("splats_") == true { node.isHidden = true }
}
let (minB, maxB) = scene.rootNode.boundingBox
let span = Double(max(maxB.x - minB.x, maxB.z - minB.z))
let tallest = Double(maxB.y)
let target = SCNVector3((minB.x + maxB.x) / 2, CGFloat(max(tallest * 0.3, 10)), (minB.z + maxB.z) / 2)

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

func ease(_ t: Double) -> Double { t < 0.5 ? 2 * t * t : 1 - pow(-2 * t + 2, 2) / 2 }
for f in 0..<frames {
    let t = Double(f) / Double(max(frames - 1, 1))
    let e = 0.15 * t + 0.85 * ease(t)
    let az = (startDeg + sweepDeg * e) * .pi / 180
    let el = (elevDeg - 6 * e) * .pi / 180
    let d = span * distFactor * (1.08 - 0.16 * e)
    camera.position = SCNVector3(target.x + CGFloat(d * cos(el) * cos(az)),
                                 target.y + CGFloat(d * sin(el)),
                                 target.z + CGFloat(d * cos(el) * sin(az)))
    camera.look(at: target)
    let image = renderer.snapshot(atTime: 0, with: CGSize(width: width, height: height), antialiasingMode: .multisampling4X)
    let rep = NSBitmapImageRep(data: image.tiffRepresentation!)!
    try rep.representation(using: .png, properties: [:])!.write(to: URL(fileURLWithPath: String(format: "%@/f%04d.png", a[2], f)))
}
print("rendered", frames, "frames to", a[2])
