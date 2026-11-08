package exp

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Henkek221/orbis/pkg/fly/c3m"
)

func TestLocalizedExportUsesMetersUpAxisAndNormals(t *testing.T) {
	dir := t.TempDir()
	exporter, err := NewLocalized(dir, "exp_", 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	tile := c3m.C3M{
		Header: c3m.Header{
			Translation: [3]float64{1000, 2000, 3000},
			Rotation:    [9]float64{1, 0, 0, 0, 1, 0, 0, 0, 1},
		},
		Meshes: []c3m.Mesh{{
			Vertices: []c3m.Vertex{
				{X: 0, Y: 0, Z: 0},
				{X: 1, Y: 0, Z: 0},
				{X: 0, Y: 1, Z: 0},
			},
			Groups: map[int]c3m.Group{
				0: {Faces: []c3m.Face{{A: 0, B: 1, C: 2}}},
			},
		}},
	}
	if err := exporter.Next(tile, "0"); err != nil {
		t.Fatal(err)
	}
	if err := exporter.Close(); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(filepath.Join(dir, "exp_model.obj"))
	if err != nil {
		t.Fatal(err)
	}
	obj := string(data)
	for _, want := range []string{
		"# Orbis local East-North-Up coordinates in meters",
		"v 0 0 0",
		"v 0 0 1",
		"vn ",
		"f 1/1/1 2/2/2 3/3/3",
	} {
		if !strings.Contains(obj, want) {
			t.Fatalf("OBJ does not contain %q:\n%s", want, obj)
		}
	}
}
