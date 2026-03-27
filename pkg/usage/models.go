package usage

import "time"

type CompletionRecord struct {
	SessionKey       string
	ProviderID       string
	ModelName        string
	RequestType      string
	PromptTokens     int
	CompletionTokens int
	TotalTokens      int
	DurationMS       int
	Status           string
	UsageSource      string
	RecordedAt       time.Time
}

type UsageRecord struct {
	ID               int64
	SessionKey       string
	ProviderID       string
	ModelName        string
	RequestType      string
	PromptTokens     int
	CompletionTokens int
	TotalTokens      int
	DurationMS       int
	Status           string
	UsageSource      string
	CreatedAt        time.Time
}

type Summary struct {
	TotalRequests         int
	TotalPromptTokens     int
	TotalCompletionTokens int
	TotalTokens           int
}

type ProviderSummary struct {
	ProviderID      string
	ModelName       string
	SessionKey      string
	SessionTokens   int
	TodayTokens     int
	WeeklyTokens    int
	SessionRequests int
	TodayRequests   int
	WeeklyRequests  int
	BudgetStatus    string
	WarningLevel    string
}

type Budget struct {
	ID              string
	Name            string
	BudgetType      string
	LimitAmount     float64
	LimitUnit       string
	ScopeType       string
	ScopeTarget     string
	AlertThresholds []int
	IsActive        bool
	WindowStart     *time.Time
	WindowEnd       *time.Time
	ResetsAt        *time.Time
}

type BudgetAlert struct {
	ID               int64
	BudgetID         string
	AlertType        string
	ThresholdPercent int
	TokensAtAlert    int
	SentAt           time.Time
	Channel          string
}

type HistoricalUsageSample struct {
	ID            int64
	ProviderID    string
	ModelName     string
	WindowType    string
	Source        string
	SampledAt     time.Time
	UsedPercent   float64
	ResetsAt      time.Time
	WindowMinutes int
	TotalTokens   int
}
