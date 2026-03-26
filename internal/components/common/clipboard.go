package common

import (
	"encoding/base64"
	"fmt"
	"io"
)

// WriteOSC52 writes text to the terminal clipboard using the OSC 52 sequence.
func WriteOSC52(w io.Writer, text string) error {
	encoded := base64.StdEncoding.EncodeToString([]byte(text))
	_, err := fmt.Fprintf(w, "\033]52;c;%s\a", encoded)
	return err
}
