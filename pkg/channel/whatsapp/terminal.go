package whatsapp

import (
	"bytes"
	"image"
	"image/color"
	"image/png"

	"github.com/skip2/go-qrcode"
)

type QRRenderer struct {
	size int
}

func NewQRRenderer(size int) *QRRenderer {
	if size <= 0 {
		size = 256
	}
	return &QRRenderer{size: size}
}

func (r *QRRenderer) RenderToASCII(data string) (string, error) {
	if data == "" {
		return "", nil
	}

	pngBytes, err := qrcode.Encode(data, qrcode.Medium, r.size)
	if err != nil {
		return "", err
	}

	img, err := png.Decode(bytes.NewReader(pngBytes))
	if err != nil {
		return "", err
	}

	return r.imageToBlocks(img), nil
}

func (r *QRRenderer) imageToBlocks(img image.Image) string {
	var buf bytes.Buffer
	width := img.Bounds().Dx()
	height := img.Bounds().Dy()

	for y := 0; y < height; y += 2 {
		for x := 0; x < width; x++ {
			c1 := r.pixelColor(img, x, y)
			c2 := r.pixelColor(img, x, y+1)
			if c1 || c2 {
				buf.WriteString("\u2588")
			} else {
				buf.WriteString(" ")
			}
		}
		if y+2 < height {
			buf.WriteString("\n")
		}
	}
	return buf.String()
}

func (r *QRRenderer) pixelColor(img image.Image, x, y int) bool {
	if x >= img.Bounds().Dx() || y >= img.Bounds().Dy() {
		return false
	}
	rgba := img.At(x, y)
	var monochrome bool
	switch c := rgba.(type) {
	case color.Gray:
		monochrome = c.Y > 128
	case color.RGBA:
		monochrome = (uint32(c.R)+uint32(c.G)+uint32(c.B))/3 > 128
	default:
		r, _, _, a := rgba.RGBA()
		monochrome = r > 32896 && a > 128
	}
	return monochrome
}
