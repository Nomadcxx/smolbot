package provider

import (
	"bufio"
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

const (
	codexAPIBase     = "https://chatgpt.com/backend-api"
	codexAPIEndpoint = codexAPIBase + "/codex/responses"
)

// OpenAICodexProvider implements the OAuthProvider interface for OpenAI Codex
// (ChatGPT Plus/Pro subscriptions). Requests are routed to the Codex backend
// at chatgpt.com instead of the standard api.openai.com endpoint.
type OpenAICodexProvider struct {
	provider   string
	token      *TokenInfo
	accountID  string
	mu         sync.Mutex
	httpClient HTTPClient
	authClient *CodexOAuthClient
	now        func() time.Time
}

func NewOpenAICodexProvider(providerName string) *OpenAICodexProvider {
	return &OpenAICodexProvider{
		provider:   providerName,
		httpClient: http.DefaultClient,
		authClient: NewCodexOAuthClient(),
		now:        time.Now,
	}
}

func (p *OpenAICodexProvider) Name() string      { return p.provider }
func (p *OpenAICodexProvider) AuthType() AuthType { return AuthTypeOAuth }

func (p *OpenAICodexProvider) GetAuthConfig() OAuthConfig {
	return OAuthConfig{
		BaseURL:  codexOAuthIssuer,
		ClientID: codexOAuthClientID,
		AuthURL:  "/oauth/authorize",
		TokenURL: "/oauth/token",
		Scopes:   strings.Split(codexOAuthScope, " "),
	}
}

func (p *OpenAICodexProvider) SetToken(t *TokenInfo) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.token = t
	if t != nil && t.AccessToken != "" {
		if accountID, _ := ExtractChatGPTAccountID(t.AccessToken, ""); accountID != "" {
			p.accountID = accountID
		}
	}
}

func (p *OpenAICodexProvider) GetToken() *TokenInfo {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.token
}

func (p *OpenAICodexProvider) SetHTTPClient(c HTTPClient) {
	p.httpClient = c
	p.authClient.SetHTTPClient(c)
}

// RefreshToken exchanges the stored refresh token for a new access token.
func (p *OpenAICodexProvider) RefreshToken(ctx context.Context) (*TokenInfo, error) {
	p.mu.Lock()
	tok := p.token
	p.mu.Unlock()
	if tok == nil || tok.RefreshToken == "" {
		return nil, fmt.Errorf("no refresh token available — run: smolbot auth codex")
	}

	form := url.Values{}
	form.Set("grant_type", "refresh_token")
	form.Set("refresh_token", tok.RefreshToken)
	form.Set("client_id", codexOAuthClientID)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, codexOAuthIssuer+"/oauth/token", strings.NewReader(form.Encode()))
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
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("codex token refresh failed: %d %s", resp.StatusCode, string(body))
	}

	var tokenResp CodexTokenResponse
	if err := json.Unmarshal(body, &tokenResp); err != nil {
		return nil, fmt.Errorf("decode refresh response: %w", err)
	}

	newToken := tokenResp.TokenInfo(p.now())
	if newToken.RefreshToken == "" {
		newToken.RefreshToken = tok.RefreshToken
	}

	accountID, _ := ExtractChatGPTAccountID(tokenResp.AccessToken, tokenResp.IDToken)

	p.mu.Lock()
	p.token = newToken
	if accountID != "" {
		p.accountID = accountID
	}
	p.mu.Unlock()
	return newToken, nil
}

func (p *OpenAICodexProvider) RevokeToken(_ context.Context) error {
	p.mu.Lock()
	p.token = nil
	p.accountID = ""
	p.mu.Unlock()
	return nil
}

func (p *OpenAICodexProvider) ensureValidToken(ctx context.Context) (*TokenInfo, error) {
	p.mu.Lock()
	tok := p.token
	p.mu.Unlock()
	if tok == nil || tok.IsExpired() {
		if tok != nil && tok.RefreshToken != "" {
			refreshed, err := p.RefreshToken(ctx)
			if err != nil {
				return nil, fmt.Errorf("codex token expired and refresh failed: %w", err)
			}
			return refreshed, nil
		}
		return nil, fmt.Errorf("no Codex OAuth token — run: smolbot auth codex")
	}
	if time.Until(tok.ExpiresAt) < 5*time.Minute && tok.RefreshToken != "" {
		if refreshed, err := p.RefreshToken(ctx); err == nil {
			return refreshed, nil
		}
		return tok, nil
	}
	return tok, nil
}

