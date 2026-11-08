// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Henrik Hellpap. Part of Orbis: https://github.com/Henkek221/orbis

package mc

import (
	"errors"
	"math"
)

// The voxelizer runs in two stages.
//
// While tiles stream in, every triangle is supersampled and each sample adds its
// texture colour to the voxel it lands in. A voxel therefore ends up with the
// average colour of the actual surface area inside it, not the colour of one
// arbitrary pixel that happened to win a race.
//
// On close the sparse voxel set is baked into a dense occupancy volume so that
// three things become possible that a sparse set cannot do:
//
//   - despeckle: photogrammetry leaves flyaway triangles, so voxels with almost
//     no neighbours are dropped;
//   - solidify: air is flood filled from outside the bounding box, and whatever
//     air never gets reached is inside a building or under the ground, so it is
//     filled with stone. Buildings become solid instead of paper shells and the
//     terrain gets a real base;
//   - surface classification: a voxel knows whether something sits above it,
//     which is what turns green ground into grass blocks.

// samplesPerBlock is the barycentric sampling density along a triangle edge.
// Three samples per block keeps the gap between neighbouring samples at about a
// third of a block, which closes the shell without exploding the sample count.
const samplesPerBlock = 3

// maxSampleSteps caps the work a single huge triangle can cause.
const maxSampleSteps = 384

// maxSurfaceVoxels bounds the sparse stage. An accumulator entry is 16 bytes
// plus map overhead, so this is roughly 1.5 GB worst case.
const maxSurfaceVoxels = 24_000_000

// maxSolidVolume bounds the dense stage at one byte per cell. Larger selections
// keep their surface but skip despeckling and solidifying.
const maxSolidVolume = 400_000_000

type accumulator struct {
	r, g, b uint32
	n       uint32
}

type builder struct {
	cells      map[[3]int32]accumulator
	untextured int
	overflow   bool
}

func newBuilder() *builder {
	return &builder{cells: make(map[[3]int32]accumulator, 1<<16)}
}

// triangle supersamples one triangle in block space and accumulates colours.
func (b *builder) triangle(pa, pb, pc [3]float64, ua, ub, uc [2]float32, tex *texture) {
	if b.overflow {
		return
	}
	steps := sampleSteps(pa, pb, pc)
	inverse := 1 / float64(steps)

	for i := 0; i <= steps; i++ {
		for j := 0; i+j <= steps; j++ {
			v := float64(i) * inverse
			w := float64(j) * inverse
			u := 1 - v - w

			key := [3]int32{
				int32(math.Round(u*pa[0] + v*pb[0] + w*pc[0])),
				int32(math.Round(u*pa[1] + v*pb[1] + w*pc[1])),
				int32(math.Round(u*pa[2] + v*pb[2] + w*pc[2])),
			}
			r, g, blue := tex.rgbAt(
				float32(u)*ua[0]+float32(v)*ub[0]+float32(w)*uc[0],
				float32(u)*ua[1]+float32(v)*ub[1]+float32(w)*uc[1],
			)

			cell, seen := b.cells[key]
			if !seen && len(b.cells) >= maxSurfaceVoxels {
				b.overflow = true
				return
			}
			cell.r += uint32(r)
			cell.g += uint32(g)
			cell.b += uint32(blue)
			cell.n++
			b.cells[key] = cell
		}
	}
}

// sampleSteps picks the subdivision count from the longest edge of a triangle.
func sampleSteps(a, b, c [3]float64) int {
	longest := math.Max(distance(a, b), math.Max(distance(b, c), distance(a, c)))
	steps := int(math.Ceil(longest * samplesPerBlock))
	if steps < 1 {
		return 1
	}
	if steps > maxSampleSteps {
		return maxSampleSteps
	}
	return steps
}

func distance(a, b [3]float64) float64 {
	dx, dy, dz := a[0]-b[0], a[1]-b[1], a[2]-b[2]
	return math.Sqrt(dx*dx + dy*dy + dz*dz)
}

// cell states inside the dense volume.
const (
	stateEmpty uint8 = iota
	stateSurface
	stateOutside
	stateInside
)

// volume is the dense occupancy grid used for despeckling and solidifying. It
// is padded by one cell on every side so the flood fill has a guaranteed start.
type volume struct {
	origin [3]int32
	size   [3]int32
	state  []uint8
}

func (v *volume) index(x, y, z int32) int {
	return int((y*v.size[2]+z)*v.size[0] + x)
}

