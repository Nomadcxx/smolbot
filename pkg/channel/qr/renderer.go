package qr

import (
	"strings"

	"github.com/skip2/go-qrcode"
)

type Renderer struct {
	// size is unused but kept for API compatibility.
	size int
}

func New(size int) *Renderer {
	if size <= 0 {
		size = 256
	}
	return &Renderer{size: size}
}

// RenderToASCII produces a compact terminal QR code using Unicode half-block
// characters (▀ ▄ █). Output is typically ~35-45 columns wide and ~20 rows
// tall — fits comfortably in any terminal.
func (r *Renderer) RenderToASCII(data string) (string, error) {
	if data == "" {
		return "", nil
	}

	code, err := qrcode.New(data, qrcode.Medium)
	if err != nil {
		return "", err
	}
	code.DisableBorder = false

	// ToSmallString packs 2 vertical modules per character row.
	// inverseColor=false → dark modules are filled blocks.
	raw := code.ToSmallString(false)

	// Indent each line for visual padding in the terminal.
	var buf strings.Builder
	for _, line := range strings.Split(strings.TrimRight(raw, "\n"), "\n") {
		buf.WriteString("  ")
		buf.WriteString(line)
		buf.WriteString("\n")
	}
	return strings.TrimRight(buf.String(), "\n"), nil
}
