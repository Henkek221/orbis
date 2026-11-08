package r3d

import (
	"encoding/binary"
	"errors"
	"fmt"
	"math"
)

// Splat is one soft triangle of a TRICO soup. Corners are metres in the
// group's local north-east-down frame.
type Splat struct {
	Corners [3][3]float32
	// RGBA is colour and opacity in 0..1. Channels Apple left empty are NaN.
	RGBA [4]float32
	// Extra are the remaining four attributes, quantized in steps of Scales[5..8].
	Extra [4]float32
}

// DecodeSoup expands the two quantized blocks of a soup: 3 corners per record
// and 8 attributes per record (opacity, red, green, blue, four extras).
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
	out := make([]Splat, n)
	for i := range out {
		for c := 0; c < 3; c++ {
			copy(out[i].Corners[c][:], positions[9*i+3*c:9*i+3*c+3])
		}
		a := attrs[8*i : 8*i+8]
		out[i].RGBA = [4]float32{a[1], a[2], a[3], a[0]}
		copy(out[i].Extra[:], a[4:])
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
