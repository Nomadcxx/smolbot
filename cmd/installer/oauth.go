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
	minimaxOAuthClientID  = "78257093-7e40-4613-99e0-527b14b39113"
	minimaxOAuthBaseURL   = "https://api.minimax.io"
	minimaxOAuthProfileID = "minimax-portal:default"
	minimaxOAuthScope     = "group_id profile model.completion"
	minimaxOAuthGrantType = "urn:ietf:params:oauth:grant-type:user_code"

	oauthMaxBody = 1 << 20 // 1 MB
)

type oauthToken struct {
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token,omitempty"`
	ExpiresAt    time.Time `json:"expires_at"`
	TokenType    string    `json:"token_type"`
	Scope        string    `json:"scope"`
}

// deviceCodeResp holds the /oauth/code response.
// expired_in is an absolute unix timestamp in milliseconds.
type deviceCodeResp struct {
	UserCode        string `json:"user_code"`
	VerificationURI string `json:"verification_uri"`
	ExpiredIn       int64  `json:"expired_in"` // absolute unix ms timestamp
	Interval        int    `json:"interval"`   // polling interval in ms; 0 → default 2s
	State           string `json:"state"`
	Error           string `json:"error,omitempty"`

	// Set locally after PKCE generation, not from JSON.
	codeVerifier string
	sentState    string
}

// initiateAuthFlow posts to MiniMax /oauth/code, opens the browser, and
// returns the verification URL and device-code descriptor for polling.
func initiateAuthFlow() (verificationURL string, dc *deviceCodeResp, err error) {
	verifier, challenge, err := generateOAuthPKCE()
	if err != nil {
		return "", nil, fmt.Errorf("generate PKCE: %w", err)
	}
	state, err := generateOAuthRandom(16)
	if err != nil {
		return "", nil, fmt.Errorf("generate state: %w", err)
	}

	form := url.Values{}
	form.Set("response_type", "code")
	form.Set("client_id", minimaxOAuthClientID)
	form.Set("scope", minimaxOAuthScope)
	form.Set("code_challenge", challenge)
	form.Set("code_challenge_method", "S256")
	form.Set("state", state)

	req, err := http.NewRequest(http.MethodPost, minimaxOAuthBaseURL+"/oauth/code",
		strings.NewReader(form.Encode()))
	if err != nil {
		return "", nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", nil, fmt.Errorf("auth request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, oauthMaxBody))
	if err != nil {
		return "", nil, fmt.Errorf("read auth response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return "", nil, fmt.Errorf("auth request failed (%d): %s", resp.StatusCode, string(body))
	}

	var d deviceCodeResp
	if err := json.Unmarshal(body, &d); err != nil {
		return "", nil, fmt.Errorf("decode auth response: %w", err)
	}
	if d.Error != "" {
		return "", nil, fmt.Errorf("auth error: %s", d.Error)
	}
	if d.VerificationURI == "" || d.UserCode == "" {
		return "", nil, fmt.Errorf("incomplete auth response (missing user_code or verification_uri): %s", string(body))
	}
	if d.State != state {
		return "", nil, fmt.Errorf("state mismatch: possible CSRF attack or session corruption")
	}

	d.codeVerifier = verifier
	d.sentState = state

	openBrowser(d.VerificationURI)
	return d.VerificationURI, &d, nil
}

