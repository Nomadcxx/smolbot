package compression

import (
	"strings"
	"testing"
)

func TestTruncateAtSentenceBoundary(t *testing.T) {
	// Text exactly at limit should return as-is
	text := strings.Repeat("a", 25)
	result := truncateAtSentence(text, 25)
	if result != text {
		t.Errorf("expected unchanged text at limit, got %q", result)
	}
	
	// Text just over limit but with no sentence structure
	// limit=30, text=28 chars (no periods) - triggers panic if bounds check missing
	text2 := strings.Repeat("a", 28)
	result2 := truncateAtSentence(text2, 30)
	if len(result2) == 0 {
		t.Error("expected truncated result, got empty")
	}
	
	// Text with sentence structure that hits boundary
	text3 := "This is sentence one. This is sentence two."
	result3 := truncateAtSentence(text3, 25)
	if len(result3) == 0 {
		t.Error("expected truncated result, got empty")
	}
}

func TestCompressorPreservesSystemMessages(t *testing.T) {
	c := NewCompressor(Config{Enabled: true, Mode: ModeAggressive})
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
	c := NewCompressor(Config{Enabled: true, Mode: ModeDefault})
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
	c := NewCompressor(Config{Enabled: true, Mode: ModeDefault, KeepRecentMessages: 2})
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
	c := NewCompressor(Config{Enabled: true, Mode: ModeAggressive})
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
	c := NewCompressor(Config{Enabled: true, Mode: ModeAggressive})
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

func TestCompressorDoesNotSplitToolCallAndResultPairs(t *testing.T) {
	// With 5 messages and keepRecent=2, splitIdx=3:
	// Original: [0]user, [1]assistant(ok), [2]assistant(tc1), [3]tool(tc1), [4]user
	// Without fix: splitIdx=3 -> [0,1,2] compressible, [3,4] recent
	//   -> assistant at index 2 (compressible), tool_result at index 3 (recent) - SEPARATED!
	// With fix: tool_result at index 3 triggers backward scan, finds paired assistant at 2, moves splitIdx to 2
	//   -> [0,1] compressible, [2,3,4] recent -> both in recent, adjacent (gap=1)
	messages := []any{
		map[string]any{"role": "user", "content": "hello"},
		map[string]any{"role": "assistant", "content": "ok"},
		map[string]any{"role": "assistant", "content": "", "tool_calls": []any{
			map[string]any{"id": "tc1", "function": map[string]any{"name": "exec", "arguments": "{}"}},
		}},
		map[string]any{"role": "tool", "content": "done", "tool_call_id": "tc1", "name": "exec"},
		map[string]any{"role": "user", "content": "continue"},
	}

	c := NewCompressor(Config{Enabled: true, KeepRecentMessages: 2})
	result := c.Compress(messages)

	var assistantIdx, resultIdx int = -1, -1
	for i, msg := range result.CompressedMessages {
		m := msg.(map[string]any)
		role := getRole(m)

		if role == "tool" {
			if tcid, ok := m["tool_call_id"].(string); ok && tcid == "tc1" {
				resultIdx = i
			}
		}
		if role == "assistant" {
			if tcs, ok := m["tool_calls"].([]any); ok {
				for _, tc := range tcs {
					if tcMap, ok := tc.(map[string]any); ok {
						if id, ok := tcMap["id"].(string); ok && id == "tc1" {
							assistantIdx = i
						}
					}
				}
			}
		}
	}

	if assistantIdx == -1 || resultIdx == -1 {
		t.Fatal("assistant or tool result missing")
	}

	// With fix: both in recent, tool_result right after assistant (indices differ by 1)
	gap := resultIdx - assistantIdx
	if gap != 1 {
		t.Errorf("expected tool_result right after assistant (gap=1), got gap=%d (assistant=%d, result=%d)", gap, assistantIdx, resultIdx)
	}
}

func TestCompressorModeConservative(t *testing.T) {
	c := NewCompressor(Config{Enabled: true, Mode: ModeConservative})
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
	c := NewCompressor(Config{Enabled: true, Mode: ModeDefault})
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
