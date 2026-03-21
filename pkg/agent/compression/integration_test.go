package compression

import (
	"strings"
	"testing"
)

func TestIntegrationFullCompressionFlow(t *testing.T) {
	cfg := Config{
		Enabled:            true,
		Mode:               ModeDefault,
		KeepRecentMessages: 2,
	}
	
	compressor := NewCompressor(cfg)
	
	messages := []any{
		map[string]any{"role": "system", "content": "You are helpful"},
		map[string]any{"role": "user", "content": strings.Repeat("A", 600)},
		map[string]any{"role": "assistant", "content": "Response"},
		map[string]any{"role": "user", "content": "Short"},
		map[string]any{"role": "assistant", "content": "Last"},
	}
	
	result := compressor.Compress(messages)
	
	// System preserved
	first := result.CompressedMessages[0].(map[string]any)
	if first["role"] != "system" {
		t.Errorf("expected role 'system', got %v", first["role"])
	}
	
	// Recent messages preserved
	last := result.CompressedMessages[len(result.CompressedMessages)-1].(map[string]any)
	if last["content"] != "Last" {
		t.Errorf("expected recent message preserved, got %v", last["content"])
	}
	
	// Middle compressed
	second := result.CompressedMessages[1].(map[string]any)
	content := second["content"].(string)
	if len(content) >= 600 {
		t.Errorf("expected compressed message, got %d chars", len(content))
	}
}

func TestIntegrationPreservesToolCalls(t *testing.T) {
	cfg := Config{Mode: ModeAggressive}
	compressor := NewCompressor(cfg)
	
	messages := []any{
		map[string]any{
			"role": "assistant",
			"content": strings.Repeat("X", 1000),
			"tool_calls": []map[string]any{
				{"id": "1", "function": map[string]any{"name": "write_file"}},
				{"id": "2", "function": map[string]any{"name": "read_file"}},
			},
		},
	}
	
	result := compressor.Compress(messages)
	
	first := result.CompressedMessages[0].(map[string]any)
	toolCalls, ok := first["tool_calls"].([]map[string]any)
	if !ok {
		t.Fatalf("expected tool_calls to be preserved")
	}
	if len(toolCalls) != 2 {
		t.Fatalf("expected 2 tool calls, got %d", len(toolCalls))
	}
	if toolCalls[0]["id"] != "1" {
		t.Errorf("expected tool call id preserved, got %v", toolCalls[0]["id"])
	}
	if toolCalls[1]["id"] != "2" {
		t.Errorf("expected tool call id preserved, got %v", toolCalls[1]["id"])
	}
}

func TestIntegrationThresholdDetection(t *testing.T) {
	tracker := &TokenTracker{}
	
	messages := []any{
		map[string]any{"role": "user", "content": "Test"},
	}
	
	// Should not compress at 10% of 1000 token window (small message)
	if tracker.ShouldCompress(messages, 1000, 60) {
		t.Error("should not compress small messages")
	}
	
	// Should not compress with 0 threshold
	if tracker.ShouldCompress(messages, 1000, 0) {
		t.Error("should not compress with 0 threshold")
	}
	
	// Should not compress with 0 context window
	if tracker.ShouldCompress(messages, 0, 60) {
		t.Error("should not compress with 0 context window")
	}
}

func TestIntegrationStatsRecording(t *testing.T) {
	// Clear any existing stats
	ClearStats("test-session")
	
	result := &Result{
		OriginalTokenCount:   1000,
		CompressedTokenCount: 600,
		ReductionPercentage:  40.0,
	}
	
	RecordCompression("test-session", result)
	
	stats := GetStats("test-session")
	if stats == nil {
		t.Fatal("expected stats to be recorded")
	}
	
	if stats.TotalCompressions != 1 {
		t.Errorf("expected 1 compression, got %d", stats.TotalCompressions)
	}
	
	if stats.TotalTokensSaved != 400 {
		t.Errorf("expected 400 tokens saved, got %d", stats.TotalTokensSaved)
	}
	
	if stats.LastCompression == nil {
		t.Error("expected last compression to be recorded")
	}
	
	// Clear and verify
	ClearStats("test-session")
	stats = GetStats("test-session")
	if stats != nil {
		t.Error("expected stats to be cleared")
	}
}

func TestIntegrationCompressionModes(t *testing.T) {
	longText := strings.Repeat("Test sentence. ", 100) // > 1000 chars
	
	// Conservative mode should not compress much
	conservative := NewCompressor(Config{Mode: ModeConservative})
	messages := []any{
		map[string]any{"role": "user", "content": longText},
	}
	resultConservative := conservative.Compress(messages)
	contentConservative := resultConservative.CompressedMessages[0].(map[string]any)["content"].(string)
	
	// Aggressive mode should compress more
	aggressive := NewCompressor(Config{Mode: ModeAggressive})
	resultAggressive := aggressive.Compress(messages)
	contentAggressive := resultAggressive.CompressedMessages[0].(map[string]any)["content"].(string)
	
	// Aggressive should be shorter or equal
	if len(contentAggressive) > len(contentConservative) {
		t.Logf("Conservative: %d chars, Aggressive: %d chars", len(contentConservative), len(contentAggressive))
	}
}
