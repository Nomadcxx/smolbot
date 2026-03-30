package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type TokenStoreEntry struct {
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token,omitempty"`
	ExpiresAt    time.Time `json:"expires_at"`
	TokenType    string    `json:"token_type"`
	Scope        string    `json:"scope"`
	ProviderID   string    `json:"provider_id,omitempty"`
	ProfileID    string    `json:"profile_id,omitempty"`
	AccountEmail string    `json:"account_email,omitempty"`
	AccountName  string    `json:"account_name,omitempty"`
	UpdatedAt    time.Time `json:"updated_at,omitempty"`
}

func (e *TokenStoreEntry) IsExpired() bool {
	return time.Now().Add(2*time.Minute).After(e.ExpiresAt)
}

type tokenStore struct {
	pathFn  func() string
	mu      sync.RWMutex
	entries map[string]map[string]TokenStoreEntry
	loaded  bool
}

type OAuthTokenStore interface {
	Save(provider, profileID string, entry *TokenStoreEntry) error
	Load(provider, profileID string) (*TokenStoreEntry, bool, error)
	Clear(provider, profileID string) error
}

func NewOAuthTokenStore(paths *Paths) (OAuthTokenStore, error) {
	if err := os.MkdirAll(paths.Root(), 0700); err != nil {
		return nil, fmt.Errorf("create oauth token store directory: %w", err)
	}
	return &tokenStore{
		pathFn:  func() string { return filepath.Join(paths.Root(), "oauth_tokens.json") },
		entries: make(map[string]map[string]TokenStoreEntry),
	}, nil
}

func (s *tokenStore) path() string {
	return s.pathFn()
}

func (s *tokenStore) Save(provider, profileID string, entry *TokenStoreEntry) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.loadAllLocked(); err != nil {
		return err
	}

	if s.entries[provider] == nil {
		s.entries[provider] = make(map[string]TokenStoreEntry)
	}
	entry.UpdatedAt = time.Now()
	s.entries[provider][profileID] = *entry

	tmp := s.path() + ".tmp"
	f, err := os.OpenFile(tmp, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return err
	}
	defer os.Remove(tmp)
	enc := json.NewEncoder(f)
	enc.SetIndent("", "")
	if err := enc.Encode(s.entries); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return os.Rename(tmp, s.path())
}

func (s *tokenStore) Load(provider, profileID string) (*TokenStoreEntry, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.loadAllLocked(); err != nil {
		return nil, false, err
	}

	if s.entries[provider] == nil {
		return nil, false, nil
	}
	entry, ok := s.entries[provider][profileID]
	if !ok {
		return nil, false, nil
	}
	return &entry, true, nil
}

func (s *tokenStore) Clear(provider, profileID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.loadAllLocked(); err != nil {
		return err
	}

	if s.entries[provider] != nil {
		delete(s.entries[provider], profileID)
		if len(s.entries[provider]) == 0 {
			delete(s.entries, provider)
		}
	}

	tmp := s.path() + ".tmp"
	f, err := os.OpenFile(tmp, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return err
	}
	defer os.Remove(tmp)
	enc := json.NewEncoder(f)
	enc.SetIndent("", "")
	if err := enc.Encode(s.entries); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return os.Rename(tmp, s.path())
}

func (s *tokenStore) loadAllLocked() error {
	if s.loaded {
		return nil
	}
	p := s.path()
	data, err := os.ReadFile(p)
	if err != nil {
		if os.IsNotExist(err) {
			s.loaded = true
			return nil
		}
		return err
	}
	if len(data) > 0 {
		if err := json.Unmarshal(data, &s.entries); err != nil {
			return err
		}
	}
	s.loaded = true
	return nil
}