// pollMiniMax polls MiniMax /oauth/token until authorized, expired, or ctx cancelled.
// Progress strings (empty = heartbeat, non-empty = transient message) are sent to progressCh.
// progressCh may be nil.
func pollMiniMax(ctx context.Context, dc *deviceCodeResp, progressCh chan<- string) (*oauthToken, error) {
	// interval is in ms; default 2s
	intervalMs := dc.Interval
	if intervalMs <= 0 {
		intervalMs = 2000
	}
	interval := time.Duration(intervalMs) * time.Millisecond

	// expired_in is an absolute unix timestamp in ms
	var expireAt time.Time
	if dc.ExpiredIn > 0 {
		expireAt = time.UnixMilli(dc.ExpiredIn)
	}
	if expireAt.IsZero() || expireAt.Before(time.Now().Add(5*time.Second)) {
		expireAt = time.Now().Add(5 * time.Minute)
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-ticker.C:
			if time.Now().After(expireAt) {
				return nil, fmt.Errorf("timed out waiting for authorization")
			}
			tok, pending, err := pollToken(ctx, dc.UserCode, dc.codeVerifier)
			if err != nil {
				sendProgress(progressCh, err.Error())
				// transient — keep polling
				continue
			}
			if pending {
				sendProgress(progressCh, "")
				continue
			}
			return tok, nil
		}
	}
}

// pollToken posts to /oauth/token and interprets the status-based response.
// Returns (token, false, nil) on success, (nil, true, nil) when pending,
// or (nil, false, err) on a terminal error.
func pollToken(ctx context.Context, userCode, codeVerifier string) (*oauthToken, bool, error) {
	form := url.Values{}
	form.Set("grant_type", minimaxOAuthGrantType)
	form.Set("client_id", minimaxOAuthClientID)
	form.Set("user_code", userCode)
	form.Set("code_verifier", codeVerifier)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		minimaxOAuthBaseURL+"/oauth/token", strings.NewReader(form.Encode()))
	if err != nil {
		return nil, false, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, false, fmt.Errorf("token request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, oauthMaxBody))
	if err != nil {
		return nil, false, fmt.Errorf("read token response: %w", err)
	}

	var tr struct {
		Status              string `json:"status"` // "success", "pending", "error"
		AccessToken         string `json:"access_token"`
		RefreshToken        string `json:"refresh_token"`
		ExpiredIn           int64  `json:"expired_in"` // unix ms timestamp
		TokenType           string `json:"token_type"`
		BaseResp            *struct {
			StatusCode int    `json:"status_code"`
			StatusMsg  string `json:"status_msg"`
		} `json:"base_resp"`
	}
	if err := json.Unmarshal(body, &tr); err != nil {
		// Non-JSON response on non-200
		return nil, false, fmt.Errorf("token request failed (%d): %s", resp.StatusCode, string(body))
	}

	switch tr.Status {
	case "success":
		if tr.AccessToken == "" || tr.RefreshToken == "" {
			return nil, false, fmt.Errorf("incomplete token payload: %s", string(body))
		}
		expiresAt := time.UnixMilli(tr.ExpiredIn)
		if expiresAt.Before(time.Now()) {
			expiresAt = time.Now().Add(time.Hour)
		}
		return &oauthToken{
			AccessToken:  tr.AccessToken,
			RefreshToken: tr.RefreshToken,
			ExpiresAt:    expiresAt,
			TokenType:    tr.TokenType,
		}, false, nil

	case "pending", "":
		return nil, true, nil

	case "error":
		msg := "authorization error"
		if tr.BaseResp != nil && tr.BaseResp.StatusMsg != "" {
			msg = tr.BaseResp.StatusMsg
		}
		return nil, false, fmt.Errorf("%s", msg)

	default:
		if resp.StatusCode != http.StatusOK {
			msg := string(body)
			if tr.BaseResp != nil && tr.BaseResp.StatusMsg != "" {
				msg = tr.BaseResp.StatusMsg
			}
			return nil, false, fmt.Errorf("token request failed (%d): %s", resp.StatusCode, msg)
		}
		// Unknown status — treat as pending
		return nil, true, nil
	}
}

func sendProgress(ch chan<- string, msg string) {
	if ch == nil {
		return
	}
	select {
	case ch <- msg:
	default:
	}
}

// saveOAuthToken writes the token to configDir/oauth_tokens.json.
func saveOAuthToken(configDir string, tok *oauthToken) error {
	if tok == nil {
		return fmt.Errorf("nil token")
	}
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
