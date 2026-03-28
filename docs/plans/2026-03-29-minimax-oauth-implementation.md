# MiniMax OAuth Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add OAuth 2.0 device code flow authentication for MiniMax, with token refresh, to enable Token Plan subscription access.

**Architecture:** Implement a new OAuthProvider type alongside existing APIKeyProvider. Extend the provider registry with device code flow, PKCE support, and token refresh. Integrate OAuth into the installer wizard and TUI model picker.

**Tech Stack:** Go standard library for OAuth (crypto/rand, encoding/json, net/http, net/url); existing Bubbletea UI framework for installer integration.

---

## Overview

Based on OpenClaw's implementation, MiniMax OAuth uses:
- **Device Code Flow** (RFC 8628) - ideal for CLI tools
- **PKCE** with S256 challenge method  
- **Token storage** with expiry metadata
- **Profile ID format** - `provider:profile-name` (e.g., `minimax-portal:default`)

OpenClaw reference constants:
```
const MINIMAX_OAUTH_CONFIG = {
  global: {
    baseUrl: "https://api.minimax.io",
    clientId: "78257093-7e40-4613-99e0-527b14b39113",
  },
};
```

---

## Task 1: Define OAuth Types and Interfaces

**Files:**
- Create: `pkg/provider/oauth.go`
- Test: `pkg/provider/oauth_test.go`

**Step 1: Create pkg/provider/oauth.go with type definitions**

```go
package provider

import "time"

// OAuthConfig holds OAuth provider configuration
type OAuthConfig struct {
	BaseURL   string // e.g., "https://api.minimax.io"
	ClientID  string
	AuthURL   string // e.g., "/oauth/code"
	TokenURL  string // e.g., "/oauth/token"
	RevokeURL string // e.g., "/oauth/revoke"
	Scopes    []string
}

// TokenInfo represents an OAuth token with metadata
type TokenInfo struct {
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token,omitempty"`
	ExpiresAt   time.Time `json:"expires_at"` // Unix timestamp ms
	TokenType   string    `json:"token_type"`
	Scope       string    `json:"scope"`
}

// IsExpired returns true if the token is expired or will expire within buffer
func (t *TokenInfo) IsExpired() bool {
	return time.Until(t.ExpiresAt) < 2*time.Minute
}

// OAuthProvider defines the interface for OAuth-based providers
type OAuthProvider interface {
	Provider
	AuthType() AuthType
	RefreshToken(ctx context.Context) (*TokenInfo, error)
	RevokeToken(ctx context.Context) error
	GetAuthConfig() OAuthConfig
}

// AuthType distinguishes key-based vs OAuth vs token-based auth
type AuthType int

const (
	AuthTypeAPIKey AuthType = iota
	AuthTypeOAuth
	AuthTypeToken
)

func (a AuthType) String() string {
	switch a {
	case AuthTypeAPIKey:
		return "api_key"
	case AuthTypeOAuth:
		return "oauth"
	case AuthTypeToken:
		return "token"
	default:
		return "unknown"
	}
}
```

**Step 2: Verify build**

Run: `go build ./pkg/provider`
Expected: OK

**Step 3: Write oauth_test.go**

```go
package provider

import (
	"testing"
	"time"
)

func TestTokenInfoIsExpired(t *testing.T) {
	token := &TokenInfo{
		AccessToken:  "test",
		RefreshToken: "refresh",
		ExpiresAt:    time.Now().Add(5 * time.Minute),
	}
	if token.IsExpired() {
		t.Fatal("token should not be expired")
	}

	token.ExpiresAt = time.Now().Add(1 * time.Minute)
	if !token.IsExpired() {
		t.Fatal("token should be expired within buffer")
	}
}

func TestAuthTypeString(t *testing.T) {
	tests := []struct {
		at     AuthType
		expect string
	}{
		{AuthTypeAPIKey, "api_key"},
		{AuthTypeOAuth, "oauth"},
		{AuthTypeToken, "token"},
	}
	for _, tt := range tests {
		if tt.at.String() != tt.expect {
			t.Errorf("AuthType(%d).String() = %q, want %q", tt.at, tt.at.String(), tt.expect)
		}
	}
}
```

**Step 4: Run tests**

Run: `go test ./pkg/provider -run TestTokenInfo -v`
Expected: PASS

**Step 5: Commit**

```bash
git add pkg/provider/oauth.go pkg/provider/oauth_test.go
git commit -m "feat(provider): add OAuth type definitions"
```

---

## Task 2: Implement PKCE Utilities

**Files:**
- Create: `pkg/provider/pkce.go`
- Test: `pkg/provider/pkce_test.go`

**Step 1: Create pkce.go**

```go
package provider

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
)

