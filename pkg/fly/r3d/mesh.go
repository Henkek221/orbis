package r3d

import (
	"encoding/binary"
	"errors"
	"fmt"
	"math"
)

// MeshData is a decoded TRICO mesh. Positions are metres in the group's local
// north-east-down frame (see Frame). Positions, UVs and normals have their own
// index triplets per triangle, as in OBJ.
type MeshData struct {
	Positions [][3]float32
	Faces     [][3]uint32
	// UVs are normalized to the atlas of their triangle: u right, v down from
	// the top-left image corner.
	UVs     [][2]float32
	UVFaces [][3]uint32
	// UVPixels are the same coordinates in atlas pixels.
	UVPixels    [][2]float32
	Normals     [][3]float32
	NormalFaces [][3]uint32
	// FaceAtlas is the index into Group.Images for each triangle.
	FaceAtlas []int32
	// VertexFlags holds VertexFlagBits bits per position, meaning unknown.
	VertexFlags    []byte
	VertexFlagBits int
}

// DecodeMesh expands the compressed streams of a mesh group. images supplies
// the atlas sizes needed to normalize UVs; pass Group.Images.
func DecodeMesh(m *Mesh, images []Image) (*MeshData, error) {
	if m == nil || len(m.Raw) < 20 {
		return nil, errors.New("mesh: no TRICO mesh")
	}
	if m.Version != 3 {
		return nil, fmt.Errorf("mesh: unsupported TRICO version %d", m.Version)
	}
	src := m.Raw
	r := &reader{src: src, at: 20}
	faceCount := r.varint()
	tokenBytes := r.varint()
	intCount := r.varint()
	if r.err == nil && (faceCount < 0 || faceCount > 10_000_000 || tokenBytes < (faceCount+7)/8) {
		return nil, errors.New("mesh: implausible topology sizes")
	}
	bitmap := r.block((faceCount+7)/8, 1)
	ext := r.block(tokenBytes-(faceCount+7)/8, 1)
	header := int32s(r.block(intCount, 4))
	if r.err != nil {
		return nil, r.err
	}
	out := &MeshData{}
	if faceCount == 0 {
		if tokenBytes != 0 || len(header) != 4 || uint32(header[0]) != topologyMagic {
			return nil, errors.New("mesh: bad empty topology")
		}
		if r.varint() != 0 || r.varint() != 0 || r.varint() != 0 || r.err != nil || r.at != len(src) {
			return nil, errors.New("mesh: empty mesh with streams")
		}
		return out, nil
	}
	topo, vertices, err := buildTopology(faceCount, append(bitmap, ext...), header)
	if err != nil {
		return nil, err
	}

	// Positions: zigzag residuals on a parallelogram predictor.
	qpos, err := predict(topo.faces, topo.opp, int32s(r.block(vertices, 12)), 3, r)
	if err != nil {
		return nil, err
	}
	out.Positions = make([][3]float32, vertices)
	for i := range out.Positions {
		for k := 0; k < 3; k++ {
			out.Positions[i][k] = dequantize(qpos[3*i+k], m.Scales[0])
		}
	}
	out.Faces = triplets(topo.faces)

	out.VertexFlagBits = r.varint()
	if out.VertexFlagBits > 0 {
		out.VertexFlags = r.block((out.VertexFlagBits*vertices+7)/8, 1)
	}

	// UVs: seam bits split the surface into patches, each with an atlas.
	if r.varint() == 0 {
		if r.err != nil {
			return nil, r.err
		}
		return nil, errors.New("mesh: no UV stream")
	}
	declaredUV := r.varint()
	seams := r.block((faceCount+1)/2, 1)
	patches := r.varint()
	patchAtlas := int32s(r.block(patches, 4))
	if r.err != nil {
		return nil, r.err
	}
	uvCorners, faceAtlas, uvCount, err := uvPatches(topo.faces, topo.opp, seams, patchAtlas)
	if err != nil {
		return nil, err
	}
	if uvCount != declaredUV {
		return nil, fmt.Errorf("mesh: %d UVs, header says %d", uvCount, declaredUV)
	}
	quv, err := predict(uvCorners, pairEdges(uvCorners), int32s(r.block(uvCount, 8)), 2, r)
	if err != nil {
		return nil, err
	}
	out.FaceAtlas = faceAtlas
	out.UVFaces = triplets(uvCorners)
	out.UVPixels = make([][2]float32, uvCount)
	out.UVs = make([][2]float32, uvCount)
	uvAtlas := make([]int32, uvCount)
	for c, uv := range uvCorners {
		uvAtlas[uv] = faceAtlas[c/3]
	}
	sizes := make([][2]int, len(images))
	for i, im := range images {
		w, h, err := im.Size()
		if err != nil {
			return nil, fmt.Errorf("mesh: atlas %d: %w", i, err)
		}
		sizes[i] = [2]int{w, h}
	}
	for i := range out.UVPixels {
		u := dequantize(quv[2*i], m.Scales[1])
		v := dequantize(quv[2*i+1], m.Scales[1])
		out.UVPixels[i] = [2]float32{u, v}
		a := uvAtlas[i]
		if a < 0 || int(a) >= len(sizes) {
			return nil, fmt.Errorf("mesh: triangle uses atlas %d of %d", a, len(sizes))
		}
		out.UVs[i] = [2]float32{u / float32(sizes[a][0]), v / float32(sizes[a][1])}
	}

	// Normals: one packed direction per (vertex, smoothing group); corners
	// after the first at a vertex pick an existing one or open a new one.
	// Tiles repackaged from classic C3M (older regions) carry none.
	if r.varint() == 0 {
		if r.err != nil {
			return nil, r.err
		}
		if r.at != len(src) {
			return nil, errors.New("mesh: trailing bytes")
		}
		return out, nil
	}
	if variant := r.varint(); variant != 0 {
		return nil, fmt.Errorf("mesh: normal variant %d", variant)
	}
	normalCount := r.varint()
	selectorCount := r.varint()
	packed := r.block(normalCount, 4)
	selectors := r.block(selectorCount, 1)
	if r.err != nil {
		return nil, r.err
	}
	if r.at != len(src) {
		return nil, errors.New("mesh: trailing bytes")
	}
	perVertex := make([][]uint32, vertices)
	normalCorners := make([]int, len(topo.faces))
	next, cursor := uint32(0), 0
	for c, v := range topo.faces {
		seen := perVertex[v]
		var idx uint32
		if len(seen) == 0 {
			idx = next
			next++
			perVertex[v] = append(seen, idx)
		} else {
			if cursor >= len(selectors) {
				return nil, errors.New("mesh: missing normal selector")
			}
			sel := int(selectors[cursor])
			cursor++
			switch {
			case sel == 0:
				idx = next
				next++
				perVertex[v] = append(seen, idx)
			case sel <= len(seen):
				idx = seen[sel-1]
			default:
				return nil, errors.New("mesh: normal selector out of range")
			}
		}
		normalCorners[c] = int(idx)
	}
	if int(next) != normalCount || cursor != len(selectors) {
		return nil, errors.New("mesh: normal count mismatch")
	}
	out.NormalFaces = triplets(normalCorners)
	out.Normals = make([][3]float32, normalCount)
	step := m.Scales[2]
	for i := range out.Normals {
		word := binary.LittleEndian.Uint32(packed[4*i:])
		theta := float32(word>>16) * step
		sinT := float32(math.Sin(float64(theta)))
		phi := float32(word&0xffff)*step/max(sinT, 1e-6) - math.Pi
		out.Normals[i] = [3]float32{
			float32(math.Cos(float64(phi))) * sinT,
			float32(math.Sin(float64(phi))) * sinT,
			float32(math.Cos(float64(theta))),
		}
	}
	return out, nil
}

