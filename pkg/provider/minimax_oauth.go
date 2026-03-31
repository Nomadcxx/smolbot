package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

const maxResponseBody = 1 << 20 // 1 MB

const minimaxOAuthGlobalBase = "https://api.minimax.io"

var defaultMiniMaxOAuthConfig = OAuthConfig{
	BaseURL:   minimaxOAuthGlobalBase,
	ClientID:  "78257093-7e40-4613-99e0-527b14b39113",
	AuthURL:   "/oauth/code",
	TokenURL:  "/oauth/token",
	RevokeURL: "/oauth/revoke",
	Scopes:    []string{},
}

type MiniMaxOAuthProvider struct {
	config     OAuthConfig
	token      *TokenInfo
	mu         sync.Mutex
	provider   string
	httpClient HTTPClient
}

type HTTPClient interface {
	Do(req *http.Request) (*http.Response, error)
}

type MiniMaxOAuthConfigOption func(*MiniMaxOAuthConfig)

type MiniMaxOAuthConfig struct {
	BaseURL string
}

func WithMiniMaxOAuthBaseURL(base string) MiniMaxOAuthConfigOption {
	return func(c *MiniMaxOAuthConfig) {
		c.BaseURL = base
	}
}

func NewMiniMaxOAuthProvider(providerName string, opts ...MiniMaxOAuthConfigOption) *MiniMaxOAuthProvider {
	cfg := defaultMiniMaxOAuthConfig
	c := &MiniMaxOAuthConfig{BaseURL: cfg.BaseURL}
	for _, opt := range opts {
		opt(c)
	}
	if c.BaseURL == "" {
		c.BaseURL = cfg.BaseURL
	}
	cfg.BaseURL = c.BaseURL
	return &MiniMaxOAuthProvider{
		config:     cfg,
		provider:   providerName,
		httpClient: http.DefaultClient,
	}
}

func (p *MiniMaxOAuthProvider) SetHTTPClient(c HTTPClient) {
	p.httpClient = c
}

func (p *MiniMaxOAuthProvider) Name() string    { return p.provider }
func (p *MiniMaxOAuthProvider) AuthType() AuthType { return AuthTypeOAuth }
func (p *MiniMaxOAuthProvider) GetAuthConfig() OAuthConfig { return p.config }

func (p *MiniMaxOAuthProvider) SetToken(t *TokenInfo) {
	p.mu.Lock()
	p.token = t
	p.mu.Unlock()
}

func (p *MiniMaxOAuthProvider) GetToken() *TokenInfo {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.token
}

type DeviceCodeResponse struct {
	UserCode        string `json:"user_code"`
	DeviceCode      string `json:"device_code"`
	VerificationURI string `json:"verification_uri"`
	ExpiresIn       int    `json:"expires_in"`
	Interval        int    `json:"interval"`
	State           string `json:"state,omitempty"`
	Verifier        string `json:"-"`
	Error           string `json:"error,omitempty"`
	ErrorDesc       string `json:"error_description,omitempty"`
}

type TokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token,omitempty"`
	ExpiresIn    int    `json:"expires_in"`
	TokenType    string `json:"token_type"`
	Scope        string `json:"scope"`
	Error        string `json:"error,omitempty"`
	ErrorDesc    string `json:"error_description,omitempty"`
}

