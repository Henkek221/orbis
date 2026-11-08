// Package mc turns Apple 3D map meshes into a playable Minecraft world.
//
// Nothing here is procedurally generated: every block comes from a triangle of
// the scanned mesh, coloured by the real photo texture that the triangle
// carries. Buildings keep their real footprint, height and colour.
package mc

import (
	"fmt"
	"math"
	"os"
	"path/filepath"
	"runtime/debug"
	"sort"
	"time"

	"github.com/Henkek221/mappy/pkg/fly/c3m"
	"github.com/Henkek221/mappy/pkg/oth"
)

const (
	worldMinY = -64
	worldMaxY = 319

	// 1.20.1. Newer Minecraft versions upgrade the save format on load, so an
	// older data version is the widest compatible choice.
	dataVersion = 3465
	versionName = "1.20.1"
)

// Options control voxel resolution and where the export sits in the world.
type Options struct {
	// Name is shown in the Minecraft world list.
	Name string
	// BlockSize is metres per block. 1 keeps real scale, 0.5 doubles detail.
	BlockSize float64
	// GroundDepth is the filler depth used only when a selection is too large
	// for the solid fill and falls back to a surface shell.
	GroundDepth int
	// BaseY is the world height the lowest voxel is placed at.
	BaseY int
	Logf  func(string)
}

// World collects voxels while tiles stream in and writes the save on Close.
type World struct {
	dir       string
	opt       Options
	builder   *builder
	sinLat    float64
	cosLat    float64
	sinLon    float64
	cosLon    float64
	origin    [3]float64
	originSet bool
	tiles     int
}

// New prepares a world directory. Latitude and longitude are the centre of the
// export and define the local east-north-up frame.
func New(dir string, latitude, longitude float64, opt Options) (*World, error) {
	if opt.BlockSize <= 0 {
		opt.BlockSize = 1
	}
	if opt.BlockSize < 0.25 {
		opt.BlockSize = 0.25
	}
	if opt.GroundDepth <= 0 {
		opt.GroundDepth = 4
	}
	if opt.BaseY == 0 {
		opt.BaseY = -60
	}
	if opt.BaseY < worldMinY {
		opt.BaseY = worldMinY
	}
	if opt.Name == "" {
		opt.Name = "Mappy export"
	}
	if err := os.MkdirAll(filepath.Join(dir, "region"), 0o755); err != nil {
		return nil, err
	}
	latRad, lonRad := latitude*math.Pi/180, longitude*math.Pi/180
	return &World{
		dir:     dir,
		opt:     opt,
		builder: newBuilder(),
		sinLat:  math.Sin(latRad),
		cosLat:  math.Cos(latRad),
		sinLon:  math.Sin(lonRad),
		cosLon:  math.Cos(lonRad),
	}, nil
}

func (w *World) logf(format string, args ...any) {
	if w.opt.Logf != nil {
		w.opt.Logf(fmt.Sprintf(format, args...))
	}
}

// Next voxelizes one tile. It matches the OBJ exporter signature so both
// formats can consume the same tile stream.
func (w *World) Next(tile c3m.C3M, _ string) (err error) {
	defer func() {
		if problem := recover(); problem != nil {
			err = fmt.Errorf("minecraft voxelizer: %v\n%s", problem, debug.Stack())
		}
	}()

	if !w.originSet {
		w.origin = tile.Header.Translation
		w.originSet = true
	}

	textures := make([]*texture, len(tile.Materials))
	for index, material := range tile.Materials {
		textures[index] = loadTexture(material)
	}

	for _, mesh := range tile.Meshes {
		positions := make([][3]float64, len(mesh.Vertices))
		for index, vertex := range mesh.Vertices {
			positions[index] = w.blockPosition(tile, vertex)
		}
		for _, group := range mesh.Groups {
			var tex *texture
			if group.Material >= 0 && group.Material < len(textures) {
				tex = textures[group.Material]
			}
			if tex == nil {
				w.builder.untextured++
			}
			for _, face := range group.Faces {
				a, b, c := int(face.A), int(face.B), int(face.C)
				if a < 0 || b < 0 || c < 0 || a >= len(positions) || b >= len(positions) || c >= len(positions) {
					continue
				}
				w.builder.triangle(
					positions[a], positions[b], positions[c],
					[2]float32{mesh.Vertices[a].U, mesh.Vertices[a].V},
					[2]float32{mesh.Vertices[b].U, mesh.Vertices[b].V},
					[2]float32{mesh.Vertices[c].U, mesh.Vertices[c].V},
					tex,
				)
			}
		}
	}
	w.tiles++
	return nil
}

