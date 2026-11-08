// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Henrik Hellpap. Part of Orbis: https://github.com/Henkek221/orbis

package orbis

import (
	"encoding/json"
	"errors"
	"io"
	"net/url"
	"os"
	"strings"

	"github.com/Henkek221/orbis/pkg/mps/config"
)

// Config is the runtime configuration: the Apple resource manifest URL and the
// static client value tokenP1. It is private; never publish it.
type Config = config.Config

var (
	// ErrNotConfigured means no usable tokenP1 was found in the config file,
	// the environment or (on macOS) the local system.
	ErrNotConfigured = errors.New("orbis is not configured: no tokenP1 in config.json, FLYOVER_TOKEN_P1 or the local system")
	// ErrManifestURL means the manifest URL is not an HTTPS apple.com address.
	ErrManifestURL = errors.New("resourceManifestURL must be an https URL on apple.com")
	// ErrToken means tokenP1 has an implausible length.
	ErrToken = errors.New("tokenP1 is missing or has an invalid length")
)

// DefaultResourceManifestURL is used when a config does not name one.
func DefaultResourceManifestURL() string {
	return config.DefaultResourceManifestURL()
}

// LoadConfig reads config.json. A missing file is not an error: values then
// come from FLYOVER_RESOURCE_MANIFEST_URL, FLYOVER_TOKEN_P1 or, on macOS, the
// local GeoServices framework.
func LoadConfig(path string) (Config, error) {
	return config.FromJSONFile(path)
}

// ParseConfig decodes a config.json supplied by a user and fills in the
// default manifest URL. Unknown fields are rejected.
func ParseConfig(r io.Reader) (Config, error) {
	var cfg Config
	decoder := json.NewDecoder(r)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&cfg); err != nil {
		return Config{}, err
	}
	if strings.TrimSpace(cfg.ResourceManifestURL) == "" {
		cfg.ResourceManifestURL = DefaultResourceManifestURL()
	}
	cfg.TokenP1 = strings.TrimSpace(cfg.TokenP1)
	return cfg, nil
}

// ValidateConfig checks a config before it is saved. It returns ErrManifestURL
// or ErrToken.
func ValidateConfig(cfg Config) error {
	manifestURL, err := url.Parse(cfg.ResourceManifestURL)
	if err != nil || manifestURL.Scheme != "https" || !strings.HasSuffix(strings.ToLower(manifestURL.Hostname()), ".apple.com") {
		return ErrManifestURL
	}
	if len(cfg.TokenP1) < 8 || len(cfg.TokenP1) > 256 {
		return ErrToken
	}
	return nil
}

// SaveConfig writes config.json atomically with owner-only permissions.
func SaveConfig(path string, cfg Config) error {
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

// ScanToken extracts tokenP1 from a GeoServices binary or dyld shared cache.
// It returns "" when the file holds no plausible value.
func ScanToken(r io.Reader) string {
	return config.ScanTokenP1(r)
}

// ConfigFromToken builds a config around a scanned tokenP1.
func ConfigFromToken(token string) Config {
	return Config{ResourceManifestURL: DefaultResourceManifestURL(), TokenP1: token}
}