// PKCEParams holds code verifier and challenge
type PKCEParams struct {
	Verifier  string
	Challenge string
	Method    string // always "S256"
}

// GeneratePKCE creates a new PKCE code verifier and S256 challenge
func GeneratePKCE() (*PKCEParams, error) {
	verifier, err := generateRandomString(64)
	if err != nil {
		return nil, err
	}
	challenge := S256Challenge(verifier)
	return &PKCEParams{
		Verifier:  verifier,
		Challenge: challenge,
		Method:    "S256",
	}, nil
}

// generateRandomString generates a URL-safe random string
func generateRandomString(length int) (string, error) {
	bytes := make([]byte, length)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(bytes), nil
}

// S256Challenge creates the S256 code challenge from a verifier
func S256Challenge(verifier string) string {
	h := sha256.New()
	h.Write([]byte(verifier))
	digest := h.Sum(nil)
	return base64.RawURLEncoding.EncodeToString(digest)
}
```

**Step 2: Write pkce_test.go**

```go
package provider

import (
	"testing"
)

func TestGeneratePKCE(t *testing.T) {
	pkce, err := GeneratePKCE()
	if err != nil {
		t.Fatalf("GeneratePKCE() error = %v", err)
	}
	if len(pkce.Verifier) == 0 {
		t.Error("Verifier should not be empty")
	}
	if pkce.Challenge == "" {
		t.Error("Challenge should not be empty")
	}
	if pkce.Method != "S256" {
		t.Errorf("Method = %q, want S256", pkce.Method)
	}
	// Verify it's non-deterministic
	pkce2, _ := GeneratePKCE()
	if pkce.Verifier == pkce2.Verifier {
		t.Error("Two PKCE generations should produce different verifiers")
	}
}

func TestS256Challenge(t *testing.T) {
	verifier := "dBjftJeZ4CVP-mB92K27uhbUJU1p1r_wW1gFWFOEjXk"
	challenge := S256Challenge(verifier)
	challenge2 := S256Challenge(verifier)
	if challenge != challenge2 {
		t.Error("S256Challenge should be deterministic")
	}
	if len(challenge) == 0 {
		t.Error("Challenge should not be empty")
	}
}
```

**Step 3: Run tests**

Run: `go test ./pkg/provider -run TestPKCE -v`
Expected: PASS

**Step 4: Commit**

```bash
git add pkg/provider/pkce.go pkg/provider/pkce_test.go
git commit -m "feat(provider): add PKCE utilities"
```

---

## Task 3: Implement MiniMax OAuth Provider

**Files:**
- Create: `pkg/provider/minimax_oauth.go`
- Test: `pkg/provider/minimax_oauth_test.go`

**Step 1: Create pkg/provider/minimax_oauth.go**

```go
package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const minimaxOAuthGlobalBase = "https://api.minimax.io"

var minimaxOAuthConfig = OAuthConfig{
	BaseURL:   minimaxOAuthGlobalBase,
	ClientID:  "78257093-7e40-4613-99e0-527b14b39113",
	AuthURL:   "/oauth/code",
	TokenURL:  "/oauth/token",
	RevokeURL: "/oauth/revoke",
	Scopes:    []string{},
}

// MiniMaxOAuthProvider implements OAuth 2.0 device code flow for MiniMax
type MiniMaxOAuthProvider struct {
	config   OAuthConfig
	token    *TokenInfo
	provider string
}

func NewMiniMaxOAuthProvider(providerName string, config OAuthConfig) *MiniMaxOAuthProvider {
	if config.BaseURL == "" {
		config.BaseURL = minimaxOAuthGlobalBase
	}
	if config.AuthURL == "" {
		config.AuthURL = "/oauth/code"
	}
	if config.TokenURL == "" {
		config.TokenURL = "/oauth/token"
	}
	return &MiniMaxOAuthProvider{
		config:   config,
		provider: providerName,
	}
}

func (p *MiniMaxOAuthProvider) Name() string                { return p.provider }
func (p *MiniMaxOAuthProvider) AuthType() AuthType          { return AuthTypeOAuth }
func (p *MiniMaxOAuthProvider) GetAuthConfig() OAuthConfig  { return p.config }
func (p *MiniMaxOAuthProvider) SetToken(t *TokenInfo)       { p.token = t }

