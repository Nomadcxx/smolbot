package provider

import (
	"encoding/json"
	"strings"
)

type Message struct {
	Role             string          `json:"role"`
	Content          any             `json:"content"`
	ToolCalls        []ToolCall      `json:"tool_calls,omitempty"`
	ToolCallID       string          `json:"tool_call_id,omitempty"`
	Name             string          `json:"name,omitempty"`
	ReasoningContent string          `json:"reasoning_content,omitempty"`
	ThinkingBlocks   []ThinkingBlock `json:"thinking_blocks,omitempty"`
}

type ContentBlock struct {
	Type     string    `json:"type"`
	Text     string    `json:"text,omitempty"`
	ImageURL *ImageURL `json:"image_url,omitempty"`
}

type ImageURL struct {
	URL    string `json:"url"`
	Detail string `json:"detail,omitempty"`
}

type ThinkingBlock struct {
	Type    string `json:"type"`
	Content string `json:"content"`
}

type ToolCall struct {
	ID       string       `json:"id"`
	Function FunctionCall `json:"function"`
	Index    int          `json:"-"`
}

type FunctionCall struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

type ToolDef struct {
	Name        string         `json:"name"`
	Description string         `json:"description,omitempty"`
	Parameters  map[string]any `json:"parameters,omitempty"`
}

type Response struct {
	Content          string          `json:"content"`
	ToolCalls        []ToolCall      `json:"tool_calls,omitempty"`
	ReasoningContent string          `json:"reasoning_content,omitempty"`
	ThinkingBlocks   []ThinkingBlock `json:"thinking_blocks,omitempty"`
	Usage            Usage           `json:"usage"`
	FinishReason     string          `json:"finish_reason,omitempty"`
}

type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

type ChatRequest struct {
	Model           string            `json:"model"`
	Messages        []Message         `json:"messages"`
	Tools           []ToolDef         `json:"tools,omitempty"`
	ToolChoice      any               `json:"tool_choice,omitempty"`
	MaxTokens       int               `json:"max_tokens,omitempty"`
	Temperature     float64           `json:"temperature,omitempty"`
	ReasoningEffort string            `json:"reasoning_effort,omitempty"`
	ExtraHeaders    map[string]string `json:"extra_headers,omitempty"`
}

type StreamDelta struct {
	Content          string     `json:"content,omitempty"`
	ToolCalls        []ToolCall `json:"tool_calls,omitempty"`
	ReasoningContent string     `json:"reasoning_content,omitempty"`
	FinishReason     *string    `json:"finish_reason,omitempty"`
	Usage            *Usage     `json:"usage,omitempty"`
}

func (m Message) StringContent() string {
	switch value := m.Content.(type) {
	case nil:
		return ""
	case string:
		return value
	case []ContentBlock:
		var builder strings.Builder
		for _, block := range value {
			switch block.Type {
			case "text", "input_text", "output_text":
				builder.WriteString(block.Text)
			}
		}
		return builder.String()
	case []any:
		blocks := make([]ContentBlock, 0, len(value))
		for _, item := range value {
			raw, err := json.Marshal(item)
			if err != nil {
				continue
			}
			var block ContentBlock
			if err := json.Unmarshal(raw, &block); err != nil {
				continue
			}
			blocks = append(blocks, block)
		}
		return Message{Content: blocks}.StringContent()
	case map[string]any:
		raw, err := json.Marshal(value)
		if err != nil {
			return ""
		}
		return string(raw)
	default:
		raw, err := json.Marshal(value)
		if err != nil {
			return ""
		}
		return string(raw)
	}
}

// ModelCapabilities describes features a model supports.
type ModelCapabilities struct {
	SupportsParallelTools bool
}

// parallelToolProviders is the set of provider prefixes that support parallel tool calls.
var parallelToolProviders = map[string]bool{
	"openai":      true,
	"anthropic":   true,
	"azure":       true,
	"deepseek":    true,
	"groq":        true,
	"openrouter":  true,
	"aihubmix":    true,
	"siliconflow": true,
	"gemini":      true,
}

// CapabilitiesForModel returns model capabilities inferred from the provider prefix in the model ID.
// Conservative default: parallel tools disabled for unknown or local providers (ollama, vllm, etc.).
func CapabilitiesForModel(modelID string) ModelCapabilities {
	prefix := modelID
	if idx := strings.Index(modelID, "/"); idx > 0 {
		prefix = modelID[:idx]
	}
	return ModelCapabilities{
		SupportsParallelTools: parallelToolProviders[strings.ToLower(prefix)],
	}
}
