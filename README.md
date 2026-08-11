# Mappy

**Select a place, export its available 3D map mesh, and explore the result in your browser.**

Mappy turns available 3D map data into a local `OBJ` package. The interface runs
in your browser while all processing happens on your computer. There is no
account, upload, or separate 3D application required.

![Select an area and export a 3D model with Mappy](docs/images/mappy-export.gif)

> [!IMPORTANT]
> Mappy is an experimental research project. Coverage is incomplete, individual
> tiles can be missing, and exported map data may be subject to third-party terms.

## What it does

- searches for places and addresses
- shows known 3D coverage directly on the map
- lets you draw a rectangular export area
- suggests useful export names based on place and date
- exports `OBJ`, `MTL`, and texture files
- exports the same area as a playable **Minecraft world**
- opens existing OBJ models by drag and drop
- lets you walk or drive through the loaded model
- keeps completed exports available after a restart

## Minecraft export

Mappy can write the selected area as a Minecraft Java Edition save. The buildings
are not procedurally generated: every block comes from a triangle of the scanned
mesh, and its material is chosen from the real photo texture that the triangle
carries. Real footprints, real heights, real colours.

How the voxelizer works:

1. **Supersampling.** Every triangle is sampled about three times per block
   along its edges, so no shell stays open.
2. **Colour averaging.** Each voxel collects the colours of all samples that
   land in it and keeps the average of the actual surface area, not one random
   winning pixel.
3. **Oklab matching.** Block colours are matched in Oklab, a perceptually
   uniform space, so dark asphalt stays grey instead of drifting into blue.
4. **Ordered dithering.** Colours that sit between two palette blocks are
   dithered across neighbouring voxels with a 4x4 Bayer pattern shifted per
   height layer. That recovers tones the block palette does not contain.
5. **Despeckling.** Voxels with almost no neighbours are dropped: those are
   flyaway triangles from the photogrammetry, never architecture.
6. **Solidifying.** Air is flood filled from outside the bounding box. Whatever
   air is never reached sits inside a building or under the ground and becomes
   stone, dirt or deepslate. Buildings end up solid instead of paper shells.
7. **Surface rules.** Green voxels with open sky above become real grass blocks.

- pick **Minecraft world** in step 3 before exporting
- block size is selectable: `1 m`, `0.5 m` for four times the detail, or `2 m`
  for a quick draft
- the world is a void superflat, so only the scanned area contains blocks
- copy the `…-minecraft` folder into your saves directory and open it in
  Minecraft Java Edition 1.20.1 or newer

