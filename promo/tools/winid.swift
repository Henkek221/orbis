// Prints "<windowID> <x> <y> <w> <h> <title>" for the frontmost large window of an app.
import CoreGraphics
import Foundation

let owner = CommandLine.arguments[1]
let list = CGWindowListCopyWindowInfo([.optionOnScreenOnly, .excludeDesktopElements], kCGNullWindowID) as! [[String: Any]]
for w in list {
    guard (w[kCGWindowOwnerName as String] as? String) == owner,
          (w[kCGWindowLayer as String] as? Int) == 0,
          let b = w[kCGWindowBounds as String] as? [String: Double],
          (b["Width"] ?? 0) > 400 else { continue }
    let id = w[kCGWindowNumber as String] as! Int
    let title = w[kCGWindowName as String] as? String ?? ""
    print(id, Int(b["X"]!), Int(b["Y"]!), Int(b["Width"]!), Int(b["Height"]!), title)
    break
}
