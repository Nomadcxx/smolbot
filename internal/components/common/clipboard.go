package common

import "github.com/atotto/clipboard"

// ClipboardWriter writes text to the system clipboard.
type ClipboardWriter func(string) error

var nativeClipboardWrite = clipboard.WriteAll

// WriteClipboard writes text using the configured native clipboard writer.
// It is intentionally small so callers can inject a test double.
func WriteClipboard(text string, writer ClipboardWriter) error {
	if writer == nil {
		writer = nativeClipboardWrite
	}
	return writer(text)
}
