// cmd/installer/oauth.go
package main

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

const (
	minimaxOAuthClientID = "78257093-7e40-4613-99e0-527b14b39113"
	minimaxOAuthBaseURL  = "https://api.minimax.io"
	minimaxOAuthProfileID = "minimax-portal:default"

	oauthMaxBody = 1 << 20 // 1 MB
)

type deviceCodeResponse struct {
	UserCode        string `json:"user_code"`
	DeviceCode      string `json:"device_code"`
	VerificationURI string `json:"verification_uri"`
	ExpiresIn       int    `json:"expires_in"`
	Interval        int    `json:"interval"`
	Error           string `json:"error,omitempty"`
	ErrorDesc       string `json:"error_description,omitempty"`

	// not from JSON — set locally
	codeVerifier string
	state        string
}

type oauthToken struct {
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token,omitempty"`
	ExpiresAt    time.Time `json:"expires_at"`
	TokenType    string    `json:"token_type"`
	Scope        string    `json:"scope"`
}

// initiateDeviceCodeAuth POSTs to the MiniMax device-code endpoint and
// returns the code the user must enter at the verification URI.
func initiateDeviceCodeAuth() (*deviceCodeResponse, error) {
	verifier, challenge, err := generateOAuthPKCE()
	if err != nil {
		return nil, fmt.Errorf("generate PKCE: %w", err)
	}
	state, err := generateOAuthRandom(16)
	if err != nil {
		return nil, fmt.Errorf("generate state: %w", err)
	}

	form := url.Values{}
	form.Set("client_id", minimaxOAuthClientID)
	form.Set("response_type", "code")
	form.Set("scope", "")
	form.Set("code_challenge", challenge)
	form.Set("code_challenge_method", "S256")
	form.Set("state", state)

	req, err := http.NewRequest(http.MethodPost, minimaxOAuthBaseURL+"/oauth/code", strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("auth request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, oauthMaxBody))
	if err != nil {
		return nil, fmt.Errorf("read auth response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("auth request failed (%d): %s", resp.StatusCode, string(body))
	}

	var dc deviceCodeResponse
	if err := json.Unmarshal(body, &dc); err != nil {
		return nil, fmt.Errorf("decode auth response: %w", err)
	}
	if dc.Error != "" {
		return nil, fmt.Errorf("%s: %s", dc.Error, dc.ErrorDesc)
	}

	dc.codeVerifier = verifier
	dc.state = state
	return &dc, nil
}

// pollForToken blocks until MiniMax grants the token or the context is cancelled.
func pollForToken(ctx context.Context, dc *deviceCodeResponse) (*oauthToken, error) {
	interval := time.Duration(dc.Interval) * time.Second
	if interval < 5*time.Second {
		interval = 5 * time.Second
	}

	deadline := time.NewTimer(time.Duration(dc.ExpiresIn) * time.Second)
	defer deadline.Stop()
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-deadline.C:
			return nil, fmt.Errorf("device code expired")
		case <-ticker.C:
			tok, err := exchangeDeviceCode(ctx, dc.DeviceCode, dc.codeVerifier)
			if err != nil {
				msg := err.Error()
				if strings.Contains(msg, "authorization_pending") {
					continue
				}
				if strings.Contains(msg, "slow_down") {
					interval += 5 * time.Second
					ticker.Reset(interval)
					continue
				}
				if strings.Contains(msg, "expired") {
					return nil, fmt.Errorf("device code expired")
				}
				return nil, err
			}
			return tok, nil
		}
	}
}

func exchangeDeviceCode(ctx context.Context, deviceCode, codeVerifier string) (*oauthToken, error) {
	form := url.Values{}
	form.Set("client_id", minimaxOAuthClientID)
	form.Set("grant_type", "urn:ietf:params:oauth:grant-type:device_code")
	form.Set("device_code", deviceCode)
	form.Set("code_verifier", codeVerifier)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, minimaxOAuthBaseURL+"/oauth/token", strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("token request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, oauthMaxBody))
	if err != nil {
		return nil, fmt.Errorf("read token response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		var errResp struct {
			Error     string `json:"error"`
			ErrorDesc string `json:"error_description"`
		}
		if json.Unmarshal(body, &errResp) == nil && errResp.Error != "" {
			return nil, fmt.Errorf("%s", errResp.Error)
		}
		return nil, fmt.Errorf("token request failed (%d): %s", resp.StatusCode, string(body))
	}

	var tr struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		ExpiresIn    int    `json:"expires_in"`
		TokenType    string `json:"token_type"`
		Scope        string `json:"scope"`
	}
	if err := json.Unmarshal(body, &tr); err != nil {
		return nil, fmt.Errorf("decode token response: %w", err)
	}

	return &oauthToken{
		AccessToken:  tr.AccessToken,
		RefreshToken: tr.RefreshToken,
		ExpiresAt:    time.Now().Add(time.Duration(tr.ExpiresIn) * time.Second),
		TokenType:    tr.TokenType,
		Scope:        tr.Scope,
	}, nil
}

// saveOAuthToken writes the token to ~/.smolbot/oauth_tokens.json in the
// same format the runtime's token store expects.
func saveOAuthToken(configDir string, tok *oauthToken) error {
	if err := os.MkdirAll(configDir, 0700); err != nil {
		return err
	}
	path := filepath.Join(configDir, "oauth_tokens.json")
	tmp := path + ".tmp"

	entry := map[string]any{
		"access_token":  tok.AccessToken,
		"refresh_token": tok.RefreshToken,
		"expires_at":    tok.ExpiresAt,
		"token_type":    tok.TokenType,
		"scope":         tok.Scope,
		"provider_id":   "minimax-portal",
		"profile_id":    minimaxOAuthProfileID,
		"updated_at":    time.Now(),
	}

	// Merge with any existing tokens
	existing := map[string]map[string]any{}
	if data, err := os.ReadFile(path); err == nil && len(data) > 0 {
		_ = json.Unmarshal(data, &existing)
	}
	if existing["minimax-portal"] == nil {
		existing["minimax-portal"] = map[string]any{}
	}
	existing["minimax-portal"][minimaxOAuthProfileID] = entry

	data, err := json.Marshal(existing)
	if err != nil {
		return err
	}
	f, err := os.OpenFile(tmp, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return err
	}
	defer os.Remove(tmp)
	if _, err := f.Write(data); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// openBrowser tries to open the URL in the default browser; errors are ignored.
func openBrowser(rawURL string) {
	_ = exec.Command("xdg-open", rawURL).Start()
}

func generateOAuthPKCE() (verifier, challenge string, err error) {
	b := make([]byte, 32)
	if _, err = rand.Read(b); err != nil {
		return
	}
	verifier = base64.RawURLEncoding.EncodeToString(b)
	h := sha256.Sum256([]byte(verifier))
	challenge = base64.RawURLEncoding.EncodeToString(h[:])
	return
}

func generateOAuthRandom(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}
