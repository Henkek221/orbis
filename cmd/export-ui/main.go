package main

import (
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"math"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/christiangda/flyover-reverse-engineering/pkg/fly"
	"github.com/christiangda/flyover-reverse-engineering/pkg/fly/c3m"
	"github.com/christiangda/flyover-reverse-engineering/pkg/fly/c3mm"
	"github.com/christiangda/flyover-reverse-engineering/pkg/fly/exp"
	"github.com/christiangda/flyover-reverse-engineering/pkg/mc"
	"github.com/christiangda/flyover-reverse-engineering/pkg/mps"
	"github.com/christiangda/flyover-reverse-engineering/pkg/mps/config"
	"github.com/christiangda/flyover-reverse-engineering/pkg/mth"
	"github.com/christiangda/flyover-reverse-engineering/pkg/oth"
	"github.com/christiangda/flyover-reverse-engineering/pkg/web"
)

//go:embed static/*
var embeddedStatic embed.FS

const defaultMaxRequests = 200000

var maxRequests = configuredMaxRequests()

var geocodeClient = &http.Client{Timeout: 8 * time.Second}

var (
	jobsMu sync.Mutex
	jobs   = map[string]*job{}

	coverageMu   sync.Mutex
	coverageBase *flyContext
)

type exportRequest struct {
	Name     string  `json:"name"`
	North    float64 `json:"north"`
	South    float64 `json:"south"`
	East     float64 `json:"east"`
	West     float64 `json:"west"`
	Zoom     int     `json:"zoom"`
	TryH     int     `json:"tryH"`
	Parallel bool    `json:"parallel"`

	OBJ       bool    `json:"obj"`
	Minecraft bool    `json:"minecraft"`
	// BlockSize is metres per Minecraft block: 1 keeps real scale, 0.5 doubles
	// the detail and quadruples the block count.
	BlockSize float64 `json:"blockSize"`
}

type exportResponse struct {
	ID string `json:"id"`
}

type exportResult struct {
	Exported int
	Output   string
	World    string
}

