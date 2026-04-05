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
	Model        string           `json:"model"`
	Instructions string           `json:"instructions"`
	Stream       bool             `json:"stream"`
	Store        bool             `json:"store"`
	Input        []codexInputItem `json:"input"`
	Tools        []codexToolDef   `json:"tools,omitempty"`
	// Codex Responses API does not accept max_output_tokens.
	Temperature  *float64         `json:"temperature,omitempty"`
}

// codexInputItem represents a typed item in the Responses API input array.
// Messages use {type:"message", role, content}, tool results use
// {type:"function_call_output", call_id, output}, and prior tool calls use
// {type:"function_call", call_id, name, arguments}.
type codexInputItem struct {
	Type      string `json:"type"`
	Role      string `json:"role,omitempty"`
	Content   string `json:"content,omitempty"`
	CallID    string `json:"call_id,omitempty"`
	Name      string `json:"name,omitempty"`
	Arguments string `json:"arguments,omitempty"`
	Output    string `json:"output,omitempty"`
}

// codexToolDef is the flat tool format for the Responses API.
type codexToolDef struct {
	Type        string         `json:"type"`
	Name        string         `json:"name"`
	Description string         `json:"description,omitempty"`
	Parameters  map[string]any `json:"parameters,omitempty"`
}

func (p *OpenAICodexProvider) buildCodexRequest(req ChatRequest) codexRequest {
	model := req.Model
	if strings.HasPrefix(model, "openai-codex/") {
		model = strings.TrimPrefix(model, "openai-codex/")
	}
	cr := codexRequest{
		Model: model,
	}

	// Collect system/developer messages into instructions field.
	var instructions []string
	for _, msg := range req.Messages {
		role := msg.Role
		content := msg.StringContent()
		if role == "system" || role == "developer" {
			instructions = append(instructions, content)
			continue
		}

		// Tool results → function_call_output items.
		// Always handle role:"tool" here; Codex only accepts user/assistant/developer roles.
		if role == "tool" {
			callID := msg.ToolCallID
			if callID == "" {
				callID = "call_unknown"
			}
			cr.Input = append(cr.Input, codexInputItem{
				Type:   "function_call_output",
				CallID: callID,
				Output: content,
			})
			continue
		}

		// Assistant messages with tool calls → message (for text) + function_call items.
		if role == "assistant" && len(msg.ToolCalls) > 0 {
			if content != "" {
				cr.Input = append(cr.Input, codexInputItem{
					Type:    "message",
					Role:    "assistant",
					Content: content,
				})
			}
			for _, tc := range msg.ToolCalls {
				cr.Input = append(cr.Input, codexInputItem{
					Type:      "function_call",
					CallID:    tc.ID,
					Name:      tc.Function.Name,
					Arguments: tc.Function.Arguments,
				})
			}
			continue
		}

		// Regular user/assistant messages.
		cr.Input = append(cr.Input, codexInputItem{
			Type:    "message",
			Role:    role,
			Content: content,
		})
	}

	if len(instructions) > 0 {
		cr.Instructions = strings.Join(instructions, "\n\n")
	} else {
		cr.Instructions = "You are a helpful assistant."
	}

	for _, tool := range req.Tools {
		if tool.Name == "" {
			continue
		}
		cr.Tools = append(cr.Tools, codexToolDef{
			Type:        "function",
			Name:        tool.Name,
			Description: tool.Description,
			Parameters:  tool.Parameters,
		})
	}

	return cr
}

// --- SSE Parsing ---

// parseSSEToResponse reads the full SSE stream and extracts the final response.
func (p *OpenAICodexProvider) parseSSEToResponse(reader io.Reader) (*Response, error) {
	var finalContent strings.Builder
	var toolCalls []ToolCall
	var usage Usage
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
				usage = extractCodexUsage(resp)
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
		Usage:        usage,
	}, nil
}

// newCodexStream wraps the Codex SSE stream into smolbot's Stream interface.
func (p *OpenAICodexProvider) newCodexStream(body io.ReadCloser) *Stream {
	scanner := bufio.NewScanner(body)
	done := false
	toolIdx := 0
	itemToIdx := make(map[string]int) // maps SSE item_id to tool index

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

				case "response.output_item.added":
					// Tool calls arrive here with name and call_id.
					item, _ := event["item"].(map[string]any)
					if item == nil {
						continue
					}
					itemType, _ := item["type"].(string)
					if itemType == "function_call" {
						tc := ToolCall{
							ID:    strFromMap(item, "call_id"),
							Index: toolIdx,
							Function: FunctionCall{
								Name: strFromMap(item, "name"),
							},
						}
						// Track item_id → toolIdx for argument deltas.
						itemID, _ := item["id"].(string)
						if itemID != "" {
							itemToIdx[itemID] = toolIdx
						}
						return &StreamDelta{ToolCalls: []ToolCall{tc}}, nil
					}

				case "response.function_call_arguments.delta":
					argDelta, _ := event["delta"].(string)
					if argDelta == "" {
						continue
					}
					itemID, _ := event["item_id"].(string)
					idx, ok := itemToIdx[itemID]
					if !ok {
						idx = toolIdx
					}
					tc := ToolCall{
						Index: idx,
						Function: FunctionCall{
							Arguments: argDelta,
						},
					}
					return &StreamDelta{ToolCalls: []ToolCall{tc}}, nil

				case "response.output_item.done":
					// Only increment tool index for function_call items;
					// message items also fire this event.
					if item, ok := event["item"].(map[string]any); ok {
						if t, _ := item["type"].(string); t == "function_call" {
							toolIdx++
						}
					}

				case "response.completed", "response.done":
					done = true
					reason := "completed"
					var u *Usage
					if resp, ok := event["response"].(map[string]any); ok {
						if s, ok := resp["status"].(string); ok {
							reason = s
						}
						usage := extractCodexUsage(resp)
						if usage.PromptTokens > 0 || usage.CompletionTokens > 0 {
							u = &usage
						}
					}
					return &StreamDelta{FinishReason: &reason, Usage: u}, nil
				}
			}
		},
		func() error {
			return body.Close()
		},
	)
}

func extractCodexUsage(resp map[string]any) Usage {
	u, ok := resp["usage"].(map[string]any)
	if !ok {
		return Usage{}
	}
	return Usage{
		PromptTokens:     intFromMap(u, "input_tokens"),
		CompletionTokens: intFromMap(u, "output_tokens"),
		TotalTokens:      intFromMap(u, "total_tokens"),
	}
}

func intFromMap(m map[string]any, key string) int {
	switch v := m[key].(type) {
	case float64:
		return int(v)
	case int:
		return v
	default:
		return 0
	}
}

func strFromMap(m map[string]any, key string) string {
	if v, ok := m[key].(string); ok {
		return v
	}
	return ""
}
