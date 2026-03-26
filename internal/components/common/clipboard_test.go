package common

import (
	"bytes"
	"testing"
)

func TestWriteOSC52EncodesClipboardPayload(t *testing.T) {
	var out bytes.Buffer
	if err := WriteOSC52(&out, "hello"); err != nil {
		t.Fatalf("WriteOSC52: %v", err)
	}

	const want = "\033]52;c;aGVsbG8=\a"
	if got := out.String(); got != want {
		t.Fatalf("clipboard sequence = %q, want %q", got, want)
	}
}
