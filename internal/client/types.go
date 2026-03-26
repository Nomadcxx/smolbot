package client

// UsageInfo contains token usage information
type UsageInfo struct {
	PromptTokens     int `json:"promptTokens"`
	CompletionTokens int `json:"completionTokens"`
	TotalTokens      int `json:"totalTokens"`
	ContextWindow    int `json:"contextWindow"`
}

// ChannelStatus represents a messaging channel state
type ChannelStatus struct {
	Name   string `json:"name"`
	Status string `json:"status"`
}

// StatusPayload contains full system status
type StatusPayload struct {
	Model    string          `json:"model"`
	Session  string          `json:"session,omitempty"`
	Usage    UsageInfo       `json:"usage"`
	Uptime   int             `json:"uptime"`
	Channels []ChannelStatus `json:"channels,omitempty"`
}

// MCPServerInfo represents an MCP server exposed by the gateway.
type MCPServerInfo struct {
	Name   string `json:"name"`
	Status string `json:"status"`
	Tools  int    `json:"tools"`
}

// CronJob represents a scheduled task exposed by the gateway.
type CronJob struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Schedule string `json:"schedule"`
	Status   string `json:"status"`
	NextRun  string `json:"nextRun"`
}
