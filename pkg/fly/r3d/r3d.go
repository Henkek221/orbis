// Package r3d reads the container of Apple's R3D v2 tiles (style 98), the
// format that replaced C3M for the newer "Reg_z9_*" regions.
//
// What is decoded here: the TLV container, the per-group local frame, the
// header fields of the TRICO splat soup and TRICO mesh, and the embedded texture
// atlases (HEIC or JPEG, byte for byte). What is not decoded: the compressed
// vertex, index, UV, normal and splat attribute streams inside the TRICO
// blocks. They use a custom byte-lane predictor with Huffman stages and an
// Edgebreaker-style topology, so Mesh and Soup only expose their headers.
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

// Soup is the header of a TRICO splat soup: soft triangles with colour and
// four extra attributes each. Records can be zero.
type Soup struct {
	Version  uint32
	Records  uint64
	Channels uint32
	Scales   [9]float32
	// Payload is the still compressed remainder.
	Payload []byte
}

// Mesh is the header of a TRICO mesh. Upper height layers often have an empty
// mesh and only splats.
type Mesh struct {
	Version uint32
	Scales  [3]float32
	Payload []byte
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
	for _, it := range items {
		b := it.body
		switch {
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
	m := &Mesh{Version: binary.LittleEndian.Uint32(b[4:]), Payload: b[20:]}
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
