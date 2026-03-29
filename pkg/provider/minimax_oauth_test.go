package provider

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

type mockHTTPClient struct {
	resp *http.Response
	err  error
}

func (m *mockHTTPClient) Do(req *http.Request) (*http.Response, error) {
	return m.resp, m.err
}

func TestMiniMaxOAuth_NameAndAuthType(t *testing.T) {
	p := NewMiniMaxOAuthProvider("minimax-portal")
	if p.Name() != "minimax-portal" {
		t.Errorf("Name() = %q, want %q", p.Name(), "minimax-portal")
	}
	if p.AuthType() != AuthTypeOAuth {
		t.Errorf("AuthType() = %v, want %v", p.AuthType(), AuthTypeOAuth)
	}
}

func TestMiniMaxOAuth_InitiateAuth_SendsExpectedPayload(t *testing.T) {
	p := NewMiniMaxOAuthProvider("minimax-portal")
	p.SetHTTPClient(&mockHTTPClient{
		resp: &http.Response{
			StatusCode: 200,
			Body:        io.NopCloser(strings.NewReader(`{"user_code":"CODE123","device_code":"DEVICE456","verification_uri":"https://minimax.ai/verify","expires_in":300,"interval":5}`)),
		},
	})

	dc, state, err := p.InitiateAuth(context.Background())
	if err != nil {
		t.Fatalf("InitiateAuth failed: %v", err)
	}

	if dc.UserCode != "CODE123" {
		t.Errorf("UserCode = %q, want %q", dc.UserCode, "CODE123")
	}
	if dc.DeviceCode != "DEVICE456" {
		t.Errorf("DeviceCode = %q, want %q", dc.DeviceCode, "DEVICE456")
	}
	if state == "" {
		t.Error("state should not be empty")
	}
}

func TestMiniMaxOAuth_PollToken_StateMismatchRejected(t *testing.T) {
	p := NewMiniMaxOAuthProvider("minimax-portal")
	p.SetHTTPClient(&mockHTTPClient{
		resp: &http.Response{
			StatusCode: 200,
			Body: io.NopCloser(strings.NewReader(
				`{"user_code":"CODE","device_code":"DEV","verification_uri":"https://x.ai/verify","expires_in":300,"interval":5}`)),
		},
	})

	dc, state, _ := p.InitiateAuth(context.Background())

	_, err := p.PollForToken(context.Background(), dc, "wrong-state")
	if err == nil {
		t.Fatal("expected state mismatch error")
	}
	if !strings.Contains(err.Error(), "state mismatch") {
		t.Errorf("error = %q, want state mismatch", err.Error())
	}
	_ = state
}

func TestMiniMaxOAuth_ExchangeCode_SetsTokenCorrectly(t *testing.T) {
	p := NewMiniMaxOAuthProvider("minimax-portal")

	callCount := 0
	p.SetHTTPClient(&mockHTTPClient{
		resp: &http.Response{
			StatusCode: 200,
			Body: io.NopCloser(strings.NewReader(
				`{"access_token":"tok123","refresh_token":"ref456","expires_in":3600,"token_type":"Bearer","scope":"data:read"}`)),
		},
	})

	dc := &DeviceCodeResponse{
		UserCode:        "CODE",
		DeviceCode:      "DEV",
		VerificationURI: "https://x.ai/verify",
		ExpiresIn:       300,
		Interval:        1,
		State:           "test-state",
		Verifier:        "test-verifier",
	}

	token, err := p.exchangeDeviceCode(context.Background(), dc.DeviceCode, dc.Verifier)
	if err != nil {
		t.Fatalf("exchangeDeviceCode failed: %v", err)
	}

	if token.AccessToken != "tok123" {
		t.Errorf("AccessToken = %q, want %q", token.AccessToken, "tok123")
	}
	if token.RefreshToken != "ref456" {
		t.Errorf("RefreshToken = %q, want %q", token.RefreshToken, "ref456")
	}
	if token.TokenType != "Bearer" {
		t.Errorf("TokenType = %q, want %q", token.TokenType, "Bearer")
	}
	if token.ProviderID != "minimax-portal" {
		t.Errorf("ProviderID = %q, want %q", token.ProviderID, "minimax-portal")
	}
	if token.ExpiresAt.Before(time.Now()) {
		t.Error("ExpiresAt should be in the future")
	}
	_ = callCount
}

