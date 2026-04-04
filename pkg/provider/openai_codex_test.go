package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestOpenAICodexProviderName(t *testing.T) {
	p := NewOpenAICodexProvider("openai-codex")
	if got := p.Name(); got != "openai-codex" {
		t.Errorf("Name() = %q, want %q", got, "openai-codex")
	}
	if got := p.AuthType(); got != AuthTypeOAuth {
		t.Errorf("AuthType() = %v, want %v", got, AuthTypeOAuth)
	}
}

func TestOpenAICodexProviderSetGetToken(t *testing.T) {
	p := NewOpenAICodexProvider("openai-codex")

	if tok := p.GetToken(); tok != nil {
		t.Fatalf("expected nil token initially, got %+v", tok)
	}

	token := &TokenInfo{
		AccessToken:  "test-access",
		RefreshToken: "test-refresh",
		ExpiresAt:    time.Now().Add(time.Hour),
	}
	p.SetToken(token)

	got := p.GetToken()
	if got == nil || got.AccessToken != "test-access" {
		t.Fatalf("GetToken() = %+v, want token with AccessToken=test-access", got)
	}
}

func TestOpenAICodexProviderRefreshTokenNoRefreshToken(t *testing.T) {
	p := NewOpenAICodexProvider("openai-codex")
	_, err := p.RefreshToken(context.Background())
	if err == nil || !strings.Contains(err.Error(), "no refresh token") {
		t.Errorf("expected 'no refresh token' error, got: %v", err)
	}
}

func TestOpenAICodexProviderRefreshToken(t *testing.T) {
	p := NewOpenAICodexProvider("openai-codex")
	p.now = func() time.Time { return time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC) }

	p.SetToken(&TokenInfo{
		AccessToken:  "old-access",
		RefreshToken: "test-refresh",
		ExpiresAt:    time.Now().Add(-time.Hour),
	})

	p.httpClient = &roundTripFunc{fn: func(req *http.Request) (*http.Response, error) {
		if req.URL.Path != "/oauth/token" {
			return nil, fmt.Errorf("unexpected path: %s", req.URL.Path)
		}
		body, _ := io.ReadAll(req.Body)
		if !strings.Contains(string(body), "grant_type=refresh_token") {
			return nil, fmt.Errorf("expected refresh_token grant_type, got: %s", body)
		}
		if !strings.Contains(string(body), "refresh_token=test-refresh") {
			return nil, fmt.Errorf("expected refresh_token=test-refresh, got: %s", body)
		}

		resp := &CodexTokenResponse{
			AccessToken:  "new-access",
			RefreshToken: "new-refresh",
			ExpiresIn:    3600,
			TokenType:    "Bearer",
		}
		data, _ := json.Marshal(resp)
		return &http.Response{
			StatusCode: 200,
			Body:       io.NopCloser(strings.NewReader(string(data))),
		}, nil
	}}

	tok, err := p.RefreshToken(context.Background())
	if err != nil {
		t.Fatalf("RefreshToken() error: %v", err)
	}
	if tok.AccessToken != "new-access" {
		t.Errorf("AccessToken = %q, want %q", tok.AccessToken, "new-access")
	}
	if tok.RefreshToken != "new-refresh" {
		t.Errorf("RefreshToken = %q, want %q", tok.RefreshToken, "new-refresh")
	}
}

func TestOpenAICodexProviderRevokeToken(t *testing.T) {
	p := NewOpenAICodexProvider("openai-codex")
	p.SetToken(&TokenInfo{AccessToken: "tok"})
	if err := p.RevokeToken(context.Background()); err != nil {
		t.Fatalf("RevokeToken() error: %v", err)
	}
	if tok := p.GetToken(); tok != nil {
		t.Errorf("expected nil token after revoke, got %+v", tok)
	}
}

