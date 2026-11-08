package r3d

import (
	"encoding/binary"
	"errors"
	"fmt"
	"math"
)

// Splat is one soft triangle of a TRICO soup, the primitive Apple's renderer
// draws for vegetation and fine detail. Corners are metres in the group's local
// north-east-down frame. A record is a real triangle, not a centre with axes.
type Splat struct {
	Corners [3][3]float32
	// RGBA is albedo (sRGB, 0..1) and opacity. Opacity 0 occurs: thin facade
	// slivers that are invisible in the colour pass. Missing channels are NaN.
	RGBA [4]float32
	// Normal is the unit surface normal in the local frame, decoded from the
	// octahedral pair in the tile. Zero when the tile has none.
	Normal [3]float32
	// Roughness (0.05..1) and SpecularWeight (0..~3.2) are the material terms
	// Apple's PBR splat shader receives.
	Roughness, SpecularWeight float32
	// Extra are the four non-colour channels in file order, undecoded.
	Extra [4]float32
}

// DecodeSoup expands the two quantized blocks of a soup: 3 corners per record
// and 8 attribute channels per record, assigned by the soup's Layout.
func DecodeSoup(s *Soup) ([]Splat, error) {
	if s == nil {
		return nil, errors.New("soup: no TRICO soup")
	}
	if s.Records == 0 {
		return nil, nil
	}
	if s.Version != 1 || s.Channels != 8 {
		return nil, fmt.Errorf("soup: unsupported version %d with %d channels", s.Version, s.Channels)
	}
	if s.Records > 10_000_000 {
		return nil, errors.New("soup: implausible record count")
	}
	n := int(s.Records)
	at := 0
	positions, at, err := quantizedBlock(s.Payload, at, 3*n, 3, []float32{s.Scales[0], s.Scales[0], s.Scales[0]})
	if err != nil {
		return nil, fmt.Errorf("soup positions: %w", err)
	}
	attrs, at, err := quantizedBlock(s.Payload, at, n, 8, s.Scales[1:])
	if err != nil {
		return nil, fmt.Errorf("soup attributes: %w", err)
	}
	if at != len(s.Payload) {
		return nil, errors.New("soup: trailing bytes")
	}
	layout := s.Layout
	if len(layout) == 0 {
		layout = defaultLayout
	}
	offsets := map[AttributeKind]int{}
	at8 := 0
	for _, a := range layout {
		offsets[a.Kind] = at8
		at8 += a.Rows * a.Cols
	}
	if at8 != 8 {
		return nil, fmt.Errorf("soup: layout covers %d of 8 channels", at8)
	}
	get := func(a []float32, k AttributeKind, i int) (float32, bool) {
		o, ok := offsets[k]
		if !ok {
			return 0, false
		}
		return a[o+i], true
	}
	out := make([]Splat, n)
	for i := range out {
		sp := &out[i]
		for c := 0; c < 3; c++ {
			copy(sp.Corners[c][:], positions[9*i+3*c:9*i+3*c+3])
		}
		a := attrs[8*i : 8*i+8]
		for c := 0; c < 3; c++ {
			sp.RGBA[c], _ = get(a, AttrAlbedo, c)
		}
		sp.RGBA[3], _ = get(a, AttrOpacity, 0)
		u, okU := get(a, AttrNormal, 0)
		v, _ := get(a, AttrNormal, 1)
		if okU && u == u && v == v {
			sp.Normal = octahedral(u, v)
		}
		sp.Roughness, _ = get(a, AttrRoughness, 0)
		sp.SpecularWeight, _ = get(a, AttrSpecularWeight, 0)
		k := 0
		for _, l := range layout {
			if l.Kind == AttrOpacity || l.Kind == AttrAlbedo {
				continue
			}
			for c := 0; c < l.Rows*l.Cols && k < 4; c++ {
				sp.Extra[k] = a[offsets[l.Kind]+c]
				k++
			}
		}
	}
	return out, nil
}

