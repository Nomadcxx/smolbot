//go:build linux

package usage

import (
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"testing"
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