// Chat sends a non-streaming request to the Codex backend.
func (p *OpenAICodexProvider) Chat(ctx context.Context, req ChatRequest) (*Response, error) {
	tok, err := p.ensureValidToken(ctx)
	if err != nil {
		return nil, err
	}
	req.Model = StripProviderPrefix(req.Model)
	codexReq := p.buildCodexRequest(req)

	payload, err := json.Marshal(codexReq)
	if err != nil {
		return nil, fmt.Errorf("marshal codex request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, codexAPIEndpoint, strings.NewReader(string(payload)))
	if err != nil {
		return nil, fmt.Errorf("create codex request: %w", err)
	}
	p.setCodexHeaders(httpReq, tok.AccessToken)

	httpResp, err := p.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("do codex request: %w", err)
	}
	defer httpResp.Body.Close()

	if httpResp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(httpResp.Body, maxResponseBody))
		return nil, fmt.Errorf("codex http %d: %s", httpResp.StatusCode, string(body))
	}

	return p.parseSSEToResponse(httpResp.Body)
}

// ChatStream sends a streaming request to the Codex backend.
func (p *OpenAICodexProvider) ChatStream(ctx context.Context, req ChatRequest) (*Stream, error) {
	tok, err := p.ensureValidToken(ctx)
	if err != nil {
		return nil, err
	}
	req.Model = StripProviderPrefix(req.Model)
	codexReq := p.buildCodexRequest(req)
	codexReq.Stream = true

	payload, err := json.Marshal(codexReq)
	if err != nil {
		return nil, fmt.Errorf("marshal codex request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, codexAPIEndpoint, strings.NewReader(string(payload)))
	if err != nil {
		return nil, fmt.Errorf("create codex request: %w", err)
	}
	p.setCodexHeaders(httpReq, tok.AccessToken)

	httpResp, err := p.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("do codex request: %w", err)
	}

	if httpResp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(httpResp.Body, maxResponseBody))
		httpResp.Body.Close()
		return nil, fmt.Errorf("codex http %d: %s", httpResp.StatusCode, string(body))
	}

	return p.newCodexStream(httpResp.Body), nil
}

func (p *OpenAICodexProvider) setCodexHeaders(req *http.Request, accessToken string) {
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("OpenAI-Beta", "responses=experimental")
	req.Header.Set("originator", codexOAuthOriginator)
	p.mu.Lock()
	accountID := p.accountID
	p.mu.Unlock()
	if accountID != "" {
		req.Header.Set("ChatGPT-Account-Id", accountID)
	}
}

// codexRequest is the Codex Responses API body format.
type codexRequest struct {
	Model       string           `json:"model"`
	Stream      bool             `json:"stream"`
	Input       []codexInputItem `json:"input"`
	Tools       []codexToolDef   `json:"tools,omitempty"`
	MaxTokens   int              `json:"max_output_tokens,omitempty"`
	Temperature *float64         `json:"temperature,omitempty"`
}

type codexInputItem struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type codexToolDef struct {
	Type     string       `json:"type"`
	Function *codexToolFn `json:"function,omitempty"`
}

type codexToolFn struct {
	Name        string         `json:"name"`
	Description string         `json:"description,omitempty"`
	Parameters  map[string]any `json:"parameters,omitempty"`
}

func (p *OpenAICodexProvider) buildCodexRequest(req ChatRequest) codexRequest {
	cr := codexRequest{
		Model:     req.Model,
		MaxTokens: req.MaxTokens,
	}

	for _, msg := range req.Messages {
		role := msg.Role
		if role == "system" {
			role = "developer"
		}
		content := msg.StringContent()
		if role == "tool" && msg.ToolCallID != "" {
			cr.Input = append(cr.Input, codexInputItem{
				Role:    "tool",
				Content: fmt.Sprintf("[tool_result id=%s]\n%s", msg.ToolCallID, content),
			})
			continue
		}
		cr.Input = append(cr.Input, codexInputItem{
			Role:    role,
			Content: content,
		})
	}

	for _, tool := range req.Tools {
		cr.Tools = append(cr.Tools, codexToolDef{
			Type: "function",
			Function: &codexToolFn{
				Name:        tool.Name,
				Description: tool.Description,
				Parameters:  tool.Parameters,
			},
		})
	}

	return cr
}

