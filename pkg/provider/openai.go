package provider

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

var openAITransientMarkers = []string{
	"rate limit",
	"overloaded",
	"capacity",
	"temporarily unavailable",
	"server error",
	"timed out",
	"timeout",
	"try again",
	"please retry",
	"connection reset",
}

type OpenAIProvider struct {
	name         string
	apiKey       string
	baseURL      string
	extraHeaders map[string]string
	client       *http.Client
	sleep        func(context.Context, int) error
}

func NewOpenAIProvider(name, apiKey, baseURL string, extraHeaders map[string]string) *OpenAIProvider {
	if baseURL == "" {
		baseURL = "https://api.openai.com/v1"
	}
	baseURL = strings.TrimRight(baseURL, "/")
	// Ollama and other OpenAI-compatible local servers require /v1 path prefix
	if name == "ollama" && !strings.HasSuffix(baseURL, "/v1") {
		baseURL = baseURL + "/v1"
	}
	return &OpenAIProvider{
		name:         name,
		apiKey:       apiKey,
		baseURL:      baseURL,
		extraHeaders: extraHeaders,
		client:       &http.Client{Timeout: 300 * time.Second},
		sleep: func(ctx context.Context, seconds int) error {
			timer := time.NewTimer(time.Duration(seconds) * time.Second)
			defer timer.Stop()
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-timer.C:
				return nil
			}
		},
	}
}

func (p *OpenAIProvider) Name() string {
	return p.name
}

func (p *OpenAIProvider) Chat(ctx context.Context, req ChatRequest) (*Response, error) {
	wireReq, sourceMessages, err := p.buildWireRequest(req, false)
	if err != nil {
		return nil, err
	}

	body, err := p.doWithRetry(ctx, wireReq, sourceMessages)
	if err != nil {
		return nil, err
	}

	var resp openAIResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("decode openai response: %w", err)
	}
	return parseOpenAIResponse(resp), nil
}

func (p *OpenAIProvider) ChatStream(ctx context.Context, req ChatRequest) (*Stream, error) {
	wireReq, _, err := p.buildWireRequest(req, true)
	if err != nil {
		return nil, err
	}

	return p.openStream(ctx, req.ExtraHeaders, wireReq)
}

