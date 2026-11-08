// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Henrik Hellpap. Part of Orbis: https://github.com/Henkek221/orbis

package r3d

import (
	"errors"
)

// TRICO meshes store connectivity Edgebreaker style: one CLERS symbol per
// triangle in traversal order, decoded backwards. Corners are numbered
// 3*triangle+k; opp[c] is the corner across the edge opposite c, -1 for none.
// Values <= -2 seeded from the header are deferred links to corner -v.

const (
	opC = 0 // new vertex
	opS = 4 // split
	opL = 5
	opR = 6
	opE = 7 // end of a component
	opX = 8 // first triangle of a closed component (three new vertices)
)

const topologyMagic = 0x5a4c0000

func nextCorner(c int) int { return c - c%3 + (c+1)%3 }
func prevCorner(c int) int { return c - c%3 + (c+2)%3 }

type topology struct {
	faces   []int // vertex per corner
	opp     []int
	counter int // next vertex number to hand out, counting down
}

func (t *topology) link(a, b int) error {
	if a >= len(t.opp) || b >= len(t.opp) {
		return errors.New("topology: link out of range")
	}
	if a >= 0 {
		t.opp[a] = b
	}
	if b >= 0 {
		t.opp[b] = a
	}
	return nil
}

// fan assigns vertex v around the star that starts at corner c, walking until
// it meets an open edge or comes back to c, and closes that edge.
func (t *topology) fan(c, v int) error {
	cur := c
	for guard := 0; guard <= len(t.opp); guard++ {
		end := prevCorner(cur)
		t.faces[nextCorner(cur)] = v
		across := t.opp[end]
		if across < 0 || across == c {
			return t.link(end, c)
		}
		cur = across
	}
	return errors.New("topology: endless star")
}

// boundary numbers the vertices of a hole's boundary loop, starting next to c.
func (t *topology) boundary(c int) error {
	n := len(t.opp)
	cur := nextCorner(c)
	found := false
	for guard := 0; guard <= n; guard++ {
		end := nextCorner(cur)
		if t.opp[end] < 0 {
			cur = end
			found = true
			break
		}
		cur = t.opp[end]
	}
	if !found {
		return errors.New("topology: endless boundary entry")
	}
	for guard := 0; guard <= n; guard++ {
		t.faces[nextCorner(cur)] = t.counter
		cur = prevCorner(cur)
		inner := false
		for g := 0; g <= n; g++ {
			if t.opp[cur] < 0 {
				inner = true
				break
			}
			cur = t.opp[cur]
			t.faces[nextCorner(cur)] = t.counter
			cur = prevCorner(cur)
		}
		if !inner {
			return errors.New("topology: endless boundary star")
		}
		t.counter--
		if t.faces[nextCorner(cur)] != -1 {
			return nil
		}
	}
	return errors.New("topology: endless boundary ring")
}