// DeviceCodeResponse from /oauth/code endpoint
type DeviceCodeResponse struct {
	UserCode        string `json:"user_code"`
	DeviceCode      string `json:"device_code"`
	VerificationURI string `json:"verification_uri"`
	ExpiresIn       int    `json:"expires_in"`
	Interval        int    `json:"interval"`
}

// TokenResponse from /oauth/token endpoint
type TokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token,omitempty"`
	ExpiresIn    int    `json:"expires_in"`
	TokenType   string `json:"token_type"`
	Scope       string `json:"scope"`
}

// InitiateDeviceCode starts the OAuth device flow
func (p *MiniMaxOAuthProvider) InitiateDeviceCode(ctx context.Context) (*DeviceCodeResponse, error) {
	data := url.Values{}
	data.Set("client_id", p.config.ClientID)
	data.Set("scope", strings.Join(p.config.Scopes, " "))

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.config.BaseURL+p.config.AuthURL, strings.NewReader(data.Encode()))
	if err != nil {
		return nil, fmt.Errorf("create device code request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("do device code request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("device code request failed: %d %s", resp.StatusCode, body)
	}

	var dc DeviceCodeResponse
	if err := json.NewDecoder(resp.Body).Decode(&dc); err != nil {
		return nil, fmt.Errorf("decode device code response: %w", err)
	}
	return &dc, nil
}

// PollForToken polls /oauth/token until the user authorizes or expires
func (p *MiniMaxOAuthProvider) PollForToken(ctx context.Context, dc *DeviceCodeResponse) (*TokenInfo, error) {
	expiresAt := time.Now().Add(time.Duration(dc.ExpiresIn) * time.Second)
	interval := time.Duration(dc.Interval) * time.Second
	if interval < 5*time.Second {
		interval = 5 * time.Second
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-ticker.C:
			token, err := p.exchangeDeviceCode(dc.DeviceCode)
			if err != nil {
				continue
			}
			token.ExpiresAt = expiresAt
			p.token = token
			return token, nil
		case <-time.After(time.Until(expiresAt)):
			return nil, fmt.Errorf("device code expired")
		}
	}
}

func (p *MiniMaxOAuthProvider) exchangeDeviceCode(deviceCode string) (*TokenInfo, error) {
	data := url.Values{}
	data.Set("client_id", p.config.ClientID)
	data.Set("grant_type", "urn:ietf:params:oauth:grant-type:device_code")
	data.Set("device_code", deviceCode)

	req, err := http.NewRequest(http.MethodPost, p.config.BaseURL+p.config.TokenURL, strings.NewReader(data.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("token request: %w", err)
	}
	defer resp.Body.Close()

	var tokenResp TokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return nil, fmt.Errorf("decode token response: %w", err)
	}

	if resp.StatusCode == http.StatusBadRequest {
		return nil, fmt.Errorf("authorization_pending")
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("token request failed: %d %s", resp.StatusCode, body)
	}

	return &TokenInfo{
		AccessToken:  tokenResp.AccessToken,
		RefreshToken: tokenResp.RefreshToken,
		ExpiresAt:    time.Now().Add(time.Duration(tokenResp.ExpiresIn) * time.Second),
		TokenType:   tokenResp.TokenType,
		Scope:       tokenResp.Scope,
	}, nil
}

// RefreshToken refreshes the access token
func (p *MiniMaxOAuthProvider) RefreshToken(ctx context.Context) (*TokenInfo, error) {
	if p.token == nil || p.token.RefreshToken == "" {
		return nil, fmt.Errorf("no refresh token available")
	}

	data := url.Values{}
	data.Set("client_id", p.config.ClientID)
	data.Set("grant_type", "refresh_token")
	data.Set("refresh_token", p.token.RefreshToken)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.config.BaseURL+p.config.TokenURL, strings.NewReader(data.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("refresh request: %w", err)
	}
	defer resp.Body.Close()

	var tokenResp TokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return nil, fmt.Errorf("decode refresh response: %w", err)
	}

	newToken := &TokenInfo{
		AccessToken:  tokenResp.AccessToken,
		RefreshToken: tokenResp.RefreshToken,
		ExpiresAt:    time.Now().Add(time.Duration(tokenResp.ExpiresIn) * time.Second),
		TokenType:   tokenResp.TokenType,
		Scope:       tokenResp.Scope,
	}
	p.token = newToken
	return newToken, nil
}

