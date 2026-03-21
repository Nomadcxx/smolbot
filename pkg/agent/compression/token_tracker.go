package compression

import (
	"github.com/Nomadcxx/smolbot/pkg/provider"
	"github.com/Nomadcxx/smolbot/pkg/tokenizer"
)

type TokenTracker struct {
	tok *tokenizer.Tokenizer
}

func NewTokenTracker(t *tokenizer.Tokenizer) *TokenTracker {
	return &TokenTracker{tok: t}
}

// ShouldCompress returns true if compression threshold is exceeded
func (tt *TokenTracker) ShouldCompress(messages []any, contextWindowTokens, thresholdPercent int) bool {
	if contextWindowTokens == 0 || thresholdPercent == 0 {
		return false
	}
	
	totalTokens := tt.CountTokens(messages)
	usagePercent := (totalTokens * 100) / contextWindowTokens
	
	return usagePercent >= thresholdPercent
}

// CalculateStats computes compression statistics
func (tt *TokenTracker) CalculateStats(original, compressed []any) (orig, comp int, reduction float64) {
	orig = tt.CountTokens(original)
	comp = tt.CountTokens(compressed)
	
	if orig > 0 {
		reduction = float64(orig-comp) * 100 / float64(orig)
	}
	
	return orig, comp, reduction
}

// GetContextState returns current context usage
func (tt *TokenTracker) GetContextState(messages []any, contextWindow int) ContextState {
	total := tt.CountTokens(messages)
	remaining := contextWindow - total
	if remaining < 0 {
		remaining = 0
	}
	
	usagePercent := 0
	if contextWindow > 0 {
		usagePercent = (total * 100) / contextWindow
	}
	
	return ContextState{
		TotalTokens:     total,
		ContextWindow:   contextWindow,
		RemainingTokens: remaining,
		UsagePercent:    usagePercent,
	}
}

// CountTokens estimates token count for messages
func (tt *TokenTracker) CountTokens(messages []any) int {
	if tt.tok == nil {
		return tt.fallbackCount(messages)
	}
	
	// Convert []any to []provider.Message
	providerMsgs := toProviderMessages(messages)
	return tt.tok.EstimatePromptTokens(providerMsgs)
}

func (tt *TokenTracker) fallbackCount(messages []any) int {
	// Rough estimate: ~4 chars per token average
	total := 0
	for _, msg := range messages {
		m := msg.(map[string]any)
		content := getContent(m)
		total += len(content) / 4
		total += 10 // overhead per message
	}
	return total
}

func toProviderMessages(a []any) []provider.Message {
	result := make([]provider.Message, 0, len(a))
	for _, item := range a {
		m := item.(map[string]any)
		msg := provider.Message{
			Role:    getString(m, "role"),
			Content: m["content"],
			Name:    getString(m, "name"),
		}
		if toolCalls, ok := m["tool_calls"].([]any); ok {
			for _, tc := range toolCalls {
				tcm := tc.(map[string]any)
				fn := getMap(tcm, "function")
				msg.ToolCalls = append(msg.ToolCalls, provider.ToolCall{
					ID: getString(tcm, "id"),
					Function: provider.FunctionCall{
						Name:      getString(fn, "name"),
						Arguments: getString(fn, "arguments"),
					},
				})
			}
		}
		result = append(result, msg)
	}
	return result
}

func getString(m map[string]any, key string) string {
	if v, ok := m[key].(string); ok {
		return v
	}
	return ""
}

func getMap(m map[string]any, key string) map[string]any {
	if v, ok := m[key].(map[string]any); ok {
		return v
	}
	return nil
}
