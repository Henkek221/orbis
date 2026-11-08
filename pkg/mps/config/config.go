package config

import (
	"encoding/json"
	"os"
	"strings"
)

// defaultResourceManifestURL identifies as a current macOS. Apple serves the
// manifest per client: an old identity (macOS 11) gets about half the styles,
// without the newer 3D formats (R3D style 98, index style 100).
const defaultResourceManifestURL = "https://gspe35-ssl.ls.apple.com/geo_manifest/dynamic/config?application=geod&application_version=1&country_code=US&hardware=Mac16,12&os=osx&os_build=26A434&os_version=27.0.1"

type Config struct {
	ResourceManifestURL string `json:"resourceManifestURL"`
	TokenP1             string `json:"tokenP1"`
}

func DefaultResourceManifestURL() string {
	return defaultResourceManifestURL
}

func FromJSONFile(file string) (cnf Config, err error) {
	raw, err := os.ReadFile(file)
	if err == nil {
		if err = json.Unmarshal(raw, &cnf); err != nil {
			return
		}
	} else if !os.IsNotExist(err) {
		return
	}
	err = nil

	if cnf.ResourceManifestURL == "" {
		cnf.ResourceManifestURL = defaultResourceManifestURL
	}
	if v := os.Getenv("FLYOVER_RESOURCE_MANIFEST_URL"); v != "" {
		cnf.ResourceManifestURL = v
	}
	if v := os.Getenv("FLYOVER_TOKEN_P1"); v != "" {
		cnf.TokenP1 = v
	}
	if cnf.TokenP1 == "" {
		cnf.TokenP1 = discoveredTokenP1()
	}
	return
}

func (config Config) IsValid() bool {
	return config.ResourceManifestURL != "" && len(strings.TrimSpace(config.TokenP1)) >= 8
}