func (p *OpenAIProvider) openStream(ctx context.Context, extraHeaders map[string]string, wireReq openAIRequest) (*Stream, error) {
	data, err := json.Marshal(wireReq)
	if err != nil {
		return nil, fmt.Errorf("marshal openai request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, p.baseURL+"/chat/completions", bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	p.setHeaders(httpReq, extraHeaders)

	httpResp, err := p.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("do request: %w", err)
	}
	if httpResp.StatusCode != http.StatusOK {
		defer httpResp.Body.Close()
		body, _ := io.ReadAll(io.LimitReader(httpResp.Body, 8192))
		if wireReq.StreamOptions != nil && shouldRetryOpenAIStreamWithoutUsage(httpResp.StatusCode, string(body)) {
			wireReq.StreamOptions = nil
			return p.openStream(ctx, extraHeaders, wireReq)
		}
		return nil, fmt.Errorf("openai stream http %d: %s", httpResp.StatusCode, string(body))
	}

	scanner := bufio.NewScanner(httpResp.Body)
	scanner.Buffer(make([]byte, 0, 4096), 1024*1024)
	return NewStream(
		func() (*StreamDelta, error) {
			for scanner.Scan() {
				line := scanner.Text()
				if !strings.HasPrefix(line, "data: ") {
					continue
				}
				data := strings.TrimPrefix(line, "data: ")
				if data == "[DONE]" {
					return nil, io.EOF
				}

				var chunk openAIStreamChunk
				if err := json.Unmarshal([]byte(data), &chunk); err != nil {
					return nil, fmt.Errorf("decode openai stream chunk: %w", err)
				}
				if len(chunk.Choices) == 0 {
					if chunk.Usage != nil {
						return &StreamDelta{Usage: chunk.Usage}, nil
					}
					continue
				}

				choice := chunk.Choices[0]
				delta := &StreamDelta{
					Content:          choice.Delta.Content,
					ReasoningContent: choice.Delta.ReasoningContent,
					FinishReason:     choice.FinishReason,
					Usage:            chunk.Usage,
				}
				for _, toolCall := range choice.Delta.ToolCalls {
					delta.ToolCalls = append(delta.ToolCalls, ToolCall{
						ID:    toolCall.ID,
						Index: toolCall.Index,
						Function: FunctionCall{
							Name:      toolCall.Function.Name,
							Arguments: toolCall.Function.Arguments,
						},
					})
				}
				return delta, nil
			}
			if err := scanner.Err(); err != nil {
				return nil, err
			}
			return nil, io.EOF
		},
		func() error {
			return httpResp.Body.Close()
		},
	), nil
}

func (p *OpenAIProvider) buildWireRequest(req ChatRequest, stream bool) (openAIRequest, []Message, error) {
	messages := SanitizeMessages(req.Messages, p.name)
	messages = maybeOmitImages(messages, false)

	wireReq := openAIRequest{
		Model:      req.Model,
		Messages:   convertOpenAIMessages(messages, supportsPromptCaching(p.name, p.baseURL)),
		ToolChoice: req.ToolChoice,
		Stream:     stream,
	}
	if stream && supportsStreamUsage(p.name, p.baseURL) {
		wireReq.StreamOptions = &openAIStreamOpts{IncludeUsage: true}
	}
	if req.MaxTokens > 0 {
		wireReq.MaxTokens = req.MaxTokens
	}
	if req.Temperature != 0 {
		wireReq.Temperature = &req.Temperature
	}
	if req.ReasoningEffort != "" {
		wireReq.Reasoning = &openAIReasoning{Effort: req.ReasoningEffort}
	}
	wireReq.Tools = convertOpenAITools(req.Tools, supportsPromptCaching(p.name, p.baseURL))
	return wireReq, messages, nil
}

func (p *OpenAIProvider) doWithRetry(ctx context.Context, req openAIRequest, sourceMessages []Message) ([]byte, error) {
	retryDelays := []int{1, 2, 4}
	request := req
	imageRetried := false

	for attempt := 0; ; attempt++ {
		data, err := json.Marshal(request)
		if err != nil {
			return nil, fmt.Errorf("marshal openai request: %w", err)
		}

		httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, p.baseURL+"/chat/completions", bytes.NewReader(data))
		if err != nil {
			return nil, fmt.Errorf("create request: %w", err)
		}
		p.setHeaders(httpReq, request.ExtraHeaders)

		httpResp, err := p.client.Do(httpReq)
		if err != nil {
			if attempt >= len(retryDelays) {
				return nil, fmt.Errorf("do request: %w", err)
			}
			if sleepErr := p.sleep(ctx, retryDelays[attempt]); sleepErr != nil {
				return nil, sleepErr
			}
			continue
		}

		body, readErr := io.ReadAll(io.LimitReader(httpResp.Body, 1<<20))
		httpResp.Body.Close()
		if readErr != nil {
			return nil, fmt.Errorf("read response: %w", readErr)
		}

		if httpResp.StatusCode == http.StatusOK {
			return body, nil
		}

		lowerBody := strings.ToLower(string(body))
		if !imageRetried && hasImageMessages(request.Messages) && isImageUnsupported(lowerBody) {
			request.Messages = convertOpenAIMessages(maybeOmitImages(sourceMessages, true), supportsPromptCaching(p.name, p.baseURL))
			imageRetried = true
			continue
		}

		if attempt < len(retryDelays) && shouldRetryOpenAI(httpResp.StatusCode, lowerBody) {
			if sleepErr := p.sleep(ctx, retryDelays[attempt]); sleepErr != nil {
				return nil, sleepErr
			}
			continue
		}

		return nil, fmt.Errorf("openai http %d: %s", httpResp.StatusCode, string(body))
	}
}

func (p *OpenAIProvider) setHeaders(req *http.Request, extra map[string]string) {
	req.Header.Set("Content-Type", "application/json")
	if p.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+p.apiKey)
	}
	for key, value := range p.extraHeaders {
		req.Header.Set(key, value)
	}
	for key, value := range extra {
		req.Header.Set(key, value)
	}
}