func (v *volume) at(x, y, z int32) uint8 {
	if x < 0 || y < 0 || z < 0 || x >= v.size[0] || y >= v.size[1] || z >= v.size[2] {
		return stateOutside
	}
	return v.state[v.index(x, y, z)]
}

var neighbours = [6][3]int32{{1, 0, 0}, {-1, 0, 0}, {0, 1, 0}, {0, -1, 0}, {0, 0, 1}, {0, 0, -1}}

// build turns the accumulated surface into a finished block grid.
func (b *builder) build(opt Options, logf func(string, ...any)) (*grid, error) {
	if len(b.cells) == 0 {
		return nil, errors.New("no mesh data reached the Minecraft export")
	}
	if b.overflow {
		logf("minecraft: hit the %d voxel limit; increase the block size for a complete world", maxSurfaceVoxels)
	}

	low, high := b.bounds()
	// Shift so the export is centred on 0/0 and its lowest point sits at BaseY.
	shift := [3]int32{
		-(low[0] + high[0]) / 2,
		int32(opt.BaseY) - low[1],
		-(low[2] + high[2]) / 2,
	}
	span := [3]int32{high[0] - low[0] + 1, high[1] - low[1] + 1, high[2] - low[2] + 1}
	logf("minecraft: %d surface voxels spanning %d x %d x %d blocks", len(b.cells), span[0], span[1], span[2])

	g := &grid{chunks: map[[2]int32]*chunk{}}
	vol := b.densify(low, high)
	if vol == nil {
		logf("minecraft: selection too large for solid fill; writing the surface shell only")
		b.writeShell(g, shift, opt)
		return g, nil
	}

	removed := vol.despeckle()
	if removed > 0 {
		logf("minecraft: removed %d loose voxels left over from the scan", removed)
	}
	interior := vol.solidify()
	logf("minecraft: filled %d interior blocks so buildings and ground are solid", interior)

	b.writeSurface(g, vol, shift)
	vol.writeInterior(g, shift)
	return g, nil
}

func (b *builder) bounds() ([3]int32, [3]int32) {
	first := true
	var low, high [3]int32
	for key := range b.cells {
		if first {
			low, high, first = key, key, false
			continue
		}
		for axis := 0; axis < 3; axis++ {
			low[axis] = min(low[axis], key[axis])
			high[axis] = max(high[axis], key[axis])
		}
	}
	return low, high
}

// densify allocates the padded dense volume, or nil when the selection is too
// big to hold one byte per cell.
func (b *builder) densify(low, high [3]int32) *volume {
	size := [3]int32{high[0] - low[0] + 3, high[1] - low[1] + 3, high[2] - low[2] + 3}
	cells := int64(size[0]) * int64(size[1]) * int64(size[2])
	if cells > maxSolidVolume {
		return nil
	}
	vol := &volume{
		origin: [3]int32{low[0] - 1, low[1] - 1, low[2] - 1},
		size:   size,
		state:  make([]uint8, cells),
	}
	for key := range b.cells {
		vol.state[vol.index(key[0]-vol.origin[0], key[1]-vol.origin[1], key[2]-vol.origin[2])] = stateSurface
	}
	return vol
}

// despeckle drops surface voxels with fewer than two solid neighbours. Those are
// almost always flyaway triangles from the photogrammetry, never architecture.
func (v *volume) despeckle() int {
	doomed := make([]int, 0, 1024)
	for y := int32(0); y < v.size[1]; y++ {
		for z := int32(0); z < v.size[2]; z++ {
			for x := int32(0); x < v.size[0]; x++ {
				if v.state[v.index(x, y, z)] != stateSurface {
					continue
				}
				touching := 0
				for _, step := range neighbours {
					if v.at(x+step[0], y+step[1], z+step[2]) == stateSurface {
						touching++
					}
				}
				if touching < 2 {
					doomed = append(doomed, v.index(x, y, z))
				}
			}
		}
	}
	for _, index := range doomed {
		v.state[index] = stateEmpty
	}
	return len(doomed)
}

