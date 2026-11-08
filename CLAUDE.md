# Orbis

Select an area, pull Apple's 3D map mesh for it, export it as GLB, USD(Z), OBJ
and/or a Minecraft world. One Go binary, `cmd/orbis`, is a CLI (`export`,
`coverage`, `search`, `regions`, `config`) and serves the browser UI on
`localhost:8787` (`orbis ui`). All logic lives in the library `pkg/orbis`.
Nothing is uploaded, there is no account.

## Working agreements

- **Do not write tests and do not run tests.** The maintainer verifies by
  building and using the app. No `*_test.go` for new code. The two inherited
  test files may stay.
- **Do not install toolchains** to verify work. Write carefully, then say what
  the maintainer should run.
- Dependencies are vendored. Always build with `-mod=vendor` and never add a
  module that a few lines of stdlib can replace.
- Never commit `config.json`, tokens, cache or export output. `.gitignore`
  covers them.

```bash
go build -mod=vendor ./... && go run -mod=vendor ./cmd/orbis ui
go run -mod=vendor ./cmd/orbis export --place "Arc de Triomphe" --size 150
```

## How a tile becomes a file

```
config.json (tokenP1)
  └─ mps.Init            → ResourceManifest (protobuf, cached 1 h)
      └─ fly.GetAltitudeManifest → per region: dataset version + radius (cached 6 h)
          └─ C3MM octree index   → does tile x/y/z/h exist?
              └─ C3M tile        → mesh + JPEG/HEIC texture
                  └─ exp.Export  → OBJ + MTL + textures
                  └─ mc.World    → Anvil world save
```

The whole scan lives in `Client.Export` in
[pkg/orbis/export.go](pkg/orbis/export.go). Tiles are fetched in parallel
and funnelled through one channel into a single consumer goroutine, so exporters
are **not** called concurrently and need no locking.

`multiExport` fans that one tile stream out to every selected format. Adding a
format means implementing `exp.Export` (`Next(c3m.C3M, string) error` and
`Close() error`) and appending it there; library users can pass their own via
`Request.Writers`.

## Package map

| Path | Role |
| --- | --- |
| `cmd/orbis` | The binary users run: CLI subcommands, `ui` starts the server. No args from a terminal prints help; launched without a terminal (Finder, Explorer) it starts the UI. |
| `pkg/orbis` | The toolkit: `Open`/`Connect`, `Export`, `Coverage`, `Region(s)`, geocoding, config helpers, bounds and naming. Server and CLI only call this. |
| `internal/server` | HTTP API, job state, embedded static UI (`static/`). |
| `cmd/export-obj`, `cmd/auth`, `cmd/parse-*` | Low-level tools inherited from upstream. |
| `pkg/mps` | Apple map service: auth, session, ResourceManifest, cache. |
| `pkg/fly` | Flyover layer: altitude manifest (region → version), C3M and C3MM parsers. |
| `pkg/fly/exp` | OBJ, GLB (`glb.go`) and USD (`usd.go`) writers. GLB/USD share the local frame: origin = export centre at first-tile height, +X east, +Y up, +Z south. |
| `pkg/fly/r3d` | Apple's new R3D v2 format (style 98) and its C3MM v2 index (style 100): container, frame, textures, index. TRICO geometry streams are not decoded yet. |
| `pkg/mc` | Minecraft world writer. Own package, no upstream code. |
| `pkg/oth` | Small shared helpers, including the HEIC converter lookup. |

## pkg/mc, the voxelizer

Seven stages, all in [pkg/mc/voxel.go](pkg/mc/voxel.go) unless noted:

1. **Supersample** each triangle about 3 samples per block along its edges.
2. **Accumulate** colour per voxel and average it, so a voxel gets the colour of
   the surface area inside it, not of one arbitrary winning pixel.
3. **Match in Oklab** ([color.go](pkg/mc/color.go)), not RGB, via a 32³ lookup
   table built once.
4. **Dither** between the two nearest palette blocks with a 4x4 Bayer matrix
   shifted per height layer, which recovers tones the palette lacks.
