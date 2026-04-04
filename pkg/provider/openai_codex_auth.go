package provider

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"html"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	codexOAuthIssuer            = "https://auth.openai.com"
	codexOAuthClientID          = "app_EMoamEEZ73f0CkXaXp7hrann"
	codexOAuthScope             = "openid profile email offline_access"
	codexOAuthOriginator        = "smolbot"
	codexOAuthCallbackPort      = 1455
	codexOAuthRedirectURI       = "http://localhost:1455/auth/callback"
	codexOAuthDeviceRedirectURI = "https://auth.openai.com/deviceauth/callback"
	codexOAuthDeviceURL         = "https://auth.openai.com/codex/device"
)

const (
	codexAuthHTMLSuccess = `<!doctype html><html><body>Authorization successful. You can close this window.</body></html>`
	codexAuthHTMLError   = `<!doctype html><html><body>Authorization failed.</body></html>`
)

type CodexJWTClaims struct {
	ChatGPTAccountID string `json:"chatgpt_account_id,omitempty"`
	Organizations    []struct {
		ID string `json:"id"`
	} `json:"organizations,omitempty"`
	OpenAIAuth *struct {
		ChatGPTAccountID string `json:"chatgpt_account_id,omitempty"`
	} `json:"https://api.openai.com/auth,omitempty"`
	Email string `json:"email,omitempty"`
}

func (c *CodexJWTClaims) AccountID() string {
	if c == nil {
		return ""
	}
	if c.ChatGPTAccountID != "" {
		return c.ChatGPTAccountID
	}
	if c.OpenAIAuth != nil && c.OpenAIAuth.ChatGPTAccountID != "" {
		return c.OpenAIAuth.ChatGPTAccountID
	}
	if len(c.Organizations) > 0 {
		return c.Organizations[0].ID
	}
	return ""
}

func ParseJWTClaims(token string) (*CodexJWTClaims, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return nil, fmt.Errorf("invalid jwt format")
	}

	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, fmt.Errorf("decode jwt payload: %w", err)
	}

	var claims CodexJWTClaims
	if err := json.Unmarshal(payload, &claims); err != nil {
		return nil, fmt.Errorf("decode jwt claims: %w", err)
	}
	return &claims, nil
}

func ExtractChatGPTAccountID(accessToken, idToken string) (string, error) {
	var lastErr error
	for _, token := range []string{idToken, accessToken} {
		if strings.TrimSpace(token) == "" {
			continue
		}
		claims, err := ParseJWTClaims(token)
		if err != nil {
			lastErr = err
			continue
		}
		if accountID := claims.AccountID(); accountID != "" {
			return accountID, nil
		}
	}
	return "", lastErr
}

func BuildCodexAuthorizeURL(redirectURI string, pkce *PKCEParams, state string) string {
	params := url.Values{}
	params.Set("response_type", "code")
	params.Set("client_id", codexOAuthClientID)
	params.Set("redirect_uri", redirectURI)
	params.Set("scope", codexOAuthScope)
	params.Set("code_challenge", pkce.Challenge)
	params.Set("code_challenge_method", firstNonEmpty(pkce.Method, "S256"))
	params.Set("id_token_add_organizations", "true")
	params.Set("codex_cli_simplified_flow", "true")
	params.Set("state", state)
	params.Set("originator", codexOAuthOriginator)

	return codexOAuthIssuer + "/oauth/authorize?" + params.Encode()
}

type CodexTokenResponse struct {
	IDToken      string `json:"id_token,omitempty"`
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token,omitempty"`
	ExpiresIn    int    `json:"expires_in,omitempty"`
	TokenType    string `json:"token_type"`
	Scope        string `json:"scope"`
	AccountID    string `json:"-"`
}

func (r *CodexTokenResponse) TokenInfo(now time.Time) *TokenInfo {
	return &TokenInfo{
		AccessToken:  r.AccessToken,
		RefreshToken: r.RefreshToken,
		ExpiresAt:    now.Add(time.Duration(r.ExpiresIn) * time.Second),
		TokenType:    r.TokenType,
		Scope:        r.Scope,
		ProviderID:   "openai-codex",
		ProfileID:    "openai-codex:default",
	}
}

type BrowserAuthResult struct {
	Token *CodexTokenResponse
	Err   error
}

type CodexBrowserAuthorization struct {
	AuthURL     string
	RedirectURI string
	State       string

	results   chan BrowserAuthResult
	server    *http.Server
	listener  net.Listener
	closeOnce sync.Once
}

