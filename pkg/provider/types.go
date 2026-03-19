package provider

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
}