// --- SSE Parsing ---

// parseSSEToResponse reads the full SSE stream and extracts the final response.
func (p *OpenAICodexProvider) parseSSEToResponse(reader io.Reader) (*Response, error) {
	var finalContent strings.Builder
	var toolCalls []ToolCall
	finishReason := ""

	scanner := bufio.NewScanner(reader)
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		data := strings.TrimPrefix(line, "data: ")
		if data == "[DONE]" {
			break
		}

		var event map[string]any
		if err := json.Unmarshal([]byte(data), &event); err != nil {
			continue
		}

		eventType, _ := event["type"].(string)
		switch eventType {
		case "response.output_text.delta":
			if delta, ok := event["delta"].(string); ok {
				finalContent.WriteString(delta)
			}
		case "response.completed", "response.done":
			if resp, ok := event["response"].(map[string]any); ok {
				if output, ok := resp["output"].([]any); ok {
					for _, item := range output {
						m, ok := item.(map[string]any)
						if !ok {
							continue
						}
						itemType, _ := m["type"].(string)
						if itemType == "message" {
							if content, ok := m["content"].([]any); ok {
								for _, c := range content {
									if cm, ok := c.(map[string]any); ok {
										if text, ok := cm["text"].(string); ok {
											finalContent.Reset()
											finalContent.WriteString(text)
										}
									}
								}
							}
						} else if itemType == "function_call" {
							tc := ToolCall{
								ID: strFromMap(m, "call_id"),
								Function: FunctionCall{
									Name:      strFromMap(m, "name"),
									Arguments: strFromMap(m, "arguments"),
								},
							}
							toolCalls = append(toolCalls, tc)
						}
					}
				}
				if status, ok := resp["status"].(string); ok {
					finishReason = status
				}
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("read codex sse: %w", err)
	}

	return &Response{
		Content:      finalContent.String(),
		FinishReason: finishReason,
		ToolCalls:    toolCalls,
	}, nil
}

// newCodexStream wraps the Codex SSE stream into smolbot's Stream interface.
func (p *OpenAICodexProvider) newCodexStream(body io.ReadCloser) *Stream {
	scanner := bufio.NewScanner(body)
	done := false
	toolIdx := 0

	return NewStream(
		func() (*StreamDelta, error) {
			for {
				if done {
					return nil, io.EOF
				}
				if !scanner.Scan() {
					if err := scanner.Err(); err != nil {
						return nil, err
					}
					return nil, io.EOF
				}
				line := scanner.Text()
				if !strings.HasPrefix(line, "data: ") {
					continue
				}
				data := strings.TrimPrefix(line, "data: ")
				if data == "[DONE]" {
					done = true
					return nil, io.EOF
				}

				var event map[string]any
				if err := json.Unmarshal([]byte(data), &event); err != nil {
					continue
				}

				eventType, _ := event["type"].(string)
				switch eventType {
				case "response.output_text.delta":
					delta, _ := event["delta"].(string)
					if delta != "" {
						return &StreamDelta{Content: delta}, nil
					}
				case "response.function_call_arguments.delta":
					argDelta, _ := event["delta"].(string)
					callID, _ := event["call_id"].(string)
					name, _ := event["name"].(string)
					if argDelta != "" || callID != "" {
						tc := ToolCall{
							ID:    callID,
							Index: toolIdx,
							Function: FunctionCall{
								Name:      name,
								Arguments: argDelta,
							},
						}
						return &StreamDelta{ToolCalls: []ToolCall{tc}}, nil
					}
				case "response.output_item.done":
					// Finished one output item, advance tool index
					toolIdx++
				case "response.completed", "response.done":
					done = true
					reason := "completed"
					if resp, ok := event["response"].(map[string]any); ok {
						if s, ok := resp["status"].(string); ok {
							reason = s
						}
					}
					return &StreamDelta{FinishReason: &reason}, nil
				}
			}
		},
		func() error {
			return body.Close()
		},
	)
}

func strFromMap(m map[string]any, key string) string {
	if v, ok := m[key].(string); ok {
		return v
	}
	return ""
}
