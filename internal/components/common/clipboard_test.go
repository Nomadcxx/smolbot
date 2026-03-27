package common

import (
	"errors"
	"testing"
)

func TestWriteClipboardUsesInjectedWriter(t *testing.T) {
	var got string
	if err := WriteClipboard("hello", func(text string) error {
		got = text
		return nil
	}); err != nil {
		t.Fatalf("WriteClipboard: %v", err)
	}
	if got != "hello" {
		t.Fatalf("native clipboard text = %q, want hello", got)
	}
}

func TestWriteClipboardFallsBackToConfiguredWriter(t *testing.T) {
	old := nativeClipboardWrite
	t.Cleanup(func() { nativeClipboardWrite = old })

	var got string
	nativeClipboardWrite = func(text string) error {
		got = text
		return nil
	}

	if err := WriteClipboard("world", nil); err != nil {
		t.Fatalf("WriteClipboard: %v", err)
	}
	if got != "world" {
		t.Fatalf("native clipboard text = %q, want world", got)
	}
}

func TestWriteClipboardPropagatesErrors(t *testing.T) {
	err := WriteClipboard("boom", func(string) error {
		return errors.New("boom")
	})
	if err == nil {
		t.Fatal("expected error from clipboard writer")
	}
}
