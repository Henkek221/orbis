// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Henrik Hellpap. Part of Orbis: https://github.com/Henkek221/orbis

// Package server is Orbis's local browser UI: a small HTTP API around
// pkg/orbis plus the embedded static frontend.
package server

import (
	"context"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Henkek221/orbis/pkg/orbis"
)

//go:embed static/*
var embeddedStatic embed.FS

// Data lives relative to the working directory, which Run selects.
const (
	configPath = "config.json"
	cacheDir   = "cache"
)

var exportRoot = filepath.Join("downloaded_files", "obj")

var maxRequests = orbis.MaxRequestsFromEnv()

var (
	jobsMu sync.Mutex
	jobs   = map[string]*job{}

	coverageMu     sync.Mutex
	coverageClient *orbis.Client
)

// Options configures the UI server.
type Options struct {
	// Addr is the listen address, ":8787" by default.
	Addr string
	// Home is the directory holding config.json, cache and exports. Empty
	// keeps the app-bundle behaviour: ORBIS_HOME, the working directory if it
	// has a config.json, else the folder next to the executable.
	Home string
	// OpenBrowser opens the UI once the server listens.
	OpenBrowser bool
}

// Run serves the UI until the listener fails.
func Run(opts Options) error {
	if opts.Home != "" {
		if err := os.MkdirAll(opts.Home, 0o755); err != nil {
			return err
		}
		if err := os.Chdir(opts.Home); err != nil {
			return err
		}
	} else {
		configureWorkingDirectory()
	}
	restoreCompletedExports(exportRoot)

	staticRoot, err := fs.Sub(embeddedStatic, "static")
	if err != nil {
		return err
	}

	mux := http.NewServeMux()
	mux.Handle("/", http.FileServer(http.FS(staticRoot)))
	mux.HandleFunc("POST /api/export", createExport)
	mux.HandleFunc("GET /api/jobs/{id}", getJob)
	mux.HandleFunc("GET /api/jobs/{id}/files/{name}", serveJobFile)
	mux.HandleFunc("GET /api/search", searchPlaces)
	mux.HandleFunc("GET /api/place-name", reversePlaceName)
	mux.HandleFunc("GET /api/coverage", getCoverage)
	mux.HandleFunc("GET /api/settings", getSettings)
	mux.HandleFunc("POST /api/setup/config", saveSetupConfig)
	mux.HandleFunc("POST /api/setup/scan", scanSetupFile)

	addr := opts.Addr
	if addr == "" {
		addr = ":8787"
	}
	appURL := "http://localhost" + addr
	if strings.HasPrefix(addr, "localhost:") || strings.HasPrefix(addr, "127.0.0.1:") {
		appURL = "http://" + addr
	}
	if wd, err := os.Getwd(); err == nil {
		log.Printf("Orbis data: %s", wd)
	}
	log.Printf("Orbis listening on %s", appURL)
	if opts.OpenBrowser {
		go func() {
			time.Sleep(450 * time.Millisecond)
			if err := openBrowser(appURL); err != nil {
				log.Printf("open %s in your browser", appURL)
			}
		}()
	}
	return http.ListenAndServe(addr, mux)
}

type exportRequest struct {
	Name     string  `json:"name"`
	North    float64 `json:"north"`
	South    float64 `json:"south"`
	East     float64 `json:"east"`
	West     float64 `json:"west"`
	Zoom     int     `json:"zoom"`
	TryH     int     `json:"tryH"`
	Parallel bool    `json:"parallel"`

	GLB       bool    `json:"glb"`
	USD       bool    `json:"usd"`
	Splats    bool    `json:"splats"`
	OBJ       bool    `json:"obj"`
	Minecraft bool    `json:"minecraft"`
	BlockSize float64 `json:"blockSize"`
}

