// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Henrik Hellpap. Part of Orbis: https://github.com/Henkek221/orbis

package exp

import "github.com/Henkek221/orbis/pkg/fly/c3m"

// orientMesh returns the mesh with every face wound counter-clockwise seen from
// outside. Apple's winding is consistent inside a connected piece of a tile but
// not across pieces, and viewers such as Quick Look ignore double-sided
// materials, so each piece is oriented on its own: its roughly horizontal faces
// (ground, roofs) vote, weighted by area, on whether they face up. A piece
// without horizontal faces keeps the common source winding (clockwise), which
// is flipped. positions are in the output frame with +Y up.
func orientMesh(mesh c3m.Mesh, positions []vector3) c3m.Mesh {
	parent := make([]int, len(positions))
	for i := range parent {
		parent[i] = i
	}
	var find func(int) int
	find = func(i int) int {
		for parent[i] != i {
			parent[i] = parent[parent[i]]
			i = parent[i]
		}
		return i
	}
	union := func(a, b int) {
		if ra, rb := find(a), find(b); ra != rb {
			parent[ra] = rb
		}
	}
	valid := func(f c3m.Face) bool {
		n := int32(len(positions))
		return f.A >= 0 && f.B >= 0 && f.C >= 0 && f.A < n && f.B < n && f.C < n
	}
	for _, g := range mesh.Groups {
		for _, f := range g.Faces {
			if valid(f) {
				union(int(f.A), int(f.B))
				union(int(f.A), int(f.C))
			}
		}
	}

	vote := map[int]float64{}
	for _, g := range mesh.Groups {
		for _, f := range g.Faces {
			if !valid(f) {
				continue
			}
			n := cross(subtract(positions[f.B], positions[f.A]), subtract(positions[f.C], positions[f.A]))
			if l := length(n); l > 0 && n.y*n.y > 0.25*l*l {
				vote[find(int(f.A))] += n.y
			}
		}
	}

	out := c3m.Mesh{Vertices: mesh.Vertices, Groups: make(map[int]c3m.Group, len(mesh.Groups))}
	for id, g := range mesh.Groups {
		faces := make([]c3m.Face, 0, len(g.Faces))
		for _, f := range g.Faces {
			if valid(f) && vote[find(int(f.A))] <= 0 {
				f.B, f.C = f.C, f.B
			}
			faces = append(faces, f)
		}
		out.Groups[id] = c3m.Group{Material: g.Material, Faces: faces}
	}
	return out
}
