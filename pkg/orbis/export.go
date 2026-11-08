// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Henrik Hellpap. Part of Orbis: https://github.com/Henkek221/orbis

package orbis

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"github.com/Henkek221/orbis/pkg/fly/c3m"
	"github.com/Henkek221/orbis/pkg/fly/exp"
	"github.com/Henkek221/orbis/pkg/mc"
	"github.com/Henkek221/orbis/pkg/oth"
)

// Defaults match the browser UI.
const (
	DefaultZoom         = 20
	DefaultHeightLayers = 24
	DefaultParallel     = 16
	DefaultBlockSize    = 1.0
	DefaultMaxRequests  = 200000
)

// TileWriter receives every decoded mesh tile of an export. Implement it to add
// an output format; Next is never called concurrently.
type TileWriter = exp.Export

// Request describes one export.
type Request struct {
	// Name labels the export folder and the Minecraft world. Empty falls back to
	// the centre coordinates and the date.
	Name   string
	Bounds Bounds
	// Zoom is the tile level, 13 to 20. 20 is source quality.
	Zoom int
	// HeightLayers is how many octree height levels are scanned per tile. Too
	// few silently cut tall buildings.
	HeightLayers int
	// Parallel is the number of concurrent tile downloads.
	Parallel int

	OBJ bool
	// GLB writes one binary glTF with embedded textures, Y up, real scale:
	// the format game engines import directly.
	GLB bool
	// USD writes a .usda stage with textures and, on macOS, a packaged .usdz for
	// Xcode, RealityKit, Reality Composer and Quick Look.
	USD bool
	// Splats writes Apple's soft triangles (newer regions only) as standard
	// 3D Gaussian Splatting PLY for splat viewers and engine plugins.
	Splats    bool
	Minecraft bool
	// BlockSize is metres per Minecraft block: 1 keeps real scale, 0.5 doubles
	// the detail and quadruples the block count.
	BlockSize float64
	// Writers are extra outputs fed alongside OBJ and Minecraft.
	Writers []TileWriter

	// OutputRoot receives one folder per export. Empty means the current
	// directory.
	OutputRoot string
	// MaxRequests guards against accidentally huge scans.
	MaxRequests int
}

// Normalize fills zero values with defaults and clamps the block size. Without
// any output selected it exports OBJ.
func (r Request) Normalize() Request {
	if !r.OBJ && !r.GLB && !r.USD && !r.Splats && !r.Minecraft && len(r.Writers) == 0 {
		r.OBJ = true
	}
	if r.Zoom == 0 {
		r.Zoom = DefaultZoom
	}
	if r.HeightLayers == 0 {
		r.HeightLayers = DefaultHeightLayers
	}
	if r.Parallel == 0 {
		r.Parallel = DefaultParallel
	}
	switch {
	case r.BlockSize <= 0:
		r.BlockSize = DefaultBlockSize
	case r.BlockSize < 0.25:
		r.BlockSize = 0.25
	case r.BlockSize > 4:
		r.BlockSize = 4
	}
	if r.MaxRequests == 0 {
		r.MaxRequests = MaxRequestsFromEnv()
	}
	if r.OutputRoot == "" {
		r.OutputRoot = "."
	}
	return r
}

// Validate checks a normalized request.
func (r Request) Validate() error {
	if err := r.Bounds.Validate(); err != nil {
		return err
	}
	if r.Zoom < 13 || r.Zoom > 20 {
		return errors.New("zoom must be between 13 and 20")
	}
	if r.HeightLayers < 1 || r.HeightLayers > 128 {
		return errors.New("height scan must be between 1 and 128")
	}
	if r.Parallel < 1 || r.Parallel > 64 {
		return errors.New("parallel downloads must be between 1 and 64")
	}
	if n := r.RequestCount(); r.MaxRequests > 0 && n > r.MaxRequests {
		return fmt.Errorf("selection would scan %d tile-height requests; current limit is %d (set ORBIS_MAX_REQUESTS to raise it)", n, r.MaxRequests)
	}
	return nil
}