func (req exportRequest) toLibrary() orbis.Request {
	parallel := 1
	if req.Parallel {
		parallel = orbis.DefaultParallel
	}
	return orbis.Request{
		Name:         req.Name,
		Bounds:       orbis.Bounds{North: req.North, South: req.South, East: req.East, West: req.West},
		Zoom:         req.Zoom,
		HeightLayers: req.TryH,
		Parallel:     parallel,
		GLB:          req.GLB,
		USD:          req.USD,
		Splats:       req.Splats,
		OBJ:          req.OBJ,
		Minecraft:    req.Minecraft,
		BlockSize:    req.BlockSize,
		OutputRoot:   exportRoot,
		MaxRequests:  maxRequests,
	}.Normalize()
}

type exportResponse struct {
	ID string `json:"id"`
}

type job struct {
	ID        string   `json:"id"`
	Name      string   `json:"name"`
	State     string   `json:"state"`
	Error     string   `json:"error,omitempty"`
	Output    string   `json:"output,omitempty"`
	World     string   `json:"world,omitempty"`
	Exported  int      `json:"exported"`
	Processed int      `json:"processed"`
	Total     int      `json:"total"`
	Logs      []string `json:"logs"`
	Started   string   `json:"started"`
	Ended     string   `json:"ended,omitempty"`

	mu sync.Mutex
}

func restoreCompletedExports(root string) {
	entries, err := os.ReadDir(root)
	if err != nil {
		if !os.IsNotExist(err) {
			log.Printf("existing exports could not be indexed: %v", err)
		}
		return
	}
	for _, entry := range entries {
		if !entry.IsDir() || filepath.Base(entry.Name()) != entry.Name() {
			continue
		}
		output := filepath.Join(root, entry.Name())
		modified, world, ok := completedExport(output)
		if !ok {
			continue
		}
		jobsMu.Lock()
		if jobs[entry.Name()] == nil {
			jobs[entry.Name()] = &job{
				ID:      entry.Name(),
				Name:    entry.Name(),
				State:   "done",
				Output:  output,
				World:   world,
				Started: modified.Format(time.RFC3339),
				Ended:   modified.Format(time.RFC3339),
			}
		}
		jobsMu.Unlock()
	}
}

// completedExport reports whether a directory holds a finished export: either a
// non-empty OBJ pair or a Minecraft world with a level.dat.
func completedExport(output string) (time.Time, string, bool) {
	objInfo, objErr := os.Stat(filepath.Join(output, "exp_model.obj"))
	mtlInfo, mtlErr := os.Stat(filepath.Join(output, "exp_model.mtl"))
	hasOBJ := objErr == nil && mtlErr == nil && objInfo.Size() > 0 && mtlInfo.Size() > 0

	world := ""
	var worldModified time.Time
	if matches, _ := filepath.Glob(filepath.Join(output, "*-minecraft", "level.dat")); len(matches) > 0 {
		if info, err := os.Stat(matches[0]); err == nil {
			world = filepath.Dir(matches[0])
			worldModified = info.ModTime()
		}
	}
	if !hasOBJ && world == "" {
		return time.Time{}, "", false
	}

	modified := worldModified
	if hasOBJ {
		modified = objInfo.ModTime()
		if mtlInfo.ModTime().After(modified) {
			modified = mtlInfo.ModTime()
		}
		if worldModified.After(modified) {
			modified = worldModified
		}
	}
	return modified, world, true
}

