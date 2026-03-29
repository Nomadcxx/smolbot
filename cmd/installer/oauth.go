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
	"net"
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

	oauthMaxBody = 1 << 20 // 1 MB
)

// oauthFlowState holds in-flight state between the init and callback phases.
type oauthFlowState struct {
	verifier    string
	redirectURI string
	state       string
	codeCh      chan string
	errCh       chan error
	srv         *http.Server
}

// Close shuts down the callback HTTP server.
func (f *oauthFlowState) Close() {
	if f.srv != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = f.srv.Shutdown(ctx)
	}
}

type oauthToken struct {
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token,omitempty"`
	ExpiresAt    time.Time `json:"expires_at"`
	TokenType    string    `json:"token_type"`
	Scope        string    `json:"scope"`
}

// initiateAuthFlow starts a localhost callback server, posts to MiniMax's
// /oauth/code endpoint (with redirect_uri), and opens the browser.
// The returned oauthFlowState must be passed to waitForAuthCode.
func initiateAuthFlow() (string, *oauthFlowState, error) {
	// PKCE
	verifier, challenge, err := generateOAuthPKCE()
	if err != nil {
		return "", nil, fmt.Errorf("generate PKCE: %w", err)
	}
	state, err := generateOAuthRandom(16)
	if err != nil {
		return "", nil, fmt.Errorf("generate state: %w", err)
	}

	// Start local callback server on a random port.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return "", nil, fmt.Errorf("bind callback server: %w", err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	redirectURI := fmt.Sprintf("http://127.0.0.1:%d/callback", port)

	codeCh := make(chan string, 1)
	errCh := make(chan error, 1)

	mux := http.NewServeMux()
	mux.HandleFunc("/callback", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if q.Get("state") != state {
			errCh <- fmt.Errorf("state mismatch in callback")
			http.Error(w, "state mismatch", http.StatusBadRequest)
			return
		}
		if errParam := q.Get("error"); errParam != "" {
			desc := q.Get("error_description")
			if desc == "" {
				desc = errParam
			}
			errCh <- fmt.Errorf("%s", desc)
			w.Header().Set("Content-Type", "text/html")
			fmt.Fprintf(w, `<html><body><h2>Authorization failed</h2><p>%s</p><p>You can close this tab.</p></body></html>`, desc)
			return
		}
		code := q.Get("code")
		if code == "" {
			errCh <- fmt.Errorf("no code in callback (params: %s)", r.URL.RawQuery)
			http.Error(w, "missing code", http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprint(w, `<html><body><h2>Authorization successful</h2><p>You can close this tab and return to the installer.</p></body></html>`)
		codeCh <- code
	})

	srv := &http.Server{Handler: mux}
	go srv.Serve(ln) //nolint:errcheck

	flow := &oauthFlowState{
		verifier:    verifier,
		redirectURI: redirectURI,
		state:       state,
		codeCh:      codeCh,
		errCh:       errCh,
		srv:         srv,
	}

	// POST to /oauth/code to get the authorization URL.
	form := url.Values{}
	form.Set("client_id", minimaxOAuthClientID)
	form.Set("response_type", "code")
	form.Set("scope", "")
	form.Set("code_challenge", challenge)
	form.Set("code_challenge_method", "S256")
	form.Set("state", state)
	form.Set("redirect_uri", redirectURI)

	req, err := http.NewRequest(http.MethodPost, minimaxOAuthBaseURL+"/oauth/code", strings.NewReader(form.Encode()))
	if err != nil {
		flow.Close()
		return "", nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		flow.Close()
		return "", nil, fmt.Errorf("auth request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, oauthMaxBody))
	if err != nil {
		flow.Close()
		return "", nil, fmt.Errorf("read auth response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		flow.Close()
		return "", nil, fmt.Errorf("auth request failed (%d): %s", resp.StatusCode, string(body))
	}

	var dc struct {
		VerificationURI string `json:"verification_uri"`
		UserCode        string `json:"user_code"`
		Error           string `json:"error,omitempty"`
		ErrorDesc       string `json:"error_description,omitempty"`
	}
	if err := json.Unmarshal(body, &dc); err != nil {
		flow.Close()
		return "", nil, fmt.Errorf("decode auth response: %w", err)
	}
	if dc.Error != "" {
		flow.Close()
		return "", nil, fmt.Errorf("%s: %s", dc.Error, dc.ErrorDesc)
	}
	if dc.VerificationURI == "" {
		flow.Close()
		return "", nil, fmt.Errorf("no verification_uri in response: %s", string(body))
	}

	openBrowser(dc.VerificationURI)
	return dc.VerificationURI, flow, nil
}

// waitForAuthCode blocks until the callback arrives or ctx is cancelled.
func waitForAuthCode(ctx context.Context, flow *oauthFlowState) (*oauthToken, error) {
	defer flow.Close()

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case err := <-flow.errCh:
		return nil, err
	case code := <-flow.codeCh:
		return exchangeAuthCode(ctx, code, flow.verifier, flow.redirectURI)
	}
}

func exchangeAuthCode(ctx context.Context, code, verifier, redirectURI string) (*oauthToken, error) {
	form := url.Values{}
	form.Set("client_id", minimaxOAuthClientID)
	form.Set("grant_type", "authorization_code")
	form.Set("code", code)
	form.Set("redirect_uri", redirectURI)
	form.Set("code_verifier", verifier)

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
			return nil, fmt.Errorf("%s: %s", errResp.Error, errResp.ErrorDesc)
		}
		return nil, fmt.Errorf("token exchange failed (%d): %s", resp.StatusCode, string(body))
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
	if tr.AccessToken == "" {
		return nil, fmt.Errorf("no access_token in response: %s", string(body))
	}

	expiresIn := tr.ExpiresIn
	if expiresIn <= 0 {
		expiresIn = 3600
	}
	return &oauthToken{
		AccessToken:  tr.AccessToken,
		RefreshToken: tr.RefreshToken,
		ExpiresAt:    time.Now().Add(time.Duration(expiresIn) * time.Second),
		TokenType:    tr.TokenType,
		Scope:        tr.Scope,
	}, nil
}

// saveOAuthToken writes the token to configDir/oauth_tokens.json in the
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
