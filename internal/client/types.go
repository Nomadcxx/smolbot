package client

import "time"

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

type UsageSummary struct {
	ProviderID      string        `json:"providerId"`
	ModelName       string        `json:"modelName"`
	SessionTokens   int           `json:"sessionTokens"`
	TodayTokens     int           `json:"todayTokens"`
	WeeklyTokens    int           `json:"weeklyTokens"`
	SessionRequests int           `json:"sessionRequests,omitempty"`
	TodayRequests   int           `json:"todayRequests,omitempty"`
	WeeklyRequests  int           `json:"weeklyRequests,omitempty"`
	EstimatedCost   string        `json:"estimatedCost,omitempty"`
	BudgetStatus    string        `json:"budgetStatus,omitempty"`
	WarningLevel    string        `json:"warningLevel,omitempty"`
	Quota           *QuotaSummary `json:"quota,omitempty"`
}

type QuotaSummary struct {
	ProviderID           string    `json:"providerId"`
	AccountName          string    `json:"accountName,omitempty"`
	AccountEmail         string    `json:"accountEmail,omitempty"`
	PlanName             string    `json:"planName,omitempty"`
	SessionUsedPercent   float64   `json:"sessionUsedPercent,omitempty"`
	SessionResetsAt      time.Time `json:"sessionResetsAt,omitempty"`
	WeeklyUsedPercent    float64   `json:"weeklyUsedPercent,omitempty"`
	WeeklyResetsAt       time.Time `json:"weeklyResetsAt,omitempty"`
	NotifyUsageLimits    bool      `json:"notifyUsageLimits,omitempty"`
	State                string    `json:"state,omitempty"`
	Source               string    `json:"source,omitempty"`
	FetchedAt            time.Time `json:"fetchedAt,omitempty"`
	ExpiresAt            time.Time `json:"expiresAt,omitempty"`
	IdentityState        string    `json:"identityState,omitempty"`
	IdentitySource       string    `json:"identitySource,omitempty"`
	IdentityAccountName  string    `json:"identityAccountName,omitempty"`
	IdentityAccountEmail string    `json:"identityAccountEmail,omitempty"`
}

type UsageAlert struct {
	ProviderID   string `json:"providerId"`
	ModelName    string `json:"modelName"`
	BudgetStatus string `json:"budgetStatus,omitempty"`
	WarningLevel string `json:"warningLevel,omitempty"`
	Message      string `json:"message,omitempty"`
}

// StatusPayload contains full system status
type StatusPayload struct {
	Model          string          `json:"model"`
	Provider       string          `json:"provider,omitempty"`
	Session        string          `json:"session,omitempty"`
	Usage          UsageInfo       `json:"usage"`
	PersistedUsage *UsageSummary   `json:"persistedUsage,omitempty"`
	UsageAlert     *UsageAlert     `json:"usageAlert,omitempty"`
	Uptime         int             `json:"uptime"`
	Channels       []ChannelStatus `json:"channels,omitempty"`
}

type CompactResult struct {
	Session          string  `json:"session,omitempty"`
	Compacted        bool    `json:"compacted"`
	Reason           string  `json:"reason,omitempty"`
	OriginalTokens   int     `json:"originalTokens"`
	CompressedTokens int     `json:"compressedTokens"`
	ReductionPercent float64 `json:"reductionPercent"`
}

type SkillInfo struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Status      string `json:"status"`
}

type MCPServerInfo struct {
	Name    string `json:"name"`
	Command string `json:"command,omitempty"`
	Status  string `json:"status"`
	Tools   int    `json:"tools,omitempty"`
}

// CronJob represents a scheduled task exposed by the gateway.
type CronJob struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Schedule string `json:"schedule"`
	Status   string `json:"status"`
	NextRun  string `json:"nextRun"`
}

// AgentSpawnedPayload describes a delegated child agent that has been started.
type AgentSpawnedPayload struct {
	ID              string `json:"id"`
	Name            string `json:"name"`
	AgentType       string `json:"agentType"`
	Model           string `json:"model,omitempty"`
	ReasoningEffort string `json:"reasoningEffort,omitempty"`
	Description     string `json:"description"`
	PromptPreview   string `json:"promptPreview,omitempty"`
}

// AgentCompletedPayload describes a delegated child agent that has finished.
type AgentCompletedPayload struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	AgentType     string `json:"agentType"`
	Status        string `json:"status"`
	Description   string `json:"description,omitempty"`
	PromptPreview string `json:"promptPreview,omitempty"`
	Summary       string `json:"summary,omitempty"`
	Error         string `json:"error,omitempty"`
}

// AgentWaitAgent identifies a child agent included in a wait state.
type AgentWaitAgent struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	AgentType string `json:"agentType"`
}

// AgentWaitStartedPayload describes outstanding delegated children.
type AgentWaitStartedPayload struct {
	Count  int              `json:"count"`
	Agents []AgentWaitAgent `json:"agents"`
}

// AgentWaitResult describes a completed child agent in a wait summary.
type AgentWaitResult struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	AgentType     string `json:"agentType"`
	Status        string `json:"status"`
	Description   string `json:"description,omitempty"`
	PromptPreview string `json:"promptPreview,omitempty"`
	Summary       string `json:"summary,omitempty"`
	Error         string `json:"error,omitempty"`
}

// AgentWaitCompletedPayload describes the results of a completed wait.
type AgentWaitCompletedPayload struct {
	Count   int               `json:"count"`
	Results []AgentWaitResult `json:"results"`
}