// RequestCount is the number of tile-height lookups the export performs.
func (r Request) RequestCount() int {
	return r.Bounds.Tiles(r.Zoom).Count() * r.HeightLayers
}

// MaxRequestsFromEnv reads ORBIS_MAX_REQUESTS and falls back to
// DefaultMaxRequests.
func MaxRequestsFromEnv() int {
	raw := strings.TrimSpace(os.Getenv("ORBIS_MAX_REQUESTS"))
	if raw == "" {
		return DefaultMaxRequests
	}
	value, err := strconv.Atoi(raw)
	if err != nil || value < 1 {
		return DefaultMaxRequests
	}
	return value
}

// Progress receives log lines and scan progress. Both callbacks are optional
// and may be called from several goroutines.
type Progress struct {
	Log  func(string)
	Step func(processed, total int)
}

func (p Progress) logf(format string, args ...any) {
	if p.Log != nil {
		p.Log(fmt.Sprintf(format, args...))
	}
}

func (p Progress) step(processed, total int) {
	if p.Step != nil {
		p.Step(processed, total)
	}
}

// Result describes a finished or partially finished export.
type Result struct {
	Name   string `json:"name"`
	Region Region `json:"region"`
	// Format is "classic" (C3M) or "r3d" (Apple's newer format with splats).
	Format   string `json:"format"`
	Requests int    `json:"requests"`
	Exported int    `json:"exported"`
	Output   string `json:"output"`
	// OBJ is the model file, empty when OBJ was not requested.
	OBJ string `json:"obj,omitempty"`
	// GLB is the binary glTF, empty when not requested or nothing was found.
	GLB string `json:"glb,omitempty"`
	// USD is the .usda stage; USDZ the packaged file when usdzip was available.
	USD  string `json:"usd,omitempty"`
	USDZ string `json:"usdz,omitempty"`
	// World is the Minecraft save folder, empty when not requested.
	World string `json:"world,omitempty"`
	// SplatsPLY is the Gaussian splat file and SplatCount the splats in it.
	SplatsPLY  string `json:"splats,omitempty"`
	SplatCount int    `json:"splatCount,omitempty"`
}

// OBJFileName is the model file inside an export folder.
const OBJFileName = "exp_model.obj"

