// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Henrik Hellpap. Part of Orbis: https://github.com/Henkek221/orbis

package orbis

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/Henkek221/orbis/pkg/fly/c3m"
	"github.com/Henkek221/orbis/pkg/fly/exp"
	"github.com/Henkek221/orbis/pkg/fly/r3d"
	"github.com/Henkek221/orbis/pkg/mth"
)

// Style IDs of Apple's newer 3D format.
const (
	styleR3D      = 98
	styleR3DIndex = 100
)

// r3dSource is where R3D tiles and their index are served from.
type r3dSource struct {
	tilePrefix  string
	indexPrefix string
	version     int
}

// r3d reads the R3D endpoints and the current data version from the manifest.
func (c *Client) r3d() (r3dSource, error) {
	rm := c.base.Context.ResourceManifest
	tilePrefix, version, err := rm.StyleVersion(styleR3D)
	if err != nil {
		return r3dSource{}, err
	}
	indexPrefix, _, err := rm.StyleVersion(styleR3DIndex)
	if err != nil {
		return r3dSource{}, err
	}
	if version == 0 {
		return r3dSource{}, errors.New("manifest announces no R3D version")
	}
	return r3dSource{tilePrefix: tilePrefix, indexPrefix: indexPrefix, version: version}, nil
}

// tileItem is what a fetch hands to the single writer goroutine: zero or more
// meshes, splats, and an ECEF point near the ground for the shared origin.
type tileItem struct {
	meshes    []c3m.C3M
	splats    []exp.Splat
	reference [3]float64
	// classic marks old Flyover geometry repackaged as R3D; column is its
	// TMS x/y, used to drop it where newer data exists.
	classic bool
	column  [2]int
}

// fetchR3D downloads and decodes one R3D tile. x and y are TMS.
func (c *Client) fetchR3D(src r3dSource, zoom, x, y, h int) (tileItem, error) {
	yXYZ := mth.TileCountPerAxis(zoom) - 1 - y
	url := fmt.Sprintf("%s?style=%d&v=%d&z=%d&x=%d&y=%d&h=%d", src.tilePrefix, styleR3D, src.version, zoom, x, yXYZ, h)
	data, err := c.base.get(url)
	if err != nil {
		return tileItem{}, err
	}
	if len(data) == 0 {
		return tileItem{}, errors.New("empty tile")
	}
	tile, err := r3d.Parse(data)
	if err != nil {
		return tileItem{}, err
	}
	item := tileItem{classic: tile.Classic(), column: [2]int{x, y}}
	referenced := false
	for _, g := range tile.Groups {
		if g.Frame == nil {
			continue
		}
		frame := *g.Frame
		if g.Mesh != nil {
			md, err := r3d.DecodeMesh(g.Mesh, g.Images)
			if err != nil {
				return tileItem{}, err
			}
			if len(md.Faces) > 0 {
				item.meshes = append(item.meshes, meshToC3M(frame, md, g.Images))
				if !referenced {
					p := md.Positions[md.Faces[0][0]]
					item.reference = frame.ToECEF([3]float64{float64(p[0]), float64(p[1]), float64(p[2])})
					referenced = true
				}
			}
		}
		if g.Soup != nil && g.Soup.Records > 0 {
			splats, err := r3d.DecodeSoup(g.Soup)
			if err != nil {
				return tileItem{}, err
			}
			for _, s := range splats {
				var out exp.Splat
				for k, corner := range s.Corners {
					out.Corners[k] = frame.ToECEF([3]float64{float64(corner[0]), float64(corner[1]), float64(corner[2])})
				}
				out.RGBA = s.RGBA
				item.splats = append(item.splats, out)
			}
			if !referenced && len(item.splats) > 0 {
				item.reference = item.splats[0].Corners[0]
				referenced = true
			}
		}
		if !referenced {
			item.reference = frame.Translation
		}
	}
	if len(item.meshes) == 0 && len(item.splats) == 0 {
		return tileItem{}, errors.New("empty tile")
	}
	return item, nil
}

