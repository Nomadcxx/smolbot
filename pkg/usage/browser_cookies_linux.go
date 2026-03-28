//go:build linux

package usage

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

func discoverLinuxChromiumCookieDBs(home string) []string {
	if strings.TrimSpace(home) == "" {
		home, _ = os.UserHomeDir()
	}
	home = filepath.Clean(home)

	candidates := []string{
		filepath.Join(home, ".config", "google-chrome", "Default", "Cookies"),
		filepath.Join(home, ".config", "google-chrome", "Profile 1", "Cookies"),
		filepath.Join(home, ".config", "google-chrome", "Profile 2", "Cookies"),
		filepath.Join(home, ".config", "chromium", "Default", "Cookies"),
		filepath.Join(home, ".config", "chromium", "Profile 1", "Cookies"),
		filepath.Join(home, ".config", "chromium", "Profile 2", "Cookies"),
		filepath.Join(home, ".config", "BraveSoftware", "Brave-Browser", "Default", "Cookies"),
		filepath.Join(home, ".config", "BraveSoftware", "Brave-Browser", "Profile 1", "Cookies"),
		filepath.Join(home, ".config", "microsoft-edge", "Default", "Cookies"),
		filepath.Join(home, ".config", "microsoft-edge", "Profile 1", "Cookies"),
		filepath.Join(home, ".config", "vivaldi", "Default", "Cookies"),
		filepath.Join(home, ".config", "opera", "Default", "Cookies"),
		filepath.Join(home, ".var", "app", "com.google.Chrome", "config", "google-chrome", "Default", "Cookies"),
		filepath.Join(home, ".var", "app", "com.brave.Browser", "config", "BraveSoftware", "Brave-Browser", "Default", "Cookies"),
		filepath.Join(home, ".var", "app", "com.microsoft.Edge", "config", "microsoft-edge", "Default", "Cookies"),
	}

	found := make([]string, 0, len(candidates))
	for _, candidate := range candidates {
		info, err := os.Stat(candidate)
		if err != nil || info.IsDir() {
			continue
		}
		found = append(found, candidate)
	}
	return found
}

func filterOllamaCookies(cookies []*http.Cookie) []*http.Cookie {
	if len(cookies) == 0 {
		return nil
	}

	filtered := make([]*http.Cookie, 0, len(cookies))
	for _, cookie := range cookies {
		if isOllamaCookie(cookie) {
			filtered = append(filtered, cloneCookie(cookie))
		}
	}
	return filtered
}

func isOllamaCookie(cookie *http.Cookie) bool {
	if cookie == nil {
		return false
	}
	domain := strings.ToLower(strings.TrimSpace(cookie.Domain))
	domain = strings.TrimPrefix(domain, ".")
	if domain == "" {
		return false
	}
	return domain == "ollama.com" || strings.HasSuffix(domain, ".ollama.com")
}

func cloneCookie(cookie *http.Cookie) *http.Cookie {
	if cookie == nil {
		return nil
	}
	clone := *cookie
	return &clone
}