// Export scans the requested area and writes every selected format. Cancelling
// ctx stops the scan; the tiles fetched so far are still written.
func (c *Client) Export(ctx context.Context, req Request, progress Progress) (Result, error) {
	req = req.Normalize()
	if err := req.Validate(); err != nil {
		return Result{}, err
	}
	scan := c.scanner()

	centerLat, centerLon := req.Bounds.Center()
	place, placeErr := scan.findPlace(centerLat, centerLon)
	result := Result{Region: regionFromTrigger(place), Requests: req.RequestCount(), Format: "classic"}

	// Classic regions have a C3MM index. Newer ones only serve R3D.
	directScan := false
	var src r3dSource
	if placeErr == nil {
		progress.logf("region: %s version=%d region=%d", place.Name, place.Version, place.Region)
		if _, err := scan.getC3mm(place, 0); err != nil {
			directScan = true
		}
	}
	if placeErr != nil || directScan {
		if s, err := c.r3d(); err == nil {
			src = s
			result.Format = "r3d"
			directScan = false
			progress.logf("format: R3D v2 (version %d), meshes and splats", src.version)
		} else if placeErr != nil {
			return Result{}, placeErr
		} else {
			progress.logf("C3MM index unavailable; direct C3M scan enabled")
		}
	}

	tiles := req.Bounds.Tiles(req.Zoom)
	requests := result.Requests
	progress.step(0, requests)
	progress.logf("tiles: x=%d..%d y=%d..%d zoom=%d h=0..%d (%d requests)", tiles.XMin, tiles.XMax, tiles.YMin, tiles.YMax, req.Zoom, req.HeightLayers-1, requests)

	result.Name = ExportName(req.Name, req.Bounds)
	output, err := AvailableOutputPath(req.OutputRoot, result.Name)
	if err != nil {
		return result, err
	}
	if err := os.MkdirAll(output, 0o755); err != nil {
		return result, err
	}
	result.Output = output

	exporter := multiExport(append([]TileWriter(nil), req.Writers...))
	if req.OBJ {
		objExport, err := exp.NewLocalized(output, "exp_", centerLat, centerLon)
		if err != nil {
			return result, err
		}
		exporter = append(exporter, objExport)
		result.OBJ = filepath.Join(output, OBJFileName)
	}
	if req.GLB {
		glb, err := exp.NewGLB(filepath.Join(output, result.Name+".glb"), centerLat, centerLon)
		if err != nil {
			return result, err
		}
		exporter = append(exporter, glb)
		result.GLB = glb.Path()
	}
	var usd *exp.USDExport
	if req.USD {
		usd, err = exp.NewUSD(output, result.Name, centerLat, centerLon)
		if err != nil {
			return result, err
		}
		exporter = append(exporter, usd)
		result.USD = usd.Path()
	}
	var ply *exp.PLYExport
	if req.Splats {
		ply, err = exp.NewPLY(filepath.Join(output, result.Name+"-splats.ply"), centerLat, centerLon)
		if err != nil {
			return result, err
		}
		exporter = append(exporter, ply)
	}
	if req.Minecraft {
		levelName := strings.TrimSpace(req.Name)
		if levelName == "" {
			levelName = result.Name
		}
		result.World = filepath.Join(output, result.Name+"-minecraft")
		world, err := mc.New(result.World, centerLat, centerLon, mc.Options{
			Name:      levelName,
			BlockSize: req.BlockSize,
			Logf:      progress.Log,
		})
		if err != nil {
			return result, err
		}
		progress.logf("minecraft: writing a world at %.2f m per block to %s", req.BlockSize, result.World)
		if converter := oth.HEICConverter(); converter == "" {
			progress.logf("minecraft: no HEIC converter found; install ImageMagick or ffmpeg if colours look flat")
		}
		exporter = append(exporter, world)
	}
	c3m.DisableLogs()

	sem := make(chan struct{}, req.Parallel)
	tileCh := make(chan tileItem, req.Parallel)
	var wg sync.WaitGroup
	var firstErr error
	var firstErrMu sync.Mutex
	stats := &scanStats{}
	processed := 0
	var progressMu sync.Mutex
	markProcessed := func() {
		progressMu.Lock()
		processed++
		current := processed
		progressMu.Unlock()
		progress.step(current, requests)
	}

	setErr := func(err error) {
		firstErrMu.Lock()
		defer firstErrMu.Unlock()
		if firstErr == nil {
			firstErr = err
		}
	}

	exported, splatCount := 0, 0
	done := make(chan struct{})
	go func() {
		defer close(done)
		originSet := false
		write := func(item tileItem) {
			if !originSet {
				// One origin for every output, so meshes and splats line up.
				origin := exp.CentredOrigin(centerLat, centerLon, item.reference)
				for _, w := range exporter {
					if o, ok := w.(exp.OriginSetter); ok {
						o.SetOrigin(origin)
					}
				}
				originSet = true
			}
			for _, mesh := range item.meshes {
				if err := exporter.Next(mesh, strconv.Itoa(exported)); err != nil {
					setErr(err)
					continue
				}
				exported++
			}
			if len(item.splats) > 0 {
				for _, w := range exporter {
					if sw, ok := w.(exp.SplatWriter); ok {
						if err := sw.Splats(item.splats); err != nil {
							setErr(err)
						}
					}
				}
				splatCount += len(item.splats)
			}
		}
		// Newer regions also serve old Flyover geometry in upper layers of a
		// column; it z-fights with the new mesh. Hold classic R3D tiles until
		// the scan ends and keep them only where their column has nothing newer.
		modern := map[[2]int]bool{}
		var classic []tileItem
		for item := range tileCh {
			if item.classic {
				classic = append(classic, item)
				continue
			}
			modern[item.column] = true
			write(item)
		}
		for _, item := range classic {
			if !modern[item.column] {
				write(item)
			}
		}
	}()

	fetch := func(x, y, h int) {
		defer wg.Done()
		defer func() { <-sem }()
		defer markProcessed()
		if result.Format == "r3d" {
			item, err := c.fetchR3D(src, req.Zoom, x, y, h)
			stats.record(err)
			if err == nil {
				tileCh <- item
			}
			return
		}
		tile, err := scan.getTile(place, req.Zoom, y, x, h)
		stats.record(err)
		if err != nil {
			return
		}
		tileCh <- tileItem{meshes: []c3m.C3M{tile}, reference: tile.Header.Translation}
	}

	index := newR3DIndex(c, src)
scanLoop:
	for x := tiles.XMin; x <= tiles.XMax; x++ {
		for y := tiles.YMin; y <= tiles.YMax; y++ {
			if result.Format == "r3d" {
				occupied, err := index.occupied(req.Zoom, x, y)
				if err == nil && !occupied {
					for h := 0; h < req.HeightLayers; h++ {
						markProcessed()
					}
					continue
				}
			}
			for h := 0; h < req.HeightLayers; h++ {
				if ctx.Err() != nil {
					break scanLoop
				}
				if result.Format == "classic" && !directScan {
					hasTile, err := scan.checkTile(place, req.Zoom, y, x, h)
					if err != nil {
						stats.record(err)
						setErr(err)
						markProcessed()
						continue
					}
					if !hasTile {
						markProcessed()
						continue
					}
				}

				select {
				case sem <- struct{}{}:
				case <-ctx.Done():
					break scanLoop
				}
				wg.Add(1)
				go fetch(x, y, h)
			}
		}
	}

	wg.Wait()
	close(tileCh)
	<-done
	if ctx.Err() != nil {
		progress.logf("cancelled; writing the %d tiles fetched so far", exported)
	}
	if req.Minecraft && exported > 0 {
		progress.logf("minecraft: converting the mesh into blocks…")
	}
	closeErr := exporter.Close()
	result.Exported = exported
	if usd != nil {
		result.USDZ = usd.USDZPath()
	}
	if ply != nil && ply.Count() > 0 {
		result.SplatsPLY = ply.Path()
		result.SplatCount = ply.Count()
	} else if splatCount > 0 {
		result.SplatCount = splatCount
	}
	if splatCount > 0 {
		progress.logf("splats: %d soft triangles", splatCount)
	}
	stats.log(progress, exported)
	if err := ctx.Err(); err != nil {
		return result, err
	}
	if firstErr != nil {
		return result, firstErr
	}
	if closeErr != nil {
		return result, closeErr
	}
	return result, nil
}

