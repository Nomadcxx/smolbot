package compression

import (
	"fmt"
	"strings"
)

type Compressor struct {
	config Config
}

func NewCompressor(config Config) *Compressor {
	if config.KeepRecentMessages == 0 {
		config.KeepRecentMessages = DefaultKeepRecentMessages
	}
	return &Compressor{config: config}
}

func (c *Compressor) Compress(messages []any) Result {
	if !c.config.Enabled || len(messages) == 0 {
		return Result{CompressedMessages: messages}
	}
	
	keepRecent := c.config.KeepRecentMessages

	// Separate messages by type and position
	var systemMessages []any
	var compressible []any
	var recentMessages []any

	splitIdx := len(messages) - keepRecent
	if splitIdx < 0 {
		splitIdx = 0
	}

	// Scan backward from split point to avoid splitting tool_call/result pairs
	for i := splitIdx; i < len(messages); i++ {
		m := messages[i].(map[string]any)
		if getRole(m) == "tool" {
			tcID, ok := m["tool_call_id"].(string)
			if !ok {
				continue
			}
			// Find the paired assistant message and move boundary to include it
			for j := i - 1; j >= 0; j-- {
				if msgs, ok := messages[j].(map[string]any); ok {
					if getRole(msgs) == "assistant" {
						if tcs, ok := msgs["tool_calls"].([]any); ok {
							for _, tc := range tcs {
								if tcMap, ok := tc.(map[string]any); ok {
									if id, ok := tcMap["id"].(string); ok && id == tcID {
										splitIdx = j
										goto adjusted
									}
								}
							}
						}
					}
				}
			}
		}
	}
adjusted:

	for i, msg := range messages {
		m := msg.(map[string]any)
		role := getRole(m)

		if role == "system" {
			systemMessages = append(systemMessages, msg)
		} else if i >= splitIdx {
			recentMessages = append(recentMessages, msg)
		} else {
			compressible = append(compressible, msg)
		}
	}
	
	// Compress messages in the middle
	compressed := make([]any, 0, len(compressible))
	preservedInfo := PreservedInfo{RecentMessages: len(recentMessages)}
	
	for _, msg := range compressible {
		compressedMsg := c.compressMessage(msg, &preservedInfo)
		compressed = append(compressed, compressedMsg)
	}
	
	// Combine: system + compressed middle + recent (uncompressed)
	result := make([]any, 0, len(messages))
	result = append(result, systemMessages...)
	result = append(result, compressed...)
	result = append(result, recentMessages...)
	
	return Result{
		CompressedMessages: result,
		PreservedInfo:     preservedInfo,
	}
}

func (c *Compressor) compressMessage(msg any, info *PreservedInfo) any {
	m := msg.(map[string]any)
	role := getRole(m)
	
	switch role {
	case "user":
		return c.compressUserMessage(msg)
	case "assistant":
		return c.compressAssistantMessage(msg, info)
	case "tool":
		return c.compressToolMessage(msg, info)
	default:
		return msg
	}
}

func (c *Compressor) compressUserMessage(msg any) any {
	m := msg.(map[string]any)
	content := getContent(m)
	
	threshold := UserMessageThresholdDefault
	limit := TruncationLimitDefault
	
	if c.config.Mode == ModeConservative {
		threshold = UserMessageThresholdConservative
		limit = TruncationLimitConservative
	} else if c.config.Mode == ModeAggressive {
		limit = TruncationLimitAggressive
	}
	
	if len(content) <= threshold {
		return msg
	}
	
	m["content"] = truncateAtSentence(content, limit)
	return m
}

func (c *Compressor) compressAssistantMessage(msg any, info *PreservedInfo) any {
	m := msg.(map[string]any)
	
	// Always preserve tool_calls structure - mark as key decision
	hasToolCalls := false
	if tcs, ok := m["tool_calls"].([]map[string]any); ok && len(tcs) > 0 {
		hasToolCalls = true
		info.KeyDecisions += len(tcs)
	} else if tcs, ok := m["tool_calls"].([]any); ok && len(tcs) > 0 {
		hasToolCalls = true
		info.KeyDecisions += len(tcs)
	}
	
	if hasToolCalls {
		// Only truncate content if very long and not conservative
		if c.config.Mode != ModeConservative {
			content := getContent(m)
			if len(content) > AssistantWithToolsThreshold {
				m["content"] = truncateAtSentence(content, TruncationLimitDefault)
			}
		}
		return msg
	}
	
	// For regular assistant messages
	if c.config.Mode == ModeConservative {
		return msg // Conservative: don't compress assistant
	}
	
	content := getContent(m)
	limit := TruncationLimitDefault
	if c.config.Mode == ModeAggressive {
		limit = TruncationLimitAggressive
	}
	
	if len(content) > UserMessageThresholdDefault {
		m["content"] = truncateAtSentence(content, limit)
	}
	return msg
}

