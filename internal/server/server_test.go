// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Henrik Hellpap. Part of Orbis: https://github.com/Henkek221/orbis

package server

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/Henkek221/orbis/pkg/orbis"
)

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
	valid := orbis.Config{
		ResourceManifestURL: orbis.DefaultResourceManifestURL(),
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
	cfg := orbis.Config{
		ResourceManifestURL: orbis.DefaultResourceManifestURL(),
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
	loaded, err := orbis.LoadConfig(path)
	if err != nil {
		t.Fatal(err)
	}
	if loaded != cfg {
		t.Fatalf("saved config = %#v, want %#v", loaded, cfg)
	}
}
