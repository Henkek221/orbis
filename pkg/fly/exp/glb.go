package exp

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"runtime/debug"
	"sort"

	"github.com/Henkek221/orbis/pkg/fly/c3m"
	"github.com/Henkek221/orbis/pkg/oth"
)

// GLBExport writes one binary glTF 2.0 file with embedded textures. Positions
// are metres in a local frame around the first tile, in glTF convention:
// +X east, +Y up, +Z south. Engines import it at real scale with no rotation.
type GLBExport struct {
	path   string
	tmpDir string
	latRad float64
	lonRad float64
	latDeg float64
	lonDeg float64

	originSet bool
	origin    [3]float64

	bin       bytes.Buffer
	doc       gltfDocument
	minBounds [3]float64
	maxBounds [3]float64
	triangles int
}

// NewGLB prepares a GLB written to path on Close. Latitude and longitude are
// the centre of the export and define the local frame.
func NewGLB(path string, latitude, longitude float64) (*GLBExport, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	e := &GLBExport{
		path:      path,
		tmpDir:    filepath.Dir(path),
		latRad:    latitude * math.Pi / 180,
		lonRad:    longitude * math.Pi / 180,
		latDeg:    latitude,
		lonDeg:    longitude,
		minBounds: [3]float64{math.Inf(1), math.Inf(1), math.Inf(1)},
		maxBounds: [3]float64{math.Inf(-1), math.Inf(-1), math.Inf(-1)},
	}
	e.doc.Asset = gltfAsset{Version: "2.0", Generator: "Orbis"}
	e.doc.Samplers = []gltfSampler{{MagFilter: 9729, MinFilter: 9987, WrapS: 33071, WrapT: 33071}}
	e.doc.Scenes = []gltfScene{{Name: "orbis", Nodes: []int{0}}}
	e.doc.Nodes = []gltfNode{{Name: "orbis"}}
	e.doc.ExtensionsUsed = []string{"KHR_materials_unlit"}
	return e, nil
}