// RevokeToken revokes the current token
func (p *MiniMaxOAuthProvider) RevokeToken(ctx context.Context) error {
	if p.token == nil {
		return nil
	}

	data := url.Values{}
	data.Set("client_id", p.config.ClientID)
	data.Set("token", p.token.AccessToken)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.config.BaseURL+p.config.RevokeURL, strings.NewReader(data.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("revoke request: %w", err)
	}
	defer resp.Body.Close()

	p.token = nil
	return nil
}

// Chat delegates to OpenAI-compatible chat with OAuth token
func (p *MiniMaxOAuthProvider) Chat(ctx context.Context, req ChatRequest) (*Response, error) {
	if p.token == nil {
		return nil, fmt.Errorf("no OAuth token available")
	}
	openai := NewOpenAIProvider(p.provider, p.token.AccessToken, p.config.BaseURL, nil)
	return openai.Chat(ctx, req)
}

func (p *MiniMaxOAuthProvider) ChatStream(ctx context.Context, req ChatRequest) (*Stream, error) {
	if p.token == nil {
		return nil, fmt.Errorf("no OAuth token available")
	}
	openai := NewOpenAIProvider(p.provider, p.token.AccessToken, p.config.BaseURL, nil)
	return openai.ChatStream(ctx, req)
}
```

**Step 2: Write minimax_oauth_test.go** (use httptest.Server for mock OAuth server)

**Step 3: Run tests**

Run: `go test ./pkg/provider -run TestMiniMax -v`
Expected: PASS

**Step 4: Commit**

```bash
git add pkg/provider/minimax_oauth.go pkg/provider/minimax_oauth_test.go
git commit -m "feat(provider): implement MiniMax OAuth device code flow"
```

---

## Task 4: Add OAuth Token Storage

**Files:**
- Create: `pkg/config/oauth_store.go`
- Test: `pkg/config/oauth_store_test.go`

**Step 1: Create pkg/config/oauth_store.go**

```go
package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// OAuthTokenStore persists OAuth tokens to disk
type OAuthTokenStore struct {
	path string
	mu   sync.RWMutex
	data map[string]TokenEntry
}

