// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Henrik Hellpap. Part of Orbis: https://github.com/Henkek221/orbis

// Package r3d decodes Apple's R3D v2 tiles (style 98), the format that
// replaced C3M for the newer "Reg_z9_*" regions, and the C3MM v2 occupancy
// index (style 100).
//
// Parse reads the container: the local frame of each group, the TRICO soup and
// mesh headers and the texture atlases (HEIC or JPEG, byte for byte).
// DecodeMesh and DecodeSoup expand the compressed TRICO streams: BYET blocks
// (byte transitions, successor tables, constants and the J byte-lane coder with
// Huffman stages), Edgebreaker-style connectivity, parallelogram predicted
// positions and UVs, and spherically packed normals.
//
// The layout was worked out from tile data and Apple's simulator binaries,
// with the R3D research notes of nmt3325/apple-maps-r3d-research as a guide.
package r3d

import (
	"encoding/binary"
	"errors"
	"fmt"
	"math"
)

// Magic is the first four bytes of every R3D tile.
var Magic = [4]byte{0x3d, 0x96, 0x59, 0x2e}

var (
	soupMagic = [4]byte{0xd1, 0x22, 0xc9, 0x07}
	meshMagic = [4]byte{0xe1, 0x21, 0x66, 0x24}
)

// Group tags at the top level of a tile.
const (
	GroupSplats = 1
	GroupMesh   = 2
	GroupEnd    = 3
)

// Tile is one decoded R3D container.
type Tile struct {
	Version uint32
	Groups  []Group
}

// Group is a splat or mesh group. Both carry the same local frame.
type Group struct {
	Tag   int
	Frame *Frame
	Soup  *Soup
	Mesh  *Mesh
	// Images are the texture atlases of a mesh group, in file order.
	Images []Image
}

// Frame maps local coordinates to ECEF: ecef = Rotation · local + Translation.
// Rotation is row-major; its columns point north, east and down, so the local
// frame is north-east-down around the tile centre on the ellipsoid.
type Frame struct {
	Rotation    [9]float64
	Translation [3]float64
}

// ToECEF transforms a local point.
func (f Frame) ToECEF(p [3]float64) [3]float64 {
	r := f.Rotation
	return [3]float64{
		r[0]*p[0] + r[1]*p[1] + r[2]*p[2] + f.Translation[0],
		r[3]*p[0] + r[4]*p[1] + r[5]*p[2] + f.Translation[1],
		r[6]*p[0] + r[7]*p[1] + r[8]*p[2] + f.Translation[2],
	}
}

// LatLon returns the geodetic position of the frame origin (WGS84).
func (f Frame) LatLon() (lat, lon float64) {
	const a, e2 = 6378137.0, 6.69437999014e-3
	x, y, z := f.Translation[0], f.Translation[1], f.Translation[2]
	lon = math.Atan2(y, x)
	p := math.Hypot(x, y)
	lat = math.Atan2(z, p*(1-e2))
	for i := 0; i < 5; i++ {
		n := a / math.Sqrt(1-e2*math.Sin(lat)*math.Sin(lat))
		h := p/math.Cos(lat) - n
		lat = math.Atan2(z, p*(1-e2*n/(n+h)))
	}
	return lat * 180 / math.Pi, lon * 180 / math.Pi
}

// Soup is the header of a TRICO splat soup: soft triangles with colour,
// normal and material attributes. Records can be zero.
type Soup struct {
	Version  uint32
	Records  uint64
	Channels uint32
	Scales   [9]float32
	// Layout says which attribute channels carry what; it comes from the
	// splat group and is nil for empty soups.
	Layout []Attribute
	// Payload is the still compressed remainder.
	Payload []byte
}

// AttributeKind identifies a splat attribute in the soup layout. The numbers
// are the ones stored in the tile; the meanings follow how Apple's renderer
// packs them (albedo sRGB + alpha, octahedral normal, roughness and specular
// weight).
type AttributeKind int

const (
	AttrOpacity        AttributeKind = 1 // 1 float
	AttrAlbedo         AttributeKind = 2 // 3 floats, sRGB 0..1
	AttrSpecularWeight AttributeKind = 4 // 1 float, 0..~3.2
	AttrNormal         AttributeKind = 6 // 2 floats, octahedral 0..1
	AttrRoughness      AttributeKind = 7 // 1 float, 0.05..1
)

// Attribute is one entry of the soup layout: Rows*Cols consecutive floats.
type Attribute struct {
	Kind       AttributeKind
	Rows, Cols int
}