// Next adds every mesh of a tile as one glTF mesh with a primitive per
// material group.
func (e *GLBExport) Next(tile c3m.C3M, subPfx string) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = errors.New(fmt.Sprintln(r, string(debug.Stack())))
		}
	}()
	if !e.originSet {
		e.origin = centredOrigin(e.latRad, e.lonRad, tile.Header.Translation)
		e.originSet = true
	}

	materials := make([]int, len(tile.Materials))
	for i, material := range tile.Materials {
		materials[i] = e.addMaterial(material, fmt.Sprintf("tile_%s_%d", subPfx, i))
	}

	for meshIndex, mesh := range tile.Meshes {
		if len(mesh.Vertices) == 0 {
			continue
		}
		positions := make([]vector3, len(mesh.Vertices))
		for i, vtx := range mesh.Vertices {
			enu := e.toENU(tile, vtx)
			// ENU (x east, y north, z up) to glTF (x east, y up, z south).
			positions[i] = vector3{x: enu.x, y: enu.z, z: -enu.y}
		}
		// Apple's faces wind clockwise. glTF front faces are counter-clockwise,
		// so faces are written as a, c, b and the normals flipped to match.
		normals := outwardNormals(mesh, positions)

		posData := make([]float32, 0, 3*len(positions))
		minP := [3]float64{math.Inf(1), math.Inf(1), math.Inf(1)}
		maxP := [3]float64{math.Inf(-1), math.Inf(-1), math.Inf(-1)}
		for _, p := range positions {
			v := [3]float64{p.x, p.y, p.z}
			for k := 0; k < 3; k++ {
				minP[k] = math.Min(minP[k], float64(float32(v[k])))
				maxP[k] = math.Max(maxP[k], float64(float32(v[k])))
				e.minBounds[k] = math.Min(e.minBounds[k], v[k])
				e.maxBounds[k] = math.Max(e.maxBounds[k], v[k])
			}
			posData = append(posData, float32(p.x), float32(p.y), float32(p.z))
		}
		normData := make([]float32, 0, 3*len(normals))
		for _, n := range normals {
			normData = append(normData, float32(n.x), float32(n.y), float32(n.z))
		}
		uvData := make([]float32, 0, 2*len(mesh.Vertices))
		for _, vtx := range mesh.Vertices {
			// glTF puts the texture origin top left, OBJ bottom left.
			uvData = append(uvData, vtx.U, 1-vtx.V)
		}

		posAccessor := e.addAccessor(e.addView(float32Bytes(posData), 34962), 5126, len(positions), "VEC3", minP[:], maxP[:])
		normAccessor := e.addAccessor(e.addView(float32Bytes(normData), 34962), 5126, len(positions), "VEC3", nil, nil)
		uvAccessor := e.addAccessor(e.addView(float32Bytes(uvData), 34962), 5126, len(positions), "VEC2", nil, nil)

		groupIDs := make([]int, 0, len(mesh.Groups))
		for id := range mesh.Groups {
			groupIDs = append(groupIDs, id)
		}
		sort.Ints(groupIDs)
		var primitives []gltfPrimitive
		for _, id := range groupIDs {
			group := mesh.Groups[id]
			indices := make([]uint32, 0, 3*len(group.Faces))
			for _, f := range group.Faces {
				a, b, c := int(f.A), int(f.B), int(f.C)
				if a < 0 || b < 0 || c < 0 || a >= len(positions) || b >= len(positions) || c >= len(positions) {
					continue
				}
				indices = append(indices, uint32(a), uint32(c), uint32(b))
			}
			if len(indices) == 0 {
				continue
			}
			e.triangles += len(indices) / 3
			prim := gltfPrimitive{
				Attributes: map[string]int{"POSITION": posAccessor, "NORMAL": normAccessor, "TEXCOORD_0": uvAccessor},
				Indices:    e.addAccessor(e.addView(uint32Bytes(indices), 34963), 5125, len(indices), "SCALAR", nil, nil),
				Mode:       4,
			}
			material := group.Material
			if material < 0 || material >= len(materials) {
				material = id
			}
			if material >= 0 && material < len(materials) {
				m := materials[material]
				prim.Material = &m
			}
			primitives = append(primitives, prim)
		}
		if len(primitives) == 0 {
			continue
		}
		e.doc.Meshes = append(e.doc.Meshes, gltfMesh{Name: fmt.Sprintf("tile_%s_%d", subPfx, meshIndex), Primitives: primitives})
		meshID := len(e.doc.Meshes) - 1
		e.doc.Nodes = append(e.doc.Nodes, gltfNode{Name: fmt.Sprintf("tile_%s_%d", subPfx, meshIndex), Mesh: &meshID})
		e.doc.Nodes[0].Children = append(e.doc.Nodes[0].Children, len(e.doc.Nodes)-1)
	}
	return nil
}

// Close writes the GLB. An export without meshes writes nothing.
func (e *GLBExport) Close() error {
	if len(e.doc.Meshes) == 0 {
		return nil
	}
	e.doc.Buffers = []gltfBuffer{{ByteLength: e.bin.Len()}}
	e.doc.Asset.Extras = map[string]any{
		"orbis": map[string]any{
			"frame":      "local metres, +X east, +Y up, +Z south; origin is the export centre at ecefOrigin",
			"centre":     map[string]float64{"lat": e.latDeg, "lon": e.lonDeg},
			"ecefOrigin": e.origin,
			"boundsMin":  e.minBounds,
			"boundsMax":  e.maxBounds,
			"triangles":  e.triangles,
			"source":     "Apple Maps 3D data; see Orbis's README for terms",
		},
	}
	jsonChunk, err := json.Marshal(e.doc)
	if err != nil {
		return err
	}
	for len(jsonChunk)%4 != 0 {
		jsonChunk = append(jsonChunk, ' ')
	}
	binChunk := e.bin.Bytes()
	for len(binChunk)%4 != 0 {
		binChunk = append(binChunk, 0)
	}

	f, err := os.Create(e.path)
	if err != nil {
		return err
	}
	total := 12 + 8 + len(jsonChunk) + 8 + len(binChunk)
	header := []uint32{0x46546C67, 2, uint32(total), uint32(len(jsonChunk)), 0x4E4F534A}
	if err := binary.Write(f, binary.LittleEndian, header); err != nil {
		f.Close()
		return err
	}
	if _, err := f.Write(jsonChunk); err != nil {
		f.Close()
		return err
	}
	if err := binary.Write(f, binary.LittleEndian, []uint32{uint32(len(binChunk)), 0x004E4942}); err != nil {
		f.Close()
		return err
	}
	if _, err := f.Write(binChunk); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}

