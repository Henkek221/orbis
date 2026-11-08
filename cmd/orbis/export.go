// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Henrik Hellpap. Part of Orbis: https://github.com/Henkek221/orbis

package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/Henkek221/orbis/pkg/orbis"
)

func runExport(args []string) int {
	fs := flag.NewFlagSet("export", flag.ContinueOnError)
	var area areaFlags
	area.register(fs)
	home := fs.String("home", "", "data directory (config.json, cache)")
	out := fs.String("out", ".", "directory that receives the export folder")
	name := fs.String("name", "", "export name (default: the place or the coordinates)")
	format := fs.String("format", "glb,splats", "comma-separated outputs: glb, splats, usdz, obj, minecraft")
	zoom := fs.Int("zoom", orbis.DefaultZoom, "tile level 13-20; 20 is source quality")
	heights := fs.Int("heights", orbis.DefaultHeightLayers, "octree height layers to scan; raise for skyscrapers")
	parallel := fs.Int("parallel", orbis.DefaultParallel, "concurrent downloads")
	blockSize := fs.Float64("block-size", orbis.DefaultBlockSize, "Minecraft metres per block (0.25-4)")
	maxRequests := fs.Int("max-requests", orbis.MaxRequestsFromEnv(), "abort if the scan needs more tile lookups")
	jsonOut := fs.Bool("json", false, "print the result as JSON on stdout")
	quiet := fs.Bool("quiet", false, "no progress output")
	fs.Usage = func() {
		fmt.Fprint(os.Stderr, "Usage: orbis export (--bbox W,S,E,N | --center LAT,LON | --place NAME) [flags]\n\n")
		fs.PrintDefaults()
	}
	if _, err := parseArgs(fs, args); err != nil {
		return 2
	}

	ctx, stop := signalContext()
	defer stop()

	bounds, placeName, err := area.resolve(ctx)
	if err != nil {
		return fail(err)
	}
	req := orbis.Request{
		Name:         *name,
		Bounds:       bounds,
		Zoom:         *zoom,
		HeightLayers: *heights,
		Parallel:     *parallel,
		BlockSize:    *blockSize,
		OutputRoot:   *out,
		MaxRequests:  *maxRequests,
	}
	if req.Name == "" {
		req.Name = placeName
	}
	for _, f := range strings.Split(*format, ",") {
		switch strings.ToLower(strings.TrimSpace(f)) {
		case "glb", "gltf":
			req.GLB = true
		case "splats", "ply":
			req.Splats = true
		case "usd", "usdz", "usda":
			req.USD = true
		case "obj":
			req.OBJ = true
		case "minecraft", "mc":
			req.Minecraft = true
		case "":
		default:
			return fail(fmt.Errorf("unknown format %q (use glb, splats, usdz, obj, minecraft)", f))
		}
	}
	req = req.Normalize()
	if err := req.Validate(); err != nil {
		return fail(err)
	}

	p, err := homePaths(*home)
	if err != nil {
		return fail(err)
	}
	client, err := openClient(p)
	if err != nil {
		return fail(err)
	}

	width, height := bounds.Size()
	progress := newProgressPrinter(!*quiet)
	progress.logf("area: %.0f x %.0f m around %.5f,%.5f", width, height, (bounds.North+bounds.South)/2, (bounds.East+bounds.West)/2)
	started := time.Now()
	result, err := client.Export(ctx, req, progress.hooks())
	progress.finish()
	if err != nil && !errors.Is(err, ctx.Err()) {
		return fail(err)
	}

	if *jsonOut {
		enc := json.NewEncoder(stdout)
		enc.SetIndent("", "  ")
		_ = enc.Encode(result)
	} else {
		fmt.Fprintf(stdout, "%d mesh tiles from %s (%s) in %s\n", result.Exported, regionLabel(result), result.Format, time.Since(started).Round(time.Second))
		if result.SplatsPLY != "" {
			fmt.Fprintf(stdout, "splats:    %s (%d Gaussians)\n", result.SplatsPLY, result.SplatCount)
		}
		if result.GLB != "" {
			fmt.Fprintln(stdout, "glb:      ", result.GLB)
		}
		if result.USDZ != "" {
			fmt.Fprintln(stdout, "usdz:     ", result.USDZ)
		} else if result.USD != "" {
			fmt.Fprintln(stdout, "usd:      ", result.USD)
		}
		if result.OBJ != "" {
			fmt.Fprintln(stdout, "obj:      ", result.OBJ)
		}
		if result.World != "" {
			fmt.Fprintln(stdout, "minecraft:", result.World)
		}
	}
	if err != nil {
		return fail(err)
	}
	if result.Exported == 0 && result.SplatCount == 0 {
		fmt.Fprintln(os.Stderr, "orbis: no 3D tiles in this area; try `orbis coverage` with the same area")
		return 3
	}
	return 0
}

// progressPrinter draws a single updating line on a terminal and plain log
// lines otherwise. Callbacks arrive from several goroutines.
type progressPrinter struct {
	enabled bool
	live    bool

	mu        sync.Mutex
	lastDraw  time.Time
	processed int
	total     int
	drawn     bool
}

func newProgressPrinter(enabled bool) *progressPrinter {
	return &progressPrinter{enabled: enabled, live: enabled && isTerminal(os.Stderr)}
}

func (p *progressPrinter) hooks() orbis.Progress {
	return orbis.Progress{Log: p.log, Step: p.step}
}

func (p *progressPrinter) logf(format string, args ...any) {
	p.log(fmt.Sprintf(format, args...))
}

func (p *progressPrinter) log(msg string) {
	if !p.enabled {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	p.clear()
	fmt.Fprintln(os.Stderr, msg)
	p.draw()
}

func (p *progressPrinter) step(processed, total int) {
	if !p.live {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	p.processed, p.total = processed, total
	if time.Since(p.lastDraw) < 100*time.Millisecond && processed < total {
		return
	}
	p.draw()
}

func (p *progressPrinter) clear() {
	if p.drawn {
		fmt.Fprint(os.Stderr, "\r\033[K")
		p.drawn = false
	}
}

func (p *progressPrinter) draw() {
	if !p.live || p.total == 0 {
		return
	}
	const width = 28
	filled := width * p.processed / p.total
	bar := strings.Repeat("█", filled) + strings.Repeat("░", width-filled)
	fmt.Fprintf(os.Stderr, "\r\033[K%s %3d%%  %d/%d lookups", bar, 100*p.processed/p.total, p.processed, p.total)
	p.drawn = true
	p.lastDraw = time.Now()
}

func (p *progressPrinter) finish() {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.drawn {
		fmt.Fprintln(os.Stderr)
		p.drawn = false
	}
}

func regionLabel(r orbis.Result) string {
	if r.Region.Name == "" {
		return "R3D"
	}
	return r.Region.Name
}
