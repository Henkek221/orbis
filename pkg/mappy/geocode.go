package mappy

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// GeocodeUserAgent identifies Mappy to OpenStreetMap Nominatim, whose usage
// policy requires a descriptive agent.
var GeocodeUserAgent = "Mappy/1.0 (+https://github.com/Henkek221/mappy)"

var geocodeClient = &http.Client{Timeout: 8 * time.Second}

// Place is a geocoding result.
type Place struct {
	Name        string  `json:"name"`
	DisplayName string  `json:"displayName"`
	Lat         float64 `json:"lat"`
	Lon         float64 `json:"lon"`
}

type nominatimPlace struct {
	DisplayName string            `json:"display_name"`
	Name        string            `json:"name"`
	Lat         string            `json:"lat"`
	Lon         string            `json:"lon"`
	Address     map[string]string `json:"address"`
}

// Search looks up places and addresses with OpenStreetMap Nominatim. Keep it to
// interactive use: Nominatim allows about one request per second.
func Search(ctx context.Context, query string, limit int) ([]Place, error) {
	if limit < 1 {
		limit = 5
	}
	params := url.Values{
		"q":              {query},
		"format":         {"jsonv2"},
		"limit":          {strconv.Itoa(limit)},
		"addressdetails": {"1"},
	}
	var places []nominatimPlace
	if err := fetchNominatim(ctx, "https://nominatim.openstreetmap.org/search?"+params.Encode(), &places); err != nil {
		return nil, err
	}
	return normalizePlaces(places), nil
}

// Reverse returns a human name for a point.
func Reverse(ctx context.Context, lat, lon float64) (Place, error) {
	params := url.Values{
		"lat":            {strconv.FormatFloat(lat, 'f', 7, 64)},
		"lon":            {strconv.FormatFloat(lon, 'f', 7, 64)},
		"format":         {"jsonv2"},
		"addressdetails": {"1"},
	}
	var place nominatimPlace
	if err := fetchNominatim(ctx, "https://nominatim.openstreetmap.org/reverse?"+params.Encode(), &place); err != nil {
		return Place{}, err
	}
	results := normalizePlaces([]nominatimPlace{place})
	if len(results) == 0 {
		return Place{}, fmt.Errorf("no place name for %.5f,%.5f", lat, lon)
	}
	return results[0], nil
}

func fetchNominatim(ctx context.Context, endpoint string, target any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Accept-Language", "en")
	req.Header.Set("User-Agent", GeocodeUserAgent)
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

func normalizePlaces(places []nominatimPlace) []Place {
	results := make([]Place, 0, len(places))
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
		results = append(results, Place{
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
