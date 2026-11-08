package config

import (
	"encoding/json"
	"net/url"
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
	cnf.ResourceManifestURL = CurrentIdentity(cnf.ResourceManifestURL)
	return
}

// CurrentIdentity rewrites the device identity of an Apple geo manifest URL
// (hardware, os, os_build, os_version) to the current default. Configs and
// kvasir output often carry an old identity, and Apple then withholds the
// newer tile styles. Other URLs are returned unchanged.
func CurrentIdentity(manifestURL string) string {
	u, err := url.Parse(manifestURL)
	if err != nil || !strings.HasSuffix(u.Path, "/geo_manifest/dynamic/config") {
		return manifestURL
	}
	def, err := url.Parse(defaultResourceManifestURL)
	if err != nil {
		return manifestURL
	}
	q, dq := u.Query(), def.Query()
	for _, key := range []string{"hardware", "os", "os_build", "os_version"} {
		q.Set(key, dq.Get(key))
	}
	u.RawQuery = q.Encode()
	return u.String()
}

func (config Config) IsValid() bool {
	return config.ResourceManifestURL != "" && len(strings.TrimSpace(config.TokenP1)) >= 8
}
