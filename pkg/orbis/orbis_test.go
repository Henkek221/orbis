package orbis

import (
	"math"
	"os"
	"path/filepath"
	"testing"

	"github.com/Henkek221/orbis/pkg/mth"
)

func TestSlugifyKeepsHumanPlaceNamesSafe(t *testing.T) {
	got := Slugify("  København H — 2026-07-13  ")
	want := "københavn-h-2026-07-13"
	if got != want {
		t.Fatalf("Slugify() = %q, want %q", got, want)
	}
}

func TestSmartExportNameFallsBackToCoordinates(t *testing.T) {
	got := ExportName("", Bounds{
		North: 34.01,
		South: 34.00,
		West:  -118.50,
		East:  -118.49,
	})
	if got == "" || got[:5] != "area-" {
		t.Fatalf("ExportName() = %q, want an area fallback", got)
	}
}

func TestAvailableOutputPathAvoidsOverwriting(t *testing.T) {
	root := t.TempDir()
	first := filepath.Join(root, "santa-monica-pier")
	if err := os.Mkdir(first, 0o755); err != nil {
		t.Fatal(err)
	}

	got, err := AvailableOutputPath(root, "santa-monica-pier")
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(root, "santa-monica-pier-2")
	if got != want {
		t.Fatalf("AvailableOutputPath() = %q, want %q", got, want)
	}
}

func TestGeoDistanceMeters(t *testing.T) {
	got := GeoDistanceMetres(0, 0, 0, 1)
	if got < 111_000 || got > 112_000 {
		t.Fatalf("GeoDistanceMetres() = %f, want about 111.2 km", got)
	}
}

func TestRequestCountIncludesHeightLayers(t *testing.T) {
	req := Request{
		Bounds:       Bounds{North: 34.010, South: 34.000, West: -118.500, East: -118.490},
		Zoom:         18,
		HeightLayers: 12,
	}
	xWest, yNorth := mth.LatLonToTileTMS(req.Zoom, req.Bounds.North, req.Bounds.West)
	xEast, ySouth := mth.LatLonToTileTMS(req.Zoom, req.Bounds.South, req.Bounds.East)
	xMin, xMax := minMax(xWest, xEast)
	yMin, yMax := minMax(ySouth, yNorth)
	want := (xMax - xMin + 1) * (yMax - yMin + 1) * req.HeightLayers
	if got := req.RequestCount(); got != want {
		t.Fatalf("RequestCount() = %d, want %d", got, want)
	}
}

func TestConfiguredMaxRequestsCanBeRaised(t *testing.T) {
	t.Setenv("ORBIS_MAX_REQUESTS", "750000")
	if got := MaxRequestsFromEnv(); got != 750000 {
		t.Fatalf("MaxRequestsFromEnv() = %d, want 750000", got)
	}
}

func TestCoverageTileFromTMSContainsSourcePoint(t *testing.T) {
	x, y := 0, 0
	tile := TileBounds(1, x, y)
	if tile.West != -180 || tile.East != 0 || math.Abs(tile.South-(-85.0511287798066)) > 1e-12 || tile.North != 0 {
		t.Fatalf("TileBounds() = %#v", tile)
	}
}
