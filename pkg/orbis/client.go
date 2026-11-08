// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Henrik Hellpap. Part of Orbis: https://github.com/Henkek221/orbis

package orbis

import (
	"errors"
	"fmt"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"sort"

	"github.com/Henkek221/orbis/pkg/fly"
	"github.com/Henkek221/orbis/pkg/fly/c3m"
	"github.com/Henkek221/orbis/pkg/fly/c3mm"
	"github.com/Henkek221/orbis/pkg/mps"
	"github.com/Henkek221/orbis/pkg/mth"
	"github.com/Henkek221/orbis/pkg/web"
)

// Options says where a client finds its configuration and keeps its cache.
type Options struct {
	// ConfigPath points at config.json. A missing file is fine; see LoadConfig.
	ConfigPath string
	// CacheDir holds the session, the manifests and the C3MM octree index.
	CacheDir string
}

// Client is an authenticated connection to the 3D map service. It is safe to
// use from several goroutines; each Export and Coverage call scans with its
// own index state.
type Client struct {
	base flyContext
}

// Region is one Flyover dataset: an area around a centre point with its own
// dataset version.
type Region struct {
	Name    string  `json:"name"`
	ID      int     `json:"id"`
	Version int     `json:"version"`
	Lat     float64 `json:"lat"`
	Lon     float64 `json:"lon"`
	Radius  float64 `json:"radiusMetres"`
}

// Open loads the configuration and connects. It returns ErrNotConfigured when
// no tokenP1 is available.
func Open(opts Options) (*Client, error) {
	cfg, err := LoadConfig(opts.ConfigPath)
	if err != nil {
		return nil, err
	}
	if !cfg.IsValid() {
		return nil, ErrNotConfigured
	}
	return Connect(cfg, opts.CacheDir)
}

// Connect uses cfg directly. Use it to verify a config before saving it.
func Connect(cfg Config, cacheDir string) (*Client, error) {
	cache := mps.Cache{Enabled: true, Directory: cacheDir}
	if err := cache.Init(); err != nil {
		return nil, err
	}
	ctx, err := mps.Init(cache, cfg)
	if err != nil {
		return nil, err
	}
	am, err := fly.GetAltitudeManifest(cache, ctx.ResourceManifest)
	if err != nil {
		return nil, err
	}
	c3mmURLPrefix, err := ctx.ResourceManifest.URLPrefixFromStyleID(mps.ResourceManifest_StyleConfig_C3MM_1)
	if err != nil {
		return nil, err
	}
	c3mURLPrefix, err := ctx.ResourceManifest.URLPrefixFromStyleID(mps.ResourceManifest_StyleConfig_C3M)
	if err != nil {
		return nil, err
	}
	return &Client{base: flyContext{
		Context:          &ctx,
		AltitudeManifest: am,
		URLPrefixC3mm:    c3mmURLPrefix,
		URLPrefixC3m:     c3mURLPrefix,
		cacheDir:         cacheDir,
	}}, nil
}

// Region returns the Flyover dataset that covers a point.
func (c *Client) Region(lat, lon float64) (Region, error) {
	place, err := c.base.findPlace(lat, lon)
	if err != nil {
		return Region{}, err
	}
	return regionFromTrigger(place), nil
}

// Regions lists every dataset the service currently announces.
func (c *Client) Regions() []Region {
	regions := make([]Region, 0, len(c.base.AltitudeManifest.Triggers))
	for _, t := range c.base.AltitudeManifest.Triggers {
		regions = append(regions, regionFromTrigger(t))
	}
	return regions
}

func regionFromTrigger(t fly.Trigger) Region {
	return Region{Name: t.Name, ID: t.Region, Version: t.Version, Lat: t.Lat, Lon: t.Lon, Radius: t.Radius}
}

// scanner returns a copy of the base context with an empty index cache. The
// index cache is filled lazily and is not safe for concurrent use.
func (c *Client) scanner() *flyContext {
	scan := c.base
	scan.C3mms = nil
	return &scan
}

type flyContext struct {
	Context          *mps.Context
	AltitudeManifest fly.AltitudeManifest
	URLPrefixC3mm    string
	URLPrefixC3m     string
	C3mms            []*c3mm.C3MM

	cacheDir string
}

func (ctx *flyContext) c3mmDir(p fly.Trigger) string {
	return filepath.Join(ctx.cacheDir, "c3mm", fmt.Sprintf("%d_%d", p.Region, p.Version))
}