// quantizedBlock reads a compact-size framed block: magic 0x0025, a BYET
// coded int32 origin per channel and BYET coded int32 deltas. Deltas -1, -2
// and -3 stand for NaN, -Inf and +Inf.
func quantizedBlock(src []byte, at, count, channels int, steps []float32) ([]float32, int, error) {
	size, begin, err := compactSize(src, at)
	if err != nil {
		return nil, at, err
	}
	end := begin + size
	if end > len(src) || size < 3 {
		return nil, at, errors.New("block beyond input")
	}
	b := src[begin:end]
	if b[0] != 0x25 || b[1] != 0 {
		return nil, at, fmt.Errorf("unknown quantized block magic %02x%02x", b[0], b[1])
	}
	originSize, o, err := compactSize(b, 2)
	if err != nil {
		return nil, at, err
	}
	if o+originSize > len(b) {
		return nil, at, errors.New("origin beyond block")
	}
	originRaw, err := byetDecode(b[o:o+originSize], channels, 4)
	if err != nil {
		return nil, at, fmt.Errorf("origin: %w", err)
	}
	origin := int32s(originRaw)
	deltas, err := byetDecode(b[o+originSize:], count, 4*channels)
	if err != nil {
		return nil, at, err
	}
	out := make([]float32, count*channels)
	for i := range out {
		d := int32(binary.LittleEndian.Uint32(deltas[4*i:]))
		switch d {
		case -1:
			out[i] = float32(math.NaN())
			continue
		case -2:
			out[i] = float32(math.Inf(-1))
			continue
		case -3:
			out[i] = float32(math.Inf(1))
			continue
		}
		k := i % channels
		v := int64(d) + int64(origin[k])
		if v < math.MinInt32 || v > math.MaxInt32 {
			return nil, at, errors.New("quantized value overflow")
		}
		out[i] = dequantize(v, steps[k])
	}
	return out, end, nil
}

// defaultLayout is the layout every observed tile declares.
var defaultLayout = []Attribute{
	{AttrOpacity, 1, 1}, {AttrAlbedo, 1, 3}, {AttrRoughness, 1, 1}, {AttrNormal, 1, 2}, {AttrSpecularWeight, 1, 1},
}

// octahedral maps a 0..1 octahedral pair to a unit vector.
func octahedral(u, v float32) [3]float32 {
	x, y := 2*float64(u)-1, 2*float64(v)-1
	z := 1 - math.Abs(x) - math.Abs(y)
	if z < 0 {
		x, y = (1-math.Abs(y))*math.Copysign(1, x), (1-math.Abs(x))*math.Copysign(1, y)
	}
	l := math.Sqrt(x*x + y*y + z*z)
	return [3]float32{float32(x / l), float32(y / l), float32(z / l)}
}

// Gaussian is a 3D Gaussian with the same centre and second moments as a
// splat triangle, in the splat's local frame.
type Gaussian struct {
	Center [3]float64
	// Scales are standard deviations along the columns of Rotation; the third
	// is the thickness along the triangle normal.
	Scales [3]float64
	// Rotation is a unit quaternion (w, x, y, z) whose matrix has the
	// principal axes as columns.
	Rotation [4]float64
	RGB      [3]float32
	Opacity  float32
}

// Gaussian converts a splat by moment matching: a uniformly filled triangle
// has covariance (1/12)·Σ dᵢdᵢᵀ around its centroid, dᵢ being the corners
// relative to it. thickness is the sigma along the normal, e.g. 0.01 m.
func (s Splat) Gaussian(thickness float64) Gaussian {
	var g Gaussian
	for c := 0; c < 3; c++ {
		for k := 0; k < 3; k++ {
			g.Center[k] += float64(s.Corners[c][k]) / 3
		}
	}
	var cov [3][3]float64
	for c := 0; c < 3; c++ {
		var d [3]float64
		for k := 0; k < 3; k++ {
			d[k] = float64(s.Corners[c][k]) - g.Center[k]
		}
		for r := 0; r < 3; r++ {
			for k := 0; k < 3; k++ {
				cov[r][k] += d[r] * d[k] / 12
			}
		}
	}
	vals, vecs := symmetricEigen(cov)
	// Largest two axes span the triangle; the smallest is its normal.
	order := [3]int{0, 1, 2}
	for i := 0; i < 3; i++ {
		for j := i + 1; j < 3; j++ {
			if vals[order[j]] > vals[order[i]] {
				order[i], order[j] = order[j], order[i]
			}
		}
	}
	var m [3][3]float64
	for c, o := range order {
		g.Scales[c] = math.Sqrt(math.Max(vals[o], 0))
		for r := 0; r < 3; r++ {
			m[r][c] = vecs[r][o]
		}
	}
	g.Scales[2] = math.Max(thickness, 1e-4)
	// Keep the basis right-handed so it is a rotation.
	det := m[0][0]*(m[1][1]*m[2][2]-m[1][2]*m[2][1]) - m[0][1]*(m[1][0]*m[2][2]-m[1][2]*m[2][0]) + m[0][2]*(m[1][0]*m[2][1]-m[1][1]*m[2][0])
	if det < 0 {
		for r := 0; r < 3; r++ {
			m[r][2] = -m[r][2]
		}
	}
	g.Rotation = quaternion(m)
	copy(g.RGB[:], s.RGBA[:3])
	g.Opacity = s.RGBA[3]
	return g
}

