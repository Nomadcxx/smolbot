package qr

import (
	"bytes"
	"image"
	"image/png"

	"github.com/skip2/go-qrcode"
)

type Renderer struct {
	size int
}

func New(size int) *Renderer {
	if size <= 0 {
		size = 256
	}
	return &Renderer{size: size}
}

func (r *Renderer) RenderToASCII(data string) (string, error) {
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

func (r *Renderer) imageToBlocks(img image.Image) string {
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

func (r *Renderer) pixelColor(img image.Image, x, y int) bool {
	if x >= img.Bounds().Dx() || y >= img.Bounds().Dy() {
		return false
	}
	ir, ig, ib, a := img.At(x, y).RGBA()
	if a == 0 {
		return false
	}
	return (ir+ig+ib)/3 < 32896
}