func (ctx *flyContext) checkTile(p fly.Trigger, z, y, x, h int) (bool, error) {
	tile := c3mm.Tile{Z: z, Y: y, X: x, H: h}
	c3mm0, err := ctx.getC3mm(p, 0)
	if err != nil {
		return false, err
	}
	smallestZ := c3mm0.RootIndex.SmallestZ
	if tile.Z < smallestZ {
		return false, errors.New("z too small")
	}

	list := make([]c3mm.Tile, 0)
	for t := tile; t.Z >= smallestZ; t = t.ZoomedOut() {
		list = append(list, t)
	}

	root, listIdx := c3mm.Root{}, len(list)-1
	for ; listIdx >= 0; listIdx-- {
		t := list[listIdx]
		n := len(c3mm0.RootIndex.Entries)
		idx := sort.Search(n, func(i int) bool {
			root := c3mm0.RootIndex.Entries[i]
			return t.Less(root.Tile) || t == root.Tile
		})
		if idx == n {
			continue
		}
		root = c3mm0.RootIndex.Entries[idx]
		if t != root.Tile {
			continue
		}
		break
	}
	if listIdx < 0 {
		return false, nil
	}

	readOctant := func(octantOffset *int) (c3mm.Octant, error) {
		partNum := c3mm0.FileIndex.GetPartNumber(*octantOffset)
		c3mm1, err := ctx.getC3mm(p, partNum)
		if err != nil {
			return c3mm.Octant{}, err
		}
		return c3mm1.GetOctant(octantOffset, c3mm0.FileIndex.Entries[partNum]), nil
	}

	rootOffset := root.Offset
	octant, err := readOctant(&rootOffset)
	if err != nil {
		return false, err
	}
	if list[listIdx] == tile {
		return true, nil
	}
	if listIdx == 0 {
		return false, nil
	}
	for ; octant.Next > 0; listIdx-- {
		zoomedInActual := list[listIdx-1]
		zoomedInCandidates := list[listIdx].ZoomedInCandidates()
		bits := octant.Bits
		octantOffset := octant.Next
		matched := false
		for o := 0; o < 8; o++ {
			if (bits>>(o*2))&1 != 1 {
				continue
			}
			octant, err = readOctant(&octantOffset)
			if err != nil {
				return false, err
			}
			if zoomedInCandidates(o) == zoomedInActual {
				matched = true
				break
			}
		}
		if !matched {
			return false, nil
		}
		if tile == zoomedInActual {
			return true, nil
		}
	}
	return false, nil
}

func (ctx *flyContext) getTile(p fly.Trigger, z, y, x, h int) (c3m.C3M, error) {
	yn := mth.TileCountPerAxis(z) - 1 - y
	url := fmt.Sprintf("%s?style=%d&v=%d&region=%d&x=%d&y=%d&z=%d&h=%d",
		ctx.URLPrefixC3m, mps.ResourceManifest_StyleConfig_C3M, p.Version, p.Region, x, yn, z, h)
	data, err := ctx.get(url)
	if err != nil {
		return c3m.C3M{}, err
	}
	if len(data) == 0 {
		return c3m.C3M{}, errors.New("empty tile")
	}
	return c3m.Parse(data)
}

func (ctx *flyContext) getC3mm(p fly.Trigger, part int) (c3mm.C3MM, error) {
	if ctx.C3mms == nil || ctx.C3mms[part] == nil {
		data, err := ctx.fetchC3mm(p, part)
		if err != nil {
			return c3mm.C3MM{}, err
		}
		if part == 0 {
			ctx.C3mms = make([]*c3mm.C3MM, len(data.FileIndex.Entries))
		}
		ctx.C3mms[part] = &data
		return data, nil
	}
	return *ctx.C3mms[part], nil
}

func (ctx *flyContext) fetchC3mm(p fly.Trigger, part int) (c3mm.C3MM, error) {
	dir := ctx.c3mmDir(p)
	fn := filepath.Join(dir, fmt.Sprint(part))
	file, err := os.ReadFile(fn)
	if err != nil && !os.IsNotExist(err) {
		return c3mm.C3MM{}, err
	}
	if err == nil {
		return c3mm.Parse(file, part)
	}
	data, err := ctx.get(fmt.Sprintf("%s?style=%d&v=%d&part=%d&region=%d",
		ctx.URLPrefixC3mm, mps.ResourceManifest_StyleConfig_C3MM_1, p.Version, part, p.Region))
	if err != nil {
		return c3mm.C3MM{}, err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return c3mm.C3MM{}, err
	}
	if err := os.WriteFile(fn+".tmp", data, 0o644); err != nil {
		return c3mm.C3MM{}, err
	}
	if err := os.Rename(fn+".tmp", fn); err != nil {
		return c3mm.C3MM{}, err
	}
	return c3mm.Parse(data, part)
}

func (ctx *flyContext) findPlace(lat, lon float64) (fly.Trigger, error) {
	minDist, minPlace := math.Inf(1), fly.Trigger{}
	for _, v := range ctx.AltitudeManifest.Triggers {
		dist := GeoDistanceMetres(lat, lon, v.Lat, v.Lon)
		if dist <= v.Radius && dist < minDist {
			minDist, minPlace = dist, v
		}
	}
	if minDist == math.Inf(1) {
		return fly.Trigger{}, errors.New("no flyover region covers this point")
	}
	return minPlace, nil
}

func (ctx *flyContext) get(url string) ([]byte, error) {
	authURL, err := ctx.Context.AuthContext.AuthURL(url)
	if err != nil {
		return nil, err
	}
	jpgErr := errors.New("received jpeg")
	return web.GetWithCheck(authURL, func(res *http.Response) error {
		if res.Header.Get("content-type") == "image/jpeg" {
			return jpgErr
		}
		return nil
	})
}

// anyHeight reports whether any of the first heightLayers levels of a tile
// holds a mesh, and whether the index could be read at all.
func (ctx *flyContext) anyHeight(p fly.Trigger, zoom, y, x, heightLayers int) (available, known bool) {
	if _, err := ctx.getC3mm(p, 0); err != nil {
		return false, false
	}
	for h := 0; h < heightLayers; h++ {
		hasTile, err := ctx.checkTile(p, zoom, y, x, h)
		if err != nil {
			return false, false
		}
		if hasTile {
			return true, true
		}
	}
	return false, true
}
