package provider

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"regexp"
	"strings"
)

var malformedKeyPattern = regexp.MustCompile(`([{,]\s*)([A-Za-z_][A-Za-z0-9_-]*)(\s*:)`)
var trailingCommaPattern = regexp.MustCompile(`,\s*([}\]])`)

func SanitizeMessages(msgs []Message, providerName string) []Message {
	idMap := buildIDMap(msgs)
	sanitized := make([]Message, len(msgs))
	for i, msg := range msgs {
		sanitized[i] = sanitizeMessage(msg, providerName, idMap)
	}
	return sanitized
}

func sanitizeMessage(msg Message, providerName string, idMap map[string]string) Message {
	out := msg
	out.Content = sanitizeContent(msg.Content)

	if out.ToolCallID != "" {
		if normalized, ok := idMap[out.ToolCallID]; ok {
			out.ToolCallID = normalized
		}
	}

	if len(out.ToolCalls) > 0 {
		toolCalls := make([]ToolCall, 0, len(out.ToolCalls))
		for _, tc := range out.ToolCalls {
			tc.Function.Arguments = repairJSON(tc.Function.Arguments)
			// Skip corrupt tool calls with no function name.
			if tc.Function.Name == "" {
				continue
			}
			if tc.ID == "" {
				tc.ID = "call_sanitized_" + normalizeToolCallID(tc.Function.Name+tc.Function.Arguments)
			} else if normalized, ok := idMap[tc.ID]; ok {
				tc.ID = normalized
			}
			toolCalls = append(toolCalls, tc)
		}
		out.ToolCalls = toolCalls
	}

	if !supportsThinking(providerName) {
		out.ThinkingBlocks = nil
	}
	if !supportsReasoning(providerName) {
		out.ReasoningContent = ""
	}

	return out
}

func sanitizeContent(content any) any {
	switch value := content.(type) {
	case nil:
		return " "
	case string:
		if value == "" {
			return " "
		}
		return value
	case []ContentBlock:
		blocks := make([]ContentBlock, 0, len(value))
		for _, block := range value {
			if block.Type == "text" && block.Text == "" {
				continue
			}
			blocks = append(blocks, block)
		}
		if len(blocks) == 0 {
			return " "
		}
		return blocks
	case []any:
		blocks := make([]ContentBlock, 0, len(value))
		for _, item := range value {
			raw, err := json.Marshal(item)
			if err != nil {
				continue
			}
			var block ContentBlock
			if err := json.Unmarshal(raw, &block); err != nil {
				continue
			}
			if block.Type == "text" && block.Text == "" {
				continue
			}
			blocks = append(blocks, block)
		}
		if len(blocks) == 0 {
			return " "
		}
		return blocks
	default:
		raw, err := json.Marshal(value)
		if err != nil {
			return " "
		}
		return string(raw)
	}
}

func buildIDMap(msgs []Message) map[string]string {
	idMap := make(map[string]string)
	fallbackIdx := 0
	for _, msg := range msgs {
		for _, toolCall := range msg.ToolCalls {
			if toolCall.ID == "" {
				// Generate a stable fallback for empty IDs so the tool call
				// and its matching result stay linked after sanitization.
				fallbackIdx++
				continue
			}
			idMap[toolCall.ID] = normalizeToolCallID(toolCall.ID)
		}
		if msg.ToolCallID != "" {
			idMap[msg.ToolCallID] = normalizeToolCallID(msg.ToolCallID)
		}
	}
	return idMap
}

func normalizeToolCallID(id string) string {
	var cleaned strings.Builder
	for _, r := range strings.ToLower(id) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			cleaned.WriteRune(r)
		}
	}
	value := cleaned.String()
	if len(value) > 0 {
		return value
	}

	hash := sha256.Sum256([]byte(id))
	return hex.EncodeToString(hash[:])[:16]
}

func closeUnclosed(s string) string {
	var stack []byte
	inString := false
	escaped := false
	for i := 0; i < len(s); i++ {
		ch := s[i]
		if escaped {
			escaped = false
			continue
		}
		if ch == '\\' && inString {
			escaped = true
			continue
		}
		if ch == '"' {
			inString = !inString
			continue
		}
		if inString {
			continue
		}
		switch ch {
		case '{':
			stack = append(stack, '}')
		case '[':
			stack = append(stack, ']')
		case '}', ']':
			if len(stack) > 0 && stack[len(stack)-1] == ch {
				stack = stack[:len(stack)-1]
			}
		}
	}
	for i := len(stack) - 1; i >= 0; i-- {
		s += string(stack[i])
	}
	return s
}

func repairJSON(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return raw
	}
	if json.Valid([]byte(trimmed)) {
		var compact bytes.Buffer
		if err := json.Compact(&compact, []byte(trimmed)); err == nil {
			return compact.String()
		}
		return trimmed
	}

	repaired := malformedKeyPattern.ReplaceAllString(trimmed, `$1"$2"$3`)
	repaired = trailingCommaPattern.ReplaceAllString(repaired, `$1`)
	if !json.Valid([]byte(repaired)) {
		repaired = closeUnclosed(repaired)
	}
	if !json.Valid([]byte(repaired)) {
		return raw
	}

	var compact bytes.Buffer
	if err := json.Compact(&compact, []byte(repaired)); err == nil {
		return compact.String()
	}
	return repaired
}

func StripProviderPrefix(model string) string {
	if idx := strings.LastIndex(model, "/"); idx >= 0 {
		return model[idx+1:]
	}
	return model
}

func supportsThinking(providerName string) bool {
	name := strings.ToLower(providerName)
	return strings.Contains(name, "anthropic")
}

func supportsReasoning(providerName string) bool {
	name := strings.ToLower(providerName)
	return strings.Contains(name, "anthropic") || strings.Contains(name, "deepseek")
}