type openAIRequest struct {
	Model         string            `json:"model"`
	Messages      []openAIMessage   `json:"messages"`
	Tools         []openAITool      `json:"tools,omitempty"`
	ToolChoice    any               `json:"tool_choice,omitempty"`
	MaxTokens     int               `json:"max_tokens,omitempty"`
	Temperature   *float64          `json:"temperature,omitempty"`
	Reasoning     *openAIReasoning  `json:"reasoning,omitempty"`
	ExtraHeaders  map[string]string `json:"-"`
	Stream        bool              `json:"stream,omitempty"`
	StreamOptions *openAIStreamOpts `json:"stream_options,omitempty"`
}

type openAIStreamOpts struct {
	IncludeUsage bool `json:"include_usage"`
}

type openAIReasoning struct {
	Effort string `json:"effort,omitempty"`
}

type openAIMessage struct {
	Role             string           `json:"role"`
	Content          any              `json:"content"`
	ToolCalls        []openAIToolCall `json:"tool_calls,omitempty"`
	ToolCallID       string           `json:"tool_call_id,omitempty"`
	Name             string           `json:"name,omitempty"`
	ReasoningContent string           `json:"reasoning_content,omitempty"`
}

type openAIContentPart struct {
	Type         string              `json:"type"`
	Text         string              `json:"text,omitempty"`
	ImageURL     *ImageURL           `json:"image_url,omitempty"`
	CacheControl *openAICacheControl `json:"cache_control,omitempty"`
}

type openAIToolCall struct {
	Index    int                `json:"index,omitempty"`
	ID       string             `json:"id,omitempty"`
	Type     string             `json:"type,omitempty"`
	Function openAIToolFunction `json:"function"`
}

type openAIToolFunction struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

type openAITool struct {
	Type         string              `json:"type"`
	Function     openAIToolDef       `json:"function"`
	CacheControl *openAICacheControl `json:"cache_control,omitempty"`
}

type openAIToolDef struct {
	Name        string         `json:"name"`
	Description string         `json:"description,omitempty"`
	Parameters  map[string]any `json:"parameters,omitempty"`
}

type openAICacheControl struct {
	Type string `json:"type"`
}

type openAIResponse struct {
	Choices []openAIChoice `json:"choices"`
	Usage   Usage          `json:"usage"`
}

type openAIChoice struct {
	FinishReason string        `json:"finish_reason"`
	Message      openAIMessage `json:"message"`
}

type openAIStreamChunk struct {
	Choices []openAIStreamChoice `json:"choices"`
	Usage   *Usage               `json:"usage,omitempty"`
}

type openAIStreamChoice struct {
	Delta        openAIStreamMessage `json:"delta"`
	FinishReason *string             `json:"finish_reason"`
}

type openAIStreamMessage struct {
	Content          string           `json:"content"`
	ReasoningContent string           `json:"reasoning_content"`
	ToolCalls        []openAIToolCall `json:"tool_calls"`
}

func parseOpenAIResponse(resp openAIResponse) *Response {
	result := &Response{Usage: resp.Usage}
	if len(resp.Choices) == 0 {
		return result
	}

	choice := resp.Choices[0]
	result.FinishReason = choice.FinishReason

	switch value := choice.Message.Content.(type) {
	case string:
		result.Content = value
	case []any:
		result.Content = Message{Content: value}.StringContent()
	}
	result.ReasoningContent = choice.Message.ReasoningContent

	for _, toolCall := range choice.Message.ToolCalls {
		result.ToolCalls = append(result.ToolCalls, ToolCall{
			ID: toolCall.ID,
			Function: FunctionCall{
				Name:      toolCall.Function.Name,
				Arguments: repairJSON(toolCall.Function.Arguments),
			},
		})
	}

	return result
}

