package exp

import "math"

// WGS84 ellipsoid.
const (
	wgs84A  = 6378137.0
	wgs84E2 = 6.69437999014e-3
)

// ecefFromGeodetic converts latitude/longitude in radians and height in metres.
func ecefFromGeodetic(latRad, lonRad, height float64) [3]float64 {
	sinLat, cosLat := math.Sin(latRad), math.Cos(latRad)
	n := wgs84A / math.Sqrt(1-wgs84E2*sinLat*sinLat)
	return [3]float64{
		(n + height) * cosLat * math.Cos(lonRad),
		(n + height) * cosLat * math.Sin(lonRad),
		(n*(1-wgs84E2) + height) * sinLat,
	}
}

// ellipsoidHeight returns the height of an ECEF point above the ellipsoid.
func ellipsoidHeight(p [3]float64) float64 {
	r := math.Hypot(p[0], p[1])
	lat := math.Atan2(p[2], r*(1-wgs84E2))
	for i := 0; i < 5; i++ {
		sinLat := math.Sin(lat)
		n := wgs84A / math.Sqrt(1-wgs84E2*sinLat*sinLat)
		lat = math.Atan2(p[2]+wgs84E2*n*sinLat, r)
	}
	sinLat := math.Sin(lat)
	n := wgs84A / math.Sqrt(1-wgs84E2*sinLat*sinLat)
	return r/math.Cos(lat) - n
}

// centredOrigin is the export centre at the height of the first tile, so the
// model sits around 0,0 instead of at one corner tile.
func centredOrigin(latRad, lonRad float64, firstTile [3]float64) [3]float64 {
	return ecefFromGeodetic(latRad, lonRad, ellipsoidHeight(firstTile))
}