// Mesh is the header of a TRICO mesh. Upper height layers often have an empty
// mesh and only splats. Scales are the quantization steps of positions (m),
// UVs (atlas pixels) and normal angles (radians). DecodeMesh expands it.
type Mesh struct {
	Version uint32
	Scales  [3]float32
	Payload []byte
	// Raw is the whole TRICO block including the header.
	Raw []byte
}

// ImageFormat names an embedded texture codec.
type ImageFormat string

const (
	HEIC ImageFormat = "heic"
	JPEG ImageFormat = "jpeg"
)

// Image is a texture atlas exactly as stored in the tile.
type Image struct {
	Format ImageFormat
	Data   []byte
}

// Parse decodes the container. It never reads the compressed TRICO streams.
func Parse(data []byte) (Tile, error) {
	if len(data) < 11 || [4]byte(data[:4]) != Magic {
		return Tile{}, errors.New("r3d: not an R3D tile")
	}
	t := Tile{Version: binary.LittleEndian.Uint32(data[4:8])}
	if t.Version != 2 {
		return t, fmt.Errorf("r3d: unsupported version %d", t.Version)
	}
	// Bytes 8..10 are constant 01 02 00 in every sample; the TLV starts at 11.
	items, err := tlv(data[11:])
	if err != nil {
		return t, fmt.Errorf("r3d: %w", err)
	}
	for _, it := range items {
		if it.tag == GroupEnd {
			continue
		}
		g, err := parseGroup(it.tag, it.body)
		if err != nil {
			return t, fmt.Errorf("r3d: group %d: %w", it.tag, err)
		}
		t.Groups = append(t.Groups, g)
	}
	return t, nil
}

func parseGroup(tag int, body []byte) (Group, error) {
	g := Group{Tag: tag}
	if len(body) < 1 {
		return g, errors.New("empty group")
	}
	// One leading byte (always 01), then TLV items.
	items, err := tlv(body[1:])
	if err != nil {
		return g, err
	}
	var layout []Attribute
	for _, it := range items {
		b := it.body
		switch {
		case tag == GroupSplats && it.tag == 2 && !hasMagic(b, soupMagic):
			// Attribute layout: a count, then (kind, rows, cols) bytes.
			if len(b) < 1 || len(b) < 1+3*int(b[0]) {
				return g, errors.New("bad splat attribute layout")
			}
			for i := 0; i < int(b[0]); i++ {
				e := b[1+3*i:]
				layout = append(layout, Attribute{Kind: AttributeKind(e[0]), Rows: int(e[1]), Cols: int(e[2])})
			}
		case it.tag == 1 && len(b) >= 96:
			var f Frame
			for i := 0; i < 9; i++ {
				f.Rotation[i] = math.Float64frombits(binary.LittleEndian.Uint64(b[8*i:]))
			}
			for i := 0; i < 3; i++ {
				f.Translation[i] = math.Float64frombits(binary.LittleEndian.Uint64(b[72+8*i:]))
			}
			g.Frame = &f
		case hasMagic(b, soupMagic):
			s, err := parseSoup(b)
			if err != nil {
				return g, err
			}
			g.Soup = s
		case hasMagic(b, meshMagic):
			m, err := parseMesh(b)
			if err != nil {
				return g, err
			}
			g.Mesh = m
		case tag == GroupMesh && it.tag == 3:
			g.Images = append(g.Images, findImages(b)...)
		}
	}
	if g.Soup != nil {
		g.Soup.Layout = layout
	}
	return g, nil
}

func parseSoup(b []byte) (*Soup, error) {
	if len(b) < 16 {
		return nil, errors.New("short TRICO soup")
	}
	s := &Soup{
		Version: binary.LittleEndian.Uint32(b[4:]),
		Records: binary.LittleEndian.Uint64(b[8:]),
	}
	// An empty soup (tiles of the older regions) ends after the count.
	if len(b) < 56 {
		return s, nil
	}
	s.Channels = binary.LittleEndian.Uint32(b[16:])
	s.Payload = b[56:]
	for i := range s.Scales {
		s.Scales[i] = math.Float32frombits(binary.LittleEndian.Uint32(b[20+4*i:]))
	}
	return s, nil
}

func parseMesh(b []byte) (*Mesh, error) {
	if len(b) < 20 {
		return nil, errors.New("short TRICO mesh")
	}
	m := &Mesh{Version: binary.LittleEndian.Uint32(b[4:]), Payload: b[20:], Raw: b}
	for i := range m.Scales {
		m.Scales[i] = math.Float32frombits(binary.LittleEndian.Uint32(b[8+4*i:]))
	}
	return m, nil
}

