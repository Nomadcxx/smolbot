package dcp

import (
	"strings"

	"github.com/Nomadcxx/smolbot/pkg/provider"
)

const ErrorInputPlaceholder = "[input removed — failed tool call]"

func PurgeErroredInputs(messages []provider.Message, currentTurn int, cfg Config) int {
	// Build call-ID → (message index, toolcall index) map for O(1) lookup.
	type callLoc struct{ msgIdx, tcIdx int }
	callIndex := make(map[string]callLoc)
	for j, msg := range messages {
		if msg.Role != "assistant" {
			continue
		}
		for k, tc := range msg.ToolCalls {
			callIndex[tc.ID] = callLoc{j, k}
		}
	}

	count := 0
	for i, msg := range messages {
		if msg.Role != "tool" || !isErrorResult(msg.StringContent()) {
			continue
		}
		if IsToolProtected(msg.Name, append(cfg.ProtectedTools, cfg.PurgeErrors.ProtectedTools...)) {
			continue
		}
		callTurn := turnForMessage(messages, i)
		if currentTurn-callTurn < cfg.PurgeErrors.TurnThreshold {
			continue
		}
		if loc, ok := callIndex[msg.ToolCallID]; ok {
			messages[loc.msgIdx].ToolCalls[loc.tcIdx].Function.Arguments = ErrorInputPlaceholder
			count++
		}
	}
	return count
}

func isErrorResult(content string) bool {
	lower := strings.ToLower(strings.TrimSpace(content))
	switch {
	case strings.HasPrefix(lower, "error:"),
		strings.HasPrefix(lower, "fail"),
		strings.HasPrefix(lower, "panic:"),
		strings.HasPrefix(lower, "fatal:"):
		return true
	case strings.Contains(lower, "exit code") && !strings.Contains(lower, "exit code 0"):
		return true
	default:
		return false
	}
}
