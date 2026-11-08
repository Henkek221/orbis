package exp

import (
	"bufio"
	"errors"
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"runtime/debug"
	"sort"
	"strconv"

	"github.com/Henkek221/mappy/pkg/fly/c3m"
)

// USDExport writes a text USD stage (.usda) with UsdPreviewSurface materials
// and the textures beside it, Y up, one unit per metre. On Close it packages
// a .usdz with Apple's usdzip when that tool is installed (macOS).
type USDExport struct {
	dir    string
	name   string
	latRad float64
	lonRad float64

	originSet bool
	origin    [3]float64
	out       *bufio.Writer
	file      *os.File
	meshes    int
	usdzPath  string
}

// NewUSD prepares dir/name.usda and, if possible, dir/name.usdz.
func NewUSD(dir, name string, latitude, longitude float64) (*USDExport, error) {
	if err := os.MkdirAll(filepath.Join(dir, "textures"), 0o755); err != nil {
		return nil, err
	}
	f, err := os.Create(filepath.Join(dir, name+".usda"))
	if err != nil {
		return nil, err
	}
	e := &USDExport{
		dir: dir, name: name, file: f, out: bufio.NewWriterSize(f, 1<<20),
		latRad: latitude * math.Pi / 180, lonRad: longitude * math.Pi / 180,
	}
	fmt.Fprintf(e.out, `#usda 1.0
(
    defaultPrim = "mappy"
    metersPerUnit = 1
    upAxis = "Y"
    customLayerData = {
        string generator = "Mappy"
        double latitude = %s
        double longitude = %s
    }
)

def Xform "mappy" (
    kind = "component"
)
{
`, ftoa(latitude), ftoa(longitude))
	return e, nil
}

// Next writes each material group of each mesh as its own Mesh prim.
func (e *USDExport) Next(tile c3m.C3M, subPfx string) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = errors.New(fmt.Sprintln(r, string(debug.Stack())))
		}
	}()
	if !e.originSet {
		e.origin = centredOrigin(e.latRad, e.lonRad, tile.Header.Translation)
		e.originSet = true
	}
	glb := GLBExport{latRad: e.latRad, lonRad: e.lonRad, origin: e.origin, tmpDir: e.dir}

	materials := make([]string, len(tile.Materials))
	for i, material := range tile.Materials {
		name := fmt.Sprintf("mtl_%s_%d", subPfx, i)
		texture := ""
		if data := glb.jpegFor(material, name); len(data) > 0 {
			texture = "textures/" + name + ".jpg"
			if err := os.WriteFile(filepath.Join(e.dir, texture), data, 0o644); err != nil {
				return err
			}
		}
		e.writeMaterial(name, texture)
		materials[i] = name
	}

	for meshIndex, mesh := range tile.Meshes {
		positions := make([]vector3, len(mesh.Vertices))
		for i, vtx := range mesh.Vertices {
			enu := glb.toENU(tile, vtx)
			positions[i] = vector3{x: enu.x, y: enu.z, z: -enu.y}
		}
		normals := meshNormals(mesh, positions)

		groupIDs := make([]int, 0, len(mesh.Groups))
		for id := range mesh.Groups {
			groupIDs = append(groupIDs, id)
		}
		sort.Ints(groupIDs)
		for _, id := range groupIDs {
			group := mesh.Groups[id]
			material := group.Material
			if material < 0 || material >= len(materials) {
				material = id
			}
			binding := ""
			if material >= 0 && material < len(materials) {
				binding = materials[material]
			}
			e.writeMesh(fmt.Sprintf("tile_%s_%d_%d", subPfx, meshIndex, id), mesh, group, positions, normals, binding)
		}
	}
	return nil
}

func (e *USDExport) writeMaterial(name, texture string) {
	path := "/mappy/" + name
	fmt.Fprintf(e.out, `    def Material "%s"
    {
        token outputs:surface.connect = <%s/Surface.outputs:surface>

        def Shader "Surface"
        {
            uniform token info:id = "UsdPreviewSurface"
`, name, path)
	if texture != "" {
		fmt.Fprintf(e.out, "            color3f inputs:diffuseColor.connect = <%s/Texture.outputs:rgb>\n", path)
	} else {
		fmt.Fprintf(e.out, "            color3f inputs:diffuseColor = (0.6, 0.6, 0.6)\n")
	}
	fmt.Fprintf(e.out, `            float inputs:metallic = 0
            float inputs:roughness = 1
            token outputs:surface
        }
`)
	if texture != "" {
		fmt.Fprintf(e.out, `
        def Shader "Reader"
        {
            uniform token info:id = "UsdPrimvarReader_float2"
            string inputs:varname = "st"
            float2 outputs:result
        }

        def Shader "Texture"
        {
            uniform token info:id = "UsdUVTexture"
            asset inputs:file = @%s@
            float2 inputs:st.connect = <%s/Reader.outputs:result>
            token inputs:wrapS = "clamp"
            token inputs:wrapT = "clamp"
            float3 outputs:rgb
        }
`, texture, path)
	}
	fmt.Fprintf(e.out, "    }\n\n")
}