// Path is the file Close writes.
func (e *GLBExport) Path() string {
	return e.path
}

func (e *GLBExport) toENU(tile c3m.C3M, vertex c3m.Vertex) vector3 {
	x, y, z := float64(vertex.X), float64(vertex.Y), float64(vertex.Z)
	r := tile.Header.Rotation
	x, y, z = r[0]*x+r[1]*y+r[2]*z, r[3]*x+r[4]*y+r[5]*z, r[6]*x+r[7]*y+r[8]*z
	dx := x + tile.Header.Translation[0] - e.origin[0]
	dy := y + tile.Header.Translation[1] - e.origin[1]
	dz := z + tile.Header.Translation[2] - e.origin[2]
	sinLat, cosLat := math.Sin(e.latRad), math.Cos(e.latRad)
	sinLon, cosLon := math.Sin(e.lonRad), math.Cos(e.lonRad)
	return vector3{
		x: -sinLon*dx + cosLon*dy,
		y: -sinLat*cosLon*dx - sinLat*sinLon*dy + cosLat*dz,
		z: cosLat*cosLon*dx + cosLat*sinLon*dy + sinLat*dz,
	}
}

// addMaterial embeds the texture as JPEG. HEIC is converted with a local tool;
// without one the material stays untextured, since glTF has no HEIC.
func (e *GLBExport) addMaterial(material c3m.Material, name string) int {
	// Photogrammetry winding is not consistent across a tile, so materials are
	// double sided; otherwise engines cull parts of roofs and ground.
	m := gltfMaterial{
		Name:        name,
		DoubleSided: true,
		PBR:         gltfPBR{BaseColorFactor: []float64{1, 1, 1, 1}, MetallicFactor: 0, RoughnessFactor: 1},
		Extensions:  map[string]any{"KHR_materials_unlit": map[string]any{}},
	}
	if image := e.jpegFor(material, name); len(image) > 0 {
		view := e.addView(image, 0)
		e.doc.Images = append(e.doc.Images, gltfImage{BufferView: view, MimeType: "image/jpeg"})
		e.doc.Textures = append(e.doc.Textures, gltfTexture{Source: len(e.doc.Images) - 1, Sampler: 0})
		m.PBR.BaseColorTexture = &gltfTextureRef{Index: len(e.doc.Textures) - 1}
	} else {
		m.PBR.BaseColorFactor = []float64{0.6, 0.6, 0.6, 1}
	}
	e.doc.Materials = append(e.doc.Materials, m)
	return len(e.doc.Materials) - 1
}

func (e *GLBExport) jpegFor(material c3m.Material, name string) []byte {
	if len(material.JPEG) > 0 {
		return material.JPEG
	}
	if len(material.HEIC) == 0 {
		return nil
	}
	heicPath := filepath.Join(e.tmpDir, ".glb_"+name+".heic")
	jpgPath := filepath.Join(e.tmpDir, ".glb_"+name+".jpg")
	defer os.Remove(heicPath)
	defer os.Remove(jpgPath)
	if err := os.WriteFile(heicPath, material.HEIC, 0o644); err != nil {
		return nil
	}
	if err := oth.ConvertHEIC(heicPath, jpgPath); err != nil {
		return nil
	}
	data, err := os.ReadFile(jpgPath)
	if err != nil {
		return nil
	}
	return data
}

// addView appends data to the binary chunk at a 4-byte boundary. target is
// 34962 for vertex data, 34963 for indices and 0 for images.
func (e *GLBExport) addView(data []byte, target int) int {
	for e.bin.Len()%4 != 0 {
		e.bin.WriteByte(0)
	}
	view := gltfBufferView{Buffer: 0, ByteOffset: e.bin.Len(), ByteLength: len(data), Target: target}
	e.bin.Write(data)
	e.doc.BufferViews = append(e.doc.BufferViews, view)
	return len(e.doc.BufferViews) - 1
}

func (e *GLBExport) addAccessor(view, componentType, count int, kind string, min, max []float64) int {
	e.doc.Accessors = append(e.doc.Accessors, gltfAccessor{
		BufferView: view, ComponentType: componentType, Count: count, Type: kind, Min: min, Max: max,
	})
	return len(e.doc.Accessors) - 1
}

