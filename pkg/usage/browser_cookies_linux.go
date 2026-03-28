//go:build linux

package usage

import (
	"database/sql"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "github.com/mattn/go-sqlite3"
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

func importOllamaCookiesFromLinuxBrowsers(home, outputPath string) (int, error) {
	paths := discoverLinuxChromiumCookieDBs(home)
	if len(paths) == 0 {
		return 0, fmt.Errorf("no chromium cookie databases found")
	}

	var imported []*http.Cookie
	for _, path := range paths {
		cookies, err := readOllamaCookiesFromChromiumDB(path)
		if err != nil {
			continue
		}
		imported = append(imported, cookies...)
	}
	imported = filterOllamaCookies(imported)
	if len(imported) == 0 {
		return 0, fmt.Errorf("no readable ollama cookies found")
	}

	store := newCookieJarStore(outputPath)
	if err := store.Save(imported); err != nil {
		return 0, err
	}
	return len(imported), nil
}

func ImportOllamaCookiesFromLinuxBrowsers(home, outputPath string) (int, error) {
	return importOllamaCookiesFromLinuxBrowsers(home, outputPath)
}

func readOllamaCookiesFromChromiumDB(path string) ([]*http.Cookie, error) {
	db, err := sql.Open("sqlite3", sqliteDSN(path)+"&mode=ro&_query_only=on")
	if err != nil {
		return nil, fmt.Errorf("open cookie db: %w", err)
	}
	defer db.Close()

	rows, err := db.Query(`
		SELECT host_key, name, value, path, expires_utc, is_secure, is_httponly
		FROM cookies
		WHERE host_key LIKE '%ollama%'
		ORDER BY host_key, name
	`)
	if err != nil {
		return nil, fmt.Errorf("query cookies: %w", err)
	}
	defer rows.Close()

	var cookies []*http.Cookie
	for rows.Next() {
		var hostKey string
		var name string
		var value string
		var cookiePath string
		var expiresUTC int64
		var isSecure int
		var isHTTPOnly int
		if err := rows.Scan(&hostKey, &name, &value, &cookiePath, &expiresUTC, &isSecure, &isHTTPOnly); err != nil {
			return nil, fmt.Errorf("scan cookie: %w", err)
		}
		if strings.TrimSpace(value) == "" {
			continue
		}
		cookie := &http.Cookie{
			Name:     name,
			Value:    value,
			Domain:   hostKey,
			Path:     cookiePath,
			Secure:   isSecure != 0,
			HttpOnly: isHTTPOnly != 0,
		}
		if expiresAt := chromiumTime(expiresUTC); !expiresAt.IsZero() {
			cookie.Expires = expiresAt
		}
		cookies = append(cookies, cookie)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate cookies: %w", err)
	}
	return cookies, nil
}

func chromiumTime(v int64) time.Time {
	if v <= 0 {
		return time.Time{}
	}
	const chromiumUnixOffsetSeconds = 11644473600
	seconds := v / 1_000_000
	micros := v % 1_000_000
	return time.Unix(seconds-chromiumUnixOffsetSeconds, micros*1_000).UTC()
}
