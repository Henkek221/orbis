package r3d

import (
	"encoding/binary"
	"errors"
	"fmt"
	"math/bits"
)

// BYET blocks hold count records of `lanes` bytes. A mode byte selects
// transforms that are undone in order: byte transitions (8), a byte successor
// table (4), a constant (2) and finally the J byte-lane coder (0x80).

const maxBlockBytes = 128 << 20

// compactSize reads Apple's prefix-length integer: the number of trailing one
// bits in the first byte gives the extra bytes, the value is the
// little-endian integer shifted right by the byte width. 0xff is followed by a
// raw uint64.
func compactSize(b []byte, at int) (int, int, error) {
	if at >= len(b) {
		return 0, at, errors.New("compact size: truncated")
	}
	if b[at] == 0xff {
		if at+9 > len(b) {
			return 0, at, errors.New("compact size: truncated")
		}
		v := binary.LittleEndian.Uint64(b[at+1:])
		if v > maxBlockBytes*8 {
			return 0, at, errors.New("compact size: too large")
		}
		return int(v), at + 9, nil
	}
	width := bits.TrailingZeros8(^b[at]) + 1
	if at+width > len(b) {
		return 0, at, errors.New("compact size: truncated")
	}
	var raw uint64
	for i := width - 1; i >= 0; i-- {
		raw = raw<<8 | uint64(b[at+i])
	}
	return int(raw >> width), at + width, nil
}

// signedVarint is the TRICO integer: six value bits, a continuation bit and a
// sign bit in the first byte, then 7-bit groups.
func signedVarint(b []byte, at int) (int, int, error) {
	if at >= len(b) {
		return 0, at, errors.New("varint: truncated")
	}
	first := b[at]
	at++
	v := int(first & 63)
	more := first&64 != 0
	for shift := 6; more; shift += 7 {
		if shift > 6+7*8 || at >= len(b) {
			return 0, at, errors.New("varint: truncated")
		}
		v |= int(b[at]&127) << shift
		more = b[at]&128 != 0
		at++
	}
	if first&128 != 0 {
		v = -v
	}
	return v, at, nil
}

// readBlock reads a compact-size framed BYET block of count records.
func readBlock(src []byte, at, count, lanes int) ([]byte, int, error) {
	if count == 0 {
		return nil, at, nil
	}
	size, begin, err := compactSize(src, at)
	if err != nil {
		return nil, at, err
	}
	end := begin + size
	if end > len(src) {
		return nil, at, errors.New("byet: block beyond input")
	}
	out, err := byetDecode(src[begin:end], count, lanes)
	return out, end, err
}

func byetDecode(src []byte, count, lanes int) ([]byte, error) {
	total := count * lanes
	if count < 0 || total > maxBlockBytes {
		return nil, errors.New("byet: bad shape")
	}
	if len(src) == total {
		// Capacity is capped so callers appending to the result can never
		// write into the source.
		return src[:total:total], nil
	}
	if len(src) == 0 {
		return nil, errors.New("byet: empty block")
	}
	mode := src[0]
	data := src[1:]
	for _, flag := range []byte{8, 4} {
		if mode&flag == 0 {
			continue
		}
		n, at := total, 0
		if mode != flag {
			var err error
			if n, at, err = compactSize(data, 0); err != nil {
				return nil, err
			}
		}
		if n <= 0 || n > max(total, 1_000_000) {
			return nil, errors.New("byet: transition length out of range")
		}
		end := at + (n+7)/8
		if end > len(data) {
			return nil, errors.New("byet: truncated transition bitmap")
		}
		bitmap := data[at:end]
		set := 0
		for _, x := range bitmap {
			set += bits.OnesCount8(x)
		}
		if n > 64 {
			literals, next, err := compactSize(data, end)
			if err != nil {
				return nil, err
			}
			if literals != set {
				return nil, errors.New("byet: transition literal count mismatch")
			}
			end = next
		}
		literals := data[end:]
		if len(literals) != set {
			return nil, errors.New("byet: transition payload mismatch")
		}
		var successor [256]byte
		out := make([]byte, n)
		prev, cursor := byte(0), 0
		for i := 0; i < n; i++ {
			var v byte
			if bitmap[i/8]>>(i%8)&1 == 1 {
				v = literals[cursor]
				cursor++
				if flag == 4 && i > 0 {
					successor[prev] = v
				}
			} else if flag == 8 {
				v = prev
			} else {
				v = successor[prev]
			}
			out[i] = v
			prev = v
		}
		data = out
		mode &^= flag
	}
	if mode&2 != 0 {
		n, at := total, 0
		var err error
		if mode != 2 {
			if n, at, err = compactSize(data, 0); err != nil {
				return nil, err
			}
		}
		value, at, err := compactSize(data, at)
		if err != nil || at != len(data) || n <= 0 || n > max(total, 1_000_000) {
			return nil, errors.New("byet: bad constant block")
		}
		out := make([]byte, n)
		var word [8]byte
		binary.LittleEndian.PutUint64(word[:], uint64(value))
		copy(out, word[:])
		data = out
		mode &^= 2
	}
	switch mode {
	case 0x80:
		if len(data) < 2 || data[0] != 'J' || data[1] != 0 {
			return nil, errors.New("byet: unknown nested coder")
		}
		return decodeLanes(data[2:], count, lanes)
	case 0:
		if len(data) != total {
			return nil, errors.New("byet: length mismatch")
		}
		return data, nil
	}
	return nil, fmt.Errorf("byet: unsupported mode %#x", mode)
}