// buildTopology turns the CLERS bits and the header integers into per-corner
// vertex numbers and the opposite table.
func buildTopology(faceCount int, tokens []byte, header []int32) (*topology, int, error) {
	if len(header) < 4 || uint32(header[0]) != topologyMagic {
		return nil, 0, errors.New("topology: bad header")
	}
	if int(header[1]) != faceCount || (len(header)-4)%2 != 0 {
		return nil, 0, errors.New("topology: header does not match face count")
	}
	vertices := int(header[len(header)-2])
	holes := int(header[len(header)-1])
	if vertices <= 0 || vertices > 3*faceCount || holes < 0 || holes > faceCount {
		return nil, 0, errors.New("topology: bad vertex or hole count")
	}
	corners := 3 * faceCount
	t := &topology{faces: make([]int, corners), opp: make([]int, corners), counter: vertices - 1}
	for i := range t.faces {
		t.faces[i] = -1
		t.opp[i] = -1
	}
	for i := 2; i+1 < len(header)-2; i += 2 {
		a, b := int(header[i]), int(header[i+1])
		if a >= corners || b >= corners {
			return nil, 0, errors.New("topology: header pair out of range")
		}
		if a >= 0 {
			t.opp[a] = b
		}
		if b >= 0 {
			t.opp[b] = a
		}
	}

	// Expand the symbol string. A set bit in the first bitmap means a two-bit
	// extension follows in the second part of the token stream.
	bitmapLen := (faceCount + 7) / 8
	if len(tokens) < bitmapLen {
		return nil, 0, errors.New("topology: truncated symbol bitmap")
	}
	bitmap, ext := tokens[:bitmapLen], tokens[bitmapLen:]
	ops := make([]int, 0, faceCount)
	starts := make([]int, faceCount+1)
	depth, components, extAt, bitAt := 0, 0, 0, 0
	if holes == 0 {
		ops = append(ops, opX)
	}
	for len(ops) < faceCount {
		tri := len(ops)
		op := opC
		if bitmap[bitAt/8]>>(bitAt%8)&1 == 1 {
			if extAt/4 >= len(ext) {
				return nil, 0, errors.New("topology: truncated symbol extensions")
			}
			op = 4 | int(ext[extAt/4]>>(extAt%4*2)&3)
			extAt++
		}
		bitAt++
		ops = append(ops, op)
		switch {
		case op == opE && depth > 0:
			depth--
		case op == opE:
			components++
			if len(ops) < faceCount {
				if components < holes {
					starts[components] = len(ops)
				} else {
					ops = append(ops, opX)
				}
			}
		case op == opS && t.opp[3*tri+2] == -1:
			depth++
		}
	}
	if depth != 0 || len(ops) != faceCount {
		return nil, 0, errors.New("topology: symbol string incomplete")
	}
	if components == 0 {
		return nil, 0, errors.New("topology: no complete component")
	}

	// Replay backwards, component by component.
	var stack []int
	tri := faceCount - 1
	for components > 0 {
		components--
		prev := -1
		for components >= holes || (components >= 0 && tri >= starts[components]) {
			if tri < 0 {
				return nil, 0, errors.New("topology: component underflow")
			}
			c := 3 * tri
			op := ops[tri]
			var err error
			switch op {
			case opC, opS, opR:
				err = t.link(prev, c+1)
			case opL:
				err = t.link(prev, c+2)
			case opE:
				if prev > 0 {
					stack = append(stack, prev)
				}
			case opX:
				err = t.link(prev, c)
			default:
				return nil, 0, errors.New("topology: unknown symbol")
			}
			if err != nil {
				return nil, 0, err
			}
			switch op {
			case opC:
				if err := t.fan(c+2, t.counter); err != nil {
					return nil, 0, err
				}
				t.counter--
			case opS:
				other := t.opp[c+2]
				switch {
				case other == -1:
					if len(stack) == 0 {
						return nil, 0, errors.New("topology: split stack empty")
					}
					if err := t.link(c+2, stack[len(stack)-1]); err != nil {
						return nil, 0, err
					}
					stack = stack[:len(stack)-1]
				case other <= -2:
					if err := t.link(c+2, -other); err != nil {
						return nil, 0, err
					}
					cur, done := c+2, false
					for g := 0; g <= corners; g++ {
						end := nextCorner(cur)
						cur = t.opp[end]
						if cur < 0 {
							if err := t.boundary(end); err != nil {
								return nil, 0, err
							}
							done = true
							break
						}
					}
					if !done {
						return nil, 0, errors.New("topology: endless split boundary")
					}
				}
			case opX:
				for k, v := range [3]int{t.counter - 2, t.counter - 1, t.counter} {
					corner := c + (k+1)%3
					if err := t.fan(corner, v); err != nil {
						return nil, 0, err
					}
				}
				t.counter -= 3
				components--
			}
			prev = c
			tri--
		}
		if holes > 0 {
			if components < 0 {
				return nil, 0, errors.New("topology: hole without component")
			}
			if err := t.boundary(3*starts[components] + 1); err != nil {
				return nil, 0, err
			}
			holes--
		}
	}
	if t.counter != -1 {
		return nil, 0, errors.New("topology: vertex count mismatch")
	}
	for _, v := range t.faces {
		if v < 0 || v >= vertices {
			return nil, 0, errors.New("topology: unassigned corner")
		}
	}
	return t, vertices, nil
}