// normalizeRequest fills in defaults so older clients keep getting an OBJ.
func normalizeRequest(req exportRequest) exportRequest {
	if !req.OBJ && !req.Minecraft {
		req.OBJ = true
	}
	switch {
	case req.BlockSize <= 0:
		req.BlockSize = 1
	case req.BlockSize < 0.25:
		req.BlockSize = 0.25
	case req.BlockSize > 4:
		req.BlockSize = 4
	}
	return req
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

type nominatimPlace struct {
	DisplayName string            `json:"display_name"`
	Name        string            `json:"name"`
	Lat         string            `json:"lat"`
	Lon         string            `json:"lon"`
	Address     map[string]string `json:"address"`
}

type placeResult struct {
	Name        string  `json:"name"`
	DisplayName string  `json:"displayName"`
	Lat         float64 `json:"lat"`
	Lon         float64 `json:"lon"`
}

type coverageTile struct {
	North     float64 `json:"north"`
	South     float64 `json:"south"`
	East      float64 `json:"east"`
	West      float64 `json:"west"`
	Available bool    `json:"available"`
	Known     bool    `json:"known"`
}

type flyContext struct {
	Context          *mps.Context
	AltitudeManifest fly.AltitudeManifest
	URLPrefixC3mm    string
	URLPrefixC3m     string
	C3mms            []*c3mm.C3MM
}

type scanStats struct {
	mu        sync.Mutex
	Attempted int
	Empty     int
	FetchErr  int
	ParseErr  int
}

func main() {
	configureWorkingDirectory()
	restoreCompletedExports(filepath.Join("downloaded_files", "obj"))

	staticRoot, err := fs.Sub(embeddedStatic, "static")
	if err != nil {
		log.Fatal(err)
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

	addr := ":8787"
	if v := os.Getenv("FLYOVER_UI_ADDR"); v != "" {
		addr = v
	}
	appURL := "http://localhost" + addr
	log.Printf("Mappy listening on %s", appURL)
	if os.Getenv("MAPPY_NO_BROWSER") == "" {
		go func() {
			time.Sleep(450 * time.Millisecond)
			if err := openBrowser(appURL); err != nil {
				log.Printf("open %s in your browser", appURL)
			}
		}()
	}
	log.Fatal(http.ListenAndServe(addr, mux))
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
	if root := strings.TrimSpace(os.Getenv("MAPPY_HOME")); root != "" {
		if err := os.Chdir(root); err != nil {
			log.Printf("MAPPY_HOME could not be opened: %v", err)
		}
		return
	}
	if _, err := os.Stat("config.json"); err == nil {
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
		if _, err := os.Stat(filepath.Join(candidate, "config.json")); err == nil {
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
	var req exportRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	req = normalizeRequest(req)
	if err := validateRequest(req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	id := time.Now().UTC().Format("20060102-150405")
	name := smartExportName(req)
	j := &job{
		ID: id, Name: name, State: "running", Started: time.Now().Format(time.RFC3339),
		Total: requestCount(req),
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

func runExport(j *job, req exportRequest) {
	result, err := exportBounds(req, func(msg string) {
		j.mu.Lock()
		j.Logs = append(j.Logs, msg)
		j.mu.Unlock()
	}, func(processed, total int) {
		j.mu.Lock()
		j.Processed = processed
		j.Total = total
		j.mu.Unlock()
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

func exportBounds(req exportRequest, logf func(string), progressf func(processed, total int)) (exportResult, error) {
	cache := mps.Cache{Enabled: true, Directory: "./cache"}
	if err := cache.Init(); err != nil {
		return exportResult{}, err
	}
	cfg, err := config.FromJSONFile("./config.json")
	if err != nil {
		return exportResult{}, err
	}
	if !cfg.IsValid() {
		return exportResult{}, errors.New("please set values in config.json")
	}

	ctx, err := getContext(cache, cfg)
	if err != nil {
		return exportResult{}, err
	}

	centerLat := (req.North + req.South) / 2
	centerLon := (req.West + req.East) / 2
	place, err := ctx.findPlace(centerLat, centerLon)
	if err != nil {
		return exportResult{}, err
	}
	logf(fmt.Sprintf("region: %s version=%d region=%d", place.Name, place.Version, place.Region))

	xWest, yNorth := mth.LatLonToTileTMS(req.Zoom, req.North, req.West)
	xEast, ySouth := mth.LatLonToTileTMS(req.Zoom, req.South, req.East)
	xMin, xMax := minMax(xWest, xEast)
	yMin, yMax := minMax(ySouth, yNorth)
	requests := requestCount(req)
	if requests > maxRequests {
		return exportResult{}, fmt.Errorf("selection would scan %d tile-height requests; current limit is %d (set MAPPY_MAX_REQUESTS to raise it)", requests, maxRequests)
	}
	progressf(0, requests)
	logf(fmt.Sprintf("tiles: x=%d..%d y=%d..%d zoom=%d h=0..%d (%d requests)", xMin, xMax, yMin, yMax, req.Zoom, req.TryH-1, requests))

	c3mmDir := fmt.Sprintf("./cache/c3mm/%d_%d", place.Region, place.Version)
	if err := os.MkdirAll(c3mmDir, 0o755); err != nil {
		return exportResult{}, err
	}

	name := smartExportName(req)
	output, err := availableOutputPath(filepath.Join("downloaded_files", "obj"), name)
	if err != nil {
		return exportResult{}, err
	}
	if err := os.MkdirAll(output, 0o755); err != nil {
		return exportResult{}, err
	}
	result := exportResult{Output: output}

	directScan := false
	if _, err = ctx.getC3mm(place, 0); err != nil {
		directScan = true
		logf("C3MM index unavailable; direct C3M scan enabled: " + err.Error())
	}

	var exporter multiExport
	if req.OBJ {
		objExport, err := exp.NewLocalized(output, "exp_", centerLat, centerLon)
		if err != nil {
			return result, err
		}
		exporter = append(exporter, objExport)
	}
	if req.Minecraft {
		levelName := strings.TrimSpace(req.Name)
		if levelName == "" {
			levelName = name
		}
		result.World = filepath.Join(output, name+"-minecraft")
		world, err := mc.New(result.World, centerLat, centerLon, mc.Options{
			Name:      levelName,
			BlockSize: req.BlockSize,
			Logf:      logf,
		})
		if err != nil {
			return result, err
		}
		logf(fmt.Sprintf("minecraft: writing a world at %.2f m per block to %s", req.BlockSize, result.World))
		if converter := oth.HEICConverter(); converter == "" {
			logf("minecraft: no HEIC converter found; install ImageMagick or ffmpeg if colours look flat")
		}
		exporter = append(exporter, world)
	}
	c3m.DisableLogs()

	parallelism := 1
	if req.Parallel {
		parallelism = 16
	}

	sem := make(chan struct{}, parallelism)
	tiles := make(chan c3m.C3M, parallelism)
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
		progressf(current, requests)
	}

	setErr := func(err error) {
		firstErrMu.Lock()
		defer firstErrMu.Unlock()
		if firstErr == nil {
			firstErr = err
		}
	}

	exported := 0
	done := make(chan struct{})
	go func() {
		defer close(done)
		for tile := range tiles {
			if err := exporter.Next(tile, strconv.Itoa(exported)); err != nil {
				setErr(err)
				continue
			}
			exported++
		}
	}()

	for x := xMin; x <= xMax; x++ {
		for y := yMin; y <= yMax; y++ {
			for h := 0; h < req.TryH; h++ {
				if !directScan {
					hasTile, err := ctx.checkTile(place, req.Zoom, y, x, h)
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

				sem <- struct{}{}
				wg.Add(1)
				x, y, h := x, y, h
				go func() {
					defer wg.Done()
					defer func() { <-sem }()
					defer markProcessed()
					tile, err := ctx.getTile(place, req.Zoom, y, x, h)
					stats.record(err)
					if err != nil {
						return
					}
					tiles <- tile
				}()
			}
		}
	}

	wg.Wait()
	close(tiles)
	<-done
	if req.Minecraft && exported > 0 {
		logf("minecraft: converting the mesh into blocks…")
	}
	closeErr := exporter.Close()
	result.Exported = exported
	stats.log(logf, exported)
	if firstErr != nil {
		return result, firstErr
	}
	if closeErr != nil {
		return result, closeErr
	}
	return result, nil
}

// multiExport feeds one tile stream into every requested output format.
type multiExport []exp.Export

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

func (s *scanStats) log(logf func(string), exported int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	logf(fmt.Sprintf("summary: attempted=%d exported=%d empty=%d fetch_errors=%d parse_errors=%d", s.Attempted, exported, s.Empty, s.FetchErr, s.ParseErr))
	if exported == 0 && s.Empty > 0 && s.Empty+s.FetchErr+s.ParseErr == s.Attempted {
		logf("no mesh tiles found in this rectangle; Apple returned empty C3M responses for the scanned area")
	}
}

func validateRequest(req exportRequest) error {
	if req.North <= req.South {
		return errors.New("north must be greater than south")
	}
	if req.East <= req.West {
		return errors.New("east must be greater than west")
	}
	if req.North > 85 || req.South < -85 || req.West < -180 || req.East > 180 {
		return errors.New("bounds are outside supported map range")
	}
	if req.Zoom < 13 || req.Zoom > 20 {
		return errors.New("zoom must be between 13 and 20")
	}
	if req.TryH < 1 || req.TryH > 128 {
		return errors.New("height scan must be between 1 and 128")
	}
	return nil
}

func requestCount(req exportRequest) int {
	xWest, yNorth := mth.LatLonToTileTMS(req.Zoom, req.North, req.West)
	xEast, ySouth := mth.LatLonToTileTMS(req.Zoom, req.South, req.East)
	xMin, xMax := minMax(xWest, xEast)
	yMin, yMax := minMax(ySouth, yNorth)
	return (xMax - xMin + 1) * (yMax - yMin + 1) * req.TryH
}

func configuredMaxRequests() int {
	raw := strings.TrimSpace(os.Getenv("MAPPY_MAX_REQUESTS"))
	if raw == "" {
		return defaultMaxRequests
	}
	value, err := strconv.Atoi(raw)
	if err != nil || value < 1 {
		log.Printf("invalid MAPPY_MAX_REQUESTS=%q; using %d", raw, defaultMaxRequests)
		return defaultMaxRequests
	}
	return value
}

func getSettings(w http.ResponseWriter, _ *http.Request) {
	cfg, err := config.FromJSONFile("./config.json")
	ready := err == nil && cfg.IsValid()
	_, fileErr := os.Stat("./config.json")
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

	var supplied config.Config
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&supplied); err != nil {
		writeError(w, http.StatusBadRequest, "Das ist keine gültige config.json für Mappy.")
		return
	}
	if strings.TrimSpace(supplied.ResourceManifestURL) == "" {
		supplied.ResourceManifestURL = config.DefaultResourceManifestURL()
	}
	supplied.TokenP1 = strings.TrimSpace(supplied.TokenP1)
	if err := validateSetupConfig(supplied); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	cache := mps.Cache{Enabled: true, Directory: "./cache"}
	if err := cache.Init(); err != nil {
		writeError(w, http.StatusInternalServerError, "Mappy konnte den lokalen Ordner nicht vorbereiten.")
		return
	}
	if _, err := getContext(cache, supplied); err != nil {
		writeError(w, http.StatusBadRequest, "Die Konfiguration konnte keine Verbindung zum 3D-Kartendienst herstellen.")
		return
	}
	if err := writeSetupConfig("./config.json", supplied); err != nil {
		writeError(w, http.StatusInternalServerError, "Mappy konnte die Konfiguration nicht lokal speichern.")
		return
	}

	coverageMu.Lock()
	coverageBase = nil
	coverageMu.Unlock()
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

	token := config.ScanTokenP1(file)
	if token == "" {
		writeError(w, http.StatusBadRequest, "In dieser Datei steckt kein Apple-Clientwert. Nutze die GeoServices-Datei.")
		return
	}

	supplied := config.Config{ResourceManifestURL: config.DefaultResourceManifestURL(), TokenP1: token}
	cache := mps.Cache{Enabled: true, Directory: "./cache"}
	if err := cache.Init(); err != nil {
		writeError(w, http.StatusInternalServerError, "Mappy konnte den lokalen Ordner nicht vorbereiten.")
		return
	}
	if _, err := getContext(cache, supplied); err != nil {
		writeError(w, http.StatusBadRequest, "Der gefundene Wert wurde vom 3D-Kartendienst nicht akzeptiert.")
		return
	}
	if err := writeSetupConfig("./config.json", supplied); err != nil {
		writeError(w, http.StatusInternalServerError, "Mappy konnte die Konfiguration nicht lokal speichern.")
		return
	}

	coverageMu.Lock()
	coverageBase = nil
	coverageMu.Unlock()
	writeJSON(w, http.StatusOK, map[string]any{"ready": true})
}

func validateSetupConfig(cfg config.Config) error {
	manifestURL, err := url.Parse(cfg.ResourceManifestURL)
	if err != nil || manifestURL.Scheme != "https" || !strings.HasSuffix(strings.ToLower(manifestURL.Hostname()), ".apple.com") {
		return errors.New("Die Manifest-URL muss eine gültige Apple-HTTPS-Adresse sein.")
	}
	if len(cfg.TokenP1) < 8 || len(cfg.TokenP1) > 256 {
		return errors.New("Die config.json enthält keinen gültigen Clientwert.")
	}
	return nil
}

func writeSetupConfig(path string, cfg config.Config) error {
	raw, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	raw = append(raw, '\n')
	temporary := path + ".tmp"
	if err := os.WriteFile(temporary, raw, 0o600); err != nil {
		return err
	}
	if err := os.Rename(temporary, path); err != nil {
		_ = os.Remove(temporary)
		return err
	}
	return nil
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

	params := url.Values{
		"q":              {query},
		"format":         {"jsonv2"},
		"limit":          {"5"},
		"addressdetails": {"1"},
	}
	var places []nominatimPlace
	if err := fetchNominatim("https://nominatim.openstreetmap.org/search?"+params.Encode(), &places); err != nil {
		writeError(w, http.StatusBadGateway, "place search is unavailable")
		return
	}
	writeJSON(w, http.StatusOK, normalizePlaces(places))
}

func reversePlaceName(w http.ResponseWriter, r *http.Request) {
	lat, errLat := strconv.ParseFloat(r.URL.Query().Get("lat"), 64)
	lon, errLon := strconv.ParseFloat(r.URL.Query().Get("lon"), 64)
	if errLat != nil || errLon != nil || lat < -85 || lat > 85 || lon < -180 || lon > 180 {
		writeError(w, http.StatusBadRequest, "invalid coordinates")
		return
	}
	params := url.Values{
		"lat":            {strconv.FormatFloat(lat, 'f', 7, 64)},
		"lon":            {strconv.FormatFloat(lon, 'f', 7, 64)},
		"format":         {"jsonv2"},
		"addressdetails": {"1"},
	}
	var place nominatimPlace
	if err := fetchNominatim("https://nominatim.openstreetmap.org/reverse?"+params.Encode(), &place); err != nil {
		writeError(w, http.StatusBadGateway, "place name is unavailable")
		return
	}
	results := normalizePlaces([]nominatimPlace{place})
	if len(results) == 0 {
		writeError(w, http.StatusNotFound, "place name not found")
		return
	}
	writeJSON(w, http.StatusOK, results[0])
}

func getCoverage(w http.ResponseWriter, r *http.Request) {
	north, errNorth := strconv.ParseFloat(r.URL.Query().Get("north"), 64)
	south, errSouth := strconv.ParseFloat(r.URL.Query().Get("south"), 64)
	east, errEast := strconv.ParseFloat(r.URL.Query().Get("east"), 64)
	west, errWest := strconv.ParseFloat(r.URL.Query().Get("west"), 64)
	zoom, errZoom := strconv.Atoi(r.URL.Query().Get("zoom"))
	heightLayers, errHeight := strconv.Atoi(r.URL.Query().Get("height"))
	if errNorth != nil || errSouth != nil || errEast != nil || errWest != nil || errZoom != nil ||
		north <= south || east <= west || north > 85 || south < -85 || east > 180 || west < -180 || zoom < 13 || zoom > 20 {
		writeError(w, http.StatusBadRequest, "invalid coverage bounds")
		return
	}
	if errHeight != nil || heightLayers < 1 {
		heightLayers = 24
	}
	if heightLayers > 64 {
		heightLayers = 64
	}
	tiles, available, unknown, err := scanCoverage(north, south, east, west, zoom, heightLayers)
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, err.Error())
		return
	}
	w.Header().Set("Cache-Control", "private, max-age=300")
	writeJSON(w, http.StatusOK, map[string]any{
		"tiles":          tiles,
		"zoom":           zoom,
		"availableCount": available,
		"unknownCount":   unknown,
	})
}

func loadCoverageContext() (flyContext, error) {
	coverageMu.Lock()
	defer coverageMu.Unlock()
	if coverageBase != nil {
		ctx := *coverageBase
		ctx.C3mms = nil
		return ctx, nil
	}

	cache := mps.Cache{Enabled: true, Directory: "./cache"}
	if err := cache.Init(); err != nil {
		return flyContext{}, err
	}
	cfg, err := config.FromJSONFile("./config.json")
	if err != nil {
		return flyContext{}, err
	}
	if !cfg.IsValid() {
		return flyContext{}, errors.New("please set values in config.json")
	}
	ctx, err := getContext(cache, cfg)
	if err != nil {
		return flyContext{}, err
	}
	coverageBase = &ctx
	result := ctx
	result.C3mms = nil
	return result, nil
}

func scanCoverage(north, south, east, west float64, zoom, heightLayers int) ([]coverageTile, int, int, error) {
	base, err := loadCoverageContext()
	if err != nil {
		return nil, 0, 0, errors.New("3D coverage is unavailable")
	}
	xWest, yNorth := mth.LatLonToTileTMS(zoom, north, west)
	xEast, ySouth := mth.LatLonToTileTMS(zoom, south, east)
	xMin, xMax := minMax(xWest, xEast)
	yMin, yMax := minMax(ySouth, yNorth)
	count := (xMax - xMin + 1) * (yMax - yMin + 1)
	if count > 196 {
		return nil, 0, 0, errors.New("zoom in to check detailed 3D coverage")
	}

	contexts := make(map[int]*flyContext)
	tiles := make([]coverageTile, 0, count)
	availableCount, unknownCount := 0, 0
	for x := xMin; x <= xMax; x++ {
		for y := yMin; y <= yMax; y++ {
			tile := coverageTileFromTMS(zoom, x, y)
			centerLat := (tile.North + tile.South) / 2
			centerLon := (tile.East + tile.West) / 2
			place, err := base.findPlace(centerLat, centerLon)
			if err != nil {
				tile.Known = true
				tiles = append(tiles, tile)
				continue
			}
			regionCtx := contexts[place.Region]
			if regionCtx == nil {
				copy := base
				copy.C3mms = nil
				regionCtx = &copy
				contexts[place.Region] = regionCtx
			}
			tile.Available, tile.Known = checkTileHeights(regionCtx, place, zoom, y, x, heightLayers)
			if tile.Available {
				availableCount++
			}
			if !tile.Known {
				unknownCount++
			}
			tiles = append(tiles, tile)
		}
	}
	return tiles, availableCount, unknownCount, nil
}

func checkTileHeights(ctx *flyContext, place fly.Trigger, zoom, y, x, heightLayers int) (bool, bool) {
	if _, err := ctx.getC3mm(place, 0); err != nil {
		return false, false
	}
	for h := 0; h < heightLayers; h++ {
		hasTile, err := ctx.checkTile(place, zoom, y, x, h)
		if err != nil {
			return false, false
		}
		if hasTile {
			return true, true
		}
	}
	return false, true
}

func coverageTileFromTMS(zoom, x, y int) coverageTile {
	n := float64(mth.TileCountPerAxis(zoom))
	west := float64(x)/n*360 - 180
	east := float64(x+1)/n*360 - 180
	south := math.Atan(math.Sinh(2*math.Pi*(float64(y)/n-0.5))) * 180 / math.Pi
	north := math.Atan(math.Sinh(2*math.Pi*(float64(y+1)/n-0.5))) * 180 / math.Pi
	return coverageTile{North: north, South: south, East: east, West: west}
}

func geoDistanceMeters(latA, lonA, latB, lonB float64) float64 {
	const earthRadius = 6371000.0
	latARad := latA * math.Pi / 180
	latBRad := latB * math.Pi / 180
	dLat := (latB - latA) * math.Pi / 180
	dLon := (lonB - lonA) * math.Pi / 180
	a := math.Sin(dLat/2)*math.Sin(dLat/2) + math.Cos(latARad)*math.Cos(latBRad)*math.Sin(dLon/2)*math.Sin(dLon/2)
	return earthRadius * 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))
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

func fetchNominatim(endpoint string, target any) error {
	req, err := http.NewRequest(http.MethodGet, endpoint, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Accept-Language", "en")
	req.Header.Set("User-Agent", "Mappy export UI/1.0")
	res, err := geocodeClient.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("geocoder returned %s", res.Status)
	}
	return json.NewDecoder(res.Body).Decode(target)
}

func normalizePlaces(places []nominatimPlace) []placeResult {
	results := make([]placeResult, 0, len(places))
	seen := make(map[string]bool, len(places))
	for _, place := range places {
		key := strings.TrimSpace(strings.ToLower(place.DisplayName))
		if key == "" || seen[key] {
			continue
		}
		lat, errLat := strconv.ParseFloat(place.Lat, 64)
		lon, errLon := strconv.ParseFloat(place.Lon, 64)
		if errLat != nil || errLon != nil {
			continue
		}
		seen[key] = true
		results = append(results, placeResult{
			Name:        shortPlaceName(place),
			DisplayName: place.DisplayName,
			Lat:         lat,
			Lon:         lon,
		})
	}
	return results
}

func shortPlaceName(place nominatimPlace) string {
	if value := strings.TrimSpace(place.Name); value != "" {
		return value
	}
	for _, key := range []string{"tourism", "amenity", "building", "road", "neighbourhood", "suburb", "city", "town", "village", "county"} {
		if value := strings.TrimSpace(place.Address[key]); value != "" {
			return value
		}
	}
	if before, _, ok := strings.Cut(place.DisplayName, ","); ok {
		return strings.TrimSpace(before)
	}
	return strings.TrimSpace(place.DisplayName)
}

func smartExportName(req exportRequest) string {
	name := slugify(req.Name)
	if name != "" {
		return name
	}
	lat := (req.North + req.South) / 2
	lon := (req.West + req.East) / 2
	return fmt.Sprintf("area-%.4f-%.4f-%s", lat, lon, time.Now().Format("2006-01-02"))
}

func slugify(value string) string {
	var b strings.Builder
	dash := false
	for _, r := range strings.TrimSpace(value) {
		switch {
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			b.WriteRune(unicode.ToLower(r))
			dash = false
		case !dash && b.Len() > 0:
			b.WriteByte('-')
			dash = true
		}
	}
	name := strings.Trim(b.String(), "-")
	runes := []rune(name)
	if len(runes) > 80 {
		name = strings.TrimRight(string(runes[:80]), "-")
	}
	return name
}

func availableOutputPath(root, name string) (string, error) {
	if err := os.MkdirAll(root, 0o755); err != nil {
		return "", err
	}
	path := filepath.Join(root, name)
	for suffix := 2; ; suffix++ {
		_, err := os.Stat(path)
		if os.IsNotExist(err) {
			return path, nil
		}
		if err != nil {
			return "", err
		}
		path = filepath.Join(root, fmt.Sprintf("%s-%d", name, suffix))
	}
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
		data, err := ctx._getC3mm(p, part)
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

func (ctx *flyContext) _getC3mm(p fly.Trigger, part int) (c3mm.C3MM, error) {
	fn := fmt.Sprintf("./cache/c3mm/%d_%d/%d", p.Region, p.Version, part)
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
		dist := geoDistanceMeters(lat, lon, v.Lat, v.Lon)
		if dist <= v.Radius && dist < minDist {
			minDist, minPlace = dist, v
		}
	}
	if minDist == math.Inf(1) {
		return fly.Trigger{}, errors.New("flyover region not found for selection center")
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

func getContext(cache mps.Cache, cfg config.Config) (flyContext, error) {
	ctx, err := mps.Init(cache, cfg)
	if err != nil {
		return flyContext{}, err
	}
	am, err := fly.GetAltitudeManifest(cache, ctx.ResourceManifest)
	if err != nil {
		return flyContext{}, err
	}
	c3mmURLPrefix, err := ctx.ResourceManifest.URLPrefixFromStyleID(mps.ResourceManifest_StyleConfig_C3MM_1)
	if err != nil {
		return flyContext{}, err
	}
	c3mURLPrefix, err := ctx.ResourceManifest.URLPrefixFromStyleID(mps.ResourceManifest_StyleConfig_C3M)
	if err != nil {
		return flyContext{}, err
	}
	return flyContext{&ctx, am, c3mmURLPrefix, c3mURLPrefix, nil}, nil
}

func minMax(a, b int) (int, int) {
	if a < b {
		return a, b
	}
	return b, a
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}