func (p *MiniMaxOAuthProvider) InitiateAuth(ctx context.Context) (*DeviceCodeResponse, string, error) {
	pkce, err := GeneratePKCE()
	if err != nil {
		return nil, "", fmt.Errorf("generate PKCE: %w", err)
	}
	state, err := generateRandomString(16)
	if err != nil {
		return nil, "", fmt.Errorf("generate state: %w", err)
	}

	data := url.Values{}
	data.Set("client_id", p.config.ClientID)
	data.Set("response_type", "code")
	data.Set("scope", strings.Join(p.config.Scopes, " "))
	data.Set("code_challenge", pkce.Challenge)
	data.Set("code_challenge_method", "S256")
	data.Set("state", state)

	authURL := p.config.BaseURL + p.config.AuthURL
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, authURL, strings.NewReader(data.Encode()))
	if err != nil {
		return nil, "", fmt.Errorf("create auth request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, "", fmt.Errorf("do auth request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBody))
	if err != nil {
		return nil, "", fmt.Errorf("read auth response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, "", fmt.Errorf("auth request failed: %d %s", resp.StatusCode, string(body))
	}

	var dc DeviceCodeResponse
	if err := json.Unmarshal(body, &dc); err != nil {
		return nil, "", fmt.Errorf("decode auth response: %w", err)
	}

	if dc.Error != "" {
		return nil, "", fmt.Errorf("auth error: %s: %s", dc.Error, dc.ErrorDesc)
	}

	dc.State = state
	dc.Verifier = pkce.Verifier

	return &dc, state, nil
}

func (p *MiniMaxOAuthProvider) PollForToken(ctx context.Context, dc *DeviceCodeResponse, state string) (*TokenInfo, error) {
	if dc.State != state {
		return nil, fmt.Errorf("state mismatch: expected %q, got %q", state, dc.State)
	}

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
			token, err := p.exchangeDeviceCode(ctx, dc.DeviceCode, dc.Verifier)
			if err != nil {
				if strings.Contains(err.Error(), "authorization_pending") {
					continue
				}
				if strings.Contains(err.Error(), "slow_down") {
					interval += 5 * time.Second
					ticker.Reset(interval)
					continue
				}
				if strings.Contains(err.Error(), "expired") {
					return nil, fmt.Errorf("device code expired")
				}
				return nil, err
			}
			p.mu.Lock()
			p.token = token
			p.mu.Unlock()
			return token, nil
		}
	}
}

func (p *MiniMaxOAuthProvider) exchangeDeviceCode(ctx context.Context, deviceCode, codeVerifier string) (*TokenInfo, error) {
	data := url.Values{}
	data.Set("client_id", p.config.ClientID)
	data.Set("grant_type", "urn:ietf:params:oauth:grant-type:device_code")
	data.Set("device_code", deviceCode)
	data.Set("code_verifier", codeVerifier)

	tokenURL := p.config.BaseURL + p.config.TokenURL
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, tokenURL, strings.NewReader(data.Encode()))
	if err != nil {
		return nil, fmt.Errorf("create token request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("do token request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBody))
	if err != nil {
		return nil, fmt.Errorf("read token response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		var tokenResp TokenResponse
		if json.Unmarshal(body, &tokenResp) == nil && tokenResp.Error != "" {
			if tokenResp.Error == "authorization_pending" || tokenResp.Error == "slow_down" {
				return nil, fmt.Errorf("%s", tokenResp.Error)
			}
			return nil, fmt.Errorf("%s: %s", tokenResp.Error, tokenResp.ErrorDesc)
		}
		return nil, fmt.Errorf("token request failed: %d %s", resp.StatusCode, string(body))
	}

	var tokenResp TokenResponse
	if err := json.Unmarshal(body, &tokenResp); err != nil {
		return nil, fmt.Errorf("decode token response: %w", err)
	}

	return &TokenInfo{
		AccessToken:  tokenResp.AccessToken,
		RefreshToken: tokenResp.RefreshToken,
		ExpiresAt:    time.Now().Add(time.Duration(tokenResp.ExpiresIn) * time.Second),
		TokenType:    tokenResp.TokenType,
		Scope:        tokenResp.Scope,
		ProviderID:   p.provider,
	}, nil
}

