# X / Twitter-Thread

Video im ersten Post (16:9, mit Untertiteln, funktioniert ohne Ton). Link erst im
zweiten Post, sonst drosselt X die Reichweite. Alle Zahlen sind gemessen (8.11.2026).

---

**1/** (mit Hero-Video)
You can now turn real places into game-ready 3D models.

One command. One file. Real scale.

`orbis export --place "Space Needle"`

**2/**
It's open on GitHub: Orbis, a CLI + Go library + small browser UI.

→ GLB for Unity, Unreal, Godot, Blender, three.js
→ USDZ for Xcode, RealityKit, visionOS
→ OBJ, and yes, Minecraft worlds

github.com/Henkek221/orbis

**3/** (Screenshot: dense geometry close-up)
A 250 × 250 m block at max zoom:
~165,000 triangles
~57 megapixels of texture
metres, Y up, centred at the origin. It imports in place.

**4/** (Clip: trees grow out of the flat map)
The interesting part: Apple's new 3D data is a different format.
Trees aren't meshes anymore. They're soft "triangle splats", ~250k per city block.

Orbis decodes them (GLB, USDZ, or real Gaussian splats as .ply), which takes it
from 56 classic cities to 1,400+ regions. Tested: Berlin, New York, Tokyo, Sydney, Paris.

**5/** (Screenshot: terminal coverage map)
`orbis coverage` shows you first whether an area has 3D data, as a little map
in your terminal.

**6/**
Runs locally on a Mac. On Windows/Linux, import a config file once.
No account, no upload.

The data is Apple's, so use it for prototyping and research, not to ship.

**7/**
Built on years of reverse-engineering by others (credits in the README).
What should come next: streaming chunks, collision meshes, or a Blender add-on?
