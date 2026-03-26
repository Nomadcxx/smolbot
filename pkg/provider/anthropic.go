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

type AnthropicProvider struct {
	apiKey  string
	baseURL string
	client  *http.Client
	sleep   func(context.Context, int) error
}

func NewAnthropicProvider(apiKey, baseURL string) *AnthropicProvider {
	if baseURL == "" {
		baseURL = "https://api.anthropic.com"
	}
	return &AnthropicProvider{
		apiKey:  apiKey,
		baseURL: strings.TrimRight(baseURL, "/"),
		client:  &http.Client{Timeout: 300 * time.Second},
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

func (p *AnthropicProvider) Name() string {
	return "anthropic"
}

func (p *AnthropicProvider) Chat(ctx context.Context, req ChatRequest) (*Response, error) {
	wireReq := p.buildRequest(req, false, false)
	body, err := p.doWithRetry(ctx, wireReq, req.Messages)
	if err != nil {
		return nil, err
	}

	var resp anthropicResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("decode anthropic response: %w", err)
	}
	return parseAnthropicResponse(resp), nil
}

func (p *AnthropicProvider) ChatStream(ctx context.Context, req ChatRequest) (*Stream, error) {
	wireReq := p.buildRequest(req, true, false)
	data, err := json.Marshal(wireReq)
	if err != nil {
		return nil, fmt.Errorf("marshal anthropic request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, p.baseURL+"/v1/messages", bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	p.setHeaders(httpReq)

	httpResp, err := p.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("do request: %w", err)
	}
	if httpResp.StatusCode != http.StatusOK {
		defer httpResp.Body.Close()
		body, _ := io.ReadAll(io.LimitReader(httpResp.Body, 8192))
		return nil, fmt.Errorf("anthropic stream http %d: %s", httpResp.StatusCode, string(body))
	}

	scanner := bufio.NewScanner(httpResp.Body)
	scanner.Buffer(make([]byte, 0, 4096), 1024*1024)
	type toolState struct {
		id   string
		name string
	}
	toolStates := map[int]toolState{}

	return NewStream(
		func() (*StreamDelta, error) {
			for scanner.Scan() {
				line := scanner.Text()
				if !strings.HasPrefix(line, "data: ") {
					continue
				}

				data := strings.TrimPrefix(line, "data: ")
				var event anthropicStreamEvent
				if err := json.Unmarshal([]byte(data), &event); err != nil {
					return nil, fmt.Errorf("decode anthropic stream event: %w", err)
				}

				switch event.Type {
				case "content_block_start":
					if event.ContentBlock.Type == "tool_use" {
						toolStates[event.Index] = toolState{
							id:   event.ContentBlock.ID,
							name: event.ContentBlock.Name,
						}
					}
				case "content_block_delta":
					switch event.Delta.Type {
					case "text_delta":
						return &StreamDelta{Content: event.Delta.Text}, nil
					case "thinking_delta":
						return &StreamDelta{ReasoningContent: event.Delta.Thinking}, nil
					case "input_json_delta":
						state := toolStates[event.Index]
						return &StreamDelta{
							ToolCalls: []ToolCall{{
								ID:    state.id,
								Index: event.Index,
								Function: FunctionCall{
									Name:      state.name,
									Arguments: event.Delta.PartialJSON,
								},
							}},
						}, nil
					}
				case "message_delta":
					finishReason := mapAnthropicStopReason(event.Delta.StopReason)
					delta := &StreamDelta{FinishReason: &finishReason}
					if event.Usage.InputTokens > 0 || event.Usage.OutputTokens > 0 {
						total := event.Usage.InputTokens + event.Usage.OutputTokens
						delta.Usage = &Usage{
							PromptTokens:     event.Usage.InputTokens,
							CompletionTokens: event.Usage.OutputTokens,
							TotalTokens:      total,
						}
					}
					return delta, nil
				case "message_stop":
					return nil, io.EOF
				}
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

func (p *AnthropicProvider) doWithRetry(ctx context.Context, req anthropicRequest, originalMessages []Message) ([]byte, error) {
	retryDelays := []int{1, 2, 4}
	request := req
	imageRetried := false

	for attempt := 0; ; attempt++ {
		data, err := json.Marshal(request)
		if err != nil {
			return nil, fmt.Errorf("marshal anthropic request: %w", err)
		}

		httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, p.baseURL+"/v1/messages", bytes.NewReader(data))
		if err != nil {
			return nil, fmt.Errorf("create request: %w", err)
		}
		p.setHeaders(httpReq)

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
		if !imageRetried && hasImageBlocks(originalMessages) && isImageUnsupported(lowerBody) {
			request = p.buildRequest(ChatRequest{
				Model:           req.Model,
				Messages:        maybeOmitImages(originalMessages, true),
				Tools:           req.originalTools,
				ToolChoice:      req.ToolChoice,
				MaxTokens:       req.MaxTokens,
				Temperature:     req.originalTemp,
				ReasoningEffort: req.ReasoningEffort,
			}, req.Stream, true)
			imageRetried = true
			continue
		}

		if attempt < len(retryDelays) && shouldRetryOpenAI(httpResp.StatusCode, lowerBody) {
			if sleepErr := p.sleep(ctx, retryDelays[attempt]); sleepErr != nil {
				return nil, sleepErr
			}
			continue
		}

		return nil, fmt.Errorf("anthropic http %d: %s", httpResp.StatusCode, string(body))
	}
}

func (p *AnthropicProvider) setHeaders(req *http.Request) {
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", p.apiKey)
	req.Header.Set("anthropic-version", "2023-06-01")
	req.Header.Set("anthropic-beta", "claude-code-20250219,interleaved-thinking-2025-05-14,fine-grained-tool-streaming-2025-05-14")
}

func (p *AnthropicProvider) buildRequest(req ChatRequest, stream bool, imagesOmitted bool) anthropicRequest {
	messages := SanitizeMessages(req.Messages, p.Name())
	if imagesOmitted {
		messages = maybeOmitImages(messages, true)
	}

	wireReq := anthropicRequest{
		Model:           req.Model,
		MaxTokens:       req.MaxTokens,
		ToolChoice:      req.ToolChoice,
		Stream:          stream,
		ReasoningEffort: req.ReasoningEffort,
		originalTools:   req.Tools,
		originalTemp:    req.Temperature,
	}
	if wireReq.MaxTokens == 0 {
		wireReq.MaxTokens = 8192
	}

	cacheControl := &anthropicCacheControl{Type: "ephemeral"}
	for _, msg := range messages {
		switch msg.Role {
		case "system":
			for _, block := range anthropicBlocksFromMessage(msg) {
				block.CacheControl = cacheControl
				wireReq.System = append(wireReq.System, block)
			}
		default:
			wireReq.Messages = append(wireReq.Messages, anthropicMessage{
				Role:    anthropicRole(msg),
				Content: anthropicBlocksFromMessage(msg),
			})
		}
	}

	for i, tool := range req.Tools {
		out := anthropicTool{
			Name:        tool.Name,
			Description: tool.Description,
			InputSchema: tool.Parameters,
		}
		if i == len(req.Tools)-1 {
			out.CacheControl = cacheControl
		}
		wireReq.Tools = append(wireReq.Tools, out)
	}

	return wireReq
}

type anthropicRequest struct {
	Model           string             `json:"model"`
	System          []anthropicBlock   `json:"system,omitempty"`
	Messages        []anthropicMessage `json:"messages"`
	Tools           []anthropicTool    `json:"tools,omitempty"`
	ToolChoice      any                `json:"tool_choice,omitempty"`
	MaxTokens       int                `json:"max_tokens"`
	ReasoningEffort string             `json:"thinking,omitempty"`
	Stream          bool               `json:"stream,omitempty"`
	originalTools   []ToolDef          `json:"-"`
	originalTemp    float64            `json:"-"`
}

type anthropicMessage struct {
	Role    string           `json:"role"`
	Content []anthropicBlock `json:"content"`
}

type anthropicBlock struct {
	Type         string                 `json:"type"`
	Text         string                 `json:"text,omitempty"`
	Thinking     string                 `json:"thinking,omitempty"`
	ID           string                 `json:"id,omitempty"`
	Name         string                 `json:"name,omitempty"`
	Input        map[string]any         `json:"input,omitempty"`
	ToolUseID    string                 `json:"tool_use_id,omitempty"`
	Content      any                    `json:"content,omitempty"`
	Source       map[string]any         `json:"source,omitempty"`
	CacheControl *anthropicCacheControl `json:"cache_control,omitempty"`
}

type anthropicTool struct {
	Name         string                 `json:"name"`
	Description  string                 `json:"description,omitempty"`
	InputSchema  map[string]any         `json:"input_schema,omitempty"`
	CacheControl *anthropicCacheControl `json:"cache_control,omitempty"`
}

type anthropicCacheControl struct {
	Type string `json:"type"`
}

type anthropicResponse struct {
	Content    []anthropicBlock `json:"content"`
	StopReason string           `json:"stop_reason"`
	Usage      struct {
		InputTokens  int `json:"input_tokens"`
		OutputTokens int `json:"output_tokens"`
	} `json:"usage"`
}

type anthropicStreamEvent struct {
	Type         string `json:"type"`
	Index        int    `json:"index"`
	ContentBlock struct {
		Type string `json:"type"`
		ID   string `json:"id"`
		Name string `json:"name"`
	} `json:"content_block"`
	Delta struct {
		Type        string `json:"type"`
		Text        string `json:"text"`
		Thinking    string `json:"thinking"`
		PartialJSON string `json:"partial_json"`
		StopReason  string `json:"stop_reason"`
	} `json:"delta"`
	Usage struct {
		InputTokens  int `json:"input_tokens"`
		OutputTokens int `json:"output_tokens"`
	} `json:"usage"`
}

func anthropicRole(msg Message) string {
	if msg.Role == "tool" {
		return "user"
	}
	return msg.Role
}

func anthropicBlocksFromMessage(msg Message) []anthropicBlock {
	switch msg.Role {
	case "tool":
		return []anthropicBlock{{
			Type:      "tool_result",
			ToolUseID: msg.ToolCallID,
			Content:   msg.StringContent(),
		}}
	case "assistant":
		blocks := make([]anthropicBlock, 0, len(msg.ToolCalls)+1+len(msg.ThinkingBlocks))
		if text := msg.StringContent(); text != "" && text != " " {
			blocks = append(blocks, anthropicBlock{Type: "text", Text: text})
		}
		for _, block := range msg.ThinkingBlocks {
			blocks = append(blocks, anthropicBlock{Type: "thinking", Thinking: block.Content})
		}
		if msg.ReasoningContent != "" && len(msg.ThinkingBlocks) == 0 {
			blocks = append(blocks, anthropicBlock{Type: "thinking", Thinking: msg.ReasoningContent})
		}
		for _, toolCall := range msg.ToolCalls {
			var input map[string]any
			_ = json.Unmarshal([]byte(repairJSON(toolCall.Function.Arguments)), &input)
			blocks = append(blocks, anthropicBlock{
				Type:  "tool_use",
				ID:    toolCall.ID,
				Name:  toolCall.Function.Name,
				Input: input,
			})
		}
		return blocks
	default:
		return anthropicBlocksFromContent(msg.Content)
	}
}

func anthropicBlocksFromContent(content any) []anthropicBlock {
	switch value := content.(type) {
	case string:
		return []anthropicBlock{{Type: "text", Text: value}}
	case []ContentBlock:
		blocks := make([]anthropicBlock, 0, len(value))
		for _, block := range value {
			switch block.Type {
			case "text":
				blocks = append(blocks, anthropicBlock{Type: "text", Text: block.Text})
			case "image_url":
				blocks = append(blocks, anthropicBlock{
					Type: "image_url",
					Source: map[string]any{
						"type": "url",
						"url":  block.ImageURL.URL,
					},
				})
			}
		}
		return blocks
	default:
		return []anthropicBlock{{Type: "text", Text: Message{Content: content}.StringContent()}}
	}
}

func parseAnthropicResponse(resp anthropicResponse) *Response {
	result := &Response{
		Usage: Usage{
			PromptTokens:     resp.Usage.InputTokens,
			CompletionTokens: resp.Usage.OutputTokens,
			TotalTokens:      resp.Usage.InputTokens + resp.Usage.OutputTokens,
		},
		FinishReason: mapAnthropicStopReason(resp.StopReason),
	}

	for _, block := range resp.Content {
		switch block.Type {
		case "text":
			result.Content += block.Text
		case "thinking":
			thinking := block.Thinking
			if thinking == "" {
				thinking = block.Text
			}
			result.ReasoningContent += thinking
			result.ThinkingBlocks = append(result.ThinkingBlocks, ThinkingBlock{
				Type:    "thinking",
				Content: thinking,
			})
		case "tool_use":
			args, _ := json.Marshal(block.Input)
			result.ToolCalls = append(result.ToolCalls, ToolCall{
				ID: block.ID,
				Function: FunctionCall{
					Name:      block.Name,
					Arguments: string(args),
				},
			})
		}
	}

	return result
}

func mapAnthropicStopReason(reason string) string {
	switch reason {
	case "end_turn":
		return "stop"
	case "tool_use":
		return "tool_calls"
	case "max_tokens":
		return "length"
	default:
		return reason
	}
}

func hasImageBlocks(messages []Message) bool {
	for _, msg := range messages {
		if blocks, ok := msg.Content.([]ContentBlock); ok {
			for _, block := range blocks {
				if block.Type == "image_url" && block.ImageURL != nil {
					return true
				}
			}
		}
	}
	return false
}
