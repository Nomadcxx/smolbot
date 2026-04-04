package provider

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"strings"
	"testing"
	"time"
)

type codexTestHTTPClient struct {
	do func(req *http.Request) (*http.Response, error)
}

func (c *codexTestHTTPClient) Do(req *http.Request) (*http.Response, error) {
	return c.do(req)
}

func makeJWT(t *testing.T, payload map[string]any) string {
	t.Helper()
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"none","typ":"JWT"}`))
	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	return header + "." + base64.RawURLEncoding.EncodeToString(body) + ".sig"
}

func TestExtractChatGPTAccountIDPrefersIDToken(t *testing.T) {
	idToken := makeJWT(t, map[string]any{
		"https://api.openai.com/auth": map[string]any{
			"chatgpt_account_id": "acct-from-id-token",
		},
	})
	accessToken := makeJWT(t, map[string]any{
		"chatgpt_account_id": "acct-from-access-token",
	})

	accountID, err := ExtractChatGPTAccountID(accessToken, idToken)
	if err != nil {
		t.Fatalf("ExtractChatGPTAccountID() error = %v", err)
	}
	if accountID != "acct-from-id-token" {
		t.Fatalf("ExtractChatGPTAccountID() = %q, want %q", accountID, "acct-from-id-token")
	}
}

func TestExtractChatGPTAccountIDFallsBackToAccessTokenWhenIDTokenInvalid(t *testing.T) {
	accessToken := makeJWT(t, map[string]any{
		"chatgpt_account_id": "acct-from-access-token",
	})

	accountID, err := ExtractChatGPTAccountID(accessToken, "not-a-jwt")
	if err != nil {
		t.Fatalf("ExtractChatGPTAccountID() error = %v", err)
	}
	if accountID != "acct-from-access-token" {
		t.Fatalf("ExtractChatGPTAccountID() = %q, want %q", accountID, "acct-from-access-token")
	}
}

func TestParseJWTClaimsFallsBackToOrganizations(t *testing.T) {
	token := makeJWT(t, map[string]any{
		"organizations": []map[string]any{
			{"id": "org-first"},
		},
	})

	claims, err := ParseJWTClaims(token)
	if err != nil {
		t.Fatalf("ParseJWTClaims() error = %v", err)
	}
	if got := claims.AccountID(); got != "org-first" {
		t.Fatalf("claims.AccountID() = %q, want %q", got, "org-first")
	}
}

func TestCodexOAuthClientStartsBrowserFlowEndToEnd(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:1455")
	if err != nil {
		t.Skipf("callback port unavailable: %v", err)
	}
	_ = listener.Close()

	client := NewCodexOAuthClient()
	client.SetHTTPClient(&codexTestHTTPClient{
		do: func(req *http.Request) (*http.Response, error) {
			if req.URL.String() != "https://auth.openai.com/oauth/token" {
				t.Fatalf("token url = %q", req.URL.String())
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Body: io.NopCloser(strings.NewReader(`{
					"id_token":"` + makeJWT(t, map[string]any{"chatgpt_account_id": "acct-browser"}) + `",
					"access_token":"access-browser",
					"refresh_token":"refresh-browser",
					"expires_in":3600,
					"token_type":"Bearer",
					"scope":"openid profile email offline_access"
				}`)),
			}, nil
		},
	})

	method := reflect.ValueOf(client).MethodByName("StartBrowserAuthorization")
	if !method.IsValid() {
		t.Fatal("StartBrowserAuthorization is missing")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	values := method.Call([]reflect.Value{reflect.ValueOf(ctx)})
	if !values[1].IsNil() {
		t.Fatalf("StartBrowserAuthorization() error = %v", values[1].Interface())
	}

	session := values[0]
	authURL := session.Elem().FieldByName("AuthURL").String()
	state := session.Elem().FieldByName("State").String()

	parsed, err := url.Parse(authURL)
	if err != nil {
		t.Fatalf("url.Parse() error = %v", err)
	}
	if parsed.Host != "auth.openai.com" {
		t.Fatalf("authorize host = %q, want auth.openai.com", parsed.Host)
	}

	resp, err := http.Get("http://localhost:1455/auth/callback?code=browser-code&state=" + url.QueryEscape(state))
	if err != nil {
		t.Fatalf("http.Get() error = %v", err)
	}
	_ = resp.Body.Close()

	waitMethod := session.MethodByName("Wait")
	if !waitMethod.IsValid() {
		t.Fatal("Wait method is missing")
	}
	waitValues := waitMethod.Call([]reflect.Value{reflect.ValueOf(ctx)})
	if !waitValues[1].IsNil() {
		t.Fatalf("Wait() error = %v", waitValues[1].Interface())
	}
	token := waitValues[0].Interface().(*CodexTokenResponse)
	if token.AccessToken != "access-browser" {
		t.Fatalf("AccessToken = %q, want access-browser", token.AccessToken)
	}
	if token.AccountID != "acct-browser" {
		t.Fatalf("AccountID = %q, want acct-browser", token.AccountID)
	}
}

func TestBuildCodexAuthorizeURLIncludesRequiredParameters(t *testing.T) {
	pkce := &PKCEParams{
		Verifier:  "verifier",
		Challenge: "challenge",
		Method:    "S256",
	}

	rawURL := BuildCodexAuthorizeURL("http://localhost:1455/auth/callback", pkce, "state-123")
	parsed, err := url.Parse(rawURL)
	if err != nil {
		t.Fatalf("url.Parse() error = %v", err)
	}

	q := parsed.Query()
	if parsed.Host != "auth.openai.com" {
		t.Fatalf("authorize host = %q, want auth.openai.com", parsed.Host)
	}
	if got := q.Get("client_id"); got != codexOAuthClientID {
		t.Fatalf("client_id = %q, want %q", got, codexOAuthClientID)
	}
	if got := q.Get("scope"); got != "openid profile email offline_access" {
		t.Fatalf("scope = %q", got)
	}
	if got := q.Get("originator"); got != "smolbot" {
		t.Fatalf("originator = %q, want smolbot", got)
	}
	if got := q.Get("codex_cli_simplified_flow"); got != "true" {
		t.Fatalf("codex_cli_simplified_flow = %q, want true", got)
	}
}

func TestCodexCallbackHandlerRejectsStateMismatch(t *testing.T) {
	client := NewCodexOAuthClient()
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/auth/callback?code=abc&state=wrong", nil)
	results := make(chan BrowserAuthResult, 1)

	client.BrowserCallbackHandler("expected-state", "http://localhost:1455/auth/callback", "verifier", results).ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
	if got := rec.Header().Get("Content-Type"); !strings.Contains(got, "text/html") {
		t.Fatalf("content-type = %q, want text/html", got)
	}
	if body := rec.Body.String(); !strings.Contains(body, "<!doctype html>") {
		t.Fatalf("body = %q, want html error page", body)
	}
	select {
	case result := <-results:
		if result.Err == nil || !strings.Contains(result.Err.Error(), "state") {
			t.Fatalf("result.Err = %v, want state error", result.Err)
		}
	default:
		t.Fatal("expected callback result")
	}
}

func TestSendBrowserResultWaitsForReceiver(t *testing.T) {
	client := NewCodexOAuthClient()
	results := make(chan BrowserAuthResult, 1)
	results <- BrowserAuthResult{Err: errors.New("first")}
	done := make(chan struct{})

	go func() {
		client.sendBrowserResult(results, BrowserAuthResult{Err: errors.New("second")})
		close(done)
	}()

	select {
	case <-done:
		t.Fatal("sendBrowserResult returned before the queue had room")
	case <-time.After(20 * time.Millisecond):
	}

	select {
	case result := <-results:
		if result.Err == nil || result.Err.Error() != "first" {
			t.Fatalf("result.Err = %v, want first", result.Err)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for result")
	}

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("sendBrowserResult did not unblock after receiver read the result")
	}

	select {
	case result := <-results:
		if result.Err == nil || result.Err.Error() != "second" {
			t.Fatalf("result.Err = %v, want second", result.Err)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for queued result")
	}
}

func TestCodexCallbackHandlerExchangesCodeForTokens(t *testing.T) {
	client := NewCodexOAuthClient()
	client.SetHTTPClient(&codexTestHTTPClient{
		do: func(req *http.Request) (*http.Response, error) {
			if req.URL.String() != "https://auth.openai.com/oauth/token" {
				t.Fatalf("token url = %q", req.URL.String())
			}
			body, err := io.ReadAll(req.Body)
			if err != nil {
				t.Fatalf("ReadAll() error = %v", err)
			}
			form, err := url.ParseQuery(string(body))
			if err != nil {
				t.Fatalf("ParseQuery() error = %v", err)
			}
			if got := form.Get("code"); got != "code-123" {
				t.Fatalf("code = %q, want code-123", got)
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Body: io.NopCloser(strings.NewReader(`{
					"id_token":"` + makeJWT(t, map[string]any{"chatgpt_account_id": "acct-123"}) + `",
					"access_token":"access-123",
					"refresh_token":"refresh-123",
					"expires_in":3600,
					"token_type":"Bearer",
					"scope":"openid profile email offline_access"
				}`)),
			}, nil
		},
	})
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/auth/callback?code=code-123&state=expected-state", nil)
	results := make(chan BrowserAuthResult, 1)

	client.BrowserCallbackHandler("expected-state", "http://localhost:1455/auth/callback", "verifier-123", results).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	select {
	case result := <-results:
		if result.Err != nil {
			t.Fatalf("result.Err = %v", result.Err)
		}
		if result.Token.AccessToken != "access-123" {
			t.Fatalf("access token = %q, want access-123", result.Token.AccessToken)
		}
		if result.Token.AccountID != "acct-123" {
			t.Fatalf("account id = %q, want acct-123", result.Token.AccountID)
		}
	default:
		t.Fatal("expected callback result")
	}
}

func TestCodexInitiateDeviceAuthorization(t *testing.T) {
	client := NewCodexOAuthClient()
	client.SetHTTPClient(&codexTestHTTPClient{
		do: func(req *http.Request) (*http.Response, error) {
			if req.URL.String() != "https://auth.openai.com/api/accounts/deviceauth/usercode" {
				t.Fatalf("device auth url = %q", req.URL.String())
			}
			body, err := io.ReadAll(req.Body)
			if err != nil {
				t.Fatalf("ReadAll() error = %v", err)
			}
			if strings.TrimSpace(string(body)) != `{"client_id":"`+codexOAuthClientID+`"}` {
				t.Fatalf("request body = %s", string(body))
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(`{"device_auth_id":"dev-auth-1","user_code":"ABCD-EFGH","interval":"1"}`)),
			}, nil
		},
	})

	deviceCode, err := client.InitiateDeviceAuthorization(context.Background())
	if err != nil {
		t.Fatalf("InitiateDeviceAuthorization() error = %v", err)
	}
	if deviceCode.DeviceAuthID != "dev-auth-1" {
		t.Fatalf("DeviceAuthID = %q, want dev-auth-1", deviceCode.DeviceAuthID)
	}
	if deviceCode.VerificationURL != "https://auth.openai.com/codex/device" {
		t.Fatalf("VerificationURL = %q", deviceCode.VerificationURL)
	}
}

func TestCodexPollDeviceAuthorizationExchangesAuthorizationCode(t *testing.T) {
	client := NewCodexOAuthClient()
	client.sleep = func(time.Duration) {}
	callCount := 0
	client.SetHTTPClient(&codexTestHTTPClient{
		do: func(req *http.Request) (*http.Response, error) {
			callCount++
			switch req.URL.String() {
			case "https://auth.openai.com/api/accounts/deviceauth/token":
				if callCount == 1 {
					return &http.Response{
						StatusCode: http.StatusForbidden,
						Body:       io.NopCloser(strings.NewReader(`{"status":"pending"}`)),
					}, nil
				}
				return &http.Response{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(strings.NewReader(`{"authorization_code":"auth-code-1","code_verifier":"verifier-1"}`)),
				}, nil
			case "https://auth.openai.com/oauth/token":
				return &http.Response{
					StatusCode: http.StatusOK,
					Body: io.NopCloser(strings.NewReader(`{
						"id_token":"` + makeJWT(t, map[string]any{"chatgpt_account_id": "acct-789"}) + `",
						"access_token":"access-789",
						"refresh_token":"refresh-789",
						"expires_in":7200,
						"token_type":"Bearer",
						"scope":"openid profile email offline_access"
					}`)),
				}, nil
			default:
				t.Fatalf("unexpected url %q", req.URL.String())
				return nil, nil
			}
		},
	})

	token, err := client.PollDeviceAuthorization(context.Background(), &CodexDeviceCode{
		DeviceAuthID: "dev-auth-1",
		UserCode:     "ABCD-EFGH",
		Interval:     time.Millisecond,
	})
	if err != nil {
		t.Fatalf("PollDeviceAuthorization() error = %v", err)
	}
	if token.AccessToken != "access-789" {
		t.Fatalf("AccessToken = %q, want access-789", token.AccessToken)
	}
	if token.AccountID != "acct-789" {
		t.Fatalf("AccountID = %q, want acct-789", token.AccountID)
	}
}