func (p *MiniMaxOAuthProvider) RefreshToken(ctx context.Context) (*TokenInfo, error) {
	p.mu.Lock()
	tok := p.token
	p.mu.Unlock()
	if tok == nil || tok.RefreshToken == "" {
		return nil, fmt.Errorf("no refresh token available")
	}

	data := url.Values{}
	data.Set("client_id", p.config.ClientID)
	data.Set("grant_type", "refresh_token")
	data.Set("refresh_token", tok.RefreshToken)

	tokenURL := p.config.BaseURL + p.config.TokenURL
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, tokenURL, strings.NewReader(data.Encode()))
	if err != nil {
		return nil, fmt.Errorf("create refresh request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("do refresh request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBody))
	if err != nil {
		return nil, fmt.Errorf("read refresh response: %w", err)
	}

	var tokenResp TokenResponse
	if err := json.Unmarshal(body, &tokenResp); err != nil {
		return nil, fmt.Errorf("decode refresh response: %w", err)
	}

	if tokenResp.Error != "" {
		return nil, fmt.Errorf("%s: %s", tokenResp.Error, tokenResp.ErrorDesc)
	}

	newToken := &TokenInfo{
		AccessToken:  tokenResp.AccessToken,
		RefreshToken: tokenResp.RefreshToken,
		ExpiresAt:    time.Now().Add(time.Duration(tokenResp.ExpiresIn) * time.Second),
		TokenType:    tokenResp.TokenType,
		Scope:        tokenResp.Scope,
		ProviderID:   p.provider,
	}
	if newToken.RefreshToken == "" {
		newToken.RefreshToken = tok.RefreshToken
	}
	p.mu.Lock()
	p.token = newToken
	p.mu.Unlock()
	return newToken, nil
}

func (p *MiniMaxOAuthProvider) RevokeToken(ctx context.Context) error {
	p.mu.Lock()
	tok := p.token
	p.mu.Unlock()
	if tok == nil {
		return nil
	}

	data := url.Values{}
	data.Set("client_id", p.config.ClientID)
	data.Set("token", tok.AccessToken)

	revokeURL := p.config.BaseURL + p.config.RevokeURL
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, revokeURL, strings.NewReader(data.Encode()))
	if err != nil {
		return fmt.Errorf("create revoke request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("do revoke request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, maxResponseBody))
		return fmt.Errorf("revoke request failed: %d %s", resp.StatusCode, string(body))
	}

	p.mu.Lock()
	p.token = nil
	p.mu.Unlock()
	return nil
}

func (p *MiniMaxOAuthProvider) ensureValidToken(ctx context.Context) (*TokenInfo, error) {
	p.mu.Lock()
	tok := p.token
	p.mu.Unlock()
	if tok == nil || tok.IsExpired() {
		if tok != nil && tok.RefreshToken != "" {
			refreshed, err := p.RefreshToken(ctx)
			if err != nil {
				return nil, fmt.Errorf("token expired and refresh failed: %w", err)
			}
			return refreshed, nil
		}
		return nil, fmt.Errorf("no OAuth token available")
	}
	if time.Until(tok.ExpiresAt) < 5*time.Minute && tok.RefreshToken != "" {
		refreshed, err := p.RefreshToken(ctx)
		if err != nil {
			return tok, nil
		}
		return refreshed, nil
	}
	return tok, nil
}

func (p *MiniMaxOAuthProvider) chatBase() string {
	base := strings.TrimRight(p.config.BaseURL, "/")
	if !strings.HasSuffix(base, "/v1") {
		base += "/v1"
	}
	return base
}

func (p *MiniMaxOAuthProvider) Chat(ctx context.Context, req ChatRequest) (*Response, error) {
	tok, err := p.ensureValidToken(ctx)
	if err != nil {
		return nil, err
	}
	req.Model = StripProviderPrefix(req.Model)
	openai := NewOpenAIProvider(p.provider, tok.AccessToken, p.chatBase(), nil)
	return openai.Chat(ctx, req)
}

func (p *MiniMaxOAuthProvider) ChatStream(ctx context.Context, req ChatRequest) (*Stream, error) {
	tok, err := p.ensureValidToken(ctx)
	if err != nil {
		return nil, err
	}
	req.Model = StripProviderPrefix(req.Model)
	openai := NewOpenAIProvider(p.provider, tok.AccessToken, p.chatBase(), nil)
	return openai.ChatStream(ctx, req)
}
