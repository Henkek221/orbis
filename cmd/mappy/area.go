package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"strconv"
	"strings"

	"github.com/Henkek221/mappy/pkg/mappy"
)

// areaFlags is the shared way to describe an area on the command line.
type areaFlags struct {
	bbox   string
	center string
	place  string
	size   string
}

func (a *areaFlags) register(fs *flag.FlagSet) {
	fs.StringVar(&a.bbox, "bbox", "", "area as W,S,E,N in degrees")
	fs.StringVar(&a.center, "center", "", "area centre as LAT,LON")
	fs.StringVar(&a.place, "place", "", "area centre from a place search")
	fs.StringVar(&a.size, "size", "250", "metres around --center/--place: 250 or 300x200 (width x height)")
}

// resolve returns the bounds and, for --place, the place's short name.
func (a *areaFlags) resolve(ctx context.Context) (mappy.Bounds, string, error) {
	set := 0
	for _, v := range []string{a.bbox, a.center, a.place} {
		if v != "" {
			set++
		}
	}
	if set != 1 {
		return mappy.Bounds{}, "", errors.New("give exactly one of --bbox, --center or --place")
	}

	if a.bbox != "" {
		v, err := floats(a.bbox, 4)
		if err != nil {
			return mappy.Bounds{}, "", fmt.Errorf("--bbox: %w", err)
		}
		b := mappy.Bounds{West: v[0], South: v[1], East: v[2], North: v[3]}
		return b, "", b.Validate()
	}

	width, height, err := parseSize(a.size)
	if err != nil {
		return mappy.Bounds{}, "", err
	}
	if a.center != "" {
		v, err := floats(a.center, 2)
		if err != nil {
			return mappy.Bounds{}, "", fmt.Errorf("--center: %w", err)
		}
		b := mappy.BoundsAround(v[0], v[1], width, height)
		return b, "", b.Validate()
	}

	places, err := mappy.Search(ctx, a.place, 1)
	if err != nil {
		return mappy.Bounds{}, "", fmt.Errorf("place search: %w", err)
	}
	if len(places) == 0 {
		return mappy.Bounds{}, "", fmt.Errorf("no place found for %q", a.place)
	}
	p := places[0]
	b := mappy.BoundsAround(p.Lat, p.Lon, width, height)
	return b, p.Name, b.Validate()
}

func parseSize(value string) (width, height float64, err error) {
	w, h, found := strings.Cut(strings.ToLower(strings.TrimSpace(value)), "x")
	width, err = strconv.ParseFloat(strings.TrimSuffix(w, "m"), 64)
	if err != nil {
		return 0, 0, fmt.Errorf("--size: %q is not a number of metres", value)
	}
	height = width
	if found {
		height, err = strconv.ParseFloat(strings.TrimSuffix(h, "m"), 64)
		if err != nil {
			return 0, 0, fmt.Errorf("--size: %q is not WIDTHxHEIGHT", value)
		}
	}
	if width < 10 || height < 10 || width > 20000 || height > 20000 {
		return 0, 0, errors.New("--size must be between 10 and 20000 metres")
	}
	return width, height, nil
}

func floats(value string, n int) ([]float64, error) {
	parts := strings.Split(value, ",")
	if len(parts) != n {
		return nil, fmt.Errorf("want %d comma-separated numbers, got %q", n, value)
	}
	out := make([]float64, n)
	for i, part := range parts {
		v, err := strconv.ParseFloat(strings.TrimSpace(part), 64)
		if err != nil {
			return nil, fmt.Errorf("%q is not a number", part)
		}
		out[i] = v
	}
	return out, nil
}
