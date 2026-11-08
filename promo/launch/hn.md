# Show HN

**Titel:** Show HN: Orbis – export real places as textured 3D models (GLB/USDZ) from Apple's map data

**URL:** https://github.com/Henkek221/orbis

**Erster Kommentar (vom Autor, direkt nach dem Posten):**

Hi HN, I built Orbis because I wanted real city blocks in a game prototype
without modelling them by hand.

It talks to the same tile service Apple Maps uses for 3D cities. It walks the
octree index, downloads the mesh tiles, decodes them, and writes a single GLB
(or USDZ/OBJ) in a local metric frame. Everything runs locally: there is a CLI,
a Go library and a small browser UI.

Some things I learned along the way:

- Apple serves the resource manifest per client. The old macOS 11 identity
  that earlier tools used only gets 50 of 98 tile styles.
- Apple now runs two data generations. 56 "classic" regions still use C3M.
  ~1,380 newer regions (Berlin, NYC, London…) only serve a new format, R3D v2
  (style 98), with an LZMA-compressed occupancy index (style 100).
- R3D carries a mesh (Edgebreaker-style topology, predicted and
  Huffman-coded streams) plus "triangle splats": 3 corners, opacity, sRGB
  albedo, roughness, an octahedral normal and a specular weight. Apple sorts
  them back to front and blends them softly; that is what makes the new trees
  look soft. Orbis writes them as soft triangles in GLB (an alpha falloff
  texture) or as Gaussians in a 3DGS .ply.
- Our decoder matches the classic C3M geometry within one quantisation step
  where a region exists in both formats, and Berlin's TV tower comes out at
  its real 368 m.
- In regions that exist in both formats, the textures are byte-identical: the
  new format adds coverage and splats, not resolution.

Caveats: this uses an undocumented service, and the data is Apple's and subject
to their terms, so exports are for research and prototyping, not for shipping.
The Orbis code itself is Apache-2.0: build on it freely, the one condition is
crediting the author. A few files inherited from earlier reverse-engineering projects
have no license and are excluded (listed in LICENSING.md).

Happy to answer questions about the formats.
