package mc

import (
	"bytes"
	"image"
	"image/jpeg"
	"os"
	"path/filepath"

	"github.com/Henkek221/mappy/pkg/fly/c3m"
	"github.com/Henkek221/mappy/pkg/oth"
)

// texture turns material texture data into block colours. A nil texture means
// the material could not be decoded; callers fall back to a neutral block.
type texture struct {
	image  image.Image
	bounds image.Rectangle
}

func loadTexture(material c3m.Material) *texture {
	if len(material.JPEG) > 0 {
		if img, err := jpeg.Decode(bytes.NewReader(material.JPEG)); err == nil {
			return newTexture(img)
		}
	}
	if len(material.HEIC) > 0 {
		if img := decodeHEIC(material.HEIC); img != nil {
			return newTexture(img)
		}
	}
	return nil
}

func newTexture(img image.Image) *texture {
	bounds := img.Bounds()
	if bounds.Dx() < 1 || bounds.Dy() < 1 {
		return nil
	}
	return &texture{image: img, bounds: bounds}
}

// rgbAt samples the texture like an OBJ viewer does: V runs bottom-up. Without
// a decodable texture it returns the neutral grey of the fallback block.
func (t *texture) rgbAt(u, v float32) (uint8, uint8, uint8) {
	if t == nil {
		fallback := blocks[unknownBlock-1]
		return fallback.r, fallback.g, fallback.b
	}
	width, height := t.bounds.Dx(), t.bounds.Dy()
	x := t.bounds.Min.X + int(clampUnit(u)*float32(width-1))
	y := t.bounds.Min.Y + int(clampUnit(1-v)*float32(height-1))
	r, g, b, _ := t.image.At(x, y).RGBA()
	return uint8(r >> 8), uint8(g >> 8), uint8(b >> 8)
}

func clampUnit(value float32) float32 {
	switch {
	case value < 0 || value != value: // NaN sorts here too
		return 0
	case value > 1:
		return 1
	default:
		return value
	}
}

func decodeHEIC(data []byte) image.Image {
	if oth.HEICConverter() == "" {
		return nil
	}
	dir, err := os.MkdirTemp("", "mappy-heic")
	if err != nil {
		return nil
	}
	defer os.RemoveAll(dir)

	src := filepath.Join(dir, "texture.heic")
	dst := filepath.Join(dir, "texture.jpg")
	if err := os.WriteFile(src, data, 0o600); err != nil {
		return nil
	}
	if err := oth.ConvertHEIC(src, dst); err != nil {
		return nil
	}
	converted, err := os.ReadFile(dst)
	if err != nil {
		return nil
	}
	img, err := jpeg.Decode(bytes.NewReader(converted))
	if err != nil {
		return nil
	}
	return img
}