func hasMagic(b []byte, magic [4]byte) bool {
	return len(b) >= 4 && [4]byte(b[:4]) == magic
}

type item struct {
	tag  int
	body []byte
}

// tlv splits u8 tag + ULEB128 length records.
func tlv(b []byte) ([]item, error) {
	var out []item
	for len(b) > 0 {
		tag := int(b[0])
		n, w := binary.Uvarint(b[1:])
		if w <= 0 || uint64(len(b)-1-w) < n {
			return out, fmt.Errorf("bad TLV length at tag %d", tag)
		}
		start := 1 + w
		out = append(out, item{tag: tag, body: b[start : start+int(n)]})
		b = b[start+int(n):]
	}
	return out, nil
}

// findImages locates HEIC (ISO BMFF) and JPEG atlases inside an image record.
// The small headers around each image (dimensions, flags) are skipped; the
// image files themselves carry their size.
func findImages(b []byte) []Image {
	var out []Image
	for i := 0; i+12 <= len(b); {
		switch {
		case string(b[i+4:i+8]) == "ftyp":
			end := bmffEnd(b, i)
			if end <= i {
				i++
				continue
			}
			out = append(out, Image{Format: HEIC, Data: b[i:end]})
			i = end
		case b[i] == 0xff && b[i+1] == 0xd8 && b[i+2] == 0xff:
			end := jpegEnd(b, i)
			if end <= i {
				i++
				continue
			}
			out = append(out, Image{Format: JPEG, Data: b[i:end]})
			i = end
		default:
			i++
		}
	}
	return out
}

// bmffEnd walks top-level boxes from start until after mdat.
func bmffEnd(b []byte, start int) int {
	o := start
	for o+8 <= len(b) {
		size := int(binary.BigEndian.Uint32(b[o:]))
		kind := string(b[o+4 : o+8])
		if size < 8 || o+size > len(b) {
			return -1
		}
		o += size
		if kind == "mdat" {
			return o
		}
	}
	return -1
}

// jpegEnd follows segments to the scan and then to the EOI marker.
func jpegEnd(b []byte, start int) int {
	o := start + 2
	for o+4 <= len(b) && b[o] == 0xff {
		marker := b[o+1]
		length := int(binary.BigEndian.Uint16(b[o+2:]))
		if marker == 0xda {
			o += 2 + length
			for o+1 < len(b) {
				if b[o] == 0xff && b[o+1] == 0xd9 {
					return o + 2
				}
				o++
			}
			return -1
		}
		o += 2 + length
	}
	return -1
}

// Size returns the pixel size of the atlas: from the HEIF ispe property or the
// JPEG frame header.
func (im Image) Size() (width, height int, err error) {
	b := im.Data
	switch im.Format {
	case JPEG:
		o := 2
		for o+9 <= len(b) && b[o] == 0xff {
			marker := b[o+1]
			if marker >= 0xc0 && marker <= 0xc3 {
				return int(binary.BigEndian.Uint16(b[o+7:])), int(binary.BigEndian.Uint16(b[o+5:])), nil
			}
			o += 2 + int(binary.BigEndian.Uint16(b[o+2:]))
		}
	case HEIC:
		// The first ispe belongs to the primary item; the grid tiles of a
		// HEIC carry their own, smaller ispe later on.
		if i := indexOf(b, "ispe"); i >= 0 && i+16 <= len(b) {
			return int(binary.BigEndian.Uint32(b[i+8:])), int(binary.BigEndian.Uint32(b[i+12:])), nil
		}
	}
	return 0, 0, errors.New("r3d: image size not found")
}

func indexOf(b []byte, s string) int {
	for i := 0; i+len(s) <= len(b); i++ {
		if string(b[i:i+len(s)]) == s {
			return i
		}
	}
	return -1
}

// Classic reports whether a tile is classic Flyover geometry repackaged as
// R3D: no splat attribute layout, no splats, and a mesh without normals. Newer
// regions also answer upper h layers with such tiles; they duplicate surfaces
// of the new photogrammetry and z-fight with it, and Apple's renderer does not
// show them there. Drop classic tiles in any x/y column that has a non-classic
// tile. In classic-only regions (for example Paris) keep them all.
func (t Tile) Classic() bool {
	for _, g := range t.Groups {
		if g.Soup != nil && len(g.Soup.Layout) > 0 {
			return false
		}
	}
	return true
}
