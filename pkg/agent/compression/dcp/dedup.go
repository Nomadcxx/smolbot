package dcp

import (
	"encoding/json"
	"sort"
	"strings"

	"github.com/Nomadcxx/smolbot/pkg/provider"
)

const DedupPlaceholder = "[Output removed — superseded by later identical call]"

type toolCallLocation struct {
	messageIndex int
	callID       string
	toolName     string
}

func DeduplicateToolCalls(messages []provider.Message, cfg Config) int {
	signatures := make(map[string][]toolCallLocation)
	resultsByCallID := make(map[string]int)

	for i, msg := range messages {
		if msg.Role != "assistant" {
			continue
		}
		for _, call := range msg.ToolCalls {
			if IsToolProtected(call.Function.Name, append(cfg.ProtectedTools, cfg.Deduplication.ProtectedTools...)) {
				continue
			}
			signature := toolSignature(call.Function.Name, call.Function.Arguments)
			signatures[signature] = append(signatures[signature], toolCallLocation{
				messageIndex: i,
				callID:       call.ID,
				toolName:     call.Function.Name,
			})
		}
	}
	for i, msg := range messages {
		if msg.Role == "tool" && msg.ToolCallID != "" {
			resultsByCallID[msg.ToolCallID] = i
		}
	}

	count := 0
	for _, calls := range signatures {
		if len(calls) < 2 {
			continue
		}
		sort.SliceStable(calls, func(i, j int) bool {
			return calls[i].messageIndex < calls[j].messageIndex
		})
		for _, call := range calls[:len(calls)-1] {
			resultIndex, ok := resultsByCallID[call.callID]
			if !ok {
				continue
			}
			messages[resultIndex].Content = DedupPlaceholder
			count++
		}
	}
	return count
}

func toolSignature(name string, argsJSON string) string {
	return name + "::" + canonicalJSON(argsJSON)
}

func canonicalJSON(raw string) string {
	var value any
	if err := json.Unmarshal([]byte(raw), &value); err != nil {
		return strings.TrimSpace(raw)
	}
	canonical := canonicalizeValue(value)
	payload, err := json.Marshal(canonical)
	if err != nil {
		return strings.TrimSpace(raw)
	}
	return string(payload)
}

func canonicalizeValue(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		keys := make([]string, 0, len(typed))
		for key := range typed {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		ordered := make(map[string]any, len(typed))
		for _, key := range keys {
			ordered[key] = canonicalizeValue(typed[key])
		}
		return ordered
	case []any:
		out := make([]any, len(typed))
		for i, item := range typed {
			out[i] = canonicalizeValue(item)
		}
		return out
	default:
		return typed
	}
}