// blockPosition converts a tile vertex to Minecraft block space: X east, Z
// south, Y up, still floating point so triangles can be sampled smoothly.
func (w *World) blockPosition(tile c3m.C3M, vertex c3m.Vertex) [3]float64 {
	x, y, z := float64(vertex.X), float64(vertex.Y), float64(vertex.Z)
	r := tile.Header.Rotation
	ecefX := r[0]*x + r[1]*y + r[2]*z + tile.Header.Translation[0]
	ecefY := r[3]*x + r[4]*y + r[5]*z + tile.Header.Translation[1]
	ecefZ := r[6]*x + r[7]*y + r[8]*z + tile.Header.Translation[2]

	dx, dy, dz := ecefX-w.origin[0], ecefY-w.origin[1], ecefZ-w.origin[2]
	east := -w.sinLon*dx + w.cosLon*dy
	north := -w.sinLat*w.cosLon*dx - w.sinLat*w.sinLon*dy + w.cosLat*dz
	up := w.cosLat*w.cosLon*dx + w.cosLat*w.sinLon*dy + w.sinLat*dz

	scale := 1 / w.opt.BlockSize
	return [3]float64{east * scale, up * scale, -north * scale}
}

// Close bakes the voxels into blocks and writes the Anvil save.
func (w *World) Close() error {
	if w.builder.untextured > 0 && oth.HEICConverter() == "" {
		w.logf("minecraft: %d material(s) had no readable texture; install ImageMagick or ffmpeg for HEIC colours", w.builder.untextured)
	}

	g, err := w.builder.build(w.opt, w.logf)
	if err != nil {
		return err
	}
	w.builder = nil

	if err := g.writeRegions(filepath.Join(w.dir, "region")); err != nil {
		return err
	}
	spawnY := int32(w.opt.BaseY) + 2
	if top, ok := g.highest(0, 0); ok {
		spawnY = top + 2
	}
	if err := w.writeLevel(spawnY); err != nil {
		return err
	}
	if err := w.writeReadme(); err != nil {
		return err
	}

	w.logf("minecraft: %d blocks in %d chunks from %d tiles at %.2f m per block",
		g.count, len(g.chunks), w.tiles, w.opt.BlockSize)
	return nil
}

// Directory is where the world save was written.
func (w *World) Directory() string {
	return w.dir
}

type chunk struct {
	sections map[int32]*[4096]uint16
}

type grid struct {
	chunks map[[2]int32]*chunk
	count  int
}

func (g *grid) section(x, y, z int32) (*[4096]uint16, int) {
	key := [2]int32{x >> 4, z >> 4}
	c := g.chunks[key]
	if c == nil {
		c = &chunk{sections: map[int32]*[4096]uint16{}}
		g.chunks[key] = c
	}
	blocks := c.sections[y>>4]
	if blocks == nil {
		blocks = new([4096]uint16)
		c.sections[y>>4] = blocks
	}
	return blocks, int(y&15)*256 + int(z&15)*16 + int(x&15)
}

func (g *grid) set(x, y, z int32, block uint16) {
	if y < worldMinY || y > worldMaxY {
		return
	}
	blocks, index := g.section(x, y, z)
	if blocks[index] == airBlock {
		g.count++
	}
	blocks[index] = block
}

func (g *grid) fillEmpty(x, y, z int32, block uint16) {
	if y < worldMinY || y > worldMaxY {
		return
	}
	blocks, index := g.section(x, y, z)
	if blocks[index] != airBlock {
		return
	}
	blocks[index] = block
	g.count++
}

