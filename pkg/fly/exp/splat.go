package exp

import (
	"bufio"
	"encoding/binary"
	"fmt"
	"math"
	"os"

	"github.com/Henkek221/orbis/pkg/fly/c3m"
)

// Splat is one of Apple's soft triangles (R3D splat soup) in ECEF metres with
// straight RGBA in 0..1.
type Splat struct {
	Corners [3][3]float64
	RGBA    [4]float32
}

// SplatWriter is implemented by outputs that can store splats.
type SplatWriter interface {
	Splats([]Splat) error
}

// OriginSetter is implemented by outputs with a local frame. Setting the same
// origin on every output keeps meshes and splats aligned.
type OriginSetter interface {
	SetOrigin(ecef [3]float64)
}

// CentredOrigin is the export centre (degrees) at the ellipsoid height of a
// reference ECEF point, usually the first tile.
func CentredOrigin(latitude, longitude float64, reference [3]float64) [3]float64 {
	return centredOrigin(latitude*math.Pi/180, longitude*math.Pi/180, reference)
}

// localFrame converts ECEF to the shared output frame: metres around origin,
// +X east, +Y up, +Z south.
type localFrame struct {
	latRad, lonRad float64
	origin         [3]float64
}

func (f localFrame) toLocal(p [3]float64) vector3 {
	dx, dy, dz := p[0]-f.origin[0], p[1]-f.origin[1], p[2]-f.origin[2]
	sinLat, cosLat := math.Sin(f.latRad), math.Cos(f.latRad)
	sinLon, cosLon := math.Sin(f.lonRad), math.Cos(f.lonRad)
	east := -sinLon*dx + cosLon*dy
	north := -sinLat*cosLon*dx - sinLat*sinLon*dy + cosLat*dz
	up := cosLat*cosLon*dx + cosLat*sinLon*dy + sinLat*dz
	return vector3{x: east, y: up, z: -north}
}

// PLYExport writes the splats as standard 3D Gaussian Splatting PLY (degree 0
// spherical harmonics), in the same local frame as GLB and USD. Each soft
// triangle becomes one flat Gaussian with the triangle's area covariance.
type PLYExport struct {
	path      string
	frame     localFrame
	originSet bool
	tmp       *os.File
	out       *bufio.Writer
	count     int
}

// NewPLY prepares path. Latitude and longitude are the export centre.
func NewPLY(path string, latitude, longitude float64) (*PLYExport, error) {
	tmp, err := os.Create(path + ".body")
	if err != nil {
		return nil, err
	}
	return &PLYExport{
		path:  path,
		frame: localFrame{latRad: latitude * math.Pi / 180, lonRad: longitude * math.Pi / 180},
		tmp:   tmp,
		out:   bufio.NewWriterSize(tmp, 1<<20),
	}, nil
}

// SetOrigin fixes the local frame origin (ECEF).
func (e *PLYExport) SetOrigin(ecef [3]float64) {
	e.frame.origin = ecef
	e.originSet = true
}

// Next ignores meshes; PLY only stores splats.
func (e *PLYExport) Next(c3m.C3M, string) error { return nil }

// Splats appends Gaussians.
func (e *PLYExport) Splats(splats []Splat) error {
	if !e.originSet && len(splats) > 0 {
		e.SetOrigin(splats[0].Corners[0])
	}
	var rec [17]float32
	for _, s := range splats {
		g, ok := gaussianFromTriangle(e.frame, s)
		if !ok {
			continue
		}
		rec = g
		if err := binary.Write(e.out, binary.LittleEndian, rec); err != nil {
			return err
		}
		e.count++
	}
	return nil
}

// Close writes header plus body. Without splats nothing is written.
func (e *PLYExport) Close() error {
	if err := e.out.Flush(); err != nil {
		e.tmp.Close()
		return err
	}
	e.tmp.Close()
	body := e.path + ".body"
	defer os.Remove(body)
	if e.count == 0 {
		return nil
	}
	f, err := os.Create(e.path)
	if err != nil {
		return err
	}
	header := fmt.Sprintf("ply\nformat binary_little_endian 1.0\ncomment Orbis: Apple triangle splats as 3D Gaussians, metres, +X east +Y up +Z south\nelement vertex %d\n", e.count)
	for _, p := range []string{"x", "y", "z", "nx", "ny", "nz", "f_dc_0", "f_dc_1", "f_dc_2", "opacity", "scale_0", "scale_1", "scale_2", "rot_0", "rot_1", "rot_2", "rot_3"} {
		header += "property float " + p + "\n"
	}
	header += "end_header\n"
	if _, err := f.WriteString(header); err != nil {
		f.Close()
		return err
	}
	in, err := os.Open(body)
	if err != nil {
		f.Close()
		return err
	}
	defer in.Close()
	if _, err := bufio.NewReader(in).WriteTo(f); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}

// Path is the PLY file.
func (e *PLYExport) Path() string { return e.path }

