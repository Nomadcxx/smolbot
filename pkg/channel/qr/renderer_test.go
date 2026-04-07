package qr

import (
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

func TestQRRendererOutputIsCompact(t *testing.T) {
	renderer := New(256)
	const data = "https://example.com/whatsapp-link-token"

	out, err := renderer.RenderToASCII(data)
	if err != nil {
		t.Fatalf("RenderToASCII: %v", err)
	}

	lines := strings.Split(out, "\n")
	if len(lines) > 40 {
		t.Fatalf("expected compact output (<40 rows), got %d rows", len(lines))
	}
	for i, line := range lines {
		// Each line should be well under 80 cols (module count + 2 indent)
		if len([]rune(line)) > 80 {
			t.Fatalf("line %d too wide (%d runes): %q", i, len([]rune(line)), line)
		}
	}
}
