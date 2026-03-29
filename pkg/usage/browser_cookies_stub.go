//go:build !linux

package usage

import "fmt"

func ImportOllamaCookiesFromLinuxBrowsers(_, _ string) (int, error) {
	return 0, fmt.Errorf("linux browser cookie import is unavailable on this platform")
}