type lane struct {
	kind   int
	offset int
	size   int
}

// decodeLanes undoes the J coder: every byte position of a record (a lane) is
// predicted on its own, 16-bit kinds pair a lane with the one before it.
func decodeLanes(src []byte, count, lanes int) ([]byte, error) {
	maskLen := (lanes + 7) / 8
	if len(src) < maskLen {
		return nil, errors.New("j: truncated lane mask")
	}
	var mask uint64
	for i := maskLen - 1; i >= 0; i-- {
		mask = mask<<8 | uint64(src[i])
	}
	if lanes < 64 && mask>>lanes != 0 {
		return nil, errors.New("j: lane mask out of range")
	}
	active := bits.OnesCount64(mask)
	dir := make([]lane, lanes)
	for i := range dir {
		dir[i].kind = 1
	}
	at := maskLen
	if active > 0 {
		ctrlLen := (2*(active-1) + 7) / 8
		if len(src) < at+ctrlLen {
			return nil, errors.New("j: truncated control bitmap")
		}
		ctrl := src[at : at+ctrlLen]
		ctrlBit := func(i int) bool { return ctrl[i/8]>>(i%8)&1 == 1 }
		at += ctrlLen
		bit, prev := 0, -1
		for i := 0; i < lanes; i++ {
			if mask>>i&1 == 0 {
				continue
			}
			var err error
			if prev < 0 {
				if at >= len(src) {
					return nil, errors.New("j: truncated lane kind")
				}
				dir[i].kind = int(src[at])
				at++
				if dir[i].size, at, err = compactSize(src, at); err != nil {
					return nil, err
				}
			} else {
				dir[i].kind = dir[prev].kind
				if ctrlBit(bit) {
					if at >= len(src) {
						return nil, errors.New("j: truncated lane kind")
					}
					dir[i].kind = int(src[at])
					at++
				}
				dir[i].size = dir[prev].size
				if ctrlBit(bit + 1) {
					if dir[i].size, at, err = compactSize(src, at); err != nil {
						return nil, err
					}
				}
				bit += 2
			}
			if dir[i].kind > 6 {
				return nil, fmt.Errorf("j: unknown lane kind %d", dir[i].kind)
			}
			// 16-bit transitions and deltas claim the previous lane as their
			// low byte.
			if (dir[i].kind == 4 || dir[i].kind == 6) && i > 0 {
				dir[i-1].kind = dir[i].kind
			}
			prev = i
		}
		if bit != 2*(active-1) {
			return nil, errors.New("j: control bitmap not consumed")
		}
	}
	for i := range dir {
		dir[i].offset = at
		at += dir[i].size
		if at > len(src) {
			return nil, errors.New("j: lane beyond input")
		}
	}
	if at != len(src) {
		return nil, errors.New("j: trailing bytes")
	}

	out := make([]byte, count*lanes)
	pending := false
	for i, l := range dir {
		payload := src[l.offset : l.offset+l.size]
		if l.kind >= 4 {
			if i%2 == 0 {
				if l.size != 0 {
					return nil, errors.New("j: paired low lane has payload")
				}
				pending = true
				continue
			}
			if !pending && l.kind != 5 {
				return nil, errors.New("j: paired lane without low half")
			}
			pending = false
			values, err := decodeLane(payload, l.kind, count)
			if err != nil {
				return nil, fmt.Errorf("j: lane %d: %w", i, err)
			}
			for r := 0; r < count; r++ {
				out[r*lanes+i-1] = values[2*r]
				out[r*lanes+i] = values[2*r+1]
			}
			continue
		}
		if pending {
			return nil, errors.New("j: incomplete lane pair")
		}
		if l.size == 0 {
			if l.kind != 1 {
				return nil, errors.New("j: empty active lane")
			}
			continue
		}
		values, err := decodeLane(payload, l.kind, count)
		if err != nil {
			return nil, fmt.Errorf("j: lane %d: %w", i, err)
		}
		for r := 0; r < count; r++ {
			out[r*lanes+i] = values[r]
		}
	}
	if pending {
		return nil, errors.New("j: unterminated lane pair")
	}
	return out, nil
}

