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
		toolCalls := make([]ToolCall, len(out.ToolCalls))
		copy(toolCalls, out.ToolCalls)
		for i := range toolCalls {
			if normalized, ok := idMap[toolCalls[i].ID]; ok {
				toolCalls[i].ID = normalized
			}
			toolCalls[i].Function.Arguments = repairJSON(toolCalls[i].Function.Arguments)
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
	for _, msg := range msgs {
		for _, toolCall := range msg.ToolCalls {
			if toolCall.ID == "" {
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
	if len(value) >= 9 {
		return value[:9]
	}
	if len(value) > 0 {
		return value
	}

	hash := sha256.Sum256([]byte(id))
	return hex.EncodeToString(hash[:])[:9]
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
		return raw
	}

	var compact bytes.Buffer
	if err := json.Compact(&compact, []byte(repaired)); err == nil {
		return compact.String()
	}
	return repaired
}

func supportsThinking(providerName string) bool {
	name := strings.ToLower(providerName)
	return strings.Contains(name, "anthropic")
}

func supportsReasoning(providerName string) bool {
	name := strings.ToLower(providerName)
	return strings.Contains(name, "anthropic") || strings.Contains(name, "deepseek")
}