// multiExport feeds one tile stream into every requested output format.
type multiExport []TileWriter

func (m multiExport) Next(tile c3m.C3M, subPfx string) error {
	var firstErr error
	for _, target := range m {
		if err := target.Next(tile, subPfx); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

func (m multiExport) Close() error {
	var firstErr error
	for _, target := range m {
		if err := target.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

type scanStats struct {
	mu        sync.Mutex
	Attempted int
	Empty     int
	FetchErr  int
	ParseErr  int
}

func (s *scanStats) record(err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Attempted++
	if err == nil {
		return
	}
	if strings.Contains(err.Error(), "empty tile") {
		s.Empty++
		return
	}
	if strings.Contains(err.Error(), "http status") || strings.Contains(err.Error(), "received jpeg") {
		s.FetchErr++
		return
	}
	s.ParseErr++
}

func (s *scanStats) log(progress Progress, exported int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	progress.logf("summary: attempted=%d exported=%d empty=%d fetch_errors=%d parse_errors=%d", s.Attempted, exported, s.Empty, s.FetchErr, s.ParseErr)
	if exported == 0 && s.Empty > 0 && s.Empty+s.FetchErr+s.ParseErr == s.Attempted {
		progress.logf("no mesh tiles found in this rectangle; Apple returned empty C3M responses for the scanned area")
	}
}
