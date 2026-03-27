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
