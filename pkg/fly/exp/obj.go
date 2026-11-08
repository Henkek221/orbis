package exp

import (
	"bufio"
	"errors"
	"fmt"
	"math"
	"os"
	"path"
	"runtime/debug"
	"sort"

	"github.com/Henkek221/mappy/pkg/fly/c3m"
	"github.com/Henkek221/mappy/pkg/oth"
)

var transform = true

type Export interface {
	Next(c3m c3m.C3M, subPfx string) error
	Close() error
}

func New(dir string, fnPfx string) (Export, error) {
	return newOBJExport(dir, fnPfx)
}

// NewLocalized writes meter-scale East-North-Up coordinates around the first
// exported tile. This keeps values close to zero and Z pointing up, which is a
// safer source format for DCC tools and game engines.
func NewLocalized(dir string, fnPfx string, latitude, longitude float64) (Export, error) {
	exporter, err := newOBJExport(dir, fnPfx)
	if err != nil {
		return nil, err
	}
	exporter.localize = true
	exporter.latRad = latitude * math.Pi / 180
	exporter.lonRad = longitude * math.Pi / 180
	exporter.objWriter.write("# Mappy local East-North-Up coordinates in meters\n")
	return exporter, nil
}

func newOBJExport(dir string, fnPfx string) (*OBJExport, error) {
	objWriter, err := newWriter(path.Join(dir, fmt.Sprintf("%smodel.obj", fnPfx)))
	if err != nil {
		return nil, err
	}
	mtlWriter, err := newWriter(path.Join(dir, fmt.Sprintf("%smodel.mtl", fnPfx)))
	if err != nil {
		return nil, err
	}
	// One material library reference at the top of the file: repeating it per
	// mesh makes some importers reload the textures for every tile.
	objWriter.write(fmt.Sprintf("mtllib %smodel.mtl\n", fnPfx))
	return &OBJExport{
		dir:       dir,
		fnPfx:     fnPfx,
		vtxCount:  0,
		objWriter: objWriter,
		mtlWriter: mtlWriter,
	}, nil
}

func (e *OBJExport) Close() (err error) {
	if err = e.objWriter.done(); err != nil {
		return
	}
	if err = e.mtlWriter.done(); err != nil {
		return
	}
	return
}

type OBJExport struct {
	dir       string
	fnPfx     string
	vtxCount  int
	objWriter writer
	mtlWriter writer
	localize  bool
	originSet bool
	origin    [3]float64
	latRad    float64
	lonRad    float64
}

type vector3 struct {
	x float64
	y float64
	z float64
}

func (e *OBJExport) Next(c3m c3m.C3M, subPfx string) (err error) {
	defer func() {
		if e := recover(); e != nil {
			err = errors.New(fmt.Sprintln(e, string(debug.Stack())))
		}
	}()

	if e.localize && !e.originSet {
		e.origin = c3m.Header.Translation
		e.originSet = true
	}

	for i, material := range c3m.Materials {
		textureFile := e.writeTexture(material, subPfx, i)
		// Photo textures already contain the lighting, so ambient and specular
		// stay black. Anything else makes the scan look plastic on import.
		nxt := fmt.Sprintf(`
newmtl mtl_%s_%d
Ka 0.000 0.000 0.000
Kd 1.000 1.000 1.000
Ks 0.000 0.000 0.000
Ns 1.0
d 1.0
illum 1
map_Kd %s
`, subPfx, i, textureFile)
		e.mtlWriter.write(nxt)
	}

	for i, mesh := range c3m.Meshes {
		e.objWriter.write(fmt.Sprintf("o tile_%s_%d\n", subPfx, i))
		positions := make([]vector3, len(mesh.Vertices))
		for index, vtx := range mesh.Vertices {
			positions[index] = e.transformVertex(c3m, vtx)
		}
		normals := meshNormals(mesh, positions)
		for index, vtx := range mesh.Vertices {
			position := positions[index]
			normal := normals[index]
			e.objWriter.write(fmt.Sprintln("v", position.x, position.y, position.z))
			e.objWriter.write(fmt.Sprintln("vt", vtx.U, vtx.V))
			e.objWriter.write(fmt.Sprintln("vn", normal.x, normal.y, normal.z))
		}

		groupIDs := make([]int, 0, len(mesh.Groups))
		for groupID := range mesh.Groups {
			groupIDs = append(groupIDs, groupID)
		}
		sort.Ints(groupIDs)
		for _, groupID := range groupIDs {
			group := mesh.Groups[groupID]
			e.objWriter.write(fmt.Sprintf("g g_%s_%d\n", subPfx, groupID))
			e.objWriter.write(fmt.Sprintf("usemtl mtl_%s_%d\n", subPfx, groupID))
			for _, face := range group.Faces {
				a, b, c := int(face.A)+1+e.vtxCount, int(face.B)+1+e.vtxCount, int(face.C)+1+e.vtxCount
				e.objWriter.write(fmt.Sprintf("f %d/%d/%d %d/%d/%d %d/%d/%d\n", a, a, a, b, b, b, c, c, c))
			}
		}
		e.vtxCount += len(mesh.Vertices)
	}
	return
}

