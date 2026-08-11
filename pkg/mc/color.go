package mc

import (
	"math"
	"sync"
)

// Matching photo colours to a fixed block palette is the whole quality game.
// Two things make it look right instead of blotchy:
//
//   - distances are measured in Oklab, a perceptually uniform space, so a dark
//     asphalt grey is not "closer" to blue than to another grey;
//   - colours between two palette entries are dithered across neighbouring
//     voxels, which recovers tones the palette does not contain at all.
type oklab struct {
	l, a, b float64
}

// match holds the two best blocks for a colour bin and how strongly to mix
// them. mix is the share of second, quantised to 0..255.
type match struct {
	first  uint16
	second uint16
	mix    uint8
}

var (
	matchOnce sync.Once
	matchLUT  [32768]match
	labs      []oklab
)

// blockFor returns the palette block for a colour at a voxel position. The
// position drives the dither pattern, so flat walls get an even texture instead
// of noise.
func blockFor(r, g, b uint8, x, y, z int32) uint16 {
	matchOnce.Do(buildMatchLUT)
	entry := matchLUT[int(r>>3)<<10|int(g>>3)<<5|int(b>>3)]
	if entry.mix == 0 {
		return entry.first
	}
	if dither(x, y, z) < float64(entry.mix)/255 {
		return entry.second
	}
	return entry.first
}

// bayer4 is the classic 4x4 ordered dither matrix.
var bayer4 = [16]float64{
	0, 8, 2, 10,
	12, 4, 14, 6,
	3, 11, 1, 9,
	15, 7, 13, 5,
}

// dither shifts the 4x4 matrix per height layer so stacked walls do not repeat
// the same pattern on every course of blocks.
func dither(x, y, z int32) float64 {
	row := (x + 2*(y&1)) & 3
	column := (z + (y >> 1 & 1) + 2*(y&1)) & 3
	return (bayer4[row*4+column] + 0.5) / 16
}

func buildMatchLUT() {
	labs = make([]oklab, len(blocks))
	for index, block := range blocks {
		labs[index] = toOklab(block.r, block.g, block.b)
	}

	for ri := 0; ri < 32; ri++ {
		for gi := 0; gi < 32; gi++ {
			for bi := 0; bi < 32; bi++ {
				target := toOklab(uint8(ri<<3|4), uint8(gi<<3|4), uint8(bi<<3|4))
				matchLUT[ri<<10|gi<<5|bi] = bestPair(target)
			}
		}
	}
}

// bestPair finds the nearest palette colour and the neighbour that, blended
// with it, lands closest to the target.
func bestPair(target oklab) match {
	first, firstDist := 0, math.Inf(1)
	for index, lab := range labs {
		if dist := labDistance(target, lab); dist < firstDist {
			first, firstDist = index, dist
		}
	}
	if firstDist < 1e-6 {
		return match{first: uint16(first) + 1}
	}

	best := match{first: uint16(first) + 1}
	bestDist := firstDist
	for index, lab := range labs {
		if index == first {
			continue
		}
		// Project the target onto the line between the two palette colours.
		// The share tells how often the second block has to appear.
		share, dist := projectOnto(target, labs[first], lab)
		if share <= 0.02 || share >= 0.5 || dist >= bestDist {
			continue
		}
		bestDist = dist
		best.second = uint16(index) + 1
		best.mix = uint8(share * 255)
	}
	if best.second == 0 {
		best.mix = 0
	}
	return best
}

// projectOnto returns how much of "to" is needed to approach target from
// "from", plus the distance that remains.
func projectOnto(target, from, to oklab) (float64, float64) {
	dl, da, db := to.l-from.l, to.a-from.a, to.b-from.b
	length := dl*dl + da*da + db*db
	if length < 1e-9 {
		return 0, math.Inf(1)
	}
	tl, ta, tb := target.l-from.l, target.a-from.a, target.b-from.b
	share := (tl*dl + ta*da + tb*db) / length
	if share < 0 {
		share = 0
	}
	if share > 1 {
		share = 1
	}
	rl, ra, rb := tl-share*dl, ta-share*da, tb-share*db
	return share, rl*rl + ra*ra + rb*rb
}

func labDistance(a, b oklab) float64 {
	dl, da, db := a.l-b.l, a.a-b.a, a.b-b.b
	return dl*dl + da*da + db*db
}

// toOklab converts sRGB to Oklab (Björn Ottosson's transform).
func toOklab(r, g, b uint8) oklab {
	lr := linear(r)
	lg := linear(g)
	lb := linear(b)

	l := math.Cbrt(0.4122214708*lr + 0.5363325363*lg + 0.0514459929*lb)
	m := math.Cbrt(0.2119034982*lr + 0.6806995451*lg + 0.1073969566*lb)
	s := math.Cbrt(0.0883024619*lr + 0.2817188376*lg + 0.6299787005*lb)

	return oklab{
		l: 0.2104542553*l + 0.7936177850*m - 0.0040720468*s,
		a: 1.9779984951*l - 2.4285922050*m + 0.4505937099*s,
		b: 0.0259040371*l + 0.7827717662*m - 0.8086757660*s,
	}
}

func linear(value uint8) float64 {
	channel := float64(value) / 255
	if channel <= 0.04045 {
		return channel / 12.92
	}
	return math.Pow((channel+0.055)/1.055, 2.4)
}