func (s *CodexBrowserAuthorization) Wait(ctx context.Context) (*CodexTokenResponse, error) {
	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		_ = s.Close(shutdownCtx)
	}()

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case result := <-s.results:
		if result.Err != nil {
			return nil, result.Err
		}
		return result.Token, nil
	}
}

func (s *CodexBrowserAuthorization) Close(ctx context.Context) error {
	var closeErr error
	s.closeOnce.Do(func() {
		if s.server != nil {
			closeErr = s.server.Shutdown(ctx)
			return
		}
		if s.listener != nil {
			closeErr = s.listener.Close()
		}
	})
	return closeErr
}

type CodexDeviceCode struct {
	DeviceAuthID    string
	UserCode        string
	Interval        time.Duration
	VerificationURL string
}

type CodexOAuthClient struct {
	httpClient HTTPClient
	now        func() time.Time
	sleep      func(time.Duration)
	listen     func(network, address string) (net.Listener, error)
}

func NewCodexOAuthClient() *CodexOAuthClient {
	return &CodexOAuthClient{
		httpClient: http.DefaultClient,
		now:        time.Now,
		sleep:      time.Sleep,
		listen:     net.Listen,
	}
}

func (c *CodexOAuthClient) SetHTTPClient(client HTTPClient) {
	c.httpClient = client
}

func (c *CodexOAuthClient) StartBrowserAuthorization(ctx context.Context) (*CodexBrowserAuthorization, error) {
	pkce, err := GeneratePKCE()
	if err != nil {
		return nil, fmt.Errorf("generate pkce: %w", err)
	}
	state, err := generateRandomString(16)
	if err != nil {
		return nil, fmt.Errorf("generate state: %w", err)
	}

	results := make(chan BrowserAuthResult, 1)
	mux := http.NewServeMux()
	mux.Handle("/auth/callback", c.BrowserCallbackHandler(state, codexOAuthRedirectURI, pkce.Verifier, results))

	listener, err := c.listen("tcp", fmt.Sprintf("127.0.0.1:%d", codexOAuthCallbackPort))
	if err != nil {
		return nil, fmt.Errorf("listen for browser callback: %w", err)
	}

	session := &CodexBrowserAuthorization{
		AuthURL:     BuildCodexAuthorizeURL(codexOAuthRedirectURI, pkce, state),
		RedirectURI: codexOAuthRedirectURI,
		State:       state,
		results:     results,
		server:      &http.Server{Handler: mux},
		listener:    listener,
	}

	go func() {
		if err := session.server.Serve(listener); err != nil && err != http.ErrServerClosed {
			c.sendBrowserResult(results, BrowserAuthResult{Err: fmt.Errorf("serve browser callback: %w", err)})
		}
	}()

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		_ = session.Close(shutdownCtx)
	}()

	return session, nil
}

func (c *CodexOAuthClient) BrowserCallbackHandler(expectedState, redirectURI, codeVerifier string, results chan<- BrowserAuthResult) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		query := r.URL.Query()
		if errText := firstNonEmpty(query.Get("error_description"), query.Get("error")); errText != "" {
			c.sendBrowserResult(results, BrowserAuthResult{Err: fmt.Errorf("%s", errText)})
			writeCodexHTML(w, http.StatusBadRequest, renderCodexErrorPage(errText))
			return
		}

		if query.Get("state") != expectedState {
			c.sendBrowserResult(results, BrowserAuthResult{Err: fmt.Errorf("state mismatch")})
			writeCodexHTML(w, http.StatusBadRequest, renderCodexErrorPage("Invalid state"))
			return
		}

		code := strings.TrimSpace(query.Get("code"))
		if code == "" {
			c.sendBrowserResult(results, BrowserAuthResult{Err: fmt.Errorf("missing authorization code")})
			writeCodexHTML(w, http.StatusBadRequest, renderCodexErrorPage("Missing authorization code"))
			return
		}

		token, err := c.ExchangeAuthorizationCode(r.Context(), code, redirectURI, codeVerifier)
		if err != nil {
			c.sendBrowserResult(results, BrowserAuthResult{Err: err})
			writeCodexHTML(w, http.StatusInternalServerError, renderCodexErrorPage(err.Error()))
			return
		}

		c.sendBrowserResult(results, BrowserAuthResult{Token: token})
		writeCodexHTML(w, http.StatusOK, codexAuthHTMLSuccess)
	})
}