func configureWorkingDirectory() {
	if root := strings.TrimSpace(os.Getenv("ORBIS_HOME")); root != "" {
		if err := os.Chdir(root); err != nil {
			log.Printf("ORBIS_HOME could not be opened: %v", err)
		}
		return
	}
	if _, err := os.Stat(configPath); err == nil {
		return
	}
	executable, err := os.Executable()
	if err != nil {
		return
	}
	dir := filepath.Dir(executable)
	candidates := []string{
		dir,
		filepath.Dir(dir),
		filepath.Dir(filepath.Dir(dir)),
		filepath.Dir(filepath.Dir(filepath.Dir(dir))),
	}
	for _, candidate := range candidates {
		if _, err := os.Stat(filepath.Join(candidate, configPath)); err == nil {
			if err := os.Chdir(candidate); err == nil {
				return
			}
		}
	}

	// Finder starts unsigned app bundles with an implementation-defined working
	// directory. Keep cache and exports beside the app even when config.json is
	// no longer needed on macOS.
	fallback := dir
	if runtime.GOOS == "darwin" && filepath.Base(dir) == "MacOS" {
		fallback = filepath.Dir(filepath.Dir(filepath.Dir(dir)))
	}
	if err := os.Chdir(fallback); err != nil {
		log.Printf("app data directory could not be opened: %v", err)
	}
}

func createExport(w http.ResponseWriter, r *http.Request) {
	var body exportRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	req := body.toLibrary()
	if err := req.Validate(); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	id := time.Now().UTC().Format("20060102-150405")
	j := &job{
		ID: id, Name: orbis.ExportName(req.Name, req.Bounds), State: "running",
		Started: time.Now().Format(time.RFC3339), Total: req.RequestCount(),
	}
	jobsMu.Lock()
	jobs[id] = j
	jobsMu.Unlock()

	go runExport(j, req)
	writeJSON(w, http.StatusAccepted, exportResponse{ID: id})
}

func getJob(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	jobsMu.Lock()
	j := jobs[id]
	jobsMu.Unlock()
	if j == nil {
		writeError(w, http.StatusNotFound, "job not found")
		return
	}

	j.mu.Lock()
	defer j.mu.Unlock()
	writeJSON(w, http.StatusOK, j)
}

func serveJobFile(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	name := r.PathValue("name")
	if name == "" || filepath.Base(name) != name {
		writeError(w, http.StatusBadRequest, "invalid file name")
		return
	}
	jobsMu.Lock()
	j := jobs[id]
	jobsMu.Unlock()
	if j == nil {
		writeError(w, http.StatusNotFound, "job not found")
		return
	}
	j.mu.Lock()
	output := j.Output
	state := j.State
	j.mu.Unlock()
	if output == "" || state != "done" {
		writeError(w, http.StatusConflict, "export is not ready")
		return
	}
	allowed := map[string]bool{
		".obj": true, ".mtl": true, ".jpg": true, ".jpeg": true, ".png": true, ".heic": true,
	}
	if !allowed[strings.ToLower(filepath.Ext(name))] {
		writeError(w, http.StatusForbidden, "file type is not viewable")
		return
	}
	path := filepath.Join(output, name)
	if info, err := os.Stat(path); err != nil || info.IsDir() {
		writeError(w, http.StatusNotFound, "file not found")
		return
	}
	http.ServeFile(w, r, path)
}

func runExport(j *job, req orbis.Request) {
	result, err := exportWith(req, orbis.Progress{
		Log: func(msg string) {
			j.mu.Lock()
			j.Logs = append(j.Logs, msg)
			j.mu.Unlock()
		},
		Step: func(processed, total int) {
			j.mu.Lock()
			j.Processed = processed
			j.Total = total
			j.mu.Unlock()
		},
	})

	j.mu.Lock()
	defer j.mu.Unlock()
	j.Exported = result.Exported
	j.Output = result.Output
	j.World = result.World
	j.Ended = time.Now().Format(time.RFC3339)
	if err != nil {
		j.State = "error"
		j.Error = err.Error()
		j.Logs = append(j.Logs, "error: "+err.Error())
		return
	}
	j.State = "done"
	j.Processed = j.Total
	j.Logs = append(j.Logs, fmt.Sprintf("done: %d exported", result.Exported))
}

func exportWith(req orbis.Request, progress orbis.Progress) (orbis.Result, error) {
	client, err := orbis.Open(orbis.Options{ConfigPath: configPath, CacheDir: cacheDir})
	if err != nil {
		return orbis.Result{}, err
	}
	return client.Export(context.Background(), req, progress)
}

