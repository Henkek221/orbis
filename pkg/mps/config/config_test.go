package config

import (
	"strings"
	"testing"
)

func TestFromJSONFileUsesSafeDefaults(t *testing.T) {
	t.Setenv("FLYOVER_RESOURCE_MANIFEST_URL", "")
	t.Setenv("FLYOVER_TOKEN_P1", "LocalTestToken123")

	cfg, err := FromJSONFile(t.TempDir() + "/missing.json")
	if err != nil {
		t.Fatalf("FromJSONFile returned an error for an optional file: %v", err)
	}
	if !cfg.IsValid() {
		t.Fatal("expected environment token and default manifest URL to form a valid config")
	}
	if cfg.ResourceManifestURL != defaultResourceManifestURL {
		t.Fatalf("unexpected default resource manifest URL: %q", cfg.ResourceManifestURL)
	}
}

func TestTokenFromPrintableStrings(t *testing.T) {
	input := "noise\x00client-xyzABC-marker\x00AbcDef123_45678\x00tail"
	if got := tokenFromPrintableStrings(strings.NewReader(input)); got != "AbcDef123_45678" {
		t.Fatalf("tokenFromPrintableStrings() = %q", got)
	}
}

func TestTokenFromPrintableStringsRejectsUnrelatedValue(t *testing.T) {
	input := "client-xyzABC-marker\x00not-a-client-token\x00tail"
	if got := tokenFromPrintableStrings(strings.NewReader(input)); got != "" {
		t.Fatalf("expected no token, got %q", got)
	}
}
