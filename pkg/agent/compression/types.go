package compression

import "time"

type Mode string

const (
	ModeConservative Mode = "conservative" // Minimal compression, preserve detail
	ModeDefault      Mode = "default"       // Balanced compression
	ModeAggressive   Mode = "aggressive"   // Maximum compression, may lose nuance
)

type Config struct {
	Enabled            bool  `json:"enabled"`
	Mode               Mode  `json:"mode"`
	ThresholdPercent   int   `json:"thresholdPercent"`   // Trigger at % of context window used
	KeepRecentMessages int   `json:"keepRecentMessages"` // Messages to preserve at full detail
}

type Result struct {
	CompressedMessages   []any              `json:"-"`
	OriginalTokenCount   int                `json:"originalTokenCount"`
	CompressedTokenCount int               `json:"compressedTokenCount"`
	ReductionPercentage  float64            `json:"reductionPercentage"`
	PreservedInfo        PreservedInfo     `json:"preservedInfo"`
}

type PreservedInfo struct {
	KeyDecisions       int `json:"keyDecisions"`       // Assistant msgs with tool calls
	FileModifications  int `json:"fileModifications"` // write/edit/create tools
	ToolResults        int `json:"toolResults"`        // Tool response count
	RecentMessages     int `json:"recentMessages"`     // Messages kept uncompressed
}

type Stats struct {
	LastCompression      *Result  `json:"lastCompression"`
	LastCompressionTime time.Time `json:"lastCompressionTime"`
	TotalCompressions   int      `json:"totalCompressions"`
	TotalTokensSaved    int      `json:"totalTokensSaved"`
}

// ContextState tracks current context usage for compression decisions
type ContextState struct {
	TotalTokens     int `json:"totalTokens"`
	ContextWindow   int `json:"contextWindow"`
	RemainingTokens int `json:"remainingTokens"`
	UsagePercent    int `json:"usagePercent"` // 0-100
}

// Compression thresholds (inspired by nanocoder)
const (
	DefaultKeepRecentMessages         = 2
	DefaultThresholdPercent          = 60  // Trigger at 60% context usage
	MinThresholdPercent             = 50
	MaxThresholdPercent             = 95
	
	// Character limits per message type (soft limits for truncation)
	UserMessageThresholdDefault      = 500
	UserMessageThresholdConservative = 1000
	AssistantWithToolsThreshold     = 300
	
	// Hard truncation limits
	TruncationLimitAggressive      = 100
	TruncationLimitDefault         = 200
	TruncationLimitConservative    = 500
)
