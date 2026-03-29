//go:build linux

package usage

import (
	"database/sql"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"slices"
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
	seen := make(map[string]struct{}, len(candidates))
	addPath := func(path string) {
		path = filepath.Clean(path)
		if path == "" {
			return
		}
		info, err := os.Stat(path)
		if err != nil || info.IsDir() {
			return
		}
		if _, ok := seen[path]; ok {
			return
		}
		seen[path] = struct{}{}
		found = append(found, path)
	}

	for _, candidate := range candidates {
		addPath(candidate)
	}

	var discovered []string
	searchRoots := []string{
		filepath.Join(home, ".config"),
		filepath.Join(home, ".var", "app"),
		filepath.Join(home, ".mozilla"),
		filepath.Join(home, ".zen"),
	}
	for _, root := range searchRoots {
		_ = filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
			if err != nil || info == nil || info.IsDir() {
				return nil
			}
			name := info.Name()
			if name != "Cookies" && name != "cookies.sqlite" {
				return nil
			}
			discovered = append(discovered, path)
			return nil
		})
	}
	slices.Sort(discovered)
	for _, path := range discovered {
		addPath(path)
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
		return 0, fmt.Errorf("no browser cookie databases found")
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
	if strings.EqualFold(filepath.Base(path), "cookies.sqlite") {
		return readOllamaCookiesFromFirefoxDB(path)
	}

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

func readOllamaCookiesFromFirefoxDB(path string) ([]*http.Cookie, error) {
	snapshotPath, cleanup, err := snapshotSQLiteDB(path)
	if err != nil {
		return nil, err
	}
	defer cleanup()

	db, err := sql.Open("sqlite3", sqliteDSN(snapshotPath)+"&mode=ro&_query_only=on")
	if err != nil {
		return nil, fmt.Errorf("open firefox cookie db: %w", err)
	}
	defer db.Close()

	rows, err := db.Query(`
		SELECT host, name, value, path, expiry, isSecure, isHttpOnly
		FROM moz_cookies
		WHERE host LIKE '%ollama%'
		ORDER BY host, name
	`)
	if err != nil {
		return nil, fmt.Errorf("query firefox cookies: %w", err)
	}
	defer rows.Close()

	var cookies []*http.Cookie
	for rows.Next() {
		var host string
		var name string
		var value string
		var cookiePath string
		var expiry int64
		var isSecure int
		var isHTTPOnly int
		if err := rows.Scan(&host, &name, &value, &cookiePath, &expiry, &isSecure, &isHTTPOnly); err != nil {
			return nil, fmt.Errorf("scan firefox cookie: %w", err)
		}
		if strings.TrimSpace(value) == "" {
			continue
		}
		cookie := &http.Cookie{
			Name:     name,
			Value:    value,
			Domain:   host,
			Path:     cookiePath,
			Secure:   isSecure != 0,
			HttpOnly: isHTTPOnly != 0,
		}
		if expiry > 0 {
			cookie.Expires = time.Unix(expiry, 0).UTC()
		}
		cookies = append(cookies, cookie)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate firefox cookies: %w", err)
	}
	return cookies, nil
}

func snapshotSQLiteDB(path string) (string, func(), error) {
	info, err := os.Stat(path)
	if err != nil {
		return "", func() {}, fmt.Errorf("stat sqlite db: %w", err)
	}
	if info.IsDir() {
		return "", func() {}, fmt.Errorf("sqlite db path is a directory")
	}

	dir, err := os.MkdirTemp("", "smolbot-cookie-snapshot-*")
	if err != nil {
		return "", func() {}, fmt.Errorf("create sqlite snapshot dir: %w", err)
	}
	cleanup := func() { _ = os.RemoveAll(dir) }

	target := filepath.Join(dir, filepath.Base(path))
	if err := copyFile(path, target); err != nil {
		cleanup()
		return "", func() {}, err
	}

	for _, suffix := range []string{"-wal", "-shm"} {
		sourceSidecar := path + suffix
		if sidecarInfo, err := os.Stat(sourceSidecar); err == nil && !sidecarInfo.IsDir() {
			if err := copyFile(sourceSidecar, target+suffix); err != nil {
				cleanup()
				return "", func() {}, err
			}
		}
	}
	return target, cleanup, nil
}

func copyFile(source, target string) error {
	in, err := os.Open(source)
	if err != nil {
		return fmt.Errorf("open sqlite source: %w", err)
	}
	defer in.Close()

	out, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return fmt.Errorf("create sqlite snapshot: %w", err)
	}
	defer out.Close()

	if _, err := out.ReadFrom(in); err != nil {
		return fmt.Errorf("copy sqlite snapshot: %w", err)
	}
	return nil
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
