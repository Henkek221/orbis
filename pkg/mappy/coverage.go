package mappy

import (
	"context"
	"fmt"
)

// DefaultCoverageMaxTiles keeps a coverage scan to a few seconds.
const DefaultCoverageMaxTiles = 196

// CoverageOptions controls a coverage scan. Zero values use the defaults.
type CoverageOptions struct {
	Zoom         int
	HeightLayers int
	MaxTiles     int
}

// CoverageTile is one map tile and whether 3D mesh data exists for it.
// Known is false when the index could not be read, so absence is unproven.
type CoverageTile struct {
	Bounds
	X         int  `json:"x"`
	Y         int  `json:"y"`
	Available bool `json:"available"`
	Known     bool `json:"known"`
}

// Coverage is the result of a coverage scan, tiles ordered west to east and
// south to north.
type Coverage struct {
	Range     TileRange      `json:"range"`
	Tiles     []CoverageTile `json:"tiles"`
	Available int            `json:"availableCount"`
	Unknown   int            `json:"unknownCount"`
}

// Coverage reports, per tile, whether any height layer holds a mesh. It only
// reads the octree index, never the meshes themselves.
func (c *Client) Coverage(ctx context.Context, b Bounds, opt CoverageOptions) (Coverage, error) {
	if err := b.Validate(); err != nil {
		return Coverage{}, err
	}
	if opt.Zoom == 0 {
		opt.Zoom = 18
	}
	if opt.HeightLayers == 0 {
		opt.HeightLayers = DefaultHeightLayers
	}
	if opt.MaxTiles == 0 {
		opt.MaxTiles = DefaultCoverageMaxTiles
	}
	if opt.Zoom < 13 || opt.Zoom > 20 {
		return Coverage{}, fmt.Errorf("zoom must be between 13 and 20")
	}

	r := b.Tiles(opt.Zoom)
	if n := r.Count(); n > opt.MaxTiles {
		return Coverage{}, fmt.Errorf("area covers %d tiles at zoom %d, limit is %d: use a lower zoom or a smaller area", n, opt.Zoom, opt.MaxTiles)
	}

	scanners := make(map[int]*flyContext)
	result := Coverage{Range: r, Tiles: make([]CoverageTile, 0, r.Count())}
	for x := r.XMin; x <= r.XMax; x++ {
		for y := r.YMin; y <= r.YMax; y++ {
			if err := ctx.Err(); err != nil {
				return result, err
			}
			tile := CoverageTile{Bounds: TileBounds(opt.Zoom, x, y), X: x, Y: y}
			lat, lon := tile.Center()
			place, err := c.base.findPlace(lat, lon)
			if err != nil {
				tile.Known = true
				result.Tiles = append(result.Tiles, tile)
				continue
			}
			scan := scanners[place.Region]
			if scan == nil {
				scan = c.scanner()
				scanners[place.Region] = scan
			}
			tile.Available, tile.Known = scan.anyHeight(place, opt.Zoom, y, x, opt.HeightLayers)
			if tile.Available {
				result.Available++
			}
			if !tile.Known {
				result.Unknown++
			}
			result.Tiles = append(result.Tiles, tile)
		}
	}
	return result, nil
}