func getSettings(w http.ResponseWriter, _ *http.Request) {
	cfg, err := orbis.LoadConfig(configPath)
	ready := err == nil && cfg.IsValid()
	_, fileErr := os.Stat(configPath)
	configMode := "setup"
	if ready && fileErr == nil {
		configMode = "file"
	} else if ready {
		configMode = "automatic"
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"maxRequests": maxRequests,
		"configReady": ready,
		"platform":    runtime.GOOS,
		"configMode":  configMode,
	})
}

func saveSetupConfig(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 64*1024)
	defer r.Body.Close()

	supplied, err := orbis.ParseConfig(r.Body)
	if err != nil {
		writeError(w, http.StatusBadRequest, "Das ist keine gültige config.json für Orbis.")
		return
	}
	if err := validateSetupConfig(supplied); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if _, err := orbis.Connect(supplied, cacheDir); err != nil {
		writeError(w, http.StatusBadRequest, "Die Konfiguration konnte keine Verbindung zum 3D-Kartendienst herstellen.")
		return
	}
	if err := writeSetupConfig(configPath, supplied); err != nil {
		writeError(w, http.StatusInternalServerError, "Orbis konnte die Konfiguration nicht lokal speichern.")
		return
	}
	resetCoverageClient()
	writeJSON(w, http.StatusOK, map[string]any{"ready": true})
}

// scanSetupFile reads the static Apple client value straight out of a local
// GeoServices binary. That is what makes setup possible without a Mac and
// without someone handing over a finished config.json.
func scanSetupFile(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 8*1024)
	defer r.Body.Close()

	var request struct {
		Path string `json:"path"`
	}
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		writeError(w, http.StatusBadRequest, "Es wurde kein Dateipfad übergeben.")
		return
	}
	path := strings.Trim(strings.TrimSpace(request.Path), `"'`)
	if path == "" {
		writeError(w, http.StatusBadRequest, "Es wurde kein Dateipfad übergeben.")
		return
	}
	file, err := os.Open(path)
	if err != nil {
		writeError(w, http.StatusBadRequest, "Diese Datei konnte nicht geöffnet werden.")
		return
	}
	defer file.Close()

	token := orbis.ScanToken(file)
	if token == "" {
		writeError(w, http.StatusBadRequest, "In dieser Datei steckt kein Apple-Clientwert. Nutze die GeoServices-Datei.")
		return
	}

	supplied := orbis.ConfigFromToken(token)
	if _, err := orbis.Connect(supplied, cacheDir); err != nil {
		writeError(w, http.StatusBadRequest, "Der gefundene Wert wurde vom 3D-Kartendienst nicht akzeptiert.")
		return
	}
	if err := writeSetupConfig(configPath, supplied); err != nil {
		writeError(w, http.StatusInternalServerError, "Orbis konnte die Konfiguration nicht lokal speichern.")
		return
	}
	resetCoverageClient()
	writeJSON(w, http.StatusOK, map[string]any{"ready": true})
}

// validateSetupConfig is orbis.ValidateConfig with the German wording of the
// setup gate.
func validateSetupConfig(cfg orbis.Config) error {
	switch err := orbis.ValidateConfig(cfg); {
	case errors.Is(err, orbis.ErrManifestURL):
		return errors.New("Die Manifest-URL muss eine gültige Apple-HTTPS-Adresse sein.")
	case errors.Is(err, orbis.ErrToken):
		return errors.New("Die config.json enthält keinen gültigen Clientwert.")
	default:
		return err
	}
}

func writeSetupConfig(path string, cfg orbis.Config) error {
	return orbis.SaveConfig(path, cfg)
}