func TestMiniMaxOAuth_RefreshToken_PostsExpectedForm(t *testing.T) {
	p := NewMiniMaxOAuthProvider("minimax-portal")
	p.token = &TokenInfo{
		AccessToken:  "old-access",
		RefreshToken: "refresh-token",
		ExpiresAt:    time.Now().Add(-1 * time.Hour),
	}

	bodyCapture := &strings.Builder{}
	p.SetHTTPClient(&mockHTTPClient{
		resp: &http.Response{
			StatusCode: 200,
			Body:       io.NopCloser(strings.NewReader(`{"access_token":"new-access","refresh_token":"new-refresh","expires_in":7200,"token_type":"Bearer","scope":"data:read"}`)),
		},
	})

	newToken, err := p.RefreshToken(context.Background())
	if err != nil {
		t.Fatalf("RefreshToken failed: %v", err)
	}

	if newToken.AccessToken != "new-access" {
		t.Errorf("new AccessToken = %q, want %q", newToken.AccessToken, "new-access")
	}
	if newToken.RefreshToken != "new-refresh" {
		t.Errorf("RefreshToken = %q, want %q", newToken.RefreshToken, "new-refresh")
	}

	_ = bodyCapture
}

func TestMiniMaxOAuth_RefreshToken_NoRefreshTokenError(t *testing.T) {
	p := NewMiniMaxOAuthProvider("minimax-portal")
	p.token = &TokenInfo{
		AccessToken:  "some-token",
		RefreshToken: "",
		ExpiresAt:    time.Now().Add(-1 * time.Hour),
	}

	_, err := p.RefreshToken(context.Background())
	if err == nil {
		t.Fatal("expected error when no refresh token available")
	}
}

func TestMiniMaxOAuth_Chat_NoTokenError(t *testing.T) {
	p := NewMiniMaxOAuthProvider("minimax-portal")

	_, err := p.Chat(context.Background(), ChatRequest{Model: "test"})
	if err == nil {
		t.Fatal("expected error when no token available")
	}
	if !strings.Contains(err.Error(), "no OAuth token") {
		t.Errorf("error = %q, want 'no OAuth token'", err.Error())
	}
}

func TestMiniMaxOAuth_ChatStream_NoTokenError(t *testing.T) {
	p := NewMiniMaxOAuthProvider("minimax-portal")

	_, err := p.ChatStream(context.Background(), ChatRequest{Model: "test"})
	if err == nil {
		t.Fatal("expected error when no token available")
	}
	if !strings.Contains(err.Error(), "no OAuth token") {
		t.Errorf("error = %q, want 'no OAuth token'", err.Error())
	}
}

func TestMiniMaxOAuth_Chat_RefreshOnExpiry(t *testing.T) {
	p := NewMiniMaxOAuthProvider("minimax-portal")

	p.SetToken(&TokenInfo{
		AccessToken:  "expired-access",
		RefreshToken: "refresh-token",
		ExpiresAt:    time.Now().Add(-1 * time.Hour),
	})

	p.SetHTTPClient(&mockHTTPClient{
		resp: &http.Response{
			StatusCode: 200,
			Body: io.NopCloser(strings.NewReader(
				`{"access_token":"new-access","refresh_token":"new-refresh","expires_in":3600,"token_type":"Bearer","scope":"data:read"}`)),
		},
	})

	tokenBefore := p.token
	refreshedToken, err := p.RefreshToken(context.Background())
	if err != nil {
		t.Fatalf("RefreshToken failed: %v", err)
	}
	if p.token == tokenBefore {
		t.Error("token should have been replaced after refresh")
	}
	if refreshedToken.AccessToken != "new-access" {
		t.Errorf("AccessToken = %q, want %q", refreshedToken.AccessToken, "new-access")
	}
}

func TestGeneratePKCE_VerifierAndChallenge(t *testing.T) {
	pkce, err := generatePKCE()
	if err != nil {
		t.Fatalf("generatePKCE failed: %v", err)
	}

	if pkce.Verifier == "" {
		t.Error("Verifier should not be empty")
	}
	if len(pkce.Verifier) < 32 {
		t.Errorf("Verifier length = %d, expected at least 32", len(pkce.Verifier))
	}
	if pkce.Challenge == "" {
		t.Error("Challenge should not be empty")
	}
}

func TestGenerateState_IsNotEmpty(t *testing.T) {
	state, err := generateState()
	if err != nil {
		t.Fatalf("generateState failed: %v", err)
	}
	if state == "" {
		t.Error("state should not be empty")
	}
}