func (c *CodexOAuthClient) ExchangeAuthorizationCode(ctx context.Context, code, redirectURI, codeVerifier string) (*CodexTokenResponse, error) {
	form := url.Values{}
	form.Set("grant_type", "authorization_code")
	form.Set("code", code)
	form.Set("redirect_uri", redirectURI)
	form.Set("client_id", codexOAuthClientID)
	form.Set("code_verifier", codeVerifier)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, codexOAuthIssuer+"/oauth/token", strings.NewReader(form.Encode()))
	if err != nil {
		return nil, fmt.Errorf("create token request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	var token CodexTokenResponse
	if err := c.doJSON(req, &token); err != nil {
		return nil, err
	}

	accountID, err := ExtractChatGPTAccountID(token.AccessToken, token.IDToken)
	if err != nil {
		return nil, err
	}
	token.AccountID = accountID
	return &token, nil
}

func (c *CodexOAuthClient) InitiateDeviceAuthorization(ctx context.Context) (*CodexDeviceCode, error) {
	body := map[string]string{"client_id": codexOAuthClientID}
	payload, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshal device auth request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, codexOAuthIssuer+"/api/accounts/deviceauth/usercode", bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("create device auth request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	var resp struct {
		DeviceAuthID string `json:"device_auth_id"`
		UserCode     string `json:"user_code"`
		Interval     string `json:"interval"`
	}
	if err := c.doJSON(req, &resp); err != nil {
		return nil, err
	}

	seconds, err := strconv.Atoi(resp.Interval)
	if err != nil || seconds < 1 {
		seconds = 5
	}

	return &CodexDeviceCode{
		DeviceAuthID:    resp.DeviceAuthID,
		UserCode:        resp.UserCode,
		Interval:        time.Duration(seconds) * time.Second,
		VerificationURL: codexOAuthDeviceURL,
	}, nil
}

func (c *CodexOAuthClient) PollDeviceAuthorization(ctx context.Context, dc *CodexDeviceCode) (*CodexTokenResponse, error) {
	for {
		result, pending, err := c.pollDeviceAuthorizationOnce(ctx, dc)
		if err != nil {
			return nil, err
		}
		if pending {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			default:
			}
			c.sleep(dc.Interval)
			continue
		}

		return c.ExchangeAuthorizationCode(ctx, result.AuthorizationCode, codexOAuthDeviceRedirectURI, result.CodeVerifier)
	}
}

type codexDeviceTokenPoll struct {
	AuthorizationCode string `json:"authorization_code"`
	CodeVerifier      string `json:"code_verifier"`
}

func (c *CodexOAuthClient) pollDeviceAuthorizationOnce(ctx context.Context, dc *CodexDeviceCode) (*codexDeviceTokenPoll, bool, error) {
	body := map[string]string{
		"device_auth_id": dc.DeviceAuthID,
		"user_code":      dc.UserCode,
	}
	payload, err := json.Marshal(body)
	if err != nil {
		return nil, false, fmt.Errorf("marshal device poll request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, codexOAuthIssuer+"/api/accounts/deviceauth/token", bytes.NewReader(payload))
	if err != nil {
		return nil, false, fmt.Errorf("create device poll request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, false, fmt.Errorf("do device poll request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusForbidden || resp.StatusCode == http.StatusNotFound {
		return nil, true, nil
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, maxResponseBody))
		return nil, false, fmt.Errorf("device poll failed: %d %s", resp.StatusCode, string(body))
	}

	var result codexDeviceTokenPoll
	if err := json.NewDecoder(io.LimitReader(resp.Body, maxResponseBody)).Decode(&result); err != nil {
		return nil, false, fmt.Errorf("decode device poll response: %w", err)
	}
	return &result, false, nil
}

func (c *CodexOAuthClient) doJSON(req *http.Request, out any) error {
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBody))
	if err != nil {
		return fmt.Errorf("read response body: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("request failed: %d %s", resp.StatusCode, string(body))
	}
	if err := json.Unmarshal(body, out); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}
	return nil
}

func (c *CodexOAuthClient) sendBrowserResult(results chan<- BrowserAuthResult, result BrowserAuthResult) {
	if results == nil {
		return
	}
	results <- result
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func renderCodexErrorPage(message string) string {
	return codexAuthHTMLError + `<p>` + html.EscapeString(message) + `</p>`
}

func writeCodexHTML(w http.ResponseWriter, status int, body string) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	_, _ = io.WriteString(w, body)
}