func searchPlaces(w http.ResponseWriter, r *http.Request) {
	query := strings.TrimSpace(r.URL.Query().Get("q"))
	if len([]rune(query)) < 2 {
		writeError(w, http.StatusBadRequest, "search needs at least two characters")
		return
	}
	if len([]rune(query)) > 160 {
		writeError(w, http.StatusBadRequest, "search is too long")
		return
	}
	places, err := orbis.Search(r.Context(), query, 5)
	if err != nil {
		writeError(w, http.StatusBadGateway, "place search is unavailable")
		return
	}
	writeJSON(w, http.StatusOK, places)
}

func reversePlaceName(w http.ResponseWriter, r *http.Request) {
	lat, errLat := strconv.ParseFloat(r.URL.Query().Get("lat"), 64)
	lon, errLon := strconv.ParseFloat(r.URL.Query().Get("lon"), 64)
	if errLat != nil || errLon != nil || lat < -85 || lat > 85 || lon < -180 || lon > 180 {
		writeError(w, http.StatusBadRequest, "invalid coordinates")
		return
	}
	place, err := orbis.Reverse(r.Context(), lat, lon)
	if err != nil {
		writeError(w, http.StatusBadGateway, "place name is unavailable")
		return
	}
	writeJSON(w, http.StatusOK, place)
}

func getCoverage(w http.ResponseWriter, r *http.Request) {
	north, errNorth := strconv.ParseFloat(r.URL.Query().Get("north"), 64)
	south, errSouth := strconv.ParseFloat(r.URL.Query().Get("south"), 64)
	east, errEast := strconv.ParseFloat(r.URL.Query().Get("east"), 64)
	west, errWest := strconv.ParseFloat(r.URL.Query().Get("west"), 64)
	zoom, errZoom := strconv.Atoi(r.URL.Query().Get("zoom"))
	heightLayers, errHeight := strconv.Atoi(r.URL.Query().Get("height"))
	bounds := orbis.Bounds{North: north, South: south, East: east, West: west}
	if errNorth != nil || errSouth != nil || errEast != nil || errWest != nil || errZoom != nil ||
		bounds.Validate() != nil || zoom < 13 || zoom > 20 {
		writeError(w, http.StatusBadRequest, "invalid coverage bounds")
		return
	}
	if errHeight != nil || heightLayers < 1 {
		heightLayers = orbis.DefaultHeightLayers
	}
	if heightLayers > 64 {
		heightLayers = 64
	}
	if bounds.Tiles(zoom).Count() > orbis.DefaultCoverageMaxTiles {
		writeError(w, http.StatusServiceUnavailable, "zoom in to check detailed 3D coverage")
		return
	}
	client, err := loadCoverageClient()
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, "3D coverage is unavailable")
		return
	}
	coverage, err := client.Coverage(r.Context(), bounds, orbis.CoverageOptions{Zoom: zoom, HeightLayers: heightLayers})
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, err.Error())
		return
	}
	w.Header().Set("Cache-Control", "private, max-age=300")
	writeJSON(w, http.StatusOK, map[string]any{
		"tiles":          coverage.Tiles,
		"zoom":           zoom,
		"availableCount": coverage.Available,
		"unknownCount":   coverage.Unknown,
	})
}

func loadCoverageClient() (*orbis.Client, error) {
	coverageMu.Lock()
	defer coverageMu.Unlock()
	if coverageClient != nil {
		return coverageClient, nil
	}
	client, err := orbis.Open(orbis.Options{ConfigPath: configPath, CacheDir: cacheDir})
	if err != nil {
		return nil, err
	}
	coverageClient = client
	return client, nil
}

func resetCoverageClient() {
	coverageMu.Lock()
	coverageClient = nil
	coverageMu.Unlock()
}

func openBrowser(target string) error {
	var command *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		command = exec.Command("open", target)
	case "windows":
		command = exec.Command("rundll32", "url.dll,FileProtocolHandler", target)
	default:
		command = exec.Command("xdg-open", target)
	}
	return command.Start()
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}