// solidify floods air inwards from the padded shell. Everything the flood never
// reaches is enclosed, so it becomes solid ground or a solid building core.
//
// ponytail: the frontier queue is bounded by the open air surface, not by the
// volume. If a pathological selection ever blows it up, switch to a multi-pass
// directional sweep, which needs no queue at all.
func (v *volume) solidify() int {
	queue := make([]int32, 0, 1<<16)
	push := func(x, y, z int32) {
		index := v.index(x, y, z)
		if v.state[index] != stateEmpty {
			return
		}
		v.state[index] = stateOutside
		queue = append(queue, int32(index))
	}

	// The padding is guaranteed to be empty, so every shell cell is a seed.
	for y := int32(0); y < v.size[1]; y++ {
		for z := int32(0); z < v.size[2]; z++ {
			push(0, y, z)
			push(v.size[0]-1, y, z)
		}
		for x := int32(0); x < v.size[0]; x++ {
			push(x, y, 0)
			push(x, y, v.size[2]-1)
		}
	}
	for z := int32(0); z < v.size[2]; z++ {
		for x := int32(0); x < v.size[0]; x++ {
			push(x, 0, z)
			push(x, v.size[1]-1, z)
		}
	}

	plane := v.size[0] * v.size[2]
	for head := 0; head < len(queue); head++ {
		index := queue[head]
		x := index % v.size[0]
		z := (index / v.size[0]) % v.size[2]
		y := index / plane
		for _, step := range neighbours {
			nx, ny, nz := x+step[0], y+step[1], z+step[2]
			if nx < 0 || ny < 0 || nz < 0 || nx >= v.size[0] || ny >= v.size[1] || nz >= v.size[2] {
				continue
			}
			push(nx, ny, nz)
		}
		// Drop the processed prefix once it dominates, so peak memory tracks the
		// frontier instead of every cell ever visited. Halving keeps it amortised.
		if head > 1<<20 && head*2 > len(queue) {
			queue = append(queue[:0], queue[head+1:]...)
			head = -1
		}
	}

	interior := 0
	for index, state := range v.state {
		if state == stateEmpty {
			v.state[index] = stateInside
			interior++
		}
	}
	return interior
}

// writeSurface resolves every surviving surface voxel to a block.
func (b *builder) writeSurface(g *grid, vol *volume, shift [3]int32) {
	for key, cell := range b.cells {
		if cell.n == 0 {
			continue
		}
		local := [3]int32{key[0] - vol.origin[0], key[1] - vol.origin[1], key[2] - vol.origin[2]}
		if vol.at(local[0], local[1], local[2]) != stateSurface {
			continue // despeckled
		}
		x, y, z := key[0]+shift[0], key[1]+shift[1], key[2]+shift[2]
		if y > worldMaxY || y < worldMinY {
			continue
		}
		r := uint8(cell.r / cell.n)
		green := uint8(cell.g / cell.n)
		blue := uint8(cell.b / cell.n)

		block := blockFor(r, green, blue, x, y, z)
		// A green voxel with nothing solid above it is ground, and ground reads
		// far better as an actual grass block than as green concrete.
		if isGreen(r, green, blue) {
			if above := vol.at(local[0], local[1]+1, local[2]); above != stateSurface && above != stateInside {
				block = grassBlock
			}
		}
		g.set(x, y, z, block)
	}
}

// writeInterior fills the enclosed cells: soil right under the surface, stone
// below that, deepslate deep down. It reads like real ground when you dig in.
func (v *volume) writeInterior(g *grid, shift [3]int32) {
	for y := int32(0); y < v.size[1]; y++ {
		for z := int32(0); z < v.size[2]; z++ {
			for x := int32(0); x < v.size[0]; x++ {
				if v.state[v.index(x, y, z)] != stateInside {
					continue
				}
				worldY := v.origin[1] + y + shift[1]
				if worldY > worldMaxY || worldY < worldMinY {
					continue
				}
				block := stoneBlock
				switch {
				case v.at(x, y+1, z) == stateSurface:
					block = dirtBlock
				case worldY < -20:
					block = deepslateBlock
				}
				g.set(v.origin[0]+x+shift[0], worldY, v.origin[2]+z+shift[2], block)
			}
		}
	}
}

// writeShell is the fallback for selections too large to solidify: surface
// blocks plus a few blocks of filler underneath so nothing is walk-through.
func (b *builder) writeShell(g *grid, shift [3]int32, opt Options) {
	for key, cell := range b.cells {
		if cell.n == 0 {
			continue
		}
		x, y, z := key[0]+shift[0], key[1]+shift[1], key[2]+shift[2]
		if y > worldMaxY || y < worldMinY {
			continue
		}
		g.set(x, y, z, blockFor(uint8(cell.r/cell.n), uint8(cell.g/cell.n), uint8(cell.b/cell.n), x, y, z))
	}
	for key := range b.cells {
		x, y, z := key[0]+shift[0], key[1]+shift[1], key[2]+shift[2]
		for depth := 1; depth <= opt.GroundDepth; depth++ {
			g.fillEmpty(x, y-int32(depth), z, stoneBlock)
		}
	}
}
