package agent

import (
	"testing"
)

func TestDetectMimeReturnsPngForPngSignature(t *testing.T) {
	pngSig := []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A}
	mime := detectMime(pngSig)
	if mime != "image/png" {
		t.Fatalf("detectMime(PNG) = %q, want image/png", mime)
	}
}

func TestDetectMimeReturnsJpegForJpegSignatures(t *testing.T) {
	tests := []struct {
		name string
		data []byte
	}{
		{"JPEG SOI marker", []byte{0xFF, 0xD8, 0xFF, 0xE0, 0x00, 0x10, 0x4A, 0x46}},
		{"JPEG SOI with Exif", []byte{0xFF, 0xD8, 0xFF, 0xE1, 0x00, 0x10, 0x45, 0x78}},
		{"JPEG SOI with DQT", []byte{0xFF, 0xD8, 0xFF, 0xDB, 0x00, 0x10, 0x44, 0x51}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mime := detectMime(tt.data)
			if mime != "image/jpeg" {
				t.Fatalf("detectMime(%s) = %q, want image/jpeg", tt.name, mime)
			}
		})
	}
}

func TestDetectMimeReturnsGifForGifSignatures(t *testing.T) {
	tests := []struct {
		name string
		data []byte
	}{
		{"GIF87a", []byte{0x47, 0x49, 0x46, 0x38, 0x37, 0x61}},
		{"GIF89a", []byte{0x47, 0x49, 0x46, 0x38, 0x39, 0x61}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mime := detectMime(tt.data)
			if mime != "image/gif" {
				t.Fatalf("detectMime(%s) = %q, want image/gif", tt.name, mime)
			}
		})
	}
}

func TestDetectMimeReturnsWebpForWebpSignatures(t *testing.T) {
	webpSig := []byte{0x52, 0x49, 0x46, 0x46, 0x00, 0x00, 0x00, 0x00, 0x57, 0x45, 0x42, 0x50}
	mime := detectMime(webpSig)
	if mime != "image/webp" {
		t.Fatalf("detectMime(WebP) = %q, want image/webp", mime)
	}
}

func TestDetectMimeReturnsOctetStreamForUnknown(t *testing.T) {
	data := []byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}
	mime := detectMime(data)
	if mime != "application/octet-stream" {
		t.Fatalf("detectMime(unknown) = %q, want application/octet-stream", mime)
	}
}