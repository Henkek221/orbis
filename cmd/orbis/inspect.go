// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Henrik Hellpap. Part of Orbis: https://github.com/Henkek221/orbis

package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/Henkek221/orbis/pkg/orbis"
)

func runCoverage(args []string) int {
	fs := flag.NewFlagSet("coverage", flag.ContinueOnError)
	var area areaFlags
	area.register(fs)
	home := fs.String("home", "", "data directory (config.json, cache)")
	zoom := fs.Int("zoom", 18, "tile level 13-20")
	heights := fs.Int("heights", orbis.DefaultHeightLayers, "octree height layers to check")
	maxTiles := fs.Int("max-tiles", 1024, "refuse larger scans")
	jsonOut := fs.Bool("json", false, "print the result as JSON")
	if _, err := parseArgs(fs, args); err != nil {
		return 2
	}
	ctx, stop := signalContext()
	defer stop()

	bounds, _, err := area.resolve(ctx)
	if err != nil {
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
	lat, lon := bounds.Center()
	region, regionErr := client.Region(lat, lon)
	coverage, err := client.Coverage(ctx, bounds, orbis.CoverageOptions{Zoom: *zoom, HeightLayers: *heights, MaxTiles: *maxTiles})
	if err != nil {
		return fail(err)
	}

	if *jsonOut {
		enc := json.NewEncoder(stdout)
		enc.SetIndent("", "  ")
		_ = enc.Encode(map[string]any{"region": region, "coverage": coverage})
		return 0
	}

	if regionErr != nil {
		fmt.Fprintln(stdout, "region:   none —", regionErr)
	} else {
		fmt.Fprintf(stdout, "region:   %s (id %d, version %d)\n", region.Name, region.ID, region.Version)
	}
	r := coverage.Range
	fmt.Fprintf(stdout, "tiles:    %d at zoom %d, %d with 3D, %d unknown\n\n", len(coverage.Tiles), r.Zoom, coverage.Available, coverage.Unknown)

	// TMS y grows northwards, so print the top row first.
	state := make(map[[2]int]string, len(coverage.Tiles))
	for _, t := range coverage.Tiles {
		switch {
		case !t.Known:
			state[[2]int{t.X, t.Y}] = "?"
		case t.Available:
			state[[2]int{t.X, t.Y}] = "█"
		default:
			state[[2]int{t.X, t.Y}] = "·"
		}
	}
	for y := r.YMax; y >= r.YMin; y-- {
		var row strings.Builder
		row.WriteString("  ")
		for x := r.XMin; x <= r.XMax; x++ {
			row.WriteString(state[[2]int{x, y}])
			row.WriteString(" ")
		}
		fmt.Fprintln(stdout, row.String())
	}
	fmt.Fprintln(stdout, "\n  █ 3D mesh   · none   ? index unreadable (often a newer region format)")
	if coverage.Unknown > 0 && coverage.Available == 0 {
		fmt.Fprintln(os.Stderr, "\norbis: this region's index could not be read; it is probably served in Apple's newer format")
	}
	return 0
}

func runSearch(args []string) int {
	fs := flag.NewFlagSet("search", flag.ContinueOnError)
	limit := fs.Int("limit", 5, "number of results")
	jsonOut := fs.Bool("json", false, "print the result as JSON")
	positional, err := parseArgs(fs, args)
	if err != nil {
		return 2
	}
	query := strings.TrimSpace(strings.Join(positional, " "))
	if len([]rune(query)) < 2 {
		return fail(fmt.Errorf("usage: orbis search <place>"))
	}
	ctx, stop := signalContext()
	defer stop()
	places, err := orbis.Search(ctx, query, *limit)
	if err != nil {
		return fail(err)
	}
	if *jsonOut {
		enc := json.NewEncoder(stdout)
		enc.SetIndent("", "  ")
		_ = enc.Encode(places)
		return 0
	}
	if len(places) == 0 {
		fmt.Fprintln(os.Stderr, "no results")
		return 3
	}
	for _, p := range places {
		fmt.Fprintf(stdout, "%.6f,%.6f  %s\n", p.Lat, p.Lon, p.DisplayName)
	}
	return 0
}

func runRegions(args []string) int {
	fs := flag.NewFlagSet("regions", flag.ContinueOnError)
	home := fs.String("home", "", "data directory (config.json, cache)")
	jsonOut := fs.Bool("json", false, "print the result as JSON")
	if _, err := parseArgs(fs, args); err != nil {
		return 2
	}
	p, err := homePaths(*home)
	if err != nil {
		return fail(err)
	}
	client, err := openClient(p)
	if err != nil {
		return fail(err)
	}
	regions := client.Regions()
	sort.Slice(regions, func(i, j int) bool { return regions[i].Name < regions[j].Name })
	if *jsonOut {
		enc := json.NewEncoder(stdout)
		enc.SetIndent("", "  ")
		_ = enc.Encode(regions)
		return 0
	}
	for _, r := range regions {
		fmt.Fprintf(stdout, "%-28s v%-5d %9.4f,%-10.4f r=%.0f km\n", r.Name, r.Version, r.Lat, r.Lon, r.Radius/1000)
	}
	fmt.Fprintf(stdout, "%d regions\n", len(regions))
	return 0
}
