package usage

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type cookieJarStore struct {
	path string
}

type storedCookieJar struct {
	Version   int            `json:"version"`
	UpdatedAt time.Time      `json:"updatedAt"`
	Cookies   []storedCookie `json:"cookies"`
}

type storedCookie struct {
	Name     string    `json:"name"`
	Value    string    `json:"value"`
	Domain   string    `json:"domain"`
	Path     string    `json:"path"`
	Expires  time.Time `json:"expires,omitempty"`
	Secure   bool      `json:"secure"`
	HttpOnly bool      `json:"httpOnly"`
}

func newCookieJarStore(path string) *cookieJarStore {
	return &cookieJarStore{path: filepath.Clean(path)}
}

func NewCookieJarStore(path string) CookieLoader {
	return newCookieJarStore(path)
}

func WriteOllamaCookieHeader(path, header string) error {
	header = strings.TrimSpace(header)
	if header == "" {
		return fmt.Errorf("ollama cookie header is empty")
	}
	cookies := parseCookieHeader(header)
	if len(cookies) == 0 {
		return fmt.Errorf("ollama cookie header did not contain any cookies")
	}
	return newCookieJarStore(path).Save(cookies)
}

func (s *cookieJarStore) Save(cookies []*http.Cookie) error {
	if s == nil {
		return fmt.Errorf("cookie jar store unavailable")
	}

	filtered := filterOllamaCookies(cookies)
	payload := storedCookieJar{
		Version:   1,
		UpdatedAt: time.Now().UTC(),
		Cookies:   make([]storedCookie, 0, len(filtered)),
	}
	for _, cookie := range filtered {
		entry := storedCookie{
			Name:     cookie.Name,
			Value:    cookie.Value,
			Domain:   cookie.Domain,
			Path:     cookie.Path,
			Secure:   cookie.Secure,
			HttpOnly: cookie.HttpOnly,
		}
		if expiresAt, ok := sanitizeCookieExpiry(cookie.Expires); ok {
			entry.Expires = expiresAt
		}
		payload.Cookies = append(payload.Cookies, entry)
	}

	data, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal cookie jar: %w", err)
	}

	dir := filepath.Dir(s.path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("create cookie jar dir: %w", err)
	}

	tmp, err := os.CreateTemp(dir, ".ollama-cookies-*.tmp")
	if err != nil {
		return fmt.Errorf("create temp cookie jar: %w", err)
	}
	tmpPath := tmp.Name()
	if err := tmp.Chmod(0o600); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpPath)
		return fmt.Errorf("chmod temp cookie jar: %w", err)
	}
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpPath)
		return fmt.Errorf("write temp cookie jar: %w", err)
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("close temp cookie jar: %w", err)
	}

	if err := os.Rename(tmpPath, s.path); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("replace cookie jar: %w", err)
	}
	if err := os.Chmod(s.path, 0o600); err != nil {
		return fmt.Errorf("chmod cookie jar: %w", err)
	}
	return nil
}

func sanitizeCookieExpiry(expiresAt time.Time) (time.Time, bool) {
	if expiresAt.IsZero() {
		return time.Time{}, false
	}
	expiresAt = expiresAt.UTC()
	year := expiresAt.Year()
	if year < 1 || year > 9999 {
		return time.Time{}, false
	}
	return expiresAt, true
}

func (s *cookieJarStore) Load() ([]*http.Cookie, error) {
	if s == nil {
		return nil, fmt.Errorf("cookie jar store unavailable")
	}

	data, err := os.ReadFile(s.path)
	if err != nil {
		return nil, fmt.Errorf("read cookie jar: %w", err)
	}

	var payload storedCookieJar
	if err := json.Unmarshal(data, &payload); err != nil {
		return nil, fmt.Errorf("decode cookie jar: %w", err)
	}

	cookies := make([]*http.Cookie, 0, len(payload.Cookies))
	for _, entry := range payload.Cookies {
		cookie := &http.Cookie{
			Name:     entry.Name,
			Value:    entry.Value,
			Domain:   entry.Domain,
			Path:     entry.Path,
			Secure:   entry.Secure,
			HttpOnly: entry.HttpOnly,
		}
		if !entry.Expires.IsZero() {
			cookie.Expires = entry.Expires.UTC()
		}
		cookies = append(cookies, cookie)
	}
	return cookies, nil
}

func parseCookieHeader(header string) []*http.Cookie {
	parts := strings.Split(header, ";")
	cookies := make([]*http.Cookie, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		name, value, ok := strings.Cut(part, "=")
		if !ok {
			continue
		}
		name = strings.TrimSpace(name)
		value = strings.TrimSpace(value)
		if name == "" || value == "" {
			continue
		}
		cookies = append(cookies, &http.Cookie{
			Name:   name,
			Value:  value,
			Domain: "ollama.com",
			Path:   "/",
			Secure: true,
		})
	}
	return cookies
}