// outwardNormals are meshNormals reversed, matching the a, c, b winding.
func outwardNormals(mesh c3m.Mesh, positions []vector3) []vector3 {
	normals := meshNormals(mesh, positions)
	for i, n := range normals {
		normals[i] = vector3{-n.x, -n.y, -n.z}
	}
	return normals
}

func float32Bytes(values []float32) []byte {
	out := make([]byte, 4*len(values))
	for i, v := range values {
		binary.LittleEndian.PutUint32(out[4*i:], math.Float32bits(v))
	}
	return out
}

func uint32Bytes(values []uint32) []byte {
	out := make([]byte, 4*len(values))
	for i, v := range values {
		binary.LittleEndian.PutUint32(out[4*i:], v)
	}
	return out
}

type gltfDocument struct {
	Asset          gltfAsset        `json:"asset"`
	ExtensionsUsed []string         `json:"extensionsUsed,omitempty"`
	Scene          int              `json:"scene"`
	Scenes         []gltfScene      `json:"scenes"`
	Nodes          []gltfNode       `json:"nodes"`
	Meshes         []gltfMesh       `json:"meshes,omitempty"`
	Materials      []gltfMaterial   `json:"materials,omitempty"`
	Textures       []gltfTexture    `json:"textures,omitempty"`
	Images         []gltfImage      `json:"images,omitempty"`
	Samplers       []gltfSampler    `json:"samplers,omitempty"`
	Accessors      []gltfAccessor   `json:"accessors,omitempty"`
	BufferViews    []gltfBufferView `json:"bufferViews,omitempty"`
	Buffers        []gltfBuffer     `json:"buffers,omitempty"`
}

type gltfAsset struct {
	Version   string         `json:"version"`
	Generator string         `json:"generator"`
	Extras    map[string]any `json:"extras,omitempty"`
}

type gltfScene struct {
	Name  string `json:"name"`
	Nodes []int  `json:"nodes"`
}

type gltfNode struct {
	Name     string `json:"name"`
	Mesh     *int   `json:"mesh,omitempty"`
	Children []int  `json:"children,omitempty"`
}

type gltfMesh struct {
	Name       string          `json:"name"`
	Primitives []gltfPrimitive `json:"primitives"`
}

type gltfPrimitive struct {
	Attributes map[string]int `json:"attributes"`
	Indices    int            `json:"indices"`
	Material   *int           `json:"material,omitempty"`
	Mode       int            `json:"mode"`
}

type gltfMaterial struct {
	Name        string         `json:"name"`
	DoubleSided bool           `json:"doubleSided,omitempty"`
	PBR         gltfPBR        `json:"pbrMetallicRoughness"`
	Extensions  map[string]any `json:"extensions,omitempty"`
}

type gltfPBR struct {
	BaseColorFactor  []float64       `json:"baseColorFactor"`
	BaseColorTexture *gltfTextureRef `json:"baseColorTexture,omitempty"`
	MetallicFactor   float64         `json:"metallicFactor"`
	RoughnessFactor  float64         `json:"roughnessFactor"`
}

type gltfTextureRef struct {
	Index int `json:"index"`
}

type gltfTexture struct {
	Source  int `json:"source"`
	Sampler int `json:"sampler"`
}

type gltfImage struct {
	BufferView int    `json:"bufferView"`
	MimeType   string `json:"mimeType"`
}

type gltfSampler struct {
	MagFilter int `json:"magFilter"`
	MinFilter int `json:"minFilter"`
	WrapS     int `json:"wrapS"`
	WrapT     int `json:"wrapT"`
}

type gltfAccessor struct {
	BufferView    int       `json:"bufferView"`
	ComponentType int       `json:"componentType"`
	Count         int       `json:"count"`
	Type          string    `json:"type"`
	Min           []float64 `json:"min,omitempty"`
	Max           []float64 `json:"max,omitempty"`
}

type gltfBufferView struct {
	Buffer     int `json:"buffer"`
	ByteOffset int `json:"byteOffset"`
	ByteLength int `json:"byteLength"`
	Target     int `json:"target,omitempty"`
}

type gltfBuffer struct {
	ByteLength int `json:"byteLength"`
}