// decodeLane expands one lane. Kinds: 0 raw; 1/4 change bitmap with new
// values; 2/5 bitmap that teaches a successor table keyed by the previous
// value; 3/6 sign-toggled deltas. 4..6 are the 16-bit variants.
func decodeLane(src []byte, kind, count int) ([]byte, error) {
	if count <= 0 || count > 20_000_000 {
		return nil, errors.New("lane: bad count")
	}
	if kind == 0 {
		if len(src) != count {
			return nil, errors.New("lane: raw length mismatch")
		}
		return src[:count:count], nil
	}
	width := 8
	if kind >= 4 {
		width = 16
	}
	stride := width / 8
	var values []uint32
	var bitmap []byte
	switch {
	case len(src) < stride:
		if len(src) != 0 {
			return nil, errors.New("lane: truncated constant")
		}
		values = []uint32{0}
	case len(src) == stride:
		if stride == 1 {
			values = []uint32{uint32(src[0])}
		} else {
			values = []uint32{uint32(binary.LittleEndian.Uint16(src))}
		}
	default:
		symbols, at, err := compactSize(src, 0)
		if err != nil {
			return nil, err
		}
		if symbols < 1 || symbols > count {
			return nil, errors.New("lane: symbol count out of range")
		}
		if symbols > 1 {
			size, begin, err := compactSize(src, at)
			if err != nil {
				return nil, err
			}
			if begin+size > len(src) {
				return nil, errors.New("lane: bitmap beyond input")
			}
			if bitmap, err = entropyDecode(src[begin:begin+size], 8, (count+6)/8); err != nil {
				return nil, err
			}
			at = begin + size
		}
		size, begin, err := compactSize(src, at)
		if err != nil {
			return nil, err
		}
		if begin+size != len(src) {
			return nil, errors.New("lane: values do not cover input")
		}
		raw, err := entropyDecode(src[begin:], width, symbols)
		if err != nil {
			return nil, err
		}
		values = make([]uint32, symbols)
		for i := range values {
			if stride == 1 {
				values[i] = uint32(raw[i])
			} else {
				values[i] = uint32(binary.LittleEndian.Uint16(raw[2*i:]))
			}
		}
	}

	out := make([]byte, count*stride)
	put := func(i int, v uint32) {
		if stride == 1 {
			out[i] = byte(v)
		} else {
			binary.LittleEndian.PutUint16(out[2*i:], uint16(v))
		}
	}
	value := values[0]
	put(0, value)
	if len(values) == 1 {
		fill := value
		if kind == 2 || kind == 5 {
			fill = 0
		}
		for i := 1; i < count; i++ {
			put(i, fill)
		}
		return out, nil
	}
	mask := uint32(1)<<width - 1
	var table []uint32
	if kind == 2 || kind == 5 {
		table = make([]uint32, 1<<width)
	}
	cursor, sign := 1, uint32(0)
	next := func() (uint32, error) {
		if cursor >= len(values) {
			return 0, errors.New("lane: ran out of symbols")
		}
		cursor++
		return values[cursor-1], nil
	}
	for i := 0; i < count-1; i++ {
		bit := uint32(bitmap[i/8] >> (i % 8) & 1)
		switch kind {
		case 3, 6:
			d, err := next()
			if err != nil {
				return nil, err
			}
			if sign != bit {
				d = -d
			}
			sign ^= bit
			value = (value + d) & mask
		case 1, 4:
			if bit == 1 {
				v, err := next()
				if err != nil {
					return nil, err
				}
				value = v
			}
		default:
			if bit == 1 {
				v, err := next()
				if err != nil {
					return nil, err
				}
				table[value] = v
			}
			value = table[value]
		}
		put(i+1, value)
	}
	if cursor != len(values) {
		return nil, errors.New("lane: unused symbols")
	}
	return out, nil
}
