// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Henrik Hellpap. Part of Orbis: https://github.com/Henkek221/orbis

package r3d

import (
	"encoding/binary"
	"errors"
	"fmt"
)

// Entropy stage of the TRICO streams: canonical-free prefix codes whose table
// and payload are read MSB first out of little-endian 32-bit words.

// wordBits reads bits MSB first from consecutive little-endian uint32 words.
type wordBits struct {
	data []byte
	pos  int // bit position
}

func (r *wordBits) remaining() int { return len(r.data)/4*32 - r.pos }

func (r *wordBits) read(n int) (uint32, error) {
	if n == 0 {
		return 0, nil
	}
	if n > 32 || r.pos+n > len(r.data)/4*32 {
		return 0, errors.New("huff: truncated bit stream")
	}
	var out uint64
	for n > 0 {
		word := binary.LittleEndian.Uint32(r.data[r.pos/32*4:])
		offset := r.pos % 32
		take := 32 - offset
		if take > n {
			take = n
		}
		chunk := (word >> (32 - offset - take)) & (1<<take - 1)
		out = out<<take | uint64(chunk)
		r.pos += take
		n -= take
	}
	return uint32(out), nil
}

type huffCode struct {
	code   uint32
	bits   int
	symbol uint32
}

// huffTree is a flat binary trie. Leaves have symbol >= 0.
type huffTree struct {
	child  [][2]int32
	symbol []int32
}

func (t *huffTree) node() int32 {
	t.child = append(t.child, [2]int32{-1, -1})
	t.symbol = append(t.symbol, -1)
	return int32(len(t.child) - 1)
}

const (
	huffMarkerMask = 0x7f0000ff
	huffMarker     = 0x61000002
	huffLiteral    = 0x80000000
)

// huffDecode decodes count symbols of width 8 or 16 from one Huff block (4-byte
// marker, table, word-aligned payload) and returns them little-endian packed.
func huffDecode(data []byte, width, count int) ([]byte, error) {
	if len(data) < 4 {
		return nil, errors.New("huff: truncated header")
	}
	marker := binary.LittleEndian.Uint32(data)
	if marker&huffMarkerMask != huffMarker {
		return nil, fmt.Errorf("huff: unknown marker %08x", marker)
	}
	stride := width / 8
	if marker&huffLiteral != 0 {
		if len(data) != 4+count*stride {
			return nil, errors.New("huff: literal length mismatch")
		}
		return append([]byte(nil), data[4:]...), nil
	}
	r := &wordBits{data: data[4:]}
	codes, err := readHuffTable(r, width)
	if err != nil {
		return nil, err
	}
	payload := data[4+r.pos/8:]
	out := make([]byte, count*stride)
	put := func(i int, s uint32) {
		if stride == 1 {
			out[i] = byte(s)
		} else {
			binary.LittleEndian.PutUint16(out[2*i:], uint16(s))
		}
	}
	if len(codes) == 1 && codes[0].bits == 0 {
		for i := 0; i < count; i++ {
			put(i, codes[0].symbol)
		}
		if len(payload) != 0 {
			return nil, errors.New("huff: payload after constant table")
		}
		return out, nil
	}
	tree := &huffTree{}
	tree.node()
	for _, c := range codes {
		if c.bits == 0 {
			return nil, errors.New("huff: zero-length code among several")
		}
		n := int32(0)
		for s := c.bits - 1; s >= 0; s-- {
			if tree.symbol[n] >= 0 {
				return nil, errors.New("huff: prefix conflict")
			}
			bit := c.code >> s & 1
			if tree.child[n][bit] < 0 {
				next := tree.node()
				tree.child[n][bit] = next
			}
			n = tree.child[n][bit]
		}
		if tree.symbol[n] >= 0 || tree.child[n] != [2]int32{-1, -1} {
			return nil, errors.New("huff: duplicate code")
		}
		tree.symbol[n] = int32(c.symbol)
	}
	bits := &wordBits{data: payload}
	for i := 0; i < count; i++ {
		n := int32(0)
		for tree.symbol[n] < 0 {
			b, err := bits.read(1)
			if err != nil {
				return nil, err
			}
			n = tree.child[n][b]
			if n < 0 {
				return nil, errors.New("huff: unmatched code")
			}
		}
		put(i, uint32(tree.symbol[n]))
	}
	if len(payload) != (bits.pos+31)/32*4 {
		return nil, errors.New("huff: payload length does not match symbols")
	}
	for bits.remaining() > 0 {
		n := bits.remaining()
		if n > 32 {
			n = 32
		}
		if v, _ := bits.read(n); v != 0 {
			return nil, errors.New("huff: nonzero padding")
		}
	}
	return out, nil
}