func dequantize(q int64, step float32) float32 {
	return float32(float64(float32(q)) * float64(step))
}

// predict rebuilds quantized attributes: breadth first over triangles, the
// first vertex of a component uses the running predictor, the second its
// neighbour, the third the edge midpoint, later ones the parallelogram rule.
func predict(corners, opp []int, residuals []int32, channels int, r *reader) ([]int64, error) {
	if r.err != nil {
		return nil, r.err
	}
	if len(residuals)%channels != 0 {
		return nil, errors.New("predict: residual shape")
	}
	count := len(residuals) / channels
	val := make([]int64, len(residuals))
	set := make([]bool, count)
	pred := make([]int64, channels)
	assign := func(v int) error {
		if v < 0 || v >= count {
			return errors.New("predict: vertex out of range")
		}
		for k := 0; k < channels; k++ {
			z := int64(residuals[v*channels+k])
			val[v*channels+k] = pred[k] + (z>>1 ^ -(z & 1))
		}
		set[v] = true
		return nil
	}
	at := func(v, k int) int64 { return val[v*channels+k] }
	tris := len(corners) / 3
	visited := make([]bool, tris)
	queue := make([]int, 0, tris)
	for seed := 0; seed < tris; seed++ {
		if visited[seed] {
			continue
		}
		a, b, c := corners[3*seed], corners[3*seed+1], corners[3*seed+2]
		if a < 0 || a >= count || b < 0 || b >= count || c < 0 || c >= count {
			return nil, errors.New("predict: vertex out of range")
		}
		if !set[a] {
			if err := assign(a); err != nil {
				return nil, err
			}
		}
		if !set[b] {
			for k := range pred {
				pred[k] = at(a, k)
			}
			if err := assign(b); err != nil {
				return nil, err
			}
		}
		if !set[c] {
			for k := range pred {
				pred[k] = (at(a, k) + at(b, k) + 1) / 2
			}
			if err := assign(c); err != nil {
				return nil, err
			}
		}
		visited[seed] = true
		queue = append(queue[:0], seed)
		for qi := 0; qi < len(queue); qi++ {
			t := queue[qi]
			for corner := 3 * t; corner < 3*t+3; corner++ {
				other := opp[corner]
				if other < 0 {
					continue
				}
				if other >= len(corners) {
					return nil, errors.New("predict: adjacency out of range")
				}
				if v := corners[other]; !set[v] {
					n, p, o := corners[nextCorner(corner)], corners[prevCorner(corner)], corners[corner]
					for k := range pred {
						pred[k] = at(n, k) + at(p, k) - at(o, k)
					}
					if err := assign(v); err != nil {
						return nil, err
					}
				}
				if adj := other / 3; !visited[adj] {
					visited[adj] = true
					queue = append(queue, adj)
				}
			}
		}
	}
	for _, s := range set {
		if !s {
			return nil, errors.New("predict: unvisited vertex")
		}
	}
	return val, nil
}

