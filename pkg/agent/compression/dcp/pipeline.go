package dcp

import (
	"encoding/json"

	"github.com/Nomadcxx/smolbot/pkg/provider"
	"github.com/Nomadcxx/smolbot/pkg/tokenizer"
)

type TransformStats struct {
	MessagesIn      int
	MessagesOut     int
	TokensEstimated int
	DedupCount      int
	ErrorPurgeCount int
	BlocksApplied   int
	NudgesInjected  int
}

func Transform(messages []provider.Message, state *State, cfg Config, tok *tokenizer.Tokenizer, contextWindow int) ([]provider.Message, TransformStats) {
	if state == nil {
		state = NewState("")
	}
	if err := cfg.Validate(); err != nil {
		cfg = DefaultConfig()
	}
	out := deepCopyMessages(messages)
	state.CurrentTurn = countUserTurns(out)
	state.RequestCount++

	stats := TransformStats{
		MessagesIn: len(messages),
	}
	if cfg.Deduplication.Enabled {
		stats.DedupCount = DeduplicateToolCalls(out, cfg)
		state.Stats.TotalDedups += stats.DedupCount
	}
	if cfg.PurgeErrors.Enabled {
		stats.ErrorPurgeCount = PurgeErroredInputs(out, state.CurrentTurn, cfg)
		state.Stats.TotalErrorPurges += stats.ErrorPurgeCount
	}
	if shouldApplyDCPMetadata(out, state, cfg, tok, contextWindow) {
		AssignMessageIDs(out, state, cfg)
		out = ApplyCompressionBlocks(out, state)
		stats.BlocksApplied = len(state.ActiveBlocks())
		stats.NudgesInjected = InjectNudges(out, state, cfg, tok, contextWindow)
	}
	stats.MessagesOut = len(out)
	if tok != nil {
		stats.TokensEstimated = tok.EstimatePromptTokens(out)
	}
	return out, stats
}

func shouldApplyDCPMetadata(messages []provider.Message, state *State, cfg Config, tok *tokenizer.Tokenizer, contextWindow int) bool {
	if cfg.CompressTool.Enabled {
		return true
	}
	if len(state.ActiveBlocks()) > 0 {
		return true
	}
	tier, _ := pendingNudge(messages, state, cfg, tok, contextWindow)
	return tier != ""
}

func deepCopyMessages(msgs []provider.Message) []provider.Message {
	out := make([]provider.Message, len(msgs))
	for i, msg := range msgs {
		out[i] = provider.Message{
			Role:             msg.Role,
			Content:          cloneContent(msg.Content),
			ToolCallID:       msg.ToolCallID,
			Name:             msg.Name,
			ReasoningContent: msg.ReasoningContent,
		}
		if len(msg.ToolCalls) > 0 {
			out[i].ToolCalls = append([]provider.ToolCall(nil), msg.ToolCalls...)
		}
		if len(msg.ThinkingBlocks) > 0 {
			out[i].ThinkingBlocks = append([]provider.ThinkingBlock(nil), msg.ThinkingBlocks...)
		}
	}
	return out
}

func cloneContent(content any) any {
	switch value := content.(type) {
	case nil:
		return nil
	case string:
		return value
	case []provider.ContentBlock:
		return append([]provider.ContentBlock(nil), value...)
	case []any:
		return cloneJSONValue(value)
	case map[string]any:
		return cloneJSONValue(value)
	default:
		return cloneJSONValue(value)
	}
}

func cloneJSONValue(value any) any {
	payload, err := json.Marshal(value)
	if err != nil {
		return value
	}
	var cloned any
	if err := json.Unmarshal(payload, &cloned); err != nil {
		return value
	}
	return cloned
}

func countUserTurns(messages []provider.Message) int {
	count := 0
	for _, msg := range messages {
		if msg.Role == "user" {
			count++
		}
	}
	return count
}
