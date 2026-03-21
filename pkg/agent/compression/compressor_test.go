package compression

import (
	"strings"
	"testing"
)

func TestCompressorPreservesSystemMessages(t *testing.T) {
	c := NewCompressor(Config{Mode: ModeAggressive})
	messages := []any{
		map[string]any{"role": "system", "content": "You are helpful"},
		map[string]any{"role": "user", "content": "Hello"},
	}
	
	result := c.Compress(messages)
	
	if len(result.CompressedMessages) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(result.CompressedMessages))
	}
	first := result.CompressedMessages[0].(map[string]any)
	if first["role"] != "system" {
		t.Errorf("expected role 'system', got %v", first["role"])
	}
	if first["content"] != "You are helpful" {
		t.Errorf("expected content preserved, got %v", first["content"])
	}
}

func TestCompressorCompressesLongUserMessage(t *testing.T) {
	c := NewCompressor(Config{Mode: ModeDefault})
	longText := strings.Repeat("A", 600)
	messages := []any{
		map[string]any{"role": "system", "content": "System prompt"},
		map[string]any{"role": "user", "content": longText}, // This will be compressed (index 1, not in last 2)
		map[string]any{"role": "assistant", "content": "Response"},
		map[string]any{"role": "user", "content": "Short"},
		map[string]any{"role": "assistant", "content": "Last"},
	}
	
	result := c.Compress(messages)
	
	// Find the long user message (should be compressed)
	var userContent string
	for _, msg := range result.CompressedMessages {
		m := msg.(map[string]any)
		content := m["content"].(string)
		if m["role"] == "user" && len(content) > 100 {
			userContent = content
			break
		}
	}
	
	if len(userContent) >= 600 {
		t.Errorf("expected compressed message, got %d chars", len(userContent))
	}
	if !strings.HasSuffix(userContent, "...") {
		t.Errorf("expected truncation indicator, got: %s", userContent)
	}
}

func TestCompressorKeepsRecentMessages(t *testing.T) {
	c := NewCompressor(Config{Mode: ModeDefault, KeepRecentMessages: 2})
	messages := []any{
		map[string]any{"role": "user", "content": "First"},
		map[string]any{"role": "assistant", "content": "Second"},
		map[string]any{"role": "user", "content": "Third"},
		map[string]any{"role": "assistant", "content": "Fourth"},
	}
	
	result := c.Compress(messages)
	
	// Should keep all 4, recent 2 at full detail
	if len(result.CompressedMessages) != 4 {
		t.Fatalf("expected 4 messages, got %d", len(result.CompressedMessages))
	}
	last := result.CompressedMessages[3].(map[string]any)
	if last["content"] != "Fourth" {
		t.Errorf("expected recent message preserved, got %v", last["content"])
	}
}

func TestCompressorPreservesToolCallStructure(t *testing.T) {
	c := NewCompressor(Config{Mode: ModeAggressive})
	messages := []any{
		map[string]any{
			"role": "assistant",
			"content": "Let me help",
			"tool_calls": []map[string]any{
				{"id": "call_123", "function": map[string]any{"name": "read_file"}},
			},
		},
	}
	
	result := c.Compress(messages)
	
	first := result.CompressedMessages[0].(map[string]any)
	toolCalls, ok := first["tool_calls"].([]map[string]any)
	if !ok {
		t.Fatalf("expected tool_calls to be preserved")
	}
	if len(toolCalls) != 1 {
		t.Fatalf("expected 1 tool call, got %d", len(toolCalls))
	}
	if toolCalls[0]["id"] != "call_123" {
		t.Errorf("expected tool call id preserved, got %v", toolCalls[0]["id"])
	}
}

func TestCompressorModeAggressive(t *testing.T) {
	c := NewCompressor(Config{Mode: ModeAggressive})
	messages := []any{
		map[string]any{"role": "system", "content": "System"},
		map[string]any{"role": "user", "content": strings.Repeat("B", 1000)}, // Will be compressed
		map[string]any{"role": "assistant", "content": "Response"},
		map[string]any{"role": "user", "content": "Short"},
		map[string]any{"role": "assistant", "content": "Last"},
	}
	
	result := c.Compress(messages)
	
	// Find the long user message (should be compressed)
	var userContent string
	for _, msg := range result.CompressedMessages {
		m := msg.(map[string]any)
		content := m["content"].(string)
		if m["role"] == "user" && len(content) > 100 {
			userContent = content
			break
		}
	}
	
	if len(userContent) > 103 { // TruncationLimitAggressive + "..."
		t.Errorf("aggressive mode should truncate to ~100 chars, got %d", len(userContent))
	}
}

func TestCompressorModeConservative(t *testing.T) {
	c := NewCompressor(Config{Mode: ModeConservative})
	messages := []any{
		map[string]any{"role": "assistant", "content": "Normal response without truncation needed"},
	}
	
	result := c.Compress(messages)
	
	first := result.CompressedMessages[0].(map[string]any)
	if first["content"] != "Normal response without truncation needed" {
		t.Errorf("conservative mode should not truncate short messages, got %v", first["content"])
	}
}

func TestCompressorTracksPreservedInfo(t *testing.T) {
	c := NewCompressor(Config{Mode: ModeDefault})
	messages := []any{
		map[string]any{"role": "system", "content": "System"},
		map[string]any{"role": "user", "content": "Question"},
		map[string]any{"role": "assistant", "content": "Thinking...", "tool_calls": []any{
			map[string]any{"id": "1", "function": map[string]any{"name": "read_file"}},
		}},
		map[string]any{"role": "tool", "content": "file content here", "name": "read_file"},
		map[string]any{"role": "assistant", "content": "Final response"},
		map[string]any{"role": "user", "content": "Last"},
	}
	
	result := c.Compress(messages)
	
	// The assistant with tool_calls and tool message are at indices 2 and 3, which are compressible
	if result.PreservedInfo.KeyDecisions != 1 {
		t.Errorf("expected 1 key decision, got %d", result.PreservedInfo.KeyDecisions)
	}
	if result.PreservedInfo.ToolResults != 1 {
		t.Errorf("expected 1 tool result, got %d", result.PreservedInfo.ToolResults)
	}
}