func TestBuildCodexRequest(t *testing.T) {
	p := NewOpenAICodexProvider("openai-codex")

	req := ChatRequest{
		Model: "gpt-5.2-codex",
		Messages: []Message{
			{Role: "system", Content: "You are a helpful assistant"},
			{Role: "user", Content: "Hello"},
			{Role: "tool", Content: "result data", ToolCallID: "call_123"},
		},
		Tools: []ToolDef{
			{Name: "read_file", Description: "Read a file", Parameters: map[string]any{"type": "object"}},
		},
		MaxTokens: 4096,
	}

	cr := p.buildCodexRequest(req)

	if cr.Model != "gpt-5.2-codex" {
		t.Errorf("Model = %q, want %q", cr.Model, "gpt-5.2-codex")
	}
	if cr.MaxTokens != 4096 {
		t.Errorf("MaxTokens = %d, want %d", cr.MaxTokens, 4096)
	}
	if len(cr.Input) != 3 {
		t.Fatalf("len(Input) = %d, want 3", len(cr.Input))
	}
	if cr.Input[0].Role != "developer" {
		t.Errorf("Input[0].Role = %q, want %q", cr.Input[0].Role, "developer")
	}
	if cr.Input[1].Role != "user" {
		t.Errorf("Input[1].Role = %q, want %q", cr.Input[1].Role, "user")
	}
	if cr.Input[2].Role != "tool" {
		t.Errorf("Input[2].Role = %q, want %q", cr.Input[2].Role, "tool")
	}
	if !strings.Contains(cr.Input[2].Content, "call_123") {
		t.Errorf("tool input should contain call_id, got: %q", cr.Input[2].Content)
	}
	if len(cr.Tools) != 1 {
		t.Fatalf("len(Tools) = %d, want 1", len(cr.Tools))
	}
	if cr.Tools[0].Function.Name != "read_file" {
		t.Errorf("Tools[0].Function.Name = %q, want %q", cr.Tools[0].Function.Name, "read_file")
	}
}

func TestSetCodexHeaders(t *testing.T) {
	p := NewOpenAICodexProvider("openai-codex")
	p.accountID = "acct-123"

	req, _ := http.NewRequest("POST", "http://example.com", nil)
	p.setCodexHeaders(req, "test-token")

	if got := req.Header.Get("Authorization"); got != "Bearer test-token" {
		t.Errorf("Authorization = %q, want %q", got, "Bearer test-token")
	}
	if got := req.Header.Get("ChatGPT-Account-Id"); got != "acct-123" {
		t.Errorf("ChatGPT-Account-Id = %q, want %q", got, "acct-123")
	}
	if got := req.Header.Get("OpenAI-Beta"); got != "responses=experimental" {
		t.Errorf("OpenAI-Beta = %q, want %q", got, "responses=experimental")
	}
	if got := req.Header.Get("originator"); got != "smolbot" {
		t.Errorf("originator = %q, want %q", got, "smolbot")
	}
}

func TestParseSSEToResponse(t *testing.T) {
	p := NewOpenAICodexProvider("openai-codex")

	sseData := `data: {"type":"response.output_text.delta","delta":"Hello "}
data: {"type":"response.output_text.delta","delta":"world"}
data: {"type":"response.completed","response":{"status":"completed","output":[]}}
data: [DONE]
`
	resp, err := p.parseSSEToResponse(strings.NewReader(sseData))
	if err != nil {
		t.Fatalf("parseSSEToResponse() error: %v", err)
	}
	if resp.Content != "Hello world" {
		t.Errorf("Content = %q, want %q", resp.Content, "Hello world")
	}
	if resp.FinishReason != "completed" {
		t.Errorf("FinishReason = %q, want %q", resp.FinishReason, "completed")
	}
}

func TestParseSSEToResponseWithToolCalls(t *testing.T) {
	p := NewOpenAICodexProvider("openai-codex")

	sseData := `data: {"type":"response.completed","response":{"status":"completed","output":[{"type":"function_call","call_id":"call_abc","name":"read_file","arguments":"{\"path\":\"test.go\"}"}]}}
data: [DONE]
`
	resp, err := p.parseSSEToResponse(strings.NewReader(sseData))
	if err != nil {
		t.Fatalf("parseSSEToResponse() error: %v", err)
	}
	if len(resp.ToolCalls) != 1 {
		t.Fatalf("len(ToolCalls) = %d, want 1", len(resp.ToolCalls))
	}
	if resp.ToolCalls[0].ID != "call_abc" {
		t.Errorf("ToolCalls[0].ID = %q, want %q", resp.ToolCalls[0].ID, "call_abc")
	}
	if resp.ToolCalls[0].Function.Name != "read_file" {
		t.Errorf("ToolCalls[0].Function.Name = %q, want %q", resp.ToolCalls[0].Function.Name, "read_file")
	}
}