func (g *grid) get(x, y, z int32) uint16 {
	c := g.chunks[[2]int32{x >> 4, z >> 4}]
	if c == nil {
		return airBlock
	}
	blocks := c.sections[y>>4]
	if blocks == nil {
		return airBlock
	}
	return blocks[int(y&15)*256+int(z&15)*16+int(x&15)]
}

func (g *grid) highest(x, z int32) (int32, bool) {
	for y := int32(worldMaxY); y >= worldMinY; y-- {
		if g.get(x, y, z) != airBlock {
			return y, true
		}
	}
	return 0, false
}

func (g *grid) writeRegions(dir string) error {
	regions := map[[2]int32][][2]int32{}
	for key := range g.chunks {
		region := [2]int32{key[0] >> 5, key[1] >> 5}
		regions[region] = append(regions[region], key)
	}
	for region, keys := range regions {
		sort.Slice(keys, func(i, j int) bool {
			if keys[i][1] != keys[j][1] {
				return keys[i][1] < keys[j][1]
			}
			return keys[i][0] < keys[j][0]
		})
		name := fmt.Sprintf("r.%d.%d.mca", region[0], region[1])
		if err := g.writeRegion(filepath.Join(dir, name), keys); err != nil {
			return err
		}
	}
	return nil
}

// writeRegion writes one Anvil region file: a 4 KiB sector table, a 4 KiB
// timestamp table and zlib compressed chunks padded to whole sectors.
func (g *grid) writeRegion(path string, keys [][2]int32) error {
	locations := make([]byte, 4096)
	timestamps := make([]byte, 4096)
	body := make([]byte, 0, 1<<20)
	sector := 2
	now := uint32(time.Now().Unix())

	for _, key := range keys {
		payload, err := compressChunk(key, g.chunks[key])
		if err != nil {
			return err
		}
		if len(payload) == 0 {
			continue
		}
		frame := 5 + len(payload)
		sectors := (frame + 4095) / 4096
		if sectors > 255 {
			return fmt.Errorf("chunk %d/%d does not fit the region format", key[0], key[1])
		}
		length := uint32(len(payload) + 1)
		body = append(body, byte(length>>24), byte(length>>16), byte(length>>8), byte(length))
		body = append(body, 2) // zlib
		body = append(body, payload...)
		body = append(body, make([]byte, sectors*4096-frame)...)

		slot := 4 * (int(key[0]&31) + int(key[1]&31)*32)
		locations[slot] = byte(sector >> 16)
		locations[slot+1] = byte(sector >> 8)
		locations[slot+2] = byte(sector)
		locations[slot+3] = byte(sectors)
		timestamps[slot] = byte(now >> 24)
		timestamps[slot+1] = byte(now >> 16)
		timestamps[slot+2] = byte(now >> 8)
		timestamps[slot+3] = byte(now)
		sector += sectors
	}

	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()
	if _, err := file.Write(locations); err != nil {
		return err
	}
	if _, err := file.Write(timestamps); err != nil {
		return err
	}
	_, err = file.Write(body)
	return err
}

func (w *World) writeReadme() error {
	text := fmt.Sprintf(`Mappy Minecraft export
======================

World:      %s
Scale:      %.2f m per block
Minecraft:  %s or newer (newer versions upgrade the save automatically)

How to play it
--------------
1. Copy this whole folder into your Minecraft saves folder:
   Windows  %%APPDATA%%\.minecraft\saves
   macOS    ~/Library/Application Support/minecraft/saves
   Linux    ~/.minecraft/saves
2. Start Minecraft Java Edition and open the world from the single player list.

The world is a void superflat: only the scanned area contains blocks. Every
block comes from the real mesh and its photo texture, nothing is generated.
`, w.opt.Name, w.opt.BlockSize, versionName)
	return os.WriteFile(filepath.Join(w.dir, "HOW-TO-PLAY.txt"), []byte(text), 0o644)
}