// TokenEntry represents a stored token for a provider profile
type TokenEntry struct {
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token,omitempty"`
	ExpiresAt   int64     `json:"expires_at"` // Unix timestamp ms
	Email       string    `json:"email,omitempty"`
	Provider    string    `json:"provider"`
	ProfileID   string    `json:"profile_id"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// IsExpired returns true if the token is expired or will expire within 2 minutes
func (e *TokenEntry) IsExpired() bool {
	return time.Until(time.Unix(e.ExpiresAt/1000, 0)) < 2*time.Minute
}

func NewOAuthTokenStore(path string) *OAuthTokenStore {
	return &OAuthTokenStore{
		path: path,
		data: make(map[string]TokenEntry),
	}
}

// Load reads tokens from disk
func (s *OAuthTokenStore) Load() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	f, err := os.Open(s.path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	defer f.Close()

	if err := json.NewDecoder(f).Decode(&s.data); err != nil {
		return err
	}
	return nil
}

// Save writes tokens to disk atomically
func (s *OAuthTokenStore) Save() error {
	s.mu.RLock()
	defer s.mu.RUnlock()

	dir := filepath.Dir(s.path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}

	tmp := s.path + ".tmp"
	f, err := os.Create(tmp)
	if err != nil {
		return err
	}
	defer f.Close()

	if err := json.NewEncoder(f).Encode(s.data); err != nil {
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return os.Rename(tmp, s.path)
}

// Get retrieves a token entry by profile ID
func (s *OAuthTokenStore) Get(profileID string) (TokenEntry, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	entry, ok := s.data[profileID]
	return entry, ok
}

// Set stores a token entry
func (s *OAuthTokenStore) Set(profileID string, entry TokenEntry) {
	s.mu.Lock()
	defer s.mu.Unlock()
	entry.ProfileID = profileID
	entry.UpdatedAt = time.Now()
	s.data[profileID] = entry
}

// Delete removes a token entry
func (s *OAuthTokenStore) Delete(profileID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.data, profileID)
}

// DefaultOAuthTokenStore returns the default store
func DefaultOAuthTokenStore() (*OAuthTokenStore, error) {
	paths := DefaultPaths()
	store := NewOAuthTokenStore(filepath.Join(paths.ConfigDir(), "oauth_tokens.json"))
	if err := store.Load(); err != nil {
		return nil, err
	}
	return store, nil
}
```

**Step 2: Write oauth_store_test.go**

**Step 3: Run tests**

Run: `go test ./pkg/config -run TestOAuthTokenStore -v`
Expected: PASS

**Step 4: Commit**

```bash
git add pkg/config/oauth_store.go pkg/config/oauth_store_test.go
git commit -m "feat(config): add OAuth token persistent storage"
```

---

## Task 5: Register MiniMax OAuth in Provider Registry

**Files:**
- Modify: `pkg/provider/registry.go`

**Step 1: Add OAuth factory to NewRegistryWithDefaults**

```go
r.RegisterFactory("minimax-oauth", func(pc config.ProviderConfig) Provider {
    cfg := OAuthConfig{
        BaseURL:  "https://api.minimax.io",
        ClientID: "78257093-7e40-4613-99e0-527b14b39113",
        AuthURL:  "/oauth/code",
        TokenURL: "/oauth/token",
    }
    provider := NewMiniMaxOAuthProvider("minimax-oauth", cfg)
    if store != nil {
        if entry, ok := store.Get("minimax:oauth"); ok {
            provider.SetToken(&TokenInfo{
                AccessToken:  entry.AccessToken,
                RefreshToken: entry.RefreshToken,
                ExpiresAt:   time.Unix(entry.ExpiresAt/1000, 0),
            })
        }
    }
    return provider
})
```

Note: Registry must accept optional OAuthTokenStore.

**Step 2: Run build to verify**

Run: `go build ./pkg/provider`
Expected: OK

**Step 3: Commit**

```bash
git add pkg/provider/registry.go
git commit -m "feat(provider): register minimax-oauth factory in registry"
```

---

## Task 6: Add Installer OAuth Step for MiniMax

**Files:**
- Modify: `cmd/installer/types.go` (add stepOAuth)
- Modify: `cmd/installer/views.go` (renderOAuth)
- Modify: `cmd/installer/tasks.go` (executeOAuthTask)
- Modify: `cmd/installer/main.go` (wizard flow)

**Step 1: Add stepOAuth to installStep enum and OAuth model fields**

**Step 2: Add renderOAuth() function showing user code + verification URL + spinner**

**Step 3: Add executeOAuthTask() that initiates device code flow and polls**

**Step 4: Run build**

Run: `go build ./cmd/installer`
Expected: OK

**Step 5: Commit**

```bash
git add cmd/installer/types.go cmd/installer/views.go cmd/installer/tasks.go cmd/installer/main.go
git commit -m "feat(installer): add MiniMax OAuth wizard step"
```

---

## Task 7: Add TUI OAuth Status Display

**Files:**
- Modify: `internal/components/dialog/providers.go`
- Test: `internal/components/dialog/providers_test.go`

**Step 1: Extend ProviderInfo with IsOAuth and TokenEmail fields**

**Step 2: Show "(OAuth)" badge in renderRow for OAuth providers**

**Step 3: Write test for OAuth provider display**

**Step 4: Run tests**

Run: `go test ./internal/components/dialog -run TestProviders -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/components/dialog/providers.go internal/components/dialog/providers_test.go
git commit -m "feat(tui): display OAuth provider status in providers dialog"
```

---

## Task 8: Add Model Picker OAuth Model Filter

**Files:**
- Modify: `internal/components/dialog/models.go`

**Step 1: Filter highspeed models for MiniMax Token Plan OAuth**

When the active provider is minimax-oauth, filter out highspeed variants since they're only available via Pay-As-You-Go API keys.

**Step 2: Run tests**

Run: `go test ./internal/components/dialog -run TestModels -v`
Expected: PASS

**Step 3: Commit**

```bash
git add internal/components/dialog/models.go
git commit -m "feat(tui): filter highspeed models for minimax token plan"
```

---

## Dependencies

- Task 1 → Task 2 → Task 3 (types → PKCE → OAuth provider)
- Task 4 is independent, can run parallel with Tasks 1-3
- Task 5 depends on Tasks 1-4
- Task 6 depends on Tasks 1-5
- Task 7 depends on Tasks 1-5
- Task 8 depends on Task 7

## Verification

After all tasks:
```bash
go test ./pkg/provider ./pkg/config ./internal/components/dialog ./cmd/installer -v
go build ./cmd/smolbot
```

## Post-Implementation Checklist

- [ ] MiniMax OAuth device flow end-to-end test
- [ ] Token refresh test (expire a token, verify refresh)
- [ ] Installer OAuth wizard step manual test
- [ ] TUI `/providers` shows OAuth badge
- [ ] Model picker filters highspeed for Token Plan OAuth
- [ ] Token persistence across restarts