func TestNewCodexStream(t *testing.T) {
	p := NewOpenAICodexProvider("openai-codex")

	sseData := `data: {"type":"response.output_text.delta","delta":"Hi"}
data: {"type":"response.output_text.delta","delta":" there"}
data: {"type":"response.completed","response":{"status":"completed"}}
data: [DONE]
`
	body := io.NopCloser(strings.NewReader(sseData))
	stream := p.newCodexStream(body)

	var content strings.Builder
	for {
		delta, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("Recv() error: %v", err)
		}
		content.WriteString(delta.Content)
	}

	if got := content.String(); got != "Hi there" {
		t.Errorf("streamed content = %q, want %q", got, "Hi there")
	}
}

func TestNewCodexStreamToolCalls(t *testing.T) {
	p := NewOpenAICodexProvider("openai-codex")

	sseData := `data: {"type":"response.function_call_arguments.delta","call_id":"call_1","name":"read_file","delta":"{\"path\":"}
data: {"type":"response.function_call_arguments.delta","call_id":"","name":"","delta":"\"test.go\"}"}
data: {"type":"response.output_item.done"}
data: {"type":"response.completed","response":{"status":"completed"}}
data: [DONE]
`
	body := io.NopCloser(strings.NewReader(sseData))
	stream := p.newCodexStream(body)

	var toolCalls []ToolCall
	for {
		delta, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("Recv() error: %v", err)
		}
		toolCalls = append(toolCalls, delta.ToolCalls...)
	}

	if len(toolCalls) < 1 {
		t.Fatalf("expected at least 1 tool call delta, got %d", len(toolCalls))
	}
	if toolCalls[0].ID != "call_1" {
		t.Errorf("first tool call ID = %q, want %q", toolCalls[0].ID, "call_1")
	}
}

func TestEnsureValidTokenRefreshesExpired(t *testing.T) {
	p := NewOpenAICodexProvider("openai-codex")
	p.now = func() time.Time { return time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC) }

	p.SetToken(&TokenInfo{
		AccessToken:  "expired",
		RefreshToken: "refresh-tok",
		ExpiresAt:    time.Now().Add(-time.Hour),
	})

	p.httpClient = &roundTripFunc{fn: func(req *http.Request) (*http.Response, error) {
		resp := &CodexTokenResponse{
			AccessToken:  "fresh-access",
			RefreshToken: "fresh-refresh",
			ExpiresIn:    3600,
			TokenType:    "Bearer",
		}
		data, _ := json.Marshal(resp)
		return &http.Response{
			StatusCode: 200,
			Body:       io.NopCloser(strings.NewReader(string(data))),
		}, nil
	}}

	tok, err := p.ensureValidToken(context.Background())
	if err != nil {
		t.Fatalf("ensureValidToken() error: %v", err)
	}
	if tok.AccessToken != "fresh-access" {
		t.Errorf("AccessToken = %q, want %q", tok.AccessToken, "fresh-access")
	}
}

func TestEnsureValidTokenReturnsValidToken(t *testing.T) {
	p := NewOpenAICodexProvider("openai-codex")

	validToken := &TokenInfo{
		AccessToken: "valid-access",
		ExpiresAt:   time.Now().Add(time.Hour),
	}
	p.SetToken(validToken)

	tok, err := p.ensureValidToken(context.Background())
	if err != nil {
		t.Fatalf("ensureValidToken() error: %v", err)
	}
	if tok.AccessToken != "valid-access" {
		t.Errorf("AccessToken = %q, want %q", tok.AccessToken, "valid-access")
	}
}

func TestGetAuthConfig(t *testing.T) {
	p := NewOpenAICodexProvider("openai-codex")
	cfg := p.GetAuthConfig()

	if cfg.ClientID != codexOAuthClientID {
		t.Errorf("ClientID = %q, want %q", cfg.ClientID, codexOAuthClientID)
	}
	if len(cfg.Scopes) == 0 {
		t.Error("Scopes should not be empty")
	}
}

// roundTripFunc wraps a function as an http.RoundTripper for test HTTP mocking.
type roundTripFunc struct {
	fn func(*http.Request) (*http.Response, error)
}

func (f *roundTripFunc) Do(req *http.Request) (*http.Response, error) {
	return f.fn(req)
}
