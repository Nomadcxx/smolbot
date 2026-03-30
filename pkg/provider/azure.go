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

const azureAPIVersion = "2024-10-21"

var azureReasoningPrefixes = []string{"gpt-5", "o1", "o3", "o4-mini"}

type AzureProvider struct {
	apiKey  string
	baseURL string
	client  *http.Client
	sleep   func(context.Context, int) error
}

func NewAzureProvider(apiKey, baseURL string) *AzureProvider {
	return &AzureProvider{
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

func (p *AzureProvider) Name() string {
	return "azure_openai"
}

func (p *AzureProvider) Chat(ctx context.Context, req ChatRequest) (*Response, error) {
	req.Model = StripProviderPrefix(req.Model)
	wireReq := p.buildRequest(req, false)
	body, err := p.doWithRetry(ctx, req.Model, wireReq)
	if err != nil {
		return nil, err
	}

	var resp openAIResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("decode azure response: %w", err)
	}
	return parseOpenAIResponse(resp), nil
}

func (p *AzureProvider) ChatStream(ctx context.Context, req ChatRequest) (*Stream, error) {
	req.Model = StripProviderPrefix(req.Model)
	wireReq := p.buildRequest(req, true)
	data, err := json.Marshal(wireReq)
	if err != nil {
		return nil, fmt.Errorf("marshal azure request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, p.endpoint(req.Model), bytes.NewReader(data))
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
		return nil, fmt.Errorf("azure stream http %d: %s", httpResp.StatusCode, string(body))
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
					return nil, fmt.Errorf("decode azure stream chunk: %w", err)
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

func (p *AzureProvider) buildRequest(req ChatRequest, stream bool) azureRequest {
	messages := SanitizeMessages(req.Messages, p.Name())
	wireReq := azureRequest{
		Messages:            convertOpenAIMessages(messages, false),
		Tools:               convertOpenAITools(req.Tools, false),
		ToolChoice:          req.ToolChoice,
		MaxCompletionTokens: req.MaxTokens,
		Stream:              stream,
	}
	if stream {
		wireReq.StreamOptions = &openAIStreamOpts{IncludeUsage: true}
	}
	if req.ReasoningEffort != "" {
		wireReq.Reasoning = &openAIReasoning{Effort: req.ReasoningEffort}
	}
	if req.Temperature != 0 && !isAzureReasoningModel(req.Model) {
		wireReq.Temperature = &req.Temperature
	}
	return wireReq
}

func (p *AzureProvider) doWithRetry(ctx context.Context, model string, req azureRequest) ([]byte, error) {
	retryDelays := []int{1, 2, 4}
	data, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal azure request: %w", err)
	}

	for attempt := 0; ; attempt++ {
		httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, p.endpoint(model), bytes.NewReader(data))
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

		if attempt < len(retryDelays) && shouldRetryOpenAI(httpResp.StatusCode, strings.ToLower(string(body))) {
			if sleepErr := p.sleep(ctx, retryDelays[attempt]); sleepErr != nil {
				return nil, sleepErr
			}
			continue
		}

		return nil, fmt.Errorf("azure http %d: %s", httpResp.StatusCode, string(body))
	}
}

func (p *AzureProvider) endpoint(model string) string {
	return fmt.Sprintf("%s/openai/deployments/%s/chat/completions?api-version=%s", p.baseURL, model, azureAPIVersion)
}

func (p *AzureProvider) setHeaders(req *http.Request) {
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("api-key", p.apiKey)
}

type azureRequest struct {
	Messages            []openAIMessage  `json:"messages"`
	Tools               []openAITool     `json:"tools,omitempty"`
	ToolChoice          any              `json:"tool_choice,omitempty"`
	MaxCompletionTokens int              `json:"max_completion_tokens,omitempty"`
	Temperature         *float64         `json:"temperature,omitempty"`
	Reasoning           *openAIReasoning `json:"reasoning,omitempty"`
	Stream              bool             `json:"stream,omitempty"`
	StreamOptions       *openAIStreamOpts `json:"stream_options,omitempty"`
}

func isAzureReasoningModel(model string) bool {
	lower := strings.ToLower(model)
	for _, prefix := range azureReasoningPrefixes {
		if strings.HasPrefix(lower, prefix) {
			return true
		}
	}
	return false
}