// symmetricEigen diagonalizes a symmetric 3x3 matrix with Jacobi rotations;
// eigenvectors are the columns of vecs.
func symmetricEigen(a [3][3]float64) (vals [3]float64, vecs [3][3]float64) {
	for i := 0; i < 3; i++ {
		vecs[i][i] = 1
	}
	for sweep := 0; sweep < 50; sweep++ {
		off := a[0][1]*a[0][1] + a[0][2]*a[0][2] + a[1][2]*a[1][2]
		if off < 1e-30 {
			break
		}
		for p := 0; p < 2; p++ {
			for q := p + 1; q < 3; q++ {
				if math.Abs(a[p][q]) < 1e-300 {
					continue
				}
				theta := (a[q][q] - a[p][p]) / (2 * a[p][q])
				t := math.Copysign(1, theta) / (math.Abs(theta) + math.Sqrt(theta*theta+1))
				c := 1 / math.Sqrt(t*t+1)
				s := t * c
				for k := 0; k < 3; k++ {
					akp, akq := a[k][p], a[k][q]
					a[k][p], a[k][q] = c*akp-s*akq, s*akp+c*akq
				}
				for k := 0; k < 3; k++ {
					apk, aqk := a[p][k], a[q][k]
					a[p][k], a[q][k] = c*apk-s*aqk, s*apk+c*aqk
				}
				for k := 0; k < 3; k++ {
					vkp, vkq := vecs[k][p], vecs[k][q]
					vecs[k][p], vecs[k][q] = c*vkp-s*vkq, s*vkp+c*vkq
				}
			}
		}
	}
	return [3]float64{a[0][0], a[1][1], a[2][2]}, vecs
}

// quaternion converts a rotation matrix to (w, x, y, z).
func quaternion(m [3][3]float64) [4]float64 {
	tr := m[0][0] + m[1][1] + m[2][2]
	var q [4]float64
	switch {
	case tr > 0:
		s := math.Sqrt(tr+1) * 2
		q = [4]float64{s / 4, (m[2][1] - m[1][2]) / s, (m[0][2] - m[2][0]) / s, (m[1][0] - m[0][1]) / s}
	case m[0][0] > m[1][1] && m[0][0] > m[2][2]:
		s := math.Sqrt(1+m[0][0]-m[1][1]-m[2][2]) * 2
		q = [4]float64{(m[2][1] - m[1][2]) / s, s / 4, (m[0][1] + m[1][0]) / s, (m[0][2] + m[2][0]) / s}
	case m[1][1] > m[2][2]:
		s := math.Sqrt(1+m[1][1]-m[0][0]-m[2][2]) * 2
		q = [4]float64{(m[0][2] - m[2][0]) / s, (m[0][1] + m[1][0]) / s, s / 4, (m[1][2] + m[2][1]) / s}
	default:
		s := math.Sqrt(1+m[2][2]-m[0][0]-m[1][1]) * 2
		q = [4]float64{(m[1][0] - m[0][1]) / s, (m[0][2] + m[2][0]) / s, (m[1][2] + m[2][1]) / s, s / 4}
	}
	return q
}