5. **Despeckle**: drop voxels with fewer than two solid neighbours.
6. **Solidify**: flood fill air inwards from the padded bounding box. Air that is
   never reached is enclosed, so it becomes stone, dirt or deepslate. This is why
   buildings are solid instead of hollow shells.
7. **Surface rules**: green voxels with nothing solid above become grass blocks.

Stages 5 to 7 need a dense byte per cell. Above `maxSolidVolume` the builder
falls back to a surface shell with filler underneath and says so in the log.

The save format is hand written: [nbt.go](pkg/mc/nbt.go) is a minimal NBT writer,
[chunk.go](pkg/mc/chunk.go) packs block states and writes `level.dat`,
[world.go](pkg/mc/world.go) writes Anvil region files. `dataVersion` is 1.20.1 on
purpose: Minecraft upgrades older saves forward, so a lower number opens in more
versions.

## Non-obvious constraints

- **Manifest identity.** Apple serves the resource manifest per client. The
  default URL claims macOS 27; the old macOS 11 identity got only 50 of 98
  styles and none of the new 3D formats. Keep it current.
- **Two data generations.** ~56 named regions still serve classic C3M (styles
  14/15). ~1,380 `Reg_z9_*` regions (version 4000+, e.g. Berlin, NYC) answer
  404/empty there and only serve R3D (style 98, `v=828` from the manifest, `h`
  required; index style 100). Their coverage shows as "unknown".
- **Retries.** `pkg/web` retries network errors, 5xx and 429 three times with
  backoff; one dropped TLS handshake used to fail a whole export.

- **Coordinates.** OBJ and Minecraft both localise to east-north-up metres around
  the first tile's ECEF translation. Minecraft is X east, **Z south**, Y up, so
  north is negated.
- **Texture V is flipped.** OBJ viewers sample with V running bottom up, so
  sampling code uses `1 - v`.
- **HEIC.** Newer Apple textures are HEIC and Go cannot decode them without cgo.
  `oth.ConvertHEIC` shells out to `magick`, `heif-convert`, `ffmpeg` or `sips`.
  Without one of those, OBJ keeps the `.heic` file and Minecraft colours fall
  back to grey. Do not add a cgo HEIC dependency to fix this.
- **Altitude manifest TTL.** It carries the per-region dataset version. It used
  to be cached forever, which pinned exports to whatever Apple published on first
  launch. It now expires after 6 h; do not remove that.
- **Height layers.** `tryH` is how many octree height levels are scanned. Too low
  silently cuts tall buildings. The UI default is 24.
- **Minecraft height.** A world has 384 blocks of height. At 0.5 m per block
  anything above ~190 m is clipped, and the log says how many voxels were lost.
- **Token discovery is macOS only.** `discover_darwin.go` scrapes the GeoServices
  binary or the dyld shared cache. On other systems the setup gate accepts a
  `config.json` or a path to a GeoServices file, which
  `POST /api/setup/scan` scans with `config.ScanTokenP1`. The companion tool
  `kvasir` produces that `config.json` from an installed simulator runtime.

## Frontend

Plain HTML, CSS and JS in `internal/server/static`, embedded with `go:embed`. No
build step, no framework. Leaflet and Lucide come from unpkg.

The design language is Apple-like: system greys (`#f5f5f7` paper, `#1d1d1f`
ink), the SF system font stack, system blue for actions and selection, system
green only for 3D coverage, system orange for warnings, restrained radii and
soft neutral shadows. All tokens live in `:root` of
[styles.css](internal/server/static/styles.css). Use the variables, never raw hex.

The OpenStreetMap raster is pulled into the palette with a filter on
`.leaflet-tile-pane` only. The overlay pane stays unfiltered so coverage and
selection keep full accent saturation. The filter values were measured against
real OSM tile colours; if you change them, measure again rather than guess.

UI copy is English. The one-time setup gate and the packaging files are German,
because they are aimed at first-time local setup. Keep that split.

## Provenance

Built on `retroplasma/flyover-reverse-engineering` and `christiangda`'s fork; the
module path is now `github.com/Henkek221/orbis`. The upstream licence is unresolved, so the repo is readable but must not be presented
as OSI open source or redistributed commercially. See README and CREDITS.