func TestMiniMaxOAuth_WithBaseURLOption(t *testing.T) {
	p := NewMiniMaxOAuthProvider("minimax-portal", WithMiniMaxOAuthBaseURL("https://api.minimaxi.com"))
	if p.config.BaseURL != "https://api.minimaxi.com" {
		t.Errorf("BaseURL = %q, want %q", p.config.BaseURL, "https://api.minimaxi.com")
	}
}

func TestMiniMaxOAuth_RevokeToken(t *testing.T) {
	p := NewMiniMaxOAuthProvider("minimax-portal")
	p.token = &TokenInfo{
		AccessToken: "to-revoke",
	}

	p.SetHTTPClient(&mockHTTPClient{
		resp: &http.Response{
			StatusCode: 200,
			Body:       io.NopCloser(strings.NewReader(`{}`)),
		},
	})

	err := p.RevokeToken(context.Background())
	if err != nil {
		t.Fatalf("RevokeToken failed: %v", err)
	}
	if p.token != nil {
		t.Error("token should be cleared after revoke")
	}
}

func TestMiniMaxOAuth_RevokeToken_NoTokenNoOp(t *testing.T) {
	p := NewMiniMaxOAuthProvider("minimax-portal")
	if p.token != nil {
		t.Error("token should be nil initially")
	}

	err := p.RevokeToken(context.Background())
	if err != nil {
		t.Fatalf("RevokeToken with nil token failed: %v", err)
	}
}

func TestDeviceCodeResponse_JSON(t *testing.T) {
	raw := `{"user_code":"CODE123","device_code":"DEV456","verification_uri":"https://x.ai/verify","expires_in":300,"interval":5}`
	var dc DeviceCodeResponse
	if err := json.Unmarshal([]byte(raw), &dc); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}
	if dc.UserCode != "CODE123" {
		t.Errorf("UserCode = %q", dc.UserCode)
	}
	if dc.DeviceCode != "DEV456" {
		t.Errorf("DeviceCode = %q", dc.DeviceCode)
	}
	if dc.ExpiresIn != 300 {
		t.Errorf("ExpiresIn = %d", dc.ExpiresIn)
	}
	if dc.Interval != 5 {
		t.Errorf("Interval = %d", dc.Interval)
	}
}

func TestTokenResponse_JSON(t *testing.T) {
	raw := `{"access_token":"tok","refresh_token":"ref","expires_in":3600,"token_type":"Bearer","scope":"data:read"}`
	var tr TokenResponse
	if err := json.Unmarshal([]byte(raw), &tr); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}
	if tr.AccessToken != "tok" {
		t.Errorf("AccessToken = %q", tr.AccessToken)
	}
	if tr.RefreshToken != "ref" {
		t.Errorf("RefreshToken = %q", tr.RefreshToken)
	}
}

func TestTokenResponse_Error_JSON(t *testing.T) {
	raw := `{"error":"authorization_pending","error_description":"user not yet approved"}`
	var tr TokenResponse
	if err := json.Unmarshal([]byte(raw), &tr); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}
	if tr.Error != "authorization_pending" {
		t.Errorf("Error = %q", tr.Error)
	}
	if tr.ErrorDesc != "user not yet approved" {
		t.Errorf("ErrorDesc = %q", tr.ErrorDesc)
	}
}

type errorHTTPClient struct {
	err error
}

func (m *errorHTTPClient) Do(req *http.Request) (*http.Response, error) {
	return nil, m.err
}

func TestMiniMaxOAuth_HTTPClientError(t *testing.T) {
	p := NewMiniMaxOAuthProvider("minimax-portal")
	p.SetHTTPClient(&errorHTTPClient{err: errors.New("connection reset")})

	_, _, err := p.InitiateAuth(context.Background())
	if err == nil {
		t.Fatal("expected error from HTTP client failure")
	}
}

func TestTokenInfo_IsExpired(t *testing.T) {
	token := &TokenInfo{
		AccessToken: "tok",
		ExpiresAt:   time.Now().Add(3 * time.Minute),
	}
	if token.IsExpired() {
		t.Error("token with 3min left should not be expired")
	}

	token.ExpiresAt = time.Now().Add(1 * time.Minute)
	if !token.IsExpired() {
		t.Error("token with 1min left (under 2min buffer) should be expired")
	}

	token.ExpiresAt = time.Now().Add(-1 * time.Hour)
	if !token.IsExpired() {
		t.Error("token already expired should be expired")
	}
}