// readHuffTable reads the symbol table and aligns r to the next word.
func readHuffTable(r *wordBits, width int) ([]huffCode, error) {
	n32, err := r.read(width)
	if err != nil {
		return nil, err
	}
	n := int(n32) + 1
	lenWidth, err := r.read(3)
	if err != nil {
		return nil, err
	}
	codes := make([]huffCode, 0, n)
	symbol := uint32(0)
	for i := 0; i < n; i++ {
		var c huffCode
		switch {
		case lenWidth > 0:
			b, err := r.read(int(lenWidth))
			if err != nil {
				return nil, err
			}
			c.bits = int(b) + 1
			if c.code, err = r.read(c.bits); err != nil {
				return nil, err
			}
		case n == 2:
			c.bits = 1
			if c.code, err = r.read(1); err != nil {
				return nil, err
			}
		}
		explicit, err := r.read(1)
		if err != nil {
			return nil, err
		}
		if explicit == 1 {
			absolute, err := r.read(1)
			if err != nil {
				return nil, err
			}
			if absolute == 1 {
				symbol, err = r.read(width)
			} else {
				var skip uint32
				skip, err = r.read(width / 2)
				symbol += skip
			}
			if err != nil {
				return nil, err
			}
		}
		c.symbol = symbol
		codes = append(codes, c)
		symbol++
	}
	r.pos = (r.pos + 31) &^ 31
	if r.pos > len(r.data)*8 {
		return nil, errors.New("huff: table alignment beyond input")
	}
	return codes, nil
}

// entropyDecode dispatches the 0x24 envelopes: a single wrapped Huff block or
// a chain where each stage's output is the next stage's payload.
func entropyDecode(src []byte, width, count int) ([]byte, error) {
	if len(src) < 4 {
		return nil, errors.New("entropy: truncated dispatch")
	}
	marker := binary.LittleEndian.Uint32(src)
	switch {
	case marker == 0x24000000:
		if len(src) < 20 {
			return nil, errors.New("entropy: truncated envelope")
		}
		return huffDecode(src[16:], width, count)
	case marker&0xffff00ff == 0x24000000:
		stages := int(marker >> 8 & 0xff)
		if len(src) < 16 {
			return nil, errors.New("entropy: truncated chain")
		}
		type stage struct {
			table []byte
			count int
		}
		chain := make([]stage, 0, stages)
		size := int(binary.LittleEndian.Uint32(src[4:]))
		tail := int(binary.LittleEndian.Uint64(src[8:]))
		at := 16
		for i := 0; i < stages; i++ {
			if size < 4 || at+size+12 > len(src) {
				return nil, errors.New("entropy: truncated chain stage")
			}
			chain = append(chain, stage{table: src[at : at+size], count: tail})
			at += size
			size = int(binary.LittleEndian.Uint32(src[at:]))
			tail = int(binary.LittleEndian.Uint64(src[at+4:]))
			at += 12
		}
		if size+tail != len(src)-at {
			return nil, errors.New("entropy: chain length mismatch")
		}
		// The innermost stage always yields bytes; only the outermost one
		// produces the requested symbol width.
		out, err := huffDecode(src[at:], 8, chain[len(chain)-1].count)
		if err != nil {
			return nil, err
		}
		for i := len(chain) - 1; i >= 0; i-- {
			w, n := 8, count
			if i == 0 {
				w = width
			} else {
				n = chain[i-1].count
			}
			block := append(append([]byte(nil), chain[i].table...), out...)
			if out, err = huffDecode(block, w, n); err != nil {
				return nil, err
			}
		}
		return out, nil
	}
	return huffDecode(src, width, count)
}
