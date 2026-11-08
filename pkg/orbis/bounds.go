// Package orbis is the toolkit behind the Orbis CLI and browser UI: open a
// client against Apple's 3D map service, check coverage for an area and export
// it as OBJ and/or a Minecraft world.
package orbis

import (
	"errors"
	"math"

	"github.com/Henkek221/orbis/pkg/mth"
)

const metresPerDegreeLat = 111_320.0

// Bounds is a latitude/longitude rectangle in degrees.
type Bounds struct {
	North float64 `json:"north"`
	South float64 `json:"south"`
	East  float64 `json:"east"`
	West  float64 `json:"west"`
}

// BoundsAround returns a rectangle of the given size in metres centred on a
// point. It is accurate enough for the few hundred metres Orbis exports.
func BoundsAround(lat, lon, widthMetres, heightMetres float64) Bounds {
	dLat := heightMetres / 2 / metresPerDegreeLat
	dLon := widthMetres / 2 / (metresPerDegreeLat * math.Cos(lat*math.Pi/180))
	return Bounds{North: lat + dLat, South: lat - dLat, East: lon + dLon, West: lon - dLon}
}

// Center returns the midpoint of the rectangle.
func (b Bounds) Center() (lat, lon float64) {
	return (b.North + b.South) / 2, (b.West + b.East) / 2
}

// Size returns the width and height of the rectangle in metres, measured
// through its centre.
func (b Bounds) Size() (widthMetres, heightMetres float64) {
	lat, lon := b.Center()
	return GeoDistanceMetres(lat, b.West, lat, b.East), GeoDistanceMetres(b.South, lon, b.North, lon)
}

// Validate reports whether the rectangle is well formed and inside the range
// the tile scheme supports.
func (b Bounds) Validate() error {
	if b.North <= b.South {
		return errors.New("north must be greater than south")
	}
	if b.East <= b.West {
		return errors.New("east must be greater than west")
	}
	if b.North > 85 || b.South < -85 || b.West < -180 || b.East > 180 {
		return errors.New("bounds are outside supported map range")
	}
	return nil
}

// TileRange is the inclusive block of TMS tiles that covers some bounds.
type TileRange struct {
	Zoom int `json:"zoom"`
	XMin int `json:"xMin"`
	XMax int `json:"xMax"`
	YMin int `json:"yMin"`
	YMax int `json:"yMax"`
}

// Tiles returns the TMS tiles at zoom that cover the rectangle.
func (b Bounds) Tiles(zoom int) TileRange {
	xWest, yNorth := mth.LatLonToTileTMS(zoom, b.North, b.West)
	xEast, ySouth := mth.LatLonToTileTMS(zoom, b.South, b.East)
	xMin, xMax := minMax(xWest, xEast)
	yMin, yMax := minMax(ySouth, yNorth)
	return TileRange{Zoom: zoom, XMin: xMin, XMax: xMax, YMin: yMin, YMax: yMax}
}

// Count is the number of tiles in the range.
func (r TileRange) Count() int {
	return (r.XMax - r.XMin + 1) * (r.YMax - r.YMin + 1)
}

// TileBounds returns the rectangle a single TMS tile covers.
func TileBounds(zoom, x, y int) Bounds {
	n := float64(mth.TileCountPerAxis(zoom))
	west := float64(x)/n*360 - 180
	east := float64(x+1)/n*360 - 180
	south := math.Atan(math.Sinh(2*math.Pi*(float64(y)/n-0.5))) * 180 / math.Pi
	north := math.Atan(math.Sinh(2*math.Pi*(float64(y+1)/n-0.5))) * 180 / math.Pi
	return Bounds{North: north, South: south, East: east, West: west}
}

// GeoDistanceMetres is the great-circle distance between two points.
func GeoDistanceMetres(latA, lonA, latB, lonB float64) float64 {
	const earthRadius = 6371000.0
	latARad := latA * math.Pi / 180
	latBRad := latB * math.Pi / 180
	dLat := (latB - latA) * math.Pi / 180
	dLon := (lonB - lonA) * math.Pi / 180
	a := math.Sin(dLat/2)*math.Sin(dLat/2) + math.Cos(latARad)*math.Cos(latBRad)*math.Sin(dLon/2)*math.Sin(dLon/2)
	return earthRadius * 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))
}

func minMax(a, b int) (int, int) {
	if a < b {
		return a, b
	}
	return b, a
}
