package main

import (
	"math"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/christiangda/flyover-reverse-engineering/pkg/mps/config"
	"github.com/christiangda/flyover-reverse-engineering/pkg/mth"
)

func TestSlugifyKeepsHumanPlaceNamesSafe(t *testing.T) {
	got := slugify("  København H — 2026-07-13  ")
	want := "københavn-h-2026-07-13"
	if got != want {
		t.Fatalf("slugify() = %q, want %q", got, want)
	}
}

func TestSmartExportNameFallsBackToCoordinates(t *testing.T) {
	got := smartExportName(exportRequest{
		North: 34.01,
		South: 34.00,
		West:  -118.50,
		East:  -118.49,
	})
	if got == "" || got[:5] != "area-" {
		t.Fatalf("smartExportName() = %q, want an area fallback", got)
	}
}

func TestAvailableOutputPathAvoidsOverwriting(t *testing.T) {
	root := t.TempDir()
	first := filepath.Join(root, "santa-monica-pier")
	if err := os.Mkdir(first, 0o755); err != nil {
		t.Fatal(err)
	}

	got, err := availableOutputPath(root, "santa-monica-pier")
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(root, "santa-monica-pier-2")
	if got != want {
		t.Fatalf("availableOutputPath() = %q, want %q", got, want)
	}
}

func TestGeoDistanceMeters(t *testing.T) {
	got := geoDistanceMeters(0, 0, 0, 1)
	if got < 111_000 || got > 112_000 {
		t.Fatalf("geoDistanceMeters() = %f, want about 111.2 km", got)
	}
}

func TestRequestCountIncludesHeightLayers(t *testing.T) {
	req := exportRequest{
		North: 34.010,
		South: 34.000,
		West:  -118.500,
		East:  -118.490,
		Zoom:  18,
		TryH:  12,
	}
	xWest, yNorth := mth.LatLonToTileTMS(req.Zoom, req.North, req.West)
	xEast, ySouth := mth.LatLonToTileTMS(req.Zoom, req.South, req.East)
	xMin, xMax := minMax(xWest, xEast)
	yMin, yMax := minMax(ySouth, yNorth)
	want := (xMax - xMin + 1) * (yMax - yMin + 1) * req.TryH
	if got := requestCount(req); got != want {
		t.Fatalf("requestCount() = %d, want %d", got, want)
	}
}

func TestConfiguredMaxRequestsCanBeRaised(t *testing.T) {
	t.Setenv("MAPPY_MAX_REQUESTS", "750000")
	if got := configuredMaxRequests(); got != 750000 {
		t.Fatalf("configuredMaxRequests() = %d, want 750000", got)
	}
}

func TestCoverageTileFromTMSContainsSourcePoint(t *testing.T) {
	x, y := 0, 0
	tile := coverageTileFromTMS(1, x, y)
	if tile.West != -180 || tile.East != 0 || math.Abs(tile.South-(-85.0511287798066)) > 1e-12 || tile.North != 0 {
		t.Fatalf("coverageTileFromTMS() = %#v", tile)
	}
}

func TestServeJobFileOnlyAfterCompletedExport(t *testing.T) {
	root := t.TempDir()
	obj := []byte("o preview\nv 0 0 0\n")
	if err := os.WriteFile(filepath.Join(root, "exp_model.obj"), obj, 0o644); err != nil {
		t.Fatal(err)
	}

	const id = "viewer-test"
	jobsMu.Lock()
	jobs[id] = &job{ID: id, State: "done", Output: root}
	jobsMu.Unlock()
	t.Cleanup(func() {
		jobsMu.Lock()
		delete(jobs, id)
		jobsMu.Unlock()
	})

	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/jobs/{id}/files/{name}", serveJobFile)
	recorder := httptest.NewRecorder()
	mux.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/jobs/viewer-test/files/exp_model.obj", nil))

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d: %s", recorder.Code, http.StatusOK, recorder.Body.String())
	}
	if recorder.Body.String() != string(obj) {
		t.Fatalf("body = %q, want %q", recorder.Body.String(), obj)
	}
}

func TestServeJobFileRejectsUnsupportedAssets(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "secret.json"), []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}

	const id = "viewer-forbidden"
	jobsMu.Lock()
	jobs[id] = &job{ID: id, State: "done", Output: root}
	jobsMu.Unlock()
	t.Cleanup(func() {
		jobsMu.Lock()
		delete(jobs, id)
		jobsMu.Unlock()
	})

	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/jobs/{id}/files/{name}", serveJobFile)
	recorder := httptest.NewRecorder()
	mux.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/jobs/viewer-forbidden/files/secret.json", nil))

	if recorder.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusForbidden)
	}
}

func TestRestoreCompletedExportsMakesViewerAvailableAfterRestart(t *testing.T) {
	root := t.TempDir()
	output := filepath.Join(root, "city-square")
	if err := os.Mkdir(output, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(output, "exp_model.obj"), []byte("o preview\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(output, "exp_model.mtl"), []byte("newmtl preview\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	jobsMu.Lock()
	delete(jobs, "city-square")
	jobsMu.Unlock()
	restoreCompletedExports(root)
	t.Cleanup(func() {
		jobsMu.Lock()
		delete(jobs, "city-square")
		jobsMu.Unlock()
	})

	jobsMu.Lock()
	restored := jobs["city-square"]
	jobsMu.Unlock()
	if restored == nil || restored.State != "done" || restored.Output != output {
		t.Fatalf("restored job = %#v", restored)
	}
}

func TestValidateSetupConfig(t *testing.T) {
	valid := config.Config{
		ResourceManifestURL: config.DefaultResourceManifestURL(),
		TokenP1:             "LocalTestToken123",
	}
	if err := validateSetupConfig(valid); err != nil {
		t.Fatalf("valid config rejected: %v", err)
	}

	invalidHost := valid
	invalidHost.ResourceManifestURL = "https://example.com/manifest"
	if err := validateSetupConfig(invalidHost); err == nil {
		t.Fatal("expected a non-Apple manifest URL to be rejected")
	}

	invalidToken := valid
	invalidToken.TokenP1 = "short"
	if err := validateSetupConfig(invalidToken); err == nil {
		t.Fatal("expected a short token to be rejected")
	}
}

func TestWriteSetupConfigUsesPrivateFilePermissions(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	cfg := config.Config{
		ResourceManifestURL: config.DefaultResourceManifestURL(),
		TokenP1:             "LocalTestToken123",
	}
	if err := writeSetupConfig(path, cfg); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got != 0o600 {
		t.Fatalf("config permissions = %o, want 600", got)
	}
	loaded, err := config.FromJSONFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if loaded != cfg {
		t.Fatalf("saved config = %#v, want %#v", loaded, cfg)
	}
}
