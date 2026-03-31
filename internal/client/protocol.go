package client

import "encoding/json"

const (
	FrameReq   = "req"
	FrameRes   = "res"
	FrameEvent = "event"

	ProtocolVersion = 1
)

type Request struct {
	Type   string `json:"type"`
	ID     string `json:"id"`
	Method string `json:"method"`
	Params any    `json:"params,omitempty"`
}

type Response struct {
	Type    string          `json:"type"`
	ID      string          `json:"id"`
	OK      bool            `json:"ok"`
	Payload json.RawMessage `json:"payload,omitempty"`
	Error   *ErrorShape     `json:"error,omitempty"`
}

type Event struct {
	Type    string          `json:"type"`
	Event   string          `json:"event"`
	Payload json.RawMessage `json:"payload,omitempty"`
	Seq     int             `json:"seq,omitempty"`
}

type ErrorShape struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type Frame struct {
	Type string `json:"type"`
}

type HelloParams struct {
	Client   string `json:"client"`
	Version  string `json:"version"`
	Protocol int    `json:"protocol"`
	Platform string `json:"platform"`
}

type HelloPayload struct {
	Server   string   `json:"server"`
	Version  string   `json:"version"`
	Protocol int      `json:"protocol"`
	Methods  []string `json:"methods"`
	Events   []string `json:"events"`
}

type ChatSendParams struct {
	Session string `json:"session"`
	Message string `json:"message"`
}

type ChatSendPayload struct {
	RunID string `json:"runId"`
}

type ChatAbortParams struct {
	Session string `json:"session"`
	RunID   string `json:"runId,omitempty"`
}

type ModelsSetParams struct {
	Model string `json:"model"`
}

type ModelsSetPayload struct {
	Current  string `json:"current"`
	Previous string `json:"previous"`
}

type ProgressPayload struct {
	Content string `json:"content"`
}

type ToolStartPayload struct {
	Name  string `json:"name"`
	Input string `json:"input"`
	ID    string `json:"id"`
}

type ThinkingPayload struct {
	Content string `json:"content"`
}

type ToolDonePayload struct {
	Name                    string `json:"name"`
	Output                  string `json:"output"`
	Error                   string `json:"error,omitempty"`
	ID                      string `json:"id"`
	DeliveredToRequestTarget bool   `json:"deliveredToRequestTarget,omitempty"`
}

type ChatDonePayload struct {
	Content string `json:"content"`
}

type ChatErrorPayload struct {
	Message string `json:"message"`
}

type ThinkingDonePayload struct {
	Content string `json:"content"`
}

type UsagePayload struct {
	PromptTokens     int `json:"promptTokens"`
	CompletionTokens int `json:"completionTokens"`
	TotalTokens      int `json:"totalTokens"`
	ContextWindow    int `json:"contextWindow"`
}

type SessionInfo struct {
	Key       string `json:"key"`
	UpdatedAt string `json:"updatedAt,omitempty"`
	Preview   string `json:"preview,omitempty"`
}

type ModelInfo struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Provider    string `json:"provider"`
	Description string `json:"description,omitempty"`
	Source      string `json:"source,omitempty"`
	Capability  string `json:"capability,omitempty"`
	Selectable  bool   `json:"selectable"`
}

type HistoryMessage struct {
	Role      string `json:"role"`
	Content   string `json:"content"`
	Timestamp string `json:"timestamp,omitempty"`
}

// ChannelMessagePayload represents an inbound or outbound channel message.
type ChannelMessagePayload struct {
	Channel string `json:"channel"`
	ChatID  string `json:"chatID"`
	Content string `json:"content"`
}

// ChannelErrorPayload represents an error from channel processing.
type ChannelErrorPayload struct {
	Channel string `json:"channel"`
	ChatID  string `json:"chatID"`
	Error   string `json:"error"`
}

// CompressionInfo contains context compression state for UI display
type CompressionInfo struct {
	Enabled          bool    `json:"enabled"`
	Mode             string  `json:"mode"`              // conservative, default, aggressive
	LastRun          string  `json:"lastRun,omitempty"` // ISO timestamp
	OriginalTokens   int     `json:"originalTokens"`
	CompressedTokens int     `json:"compressedTokens"`
	ReductionPercent float64 `json:"reductionPercent"` // 0-100
}

// UsageLevel categorizes token usage for color coding
type UsageLevel int

const (
	UsageLevelLow      UsageLevel = iota // < 60%
	UsageLevelMedium                     // 60-80%
	UsageLevelHigh                       // 80-90%
	UsageLevelCritical                   // > 90%
)
