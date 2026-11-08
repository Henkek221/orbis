// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Henrik Hellpap. Part of Orbis: https://github.com/Henkek221/orbis

package r3d

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"

	"github.com/ulikunitz/xz/lzma"
)

// Index is the header record of a C3MM v2 occupancy index (style 100). One
// index at zoom z lists the occupied cells at SubZoom = z+4; mesh tiles exist
// at LeafZoom = z+7.
//
// Fetch it as style=100&v=<version>&z=13&x=<x>&y=<y> with XYZ (not TMS) y.
// It holds no height information: request style 98 tiles with h = 0, 1, 2 …
// and keep going past empty layers until a run of empty answers.
type Index struct {
	SubZoom  int
	XOrigin  uint32
	YOrigin  uint32 // flipped: 2^SubZoom - 16*(y+1)
	LeafZoom int
	Cells    []Cell
	// Records holds every decompressed record by tag. Tags 1, 5, 6 and 8 are
	// not interpreted yet.
	Records map[int][]byte
}

// Cell is an occupied cell at SubZoom, as XYZ tile coordinates.
type Cell struct {
	X, Y int
}

// LeafTiles returns the XYZ tiles at LeafZoom inside the cell.
func (idx Index) LeafTiles(c Cell) [][2]int {
	scale := 1 << (idx.LeafZoom - idx.SubZoom)
	out := make([][2]int, 0, scale*scale)
	for dy := 0; dy < scale; dy++ {
		for dx := 0; dx < scale; dx++ {
			out = append(out, [2]int{c.X*scale + dx, c.Y*scale + dy})
		}
	}
	return out
}

// ParseIndex decodes a style 100 response.
func ParseIndex(data []byte) (Index, error) {
	if len(data) < 12 || string(data[:4]) != "C3MM" {
		return Index{}, errors.New("r3d: not a C3MM index")
	}
	if v := binary.LittleEndian.Uint16(data[4:]); v != 2 {
		return Index{}, fmt.Errorf("r3d: C3MM version %d", v)
	}
	count := int(binary.LittleEndian.Uint32(data[8:]))
	idx := Index{Records: map[int][]byte{}}
	o := 12
	for i := 0; i < count; i++ {
		if o+5 > len(data) {
			return idx, errors.New("r3d: truncated C3MM record")
		}
		tag := int(data[o])
		total := int(binary.LittleEndian.Uint32(data[o+1:]))
		if total < 5 || o+total > len(data) {
			return idx, errors.New("r3d: bad C3MM record length")
		}
		body := data[o+5 : o+total]
		if len(body) >= 5 && body[0] == 1 {
			size := int(binary.LittleEndian.Uint32(body[1:]))
			decoded, err := unLZMA(body[5:], size)
			if err != nil {
				return idx, fmt.Errorf("r3d: C3MM tag %d: %w", tag, err)
			}
			body = decoded
		}
		idx.Records[tag] = body
		o += total
	}
	head := idx.Records[0]
	if len(head) < 12 {
		return idx, errors.New("r3d: C3MM index without header")
	}
	idx.SubZoom = int(head[0])
	idx.YOrigin = binary.LittleEndian.Uint32(head[1:])
	idx.XOrigin = binary.LittleEndian.Uint32(head[5:])
	idx.LeafZoom = int(head[9])
	n := int(binary.LittleEndian.Uint16(head[10:]))
	if len(head) < 12+4*n {
		return idx, errors.New("r3d: C3MM cell list truncated")
	}
	// Local cell XY sit in bits 8..11 and 20..23; local y counts from the
	// north edge of the index tile. The low bits are not understood yet.
	topY := (1 << idx.SubZoom) - int(idx.YOrigin) - 16
	for i := 0; i < n; i++ {
		v := binary.LittleEndian.Uint32(head[12+4*i:])
		lx, ly := int(v>>8&15), int(v>>20&15)
		idx.Cells = append(idx.Cells, Cell{X: int(idx.XOrigin) + lx, Y: topY + 15 - ly})
	}
	return idx, nil
}

// unLZMA reads an LZMA-alone stream of a known size. Apple writes "unknown
// size" into the stream header and may omit the end marker, so the declared
// size is patched into the header copy before decoding.
func unLZMA(b []byte, size int) ([]byte, error) {
	if len(b) < 13 {
		return nil, errors.New("short LZMA stream")
	}
	stream := append([]byte(nil), b...)
	binary.LittleEndian.PutUint64(stream[5:13], uint64(size))
	r, err := lzma.NewReader(bytes.NewReader(stream))
	if err != nil {
		return nil, err
	}
	out := make([]byte, size)
	if _, err := io.ReadFull(r, out); err != nil {
		return nil, err
	}
	return out, nil
}