func (e *OBJExport) transformVertex(tile c3m.C3M, vertex c3m.Vertex) vector3 {
	x, y, z := float64(vertex.X), float64(vertex.Y), float64(vertex.Z)
	if transform {
		x, y, z = tile.Header.Rotation[0]*x+tile.Header.Rotation[1]*y+tile.Header.Rotation[2]*z,
			tile.Header.Rotation[3]*x+tile.Header.Rotation[4]*y+tile.Header.Rotation[5]*z,
			tile.Header.Rotation[6]*x+tile.Header.Rotation[7]*y+tile.Header.Rotation[8]*z
		x += tile.Header.Translation[0]
		y += tile.Header.Translation[1]
		z += tile.Header.Translation[2]
	}
	if !e.localize {
		return vector3{x, y, z}
	}

	dx, dy, dz := x-e.origin[0], y-e.origin[1], z-e.origin[2]
	sinLat, cosLat := math.Sin(e.latRad), math.Cos(e.latRad)
	sinLon, cosLon := math.Sin(e.lonRad), math.Cos(e.lonRad)
	return vector3{
		x: -sinLon*dx + cosLon*dy,
		y: -sinLat*cosLon*dx - sinLat*sinLon*dy + cosLat*dz,
		z: cosLat*cosLon*dx + cosLat*sinLon*dy + sinLat*dz,
	}
}

func meshNormals(mesh c3m.Mesh, positions []vector3) []vector3 {
	normals := make([]vector3, len(positions))
	for _, group := range mesh.Groups {
		for _, face := range group.Faces {
			a, b, c := int(face.A), int(face.B), int(face.C)
			if a < 0 || b < 0 || c < 0 || a >= len(positions) || b >= len(positions) || c >= len(positions) {
				continue
			}
			ab := subtract(positions[b], positions[a])
			ac := subtract(positions[c], positions[a])
			faceNormal := cross(ab, ac)
			normals[a] = add(normals[a], faceNormal)
			normals[b] = add(normals[b], faceNormal)
			normals[c] = add(normals[c], faceNormal)
		}
	}
	for index, normal := range normals {
		length := math.Sqrt(normal.x*normal.x + normal.y*normal.y + normal.z*normal.z)
		if length == 0 {
			normals[index] = vector3{z: 1}
			continue
		}
		normals[index] = vector3{normal.x / length, normal.y / length, normal.z / length}
	}
	return normals
}

func subtract(a, b vector3) vector3 {
	return vector3{a.x - b.x, a.y - b.y, a.z - b.z}
}

func add(a, b vector3) vector3 {
	return vector3{a.x + b.x, a.y + b.y, a.z + b.z}
}

func cross(a, b vector3) vector3 {
	return vector3{
		a.y*b.z - a.z*b.y,
		a.z*b.x - a.x*b.z,
		a.x*b.y - a.y*b.x,
	}
}

func (e *OBJExport) writeTexture(material c3m.Material, subPfx string, index int) string {
	base := fmt.Sprintf("%s%s_%d", e.fnPfx, subPfx, index)
	if len(material.JPEG) > 0 {
		name := base + ".jpg"
		oth.CheckPanic(os.WriteFile(path.Join(e.dir, name), material.JPEG, 0o644))
		return name
	}
	if len(material.HEIC) > 0 {
		heicName := base + ".heic"
		heicPath := path.Join(e.dir, heicName)
		oth.CheckPanic(os.WriteFile(heicPath, material.HEIC, 0o644))

		// Almost no 3D tool reads HEIC. Convert with whatever is installed
		// locally and only keep the HEIC when nothing is available.
		jpgName := base + ".jpg"
		if err := oth.ConvertHEIC(heicPath, path.Join(e.dir, jpgName)); err == nil {
			_ = os.Remove(heicPath)
			return jpgName
		}
		return heicName
	}
	panic("material has no texture data")
}

func create(fn string) (*os.File, error) {
	perm := os.O_CREATE | os.O_WRONLY | os.O_TRUNC
	return os.OpenFile(fn, perm, 0o655)
}

func (w *writer) write(txt string) {
	_, err := w.writer.WriteString(txt)
	oth.CheckPanic(err)
}

type writer struct {
	file   *os.File
	writer *bufio.Writer
}

func newWriter(fn string) (writer, error) {
	f, err := create(fn)
	if err != nil {
		return writer{}, err
	}
	w := bufio.NewWriter(f)
	return writer{file: f, writer: w}, nil
}

func (w writer) done() (err error) {
	if err = w.writer.Flush(); err != nil {
		return
	}
	if err = w.file.Close(); err != nil {
		return
	}
	return
}