// meshToC3M turns a decoded R3D mesh into the classic tile structure the
// writers understand: one vertex per position/UV pair, faces grouped by atlas.
func meshToC3M(frame r3d.Frame, md *r3d.MeshData, images []r3d.Image) c3m.C3M {
	type key struct{ p, uv uint32 }
	tile := c3m.C3M{Header: c3m.Header{Rotation: frame.Rotation, Translation: frame.Translation}}
	for _, img := range images {
		if img.Format == r3d.HEIC {
			tile.Materials = append(tile.Materials, c3m.Material{HEIC: img.Data})
		} else {
			tile.Materials = append(tile.Materials, c3m.Material{JPEG: img.Data})
		}
	}

	mesh := c3m.Mesh{Groups: map[int]c3m.Group{}}
	index := make(map[key]int32, len(md.Positions))
	for f, face := range md.Faces {
		var tri [3]int32
		for k := 0; k < 3; k++ {
			kk := key{p: face[k]}
			if f < len(md.UVFaces) {
				kk.uv = md.UVFaces[f][k]
			}
			id, ok := index[kk]
			if !ok {
				p := md.Positions[kk.p]
				vtx := c3m.Vertex{X: p[0], Y: p[1], Z: p[2]}
				if f < len(md.UVFaces) && int(kk.uv) < len(md.UVs) {
					uv := md.UVs[kk.uv]
					// R3D counts v from the top, the classic tiles from the bottom.
					vtx.U, vtx.V = uv[0], 1-uv[1]
				}
				id = int32(len(mesh.Vertices))
				mesh.Vertices = append(mesh.Vertices, vtx)
				index[kk] = id
			}
			tri[k] = id
		}
		atlas := 0
		if f < len(md.FaceAtlas) {
			atlas = int(md.FaceAtlas[f])
		}
		g := mesh.Groups[atlas]
		g.Material = atlas
		g.Faces = append(g.Faces, c3m.Face{A: tri[0], B: tri[1], C: tri[2]})
		mesh.Groups[atlas] = g
	}
	tile.Meshes = []c3m.Mesh{mesh}
	return tile
}

// r3dIndex answers occupancy questions from the style 100 index, fetched per
// zoom-13 tile and cached in memory and on disk.
type r3dIndex struct {
	client *Client
	src    r3dSource
	mu     sync.Mutex
	tiles  map[[2]int]*indexEntry
}

type indexEntry struct {
	cells    map[[2]int]bool
	subZoom  int
	err      error
	resolved bool
}

func newR3DIndex(c *Client, src r3dSource) *r3dIndex {
	return &r3dIndex{client: c, src: src, tiles: map[[2]int]*indexEntry{}}
}

// occupied reports whether a TMS tile has R3D data. An error means unknown.
func (r *r3dIndex) occupied(zoom, x, y int) (bool, error) {
	if zoom < 13 {
		return true, errors.New("index needs zoom 13 or more")
	}
	yXYZ := mth.TileCountPerAxis(zoom) - 1 - y
	shift := zoom - 13
	entry := r.load(x>>shift, yXYZ>>shift)
	if entry.err != nil {
		return false, entry.err
	}
	if zoom >= entry.subZoom {
		s := zoom - entry.subZoom
		return entry.cells[[2]int{x >> s, yXYZ >> s}], nil
	}
	s := entry.subZoom - zoom
	for cell := range entry.cells {
		if cell[0]>>s == x && cell[1]>>s == yXYZ {
			return true, nil
		}
	}
	return false, nil
}

func (r *r3dIndex) load(x, y int) *indexEntry {
	r.mu.Lock()
	defer r.mu.Unlock()
	key := [2]int{x, y}
	if e, ok := r.tiles[key]; ok {
		return e
	}
	e := &indexEntry{}
	r.tiles[key] = e
	data, err := r.client.indexData(r.src, x, y)
	if err != nil {
		e.err = err
		return e
	}
	e.cells = map[[2]int]bool{}
	if len(data) == 0 {
		e.subZoom = 17
		return e
	}
	idx, err := r3d.ParseIndex(data)
	if err != nil {
		e.err = err
		return e
	}
	e.subZoom = idx.SubZoom
	for _, c := range idx.Cells {
		e.cells[[2]int{c.X, c.Y}] = true
	}
	return e
}

// indexData returns the raw zoom-13 index for an XYZ tile, cached on disk per
// data version.
func (c *Client) indexData(src r3dSource, x, y int) ([]byte, error) {
	dir := filepath.Join(c.base.cacheDir, "r3d", fmt.Sprint(src.version))
	path := filepath.Join(dir, fmt.Sprintf("index_13_%d_%d", x, y))
	if data, err := os.ReadFile(path); err == nil {
		return data, nil
	}
	url := fmt.Sprintf("%s?style=%d&v=%d&z=13&x=%d&y=%d", src.indexPrefix, styleR3DIndex, src.version, x, y)
	data, err := c.base.get(url)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(dir, 0o755); err == nil {
		_ = os.WriteFile(path+".tmp", data, 0o644)
		_ = os.Rename(path+".tmp", path)
	}
	return data, nil
}