func convertOpenAIMessages(messages []Message, withCache bool) []openAIMessage {
	converted := make([]openAIMessage, 0, len(messages))
	for _, msg := range messages {
		out := openAIMessage{
			Role:             msg.Role,
			ToolCallID:       msg.ToolCallID,
			Name:             msg.Name,
			ReasoningContent: msg.ReasoningContent,
		}

		switch value := msg.Content.(type) {
		case []ContentBlock:
			parts := make([]openAIContentPart, 0, len(value))
			for _, block := range value {
				part := openAIContentPart{
					Type:     block.Type,
					Text:     block.Text,
					ImageURL: block.ImageURL,
				}
				parts = append(parts, part)
			}
			if withCache && msg.Role == "system" && len(parts) > 0 {
				parts[0].CacheControl = &openAICacheControl{Type: "ephemeral"}
			}
			out.Content = parts
		case string:
			if withCache && msg.Role == "system" {
				out.Content = []openAIContentPart{{
					Type:         "text",
					Text:         value,
					CacheControl: &openAICacheControl{Type: "ephemeral"},
				}}
			} else {
				out.Content = value
			}
		default:
			out.Content = msg.StringContent()
		}

		for idx, toolCall := range msg.ToolCalls {
			out.ToolCalls = append(out.ToolCalls, openAIToolCall{
				Index: idx,
				ID:    toolCall.ID,
				Type:  "function",
				Function: openAIToolFunction{
					Name:      toolCall.Function.Name,
					Arguments: repairJSON(toolCall.Function.Arguments),
				},
			})
		}
		converted = append(converted, out)
	}
	return converted
}

func convertOpenAITools(tools []ToolDef, withCache bool) []openAITool {
	converted := make([]openAITool, 0, len(tools))
	for i, tool := range tools {
		out := openAITool{
			Type: "function",
			Function: openAIToolDef{
				Name:        tool.Name,
				Description: tool.Description,
				Parameters:  tool.Parameters,
			},
		}
		if withCache && i == len(tools)-1 {
			out.CacheControl = &openAICacheControl{Type: "ephemeral"}
		}
		converted = append(converted, out)
	}
	return converted
}

func supportsPromptCaching(name, baseURL string) bool {
	lowerName := strings.ToLower(name)
	lowerBase := strings.ToLower(baseURL)
	return strings.Contains(lowerName, "openrouter") || strings.Contains(lowerBase, "openrouter")
}

func supportsStreamUsage(name, baseURL string) bool {
	lowerName := strings.ToLower(name)
	lowerBase := strings.ToLower(baseURL)
	if strings.Contains(lowerName, "ollama") || strings.Contains(lowerBase, "localhost:11434") || strings.Contains(lowerBase, "127.0.0.1:11434") {
		return false
	}
	return true
}

func shouldRetryOpenAI(status int, body string) bool {
	if status == http.StatusTooManyRequests || status >= http.StatusInternalServerError {
		return true
	}
	for _, marker := range openAITransientMarkers {
		if strings.Contains(body, marker) {
			return true
		}
	}
	return false
}

func shouldRetryOpenAIStreamWithoutUsage(status int, body string) bool {
	if status != http.StatusBadRequest && status != http.StatusUnprocessableEntity {
		return false
	}
	lowerBody := strings.ToLower(body)
	return strings.Contains(lowerBody, "stream_options") || strings.Contains(lowerBody, "include_usage")
}

func hasImageMessages(messages []openAIMessage) bool {
	for _, msg := range messages {
		switch value := msg.Content.(type) {
		case []openAIContentPart:
			for _, part := range value {
				if part.Type == "image_url" && part.ImageURL != nil {
					return true
				}
			}
		}
	}
	return false
}

func maybeOmitImages(messages []Message, omit bool) []Message {
	if !omit {
		cloned := make([]Message, len(messages))
		copy(cloned, messages)
		return cloned
	}

	out := make([]Message, len(messages))
	for i, msg := range messages {
		out[i] = msg
		blocks, ok := msg.Content.([]ContentBlock)
		if !ok {
			continue
		}
		rewritten := make([]ContentBlock, 0, len(blocks))
		for _, block := range blocks {
			if block.Type == "image_url" {
				rewritten = append(rewritten, ContentBlock{Type: "text", Text: "[image omitted]"})
				continue
			}
			rewritten = append(rewritten, block)
		}
		out[i].Content = rewritten
	}
	return out
}

func isImageUnsupported(body string) bool {
	return strings.Contains(body, "image") &&
		(strings.Contains(body, "not supported") ||
			strings.Contains(body, "unsupported") ||
			strings.Contains(body, "does not support"))
}
