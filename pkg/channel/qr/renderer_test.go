package qr

import (
	"strings"
	"testing"
)

func TestRendererRenderToASCIIEmptyDataReturnsEmptyString(t *testing.T) {
	renderer := New(256)

	got, err := renderer.RenderToASCII("")
	if err != nil {
		t.Fatalf("RenderToASCII: %v", err)
	}
	if got != "" {
		t.Fatalf("expected empty output for empty data, got %q", got)
	}
}

func TestRendererRenderToASCIIReturnsStableNonEmptyOutput(t *testing.T) {
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