Newer Apple textures arrive as HEIC, which Go cannot decode on its own. Install
[ImageMagick](https://imagemagick.org/) or [ffmpeg](https://ffmpeg.org/) and
Mappy uses it automatically for block colours and for JPEG textures in the OBJ
export. Without a converter the export still works, but affected surfaces fall
back to plain grey.

## Explore the result

Open an export directly in the built-in viewer, move through the scanned area,
or switch to driving mode. Existing OBJ models can also be opened by drag and
drop.

![Explore an exported 3D model in the Mappy viewer](docs/images/mappy-viewer.gif)

## Download and run

Open [Releases](https://github.com/henrkk123/mappy/releases/latest), download the
ZIP for your computer, and extract it. No installer is required.

| System | Download |
| --- | --- |
| macOS, Intel or Apple Silicon | `mappy-…-mac.zip` |
| Windows 10/11 | `mappy-…-windows.zip` |
| Linux, Intel/AMD | `mappy-…-linux-x64.zip` |
| Linux, ARM64 | `mappy-…-linux-arm64.zip` |

Every package includes **HIER STARTEN.txt** (a short German quick-start file).
Windows and Linux packages also include a **MAPPY STARTEN** launcher that can be
opened directly.

### macOS

1. Right-click `Mappy.app` and choose **Open**.
2. Confirm **Open** once more.
3. Mappy opens in your default browser.

Mappy normally discovers the required static Apple client value from the local
macOS system cache. If that is not possible, the app displays a one-time local
`config.json` import instead.

### Windows

1. Open `MAPPY STARTEN.cmd`.
2. If Windows warns you, choose **More info** and **Run anyway**.
3. Keep the Mappy terminal window open while using the app.

### Linux

Open `MAPPY STARTEN.sh`. The launcher sets the executable permission and starts
Mappy. If your file manager opens the script as text, run:

```bash
./MAPPY\ STARTEN.sh
```

### One-time configuration on Windows and Linux

Windows and Linux do not contain the Apple system cache used by the automatic
macOS setup. The value Mappy needs is not a personal API key you have to
register for: it is a static client constant that Apple ships inside its own
`GeoServices` library. There are two ways to get it:

- drop a ready-made `config.json` into the setup window, or
- point Mappy at a local Apple `GeoServices` file in the setup window. Mappy
  scans the file itself, extracts the value, verifies it against the 3D map
  service, and stores it locally.

`scripts/get_config.sh` automates the download of that file from Apple's public
simulator SDK (no Apple account, but roughly 2 GB of traffic) and needs `bash`,
`7z` and `strings`, so on Windows run it in WSL or Git Bash.

Never commit, upload, or include `config.json` in an issue.

## Workflow

1. Search for a place.
2. Enable 3D coverage and mark a small area inside a green region.
3. Keep or edit the suggested export name.
4. Choose the formats: 3D model, Minecraft world, or both.
5. Press **Export** and wait for processing to finish.
6. Open the result in the built-in viewer, or copy the Minecraft folder into
   your saves.
7. Explore with the camera, walking mode, or driving mode.

You can also open the viewer directly and drop an existing OBJ together with its
MTL and texture files into the window.

## Current limitations

- **Coverage is not a guarantee.** Green means that Mappy found evidence of 3D
  data. Individual mesh tiles can still be unavailable or empty.
- **Start small.** An area around 150–300 metres wide is a sensible first test.
  Large exports take much longer and need more memory.
- **Source quality sets the ceiling.** The viewer improves rendering and camera
  movement, but it cannot reconstruct missing buildings or road surfaces.
- **OBJ is an interchange format.** Blender and many 3D tools can open it
  directly. BeamNG still needs game-specific collision meshes, road materials,
  spawn points, and a level layout.
- **Local only.** Mappy listens on `localhost` and does not upload your exports.
- **Minecraft height limit.** A world holds 384 blocks of height. At `0.5 m` per
  block anything above roughly 190 metres is cut off; use `1 m` for skylines.
- **Dataset freshness.** Apple publishes a dataset version per flyover region.
  Mappy refetches that manifest every few hours, so new scans appear without
  clearing `cache/`. Deleting `cache/` always forces a full refresh.

## Test feedback

Please open an issue and include:

- operating system and version
- searched place
- approximate selected area
- exact visible error
- a screenshot, if useful

Do not include `config.json`, tokens, or private credentials. A short manual test
plan is available in [docs/TEST-CHECKLISTE.md](docs/TEST-CHECKLISTE.md).

## Run from source

Go 1.24 or newer is required.

On macOS:

```bash
go run ./cmd/export-ui
```

On other systems, create a local `config.json` from the example first:

```bash
cp config.example.json config.json
# Replace the placeholder locally. Never commit this file.
go run ./cmd/export-ui
```

Mappy opens `http://localhost:8787` automatically. Run tests and build release
packages with:

```bash
go test -mod=vendor ./...
./scripts/build_mappy_releases.sh dev
```

Generated ZIP files are written to `dist/mappy/releases/`.

## Credits

Mappy is maintained as a standalone repository, not a GitHub fork, but it builds
on substantial prior work:

- [retroplasma/flyover-reverse-engineering](https://github.com/retroplasma/flyover-reverse-engineering)
  for the original Flyover format, tile, and decoder research
- [christiangda/flyover-reverse-engineering](https://github.com/christiangda/flyover-reverse-engineering)
  for updates to newer Go and Protobuf versions
- [Leaflet](https://leafletjs.com/) and
  [OpenStreetMap contributors](https://www.openstreetmap.org/copyright) for the map
- Khronos Group's CC0 [Toy Car](cmd/export-ui/static/assets/TOY-CAR-LICENSE.txt)
  used in the viewer

See [CREDITS.md](CREDITS.md) for details.

## Licensing and data terms

This repository is not developed, endorsed, or supported by Apple. Apple, Apple
Maps, and Flyover are trademarks or services of Apple Inc. Map and 3D data may
be subject to additional terms.

The inherited upstream repositories currently do not provide a clear license
for all copied code. That means this repository is publicly readable, but it
must not yet be presented as OSI-approved open source or as commercially
redistributable software. Resolve the upstream code license and the applicable
data terms before charging for, sublicensing, or broadly redistributing Mappy.