// writeMesh writes one material group with only the vertices it uses.
func (e *USDExport) writeMesh(name string, mesh c3m.Mesh, group c3m.Group, positions, normals []vector3, material string) {
	remap := make(map[int32]int)
	var order []int32
	var indices []int
	for _, f := range group.Faces {
		ok := true
		for _, v := range [3]int32{f.A, f.B, f.C} {
			if v < 0 || int(v) >= len(positions) {
				ok = false
			}
		}
		if !ok {
			continue
		}
		for _, v := range [3]int32{f.A, f.B, f.C} {
			idx, seen := remap[v]
			if !seen {
				idx = len(order)
				remap[v] = idx
				order = append(order, v)
			}
			indices = append(indices, idx)
		}
	}
	if len(indices) == 0 {
		return
	}
	e.meshes++

	minP := [3]float64{math.Inf(1), math.Inf(1), math.Inf(1)}
	maxP := [3]float64{math.Inf(-1), math.Inf(-1), math.Inf(-1)}
	for _, v := range order {
		p := positions[v]
		for k, c := range [3]float64{p.x, p.y, p.z} {
			minP[k] = math.Min(minP[k], c)
			maxP[k] = math.Max(maxP[k], c)
		}
	}

	w := e.out
	if material != "" {
		fmt.Fprintf(w, "    def Mesh \"%s\" (\n        prepend apiSchemas = [\"MaterialBindingAPI\"]\n    )\n", name)
	} else {
		fmt.Fprintf(w, "    def Mesh \"%s\"\n", name)
	}
	w.WriteString("    {\n        uniform token subdivisionScheme = \"none\"\n")
	fmt.Fprintf(w, "        float3[] extent = [(%s, %s, %s), (%s, %s, %s)]\n",
		ftoa(minP[0]), ftoa(minP[1]), ftoa(minP[2]), ftoa(maxP[0]), ftoa(maxP[1]), ftoa(maxP[2]))
	w.WriteString("        int[] faceVertexCounts = [")
	for i := 0; i < len(indices)/3; i++ {
		if i > 0 {
			w.WriteString(", ")
		}
		w.WriteString("3")
	}
	w.WriteString("]\n        int[] faceVertexIndices = [")
	for i, idx := range indices {
		if i > 0 {
			w.WriteString(", ")
		}
		w.WriteString(strconv.Itoa(idx))
	}
	w.WriteString("]\n        point3f[] points = [")
	for i, v := range order {
		if i > 0 {
			w.WriteString(", ")
		}
		p := positions[v]
		fmt.Fprintf(w, "(%s, %s, %s)", ftoa(p.x), ftoa(p.y), ftoa(p.z))
	}
	w.WriteString("]\n        normal3f[] normals = [")
	for i, v := range order {
		if i > 0 {
			w.WriteString(", ")
		}
		n := normals[v]
		fmt.Fprintf(w, "(%s, %s, %s)", ftoa(n.x), ftoa(n.y), ftoa(n.z))
	}
	w.WriteString("] (\n            interpolation = \"vertex\"\n        )\n        texCoord2f[] primvars:st = [")
	for i, v := range order {
		if i > 0 {
			w.WriteString(", ")
		}
		vtx := mesh.Vertices[v]
		fmt.Fprintf(w, "(%s, %s)", ftoa(float64(vtx.U)), ftoa(float64(vtx.V)))
	}
	w.WriteString("] (\n            interpolation = \"vertex\"\n        )\n")
	if material != "" {
		fmt.Fprintf(w, "        rel material:binding = </mappy/%s>\n", material)
	}
	w.WriteString("    }\n\n")
}

// Close finishes the stage and packages a .usdz when usdzip is available.
func (e *USDExport) Close() error {
	e.out.WriteString("}\n")
	if err := e.out.Flush(); err != nil {
		e.file.Close()
		return err
	}
	if err := e.file.Close(); err != nil {
		return err
	}
	if e.meshes == 0 {
		return nil
	}
	tool, err := exec.LookPath("usdzip")
	if err != nil {
		return nil
	}
	usdz := e.name + ".usdz"
	cmd := exec.Command(tool, "--arkitAsset", e.name+".usda", usdz)
	cmd.Dir = e.dir
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("usdzip: %v: %s", err, output)
	}
	e.usdzPath = filepath.Join(e.dir, usdz)
	return nil
}

// Path is the .usda stage.
func (e *USDExport) Path() string {
	return filepath.Join(e.dir, e.name+".usda")
}

// USDZPath is the packaged file, empty until Close succeeded with usdzip.
func (e *USDExport) USDZPath() string {
	return e.usdzPath
}

func ftoa(v float64) string {
	return strconv.FormatFloat(v, 'g', 7, 64)
}