func (c *Compressor) compressToolMessage(msg any, info *PreservedInfo) any {
	m := msg.(map[string]any)
	info.ToolResults++
	
	// Track file modifications
	if name, ok := m["name"].(string); ok {
		if isFileModificationTool(name) {
			info.FileModifications++
		}
	}
	
	content := getContent(m)
	toolName, _ := m["name"].(string)
	
	switch c.config.Mode {
	case ModeAggressive:
		m["content"] = compressToolAggressive(content)
	case ModeConservative:
		m["content"] = compressToolConservative(content)
	default:
		m["content"] = compressToolDefault(content, toolName)
	}
	
	return msg
}

// Helper functions

func getRole(m map[string]any) string {
	if role, ok := m["role"].(string); ok {
		return role
	}
	return ""
}

func getContent(m map[string]any) string {
	switch content := m["content"].(type) {
	case string:
		return content
	case []any:
		var builder strings.Builder
		for _, item := range content {
			if block, ok := item.(map[string]any); ok {
				if text, ok := block["text"].(string); ok {
					builder.WriteString(text)
				}
			}
		}
		return builder.String()
	}
	return ""
}

func truncateAtSentence(text string, limit int) string {
	if len(text) <= limit {
		return text
	}
	
	// Try to truncate at sentence boundary
	sentences := strings.Split(text, ". ")
	var result strings.Builder
	for _, sentence := range sentences {
		if result.Len()+len(sentence) > limit-3 {
			break
		}
		if result.Len() > 0 {
			result.WriteString(". ")
		}
		result.WriteString(sentence)
	}
	
	summary := result.String()
	// If no sentences were added (single long word/sentence without periods), truncate directly
	cutoff := limit - 3
	if cutoff < 0 {
		cutoff = 0
	}
	if cutoff > len(text) {
		cutoff = len(text)
	}
	if len(summary) == 0 {
		summary = text[:cutoff] + "..."
	} else if len(summary) < limit/2 {
		// Fallback: character truncation if sentence boundaries gave very short result
		summary = text[:cutoff] + "..."
	} else if !strings.HasSuffix(summary, "...") {
		summary += "..."
	}
	return summary
}

func isFileModificationTool(name string) bool {
	name = strings.ToLower(name)
	patterns := []string{"write", "edit", "create", "modify", "delete", "move", "copy"}
	for _, p := range patterns {
		if strings.Contains(name, p) {
			return true
		}
	}
	return false
}

func compressToolAggressive(content string) string {
	lower := strings.ToLower(content)
	
	// Preserve success indicators
	if strings.Contains(lower, "success") || strings.Contains(lower, "created") ||
	   strings.Contains(lower, "updated") || strings.Contains(lower, "saved") {
		return "Result: success"
	}
	
	// Extract error line
	if strings.Contains(lower, "error") {
		for _, line := range strings.Split(content, "\n") {
			if strings.Contains(strings.ToLower(line), "error") {
				return strings.TrimSpace(line)
			}
		}
	}
	
	return truncateAtSentence(content, TruncationLimitAggressive)
}

func compressToolConservative(content string) string {
	lines := strings.Split(content, "\n")
	if len(lines) <= 3 {
		return content
	}
	return strings.Join(lines[:3], "\n") + "\n[+ " + fmt.Sprintf("%d more lines", len(lines)-3) + "]"
}

func compressToolDefault(content, toolName string) string {
	lower := strings.ToLower(content)
	
	// Preserve success indicators
	if strings.Contains(lower, "success") || strings.Contains(lower, "created") ||
	   strings.Contains(lower, "updated") || strings.Contains(lower, "saved") {
		return "Result: success"
	}
	
	// Extract error line
	if strings.Contains(lower, "error") {
		for _, line := range strings.Split(content, "\n") {
			if strings.Contains(strings.ToLower(line), "error") {
				return strings.TrimSpace(line)
			}
		}
	}
	
	// Keep first significant line if short
	lines := strings.Split(content, "\n")
	if len(lines) > 0 && len(lines[0]) < 100 {
		return lines[0]
	}
	
	return truncateAtSentence(content, TruncationLimitDefault)
}