// Count is the number of Gaussians written so far.
func (e *PLYExport) Count() int { return e.count }

// gaussianFromTriangle fits a flat Gaussian to a triangle: centre at the
// centroid, in-plane covariance of the triangle's area, thin along the normal.
// Record layout: x y z, nx ny nz, f_dc 0..2, opacity (logit), log scales 0..2,
// rotation quaternion w x y z.
func gaussianFromTriangle(frame localFrame, s Splat) ([17]float32, bool) {
	var p [3]vector3
	for i := range p {
		p[i] = frame.toLocal(s.Corners[i])
	}
	c := vector3{(p[0].x + p[1].x + p[2].x) / 3, (p[0].y + p[1].y + p[2].y) / 3, (p[0].z + p[1].z + p[2].z) / 3}
	n := cross(subtract(p[1], p[0]), subtract(p[2], p[0]))
	area2 := length(n)
	if area2 < 1e-8 {
		return [17]float32{}, false
	}
	n = scale(n, 1/area2)
	u := subtract(p[1], p[0])
	u = scale(u, 1/length(u))
	w := cross(n, u)

	// 2D covariance of the uniform triangle: (1/12) * sum of centred v v^T.
	var sxx, sxy, syy float64
	for _, v := range p {
		d := subtract(v, c)
		a, b := dot(d, u), dot(d, w)
		sxx += a * a
		sxy += a * b
		syy += b * b
	}
	sxx, sxy, syy = sxx/12, sxy/12, syy/12
	tr, det := sxx+syy, sxx*syy-sxy*sxy
	disc := math.Sqrt(math.Max(tr*tr/4-det, 0))
	l1, l2 := tr/2+disc, math.Max(tr/2-disc, 1e-10)
	theta := 0.5 * math.Atan2(2*sxy, sxx-syy)
	cosT, sinT := math.Cos(theta), math.Sin(theta)
	e1 := add(scale(u, cosT), scale(w, sinT))
	e2 := cross(n, e1)

	// A little wider than one sigma so neighbouring splats close up like
	// Apple's soft triangles do.
	const widen = 1.6
	s1, s2 := widen*math.Sqrt(l1), widen*math.Sqrt(l2)
	s3 := 0.1*math.Min(s1, s2) + 0.005

	q := quaternionFromBasis(e1, e2, n)
	const shC0 = 0.28209479177387814
	col := [3]float64{}
	for i := 0; i < 3; i++ {
		v := float64(s.RGBA[i])
		if math.IsNaN(v) || math.IsInf(v, 0) {
			v = 0.5
		}
		col[i] = (math.Min(math.Max(v, 0), 1) - 0.5) / shC0
	}
	alpha := float64(s.RGBA[3])
	if math.IsNaN(alpha) || math.IsInf(alpha, 0) {
		alpha = 1
	}
	alpha = math.Min(math.Max(alpha, 0.02), 0.98)

	return [17]float32{
		float32(c.x), float32(c.y), float32(c.z),
		float32(n.x), float32(n.y), float32(n.z),
		float32(col[0]), float32(col[1]), float32(col[2]),
		float32(math.Log(alpha / (1 - alpha))),
		float32(math.Log(s1)), float32(math.Log(s2)), float32(math.Log(s3)),
		float32(q[0]), float32(q[1]), float32(q[2]), float32(q[3]),
	}, true
}

// quaternionFromBasis returns w, x, y, z for the rotation whose columns are
// a, b, c.
func quaternionFromBasis(a, b, c vector3) [4]float64 {
	m00, m01, m02 := a.x, b.x, c.x
	m10, m11, m12 := a.y, b.y, c.y
	m20, m21, m22 := a.z, b.z, c.z
	var w, x, y, z float64
	if tr := m00 + m11 + m22; tr > 0 {
		s := math.Sqrt(tr+1) * 2
		w, x, y, z = s/4, (m21-m12)/s, (m02-m20)/s, (m10-m01)/s
	} else if m00 > m11 && m00 > m22 {
		s := math.Sqrt(1+m00-m11-m22) * 2
		w, x, y, z = (m21-m12)/s, s/4, (m01+m10)/s, (m02+m20)/s
	} else if m11 > m22 {
		s := math.Sqrt(1+m11-m00-m22) * 2
		w, x, y, z = (m02-m20)/s, (m01+m10)/s, s/4, (m12+m21)/s
	} else {
		s := math.Sqrt(1+m22-m00-m11) * 2
		w, x, y, z = (m10-m01)/s, (m02+m20)/s, (m12+m21)/s, s/4
	}
	l := math.Sqrt(w*w + x*x + y*y + z*z)
	return [4]float64{w / l, x / l, y / l, z / l}
}

func dot(a, b vector3) float64 { return a.x*b.x + a.y*b.y + a.z*b.z }

func length(a vector3) float64 { return math.Sqrt(dot(a, a)) }

func scale(a vector3, f float64) vector3 { return vector3{a.x * f, a.y * f, a.z * f} }