// pairEdges builds the opposite table for an index list by matching each
// directed edge with the first unmatched reverse edge.
func pairEdges(corners []int) []int {
	opp := make([]int, len(corners))
	for i := range opp {
		opp[i] = -1
	}
	open := map[[2]int][]int{}
	for c, v := range corners {
		w := corners[nextCorner(c)]
		key := [2]int{w, v}
		if list := open[key]; len(list) > 0 {
			earlier := list[0]
			open[key] = list[1:]
			a, b := prevCorner(earlier), prevCorner(c)
			opp[a], opp[b] = b, a
			continue
		}
		open[[2]int{v, w}] = append(open[[2]int{v, w}], c)
	}
	return opp
}

// uvPatches floods triangles across non-seam edges. Each flood is one patch
// with its own UV numbering per vertex and one atlas index.
func uvPatches(faces, opp []int, seams []byte, patchAtlas []int32) ([]int, []int32, int, error) {
	tris := len(faces) / 3
	if len(seams) != (tris+1)/2 {
		return nil, nil, 0, errors.New("uv: seam mask length")
	}
	adj := append([]int(nil), opp...)
	visited := make([]bool, tris)
	uv := make([]int, len(faces))
	atlas := make([]int32, tris)
	count, patch := 0, 0
	for seed := 0; seed < tris; seed++ {
		if visited[seed] {
			continue
		}
		if patch >= len(patchAtlas) {
			return nil, nil, 0, errors.New("uv: more patches than atlas entries")
		}
		local := map[int]int{}
		stack := []int{seed}
		visited[seed] = true
		for len(stack) > 0 {
			t := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			atlas[t] = patchAtlas[patch]
			flags := seams[t/2] >> (t % 2 * 4) & 15
			for e := 0; e < 3; e++ {
				c := 3*t + e
				v := faces[c]
				idx, ok := local[v]
				if !ok {
					idx = count
					local[v] = idx
					count++
				}
				uv[c] = idx
				other := adj[c]
				if other < 0 {
					continue
				}
				if n := other / 3; flags>>e&1 == 0 && !visited[n] {
					visited[n] = true
					stack = append(stack, n)
				}
				adj[c], adj[other] = -1, -1
			}
		}
		patch++
	}
	if patch != len(patchAtlas) {
		return nil, nil, 0, errors.New("uv: unused atlas entries")
	}
	return uv, atlas, count, nil
}

type reader struct {
	src []byte
	at  int
	err error
}

func (r *reader) varint() int {
	if r.err != nil {
		return 0
	}
	v, at, err := signedVarint(r.src, r.at)
	r.at, r.err = at, err
	return v
}

func (r *reader) block(count, lanes int) []byte {
	if r.err != nil {
		return nil
	}
	if count < 0 {
		r.err = errors.New("negative block count")
		return nil
	}
	b, at, err := readBlock(r.src, r.at, count, lanes)
	r.at, r.err = at, err
	return b
}

func int32s(b []byte) []int32 {
	out := make([]int32, len(b)/4)
	for i := range out {
		out[i] = int32(binary.LittleEndian.Uint32(b[4*i:]))
	}
	return out
}

func triplets(corners []int) [][3]uint32 {
	out := make([][3]uint32, len(corners)/3)
	for i := range out {
		out[i] = [3]uint32{uint32(corners[3*i]), uint32(corners[3*i+1]), uint32(corners[3*i+2])}
	}
	return out
}
