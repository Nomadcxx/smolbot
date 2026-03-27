package qr

import (
	"image"
	"image/color"
	"strings"
	"testing"
)

func TestQRRendererRenderToASCIIEmptyDataReturnsEmptyString(t *testing.T) {
	renderer := New(256)

	got, err := renderer.RenderToASCII("")
	if err != nil {
		t.Fatalf("RenderToASCII: %v", err)
	}
	if got != "" {
		t.Fatalf("expected empty output for empty data, got %q", got)
	}
}

func TestQRRendererRenderToASCIIReturnsStableNonEmptyOutput(t *testing.T) {
	renderer := New(256)
	const data = "signal://provisioning-token"

	first, err := renderer.RenderToASCII(data)
	if err != nil {
		t.Fatalf("RenderToASCII first: %v", err)
	}
	second, err := renderer.RenderToASCII(data)
	if err != nil {
		t.Fatalf("RenderToASCII second: %v", err)
	}

	if first == "" {
		t.Fatal("expected non-empty output for valid QR data")
	}
	if first != second {
		t.Fatalf("expected stable output, got %q and %q", first, second)
	}
	if !strings.Contains(first, "\n") {
		t.Fatalf("expected multi-line output, got %q", first)
	}
	if !strings.Contains(first, "█") {
		t.Fatalf("expected block characters in output, got %q", first)
	}
}

func TestQRRendererImageToBlocksUsesDarkModules(t *testing.T) {
	renderer := New(256)

	dark := image.NewRGBA(image.Rect(0, 0, 1, 2))
	dark.SetRGBA(0, 0, color.RGBA{R: 0, G: 0, B: 0, A: 255})
	dark.SetRGBA(0, 1, color.RGBA{R: 0, G: 0, B: 0, A: 255})
	if got := renderer.imageToBlocks(dark); got != "█" {
		t.Fatalf("expected dark modules to render as filled blocks, got %q", got)
	}

	light := image.NewRGBA(image.Rect(0, 0, 1, 2))
	light.SetRGBA(0, 0, color.RGBA{R: 255, G: 255, B: 255, A: 255})
	light.SetRGBA(0, 1, color.RGBA{R: 255, G: 255, B: 255, A: 255})
	if got := renderer.imageToBlocks(light); got != " " {
		t.Fatalf("expected light modules to stay empty, got %q", got)
	}
}
