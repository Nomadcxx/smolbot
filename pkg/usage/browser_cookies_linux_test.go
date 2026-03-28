//go:build linux

package usage

import (
	"database/sql"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

func TestDiscoverLinuxChromiumCookieDBs(t *testing.T) {
	home := t.TempDir()

	candidates := []string{
		filepath.Join(home, ".config", "google-chrome", "Default", "Cookies"),
		filepath.Join(home, ".config", "chromium", "Profile 1", "Cookies"),
		filepath.Join(home, ".config", "BraveSoftware", "Brave-Browser", "Default", "Cookies"),
		filepath.Join(home, ".var", "app", "com.google.Chrome", "config", "google-chrome", "Default", "Cookies"),
	}
	for _, path := range candidates {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", path, err)
		}
		if err := os.WriteFile(path, []byte("cookie-db"), 0o644); err != nil {
			t.Fatalf("write %s: %v", path, err)
		}
	}

	got := discoverLinuxChromiumCookieDBs(home)
	want := []string{
		filepath.Join(home, ".config", "google-chrome", "Default", "Cookies"),
		filepath.Join(home, ".config", "chromium", "Profile 1", "Cookies"),
		filepath.Join(home, ".config", "BraveSoftware", "Brave-Browser", "Default", "Cookies"),
		filepath.Join(home, ".var", "app", "com.google.Chrome", "config", "google-chrome", "Default", "Cookies"),
	}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("discoverLinuxChromiumCookieDBs() = %#v, want %#v", got, want)
	}
}

func TestFilterOllamaCookies(t *testing.T) {
	in := []*http.Cookie{
		{Name: "a", Value: "1", Domain: "ollama.com"},
		{Name: "b", Value: "2", Domain: ".ollama.com"},
		{Name: "c", Value: "3", Domain: "api.ollama.com"},
		{Name: "d", Value: "4", Domain: "example.com"},
		{Name: "e", Value: "5", Domain: "evilollama.com"},
		{Name: "f", Value: "6", Domain: ""},
	}

	got := filterOllamaCookies(in)
	if len(got) != 3 {
		t.Fatalf("len(filterOllamaCookies()) = %d, want 3", len(got))
	}
	for _, cookie := range got {
		if !isOllamaCookie(cookie) {
			t.Fatalf("unexpected non-Ollama cookie returned: %+v", cookie)
		}
	}
}

func TestCookieJarStoreRoundTripAndPermissions(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "ollama_cookies.json")

	store := newCookieJarStore(path)
	want := []*http.Cookie{
		{Name: "session", Value: "abc123", Domain: "ollama.com", Path: "/", Secure: true, HttpOnly: true},
		{Name: "pref", Value: "dark", Domain: ".ollama.com", Path: "/", Secure: false, HttpOnly: false},
	}

	if err := store.Save(want); err != nil {
		t.Fatalf("Save: %v", err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat: %v", err)
	}
	if got := info.Mode().Perm(); got != 0o600 {
		t.Fatalf("file mode = %o, want 0600", got)
	}

	got, err := store.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Load() = %#v, want %#v", got, want)
	}
}

func TestImportOllamaCookiesFromLinuxBrowsers(t *testing.T) {
	home := t.TempDir()
	dbPath := filepath.Join(home, ".config", "chromium", "Default", "Cookies")
	if err := os.MkdirAll(filepath.Dir(dbPath), 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}

	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		t.Fatalf("sql.Open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	if _, err := db.Exec(`CREATE TABLE cookies(
		creation_utc INTEGER NOT NULL,
		host_key TEXT NOT NULL,
		top_frame_site_key TEXT NOT NULL,
		name TEXT NOT NULL,
		value TEXT NOT NULL,
		encrypted_value BLOB NOT NULL,
		path TEXT NOT NULL,
		expires_utc INTEGER NOT NULL,
		is_secure INTEGER NOT NULL,
		is_httponly INTEGER NOT NULL,
		last_access_utc INTEGER NOT NULL,
		has_expires INTEGER NOT NULL,
		is_persistent INTEGER NOT NULL,
		priority INTEGER NOT NULL,
		samesite INTEGER NOT NULL,
		source_scheme INTEGER NOT NULL,
		source_port INTEGER NOT NULL,
		last_update_utc INTEGER NOT NULL,
		source_type INTEGER NOT NULL,
		has_cross_site_ancestor INTEGER NOT NULL
	)`); err != nil {
		t.Fatalf("create cookies table: %v", err)
	}

	expiresAt := time.Date(2026, 3, 30, 0, 0, 0, 0, time.UTC)
	if _, err := db.Exec(`INSERT INTO cookies (
		creation_utc, host_key, top_frame_site_key, name, value, encrypted_value, path,
		expires_utc, is_secure, is_httponly, last_access_utc, has_expires, is_persistent,
		priority, samesite, source_scheme, source_port, last_update_utc, source_type, has_cross_site_ancestor
	) VALUES (?, ?, '', ?, ?, X'', ?, ?, ?, ?, 0, 1, 1, 1, 0, 1, 443, 0, 0, 0)`,
		0,
		".ollama.com",
		"session",
		"abc123",
		"/",
		chromiumMicros(expiresAt),
		1,
		1,
	); err != nil {
		t.Fatalf("insert ollama cookie: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO cookies (
		creation_utc, host_key, top_frame_site_key, name, value, encrypted_value, path,
		expires_utc, is_secure, is_httponly, last_access_utc, has_expires, is_persistent,
		priority, samesite, source_scheme, source_port, last_update_utc, source_type, has_cross_site_ancestor
	) VALUES (?, ?, '', ?, ?, X'', ?, ?, ?, ?, 0, 1, 1, 1, 0, 1, 443, 0, 0, 0)`,
		0,
		"example.com",
		"other",
		"nope",
		"/",
		chromiumMicros(expiresAt),
		0,
		0,
	); err != nil {
		t.Fatalf("insert non-ollama cookie: %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("close db: %v", err)
	}

	outputPath := filepath.Join(t.TempDir(), "ollama_cookies.json")
	count, err := importOllamaCookiesFromLinuxBrowsers(home, outputPath)
	if err != nil {
		t.Fatalf("importOllamaCookiesFromLinuxBrowsers: %v", err)
	}
	if count != 1 {
		t.Fatalf("count = %d, want 1", count)
	}

	store := newCookieJarStore(outputPath)
	cookies, err := store.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(cookies) != 1 {
		t.Fatalf("len(cookies) = %d, want 1", len(cookies))
	}
	if cookies[0].Domain != ".ollama.com" || cookies[0].Value != "abc123" {
		t.Fatalf("unexpected imported cookie: %+v", cookies[0])
	}
	if !cookies[0].Expires.Equal(expiresAt) {
		t.Fatalf("Expires = %v, want %v", cookies[0].Expires, expiresAt)
	}
}

func chromiumMicros(t time.Time) int64 {
	base := time.Date(1601, 1, 1, 0, 0, 0, 0, time.UTC)
	return (t.UTC().Unix()-base.Unix())*1_000_000 + int64(t.UTC().Nanosecond()/1_000)
}
