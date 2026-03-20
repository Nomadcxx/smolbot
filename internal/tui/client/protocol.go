package client

import "encoding/json"

const (
	FrameReq   = "request"
	FrameRes   = "response"
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
	Type   string          `json:"type"`
	ID     string          `json:"id"`
	Result json.RawMessage `json:"result,omitempty"`
	Error  *ErrorShape     `json:"error,omitempty"`
}

type Event struct {
	Type    string          `json:"type"`
	Event   string          `json:"name"`
	Payload json.RawMessage `json:"event,omitempty"`
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
	RunID string `json:"runId,omitempty"`
}

type ProgressPayload struct {
	Content string `json:"content"`
}

type ToolStartPayload struct {
	Name  string `json:"name"`
	Input string `json:"input,omitempty"`
}

type ToolDonePayload struct {
	Name                     string `json:"name"`
	DeliveredToRequestTarget bool   `json:"deliveredToRequestTarget,omitempty"`
	Output                   string `json:"output,omitempty"`
	Error                    string `json:"error,omitempty"`
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

type StatusPayload struct {
	Model            string   `json:"model"`
	Provider         string   `json:"provider"`
	UptimeSeconds    int      `json:"uptimeSeconds"`
	Channels         []string `json:"channels"`
	ConnectedClients int      `json:"connectedClients"`
}

type SessionInfo struct {
	Key       string `json:"key"`
	UpdatedAt string `json:"updatedAt,omitempty"`
	Preview   string `json:"preview,omitempty"`
}

type ModelInfo struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Provider string `json:"provider"`
}

type HistoryMessage struct {
	Role      string `json:"role"`
	Content   string `json:"content"`
	Timestamp string `json:"timestamp,omitempty"`
}
