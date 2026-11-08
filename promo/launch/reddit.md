# Reddit-Entwürfe

Jeder Sub hat eigene Regeln zur Eigenwerbung. Vor dem Posten die Sidebar lesen.
Video direkt hochladen (nicht nur als Link), Repo-Link in den ersten Kommentar
oder in den Text, je nach Sub. Alle Zahlen sind gemessen (8.11.2026).

---

## r/gamedev (Flair: "Feedback Request" / "Tool")

**Titel:** I built a CLI that turns real places into game-ready GLB models (Apple's 3D city data, textured, real scale)

I've been building real-world levels and got tired of hand-modelling city blocks
that already exist as photogrammetry. So I wrote **Orbis**:

```
orbis export --place "Space Needle, Seattle" --size 250
```

You get one `.glb` with embedded textures: metres, +Y up, origin at the centre
of the area. It drops straight into Unity (glTFast), Unreal, Godot or Blender.
It also writes USDZ for Xcode/RealityKit, OBJ, and a Minecraft world if you
want blocks.

- ~165k triangles and ~57 MP of texture for a 250 × 250 m block at max zoom
- runs locally on a Mac. Windows/Linux work after a one-time config
- CLI, a Go library, and a small browser UI with coverage map and a drive-around viewer
- it decodes Apple's newer 3D format too, including the soft splat trees (~250k splats for a 200 m block in Berlin), as GLB, USDZ or Gaussian-splat PLY

Caveats, honestly: it uses Apple's undocumented map service. The data is
Apple's, so use it for prototyping and research on your own machine, not to
ship in a commercial game. Coverage is per city.

Repo + video: https://github.com/Henkek221/orbis

I'd love feedback from people building real-world maps: what format or engine
workflow should come next? Chunked tiles for streaming? Collision meshes?

---

## r/Unity3D / r/unrealengine / r/godot

**Titel (Unity):** Real-world city blocks as one GLB: free CLI, imports with glTFast at real scale
**Titel (Unreal):** Exporting real places (photogrammetry, textured) into Unreal with one command
**Titel (Godot):** Drop real places into Godot 4: one textured GLB per area, metres, Y up

Text: like r/gamedev, but the first screenshot/GIF shows the import into that
engine. Start with the engine-specific workflow in 2–3 lines, e.g. Godot:
"Drag the .glb into the FileSystem dock and instance it. Materials are unlit
photo textures, so it looks right without lighting."

---

## r/blender

**Titel:** Textured photogrammetry of real places straight into Blender (GLB/OBJ, real scale) – free CLI

Focus: retopo/clean-up workflows, it imports at real scale, materials stay unlit.
Show a Blender viewport screenshot.

---

## r/threejs

**Titel:** One command → a real city block as GLB, rendered in three.js

Focus: file size, a three.js demo link (CodeSandbox with a small export),
`KHR_materials_unlit`.

---

## r/visionosdev / r/iOSProgramming

**Titel:** Real places as USDZ for RealityKit / Quick Look, exported with one command on your Mac

Focus: `--format usdz` passes Apple's `usdchecker --arkit`. Show a Quick Look or
Reality Composer screenshot. Mention the Go library for build pipelines.

---

## r/photogrammetry / r/gis

**Titel:** Reverse-engineered Apple's new 3D map tiles (R3D v2): mesh + "triangle splats", and an exporter

Technical post: two data generations (C3M vs R3D), 56 vs ~1,380 regions, style
98/100, TRICO mesh, triangle splats, HEIC atlases. Credit the earlier research
(retroplasma, nmt3325). This post can be the most valuable one; keep it facts-first.
