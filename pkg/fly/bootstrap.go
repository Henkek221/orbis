package fly

import (
	"encoding/xml"
	"errors"
	"math"
	"os"
	"path"
	"regexp"
	"time"

	"github.com/christiangda/flyover-reverse-engineering/pkg/mps"
	"github.com/christiangda/flyover-reverse-engineering/pkg/web"
)

type AltitudeManifest struct {
	XMLName  xml.Name  `xml:"manifest"`
	Triggers []Trigger `xml:"triggers>trigger"`
}

type Trigger struct {
	XMLName xml.Name `xml:"trigger"`
	Name    string   `xml:"name,attr"`
	LatRad  float64  `xml:"latitude,attr"`
	LonRad  float64  `xml:"longitude,attr"`
	Radius  float64  `xml:"radius,attr"`
	Region  int      `xml:"region,attr"`
	Version int      `xml:"version,attr"`

	Lat float64 // converted from LatRad
	Lon float64 // converted from LonRad
}

var regexAltitudeFile = regexp.MustCompile(`^altitude[a-zA-Z0-9-]*\.xml$`)

// The altitude manifest carries the dataset version of every flyover region and
// Apple bumps it whenever new scans go live. Caching it without an expiry pins
// exports to the version that was current on first launch, so it is refetched
// like the resource manifest.
const altitudeManifestMaxAge = 6 * time.Hour

// GetAltitudeManifest finds altitude manifest reference in resource manifest
// and fetches its contents from cache or web and decodes it
func GetAltitudeManifest(cache mps.Cache, rm *mps.ResourceManifest) (am AltitudeManifest, err error) {
	// find altitude file name
	altitudeFile, err := rm.CacheFileNameFromRegexp(regexAltitudeFile)
	if err != nil {
		return
	}

	// get altitude manifest
	rawAmCachePath := path.Join(cache.Directory, altitudeFile)
	var rawAm []byte
	var info os.FileInfo
	if cache.Enabled {
		info, err = os.Stat(rawAmCachePath)
	}
	if !cache.Enabled || os.IsNotExist(err) || err == nil && time.Since(info.ModTime()) > altitudeManifestMaxAge {
		// from url
		cacheBaseURL := rm.CacheBaseURL()
		if cacheBaseURL == "" {
			return am, errors.New("resource manifest does not contain a cache base URL")
		}
		if rawAm, err = web.Get(cacheBaseURL + "xml/" + altitudeFile); err != nil {
			return
		}
		if cache.Enabled {
			// to cache
			if err = os.WriteFile(rawAmCachePath, rawAm, 0o644); err != nil {
				return
			}
		}
	} else if err == nil {
		// from cache
		if rawAm, err = os.ReadFile(rawAmCachePath); err != nil {
			return
		}
	} else {
		return
	}

	// decode altitude manifest
	am = AltitudeManifest{}
	if err = xml.Unmarshal(rawAm, &am); err != nil {
		return
	}

	// lat lon deg for convenience
	for i := range am.Triggers {
		t := &am.Triggers[i]
		t.Lat = t.LatRad / math.Pi * 180
		t.Lon = t.LonRad / math.Pi * 180
	}

	return
}
