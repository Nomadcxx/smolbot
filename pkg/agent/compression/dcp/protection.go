package dcp

import (
	"path/filepath"
	"strings"

	"github.com/Nomadcxx/smolbot/pkg/provider"
)

var DefaultProtectedTools = []string{
	"write_file",
	"edit_file",
	"create_file",
	"compress",
	"message",
	"spawn",
	"task",
	"wait",
}

func IsToolProtected(toolName string, protectedPatterns []string) bool {
	// Check default patterns first, then user-supplied patterns.
	for _, pattern := range DefaultProtectedTools {
		pattern = strings.TrimSpace(pattern)
		if pattern == "" {
			continue
		}
		if pattern == toolName {
			return true
		}
		if matched, _ := filepath.Match(pattern, toolName); matched {
			return true
		}
	}
	for _, pattern := range protectedPatterns {
		pattern = strings.TrimSpace(pattern)
		if pattern == "" {
			continue
		}
		if pattern == toolName {
			return true
		}
		if matched, _ := filepath.Match(pattern, toolName); matched {
			return true
		}
	}
	return false
}

func IsTurnProtected(msgTurn, totalTurns, turnProtection int) bool {
	if turnProtection <= 0 || totalTurns <= 0 {
		return false
	}
	return msgTurn > 0 && msgTurn > totalTurns-turnProtection
}

func IsMessageProtected(msg provider.Message, msgIndex int, messages []provider.Message, currentTurn int, cfg Config) bool {
	if msg.Role == "user" && cfg.CompressTool.ProtectUserMsgs {
		return true
	}

	if toolName := messageToolName(msg); toolName != "" {
		if IsToolProtected(toolName, combinedProtectedTools(cfg)) {
			return true
		}
	}

	msgTurn := turnForMessage(messages, msgIndex)
	return IsTurnProtected(msgTurn, currentTurn, cfg.TurnProtection)
}

func combinedProtectedTools(cfg Config) []string {
	combined := append([]string{}, cfg.ProtectedTools...)
	combined = append(combined, cfg.Deduplication.ProtectedTools...)
	combined = append(combined, cfg.PurgeErrors.ProtectedTools...)
	combined = append(combined, cfg.CompressTool.ProtectedTools...)
	return combined
}

func messageToolName(msg provider.Message) string {
	if msg.Role == "tool" && msg.Name != "" {
		return msg.Name
	}
	if len(msg.ToolCalls) > 0 {
		return msg.ToolCalls[0].Function.Name
	}
	return ""
}

func turnForMessage(messages []provider.Message, msgIndex int) int {
	turn := 0
	for i, msg := range messages {
		if msg.Role == "user" {
			turn++
		}
		if i == msgIndex {
			return turn
		}
	}
	return turn
}
