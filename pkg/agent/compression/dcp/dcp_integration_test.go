package dcp

import (
	"strings"
	"testing"

	"github.com/Nomadcxx/smolbot/pkg/provider"
	"github.com/Nomadcxx/smolbot/pkg/tokenizer"
)

func TestDCPIntegration_FullPipeline(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Nudge.MinContextLimit = 10
	cfg.Nudge.MaxContextLimit = 1000000
	cfg.Nudge.NudgeFrequency = 5
	msgs := []provider.Message{
		makeUserMsg("turn1"),
		makeAssistantWithToolCall("", "read_file", `{"path":"a.txt"}`, "tc1"),
		makeToolResult("tc1", "read_file", "A"),
		makeAssistantWithToolCall("", "read_file", `{"path":"a.txt"}`, "tc2"),
		makeToolResult("tc2", "read_file", "A"),
		makeAssistantWithToolCall("", "exec", `{"cmd":"bad"}`, "tc3"),
		makeToolError("tc3", "exec", "Error: boom"),
		makeUserMsg(strings.Repeat("x", 120)),
		makeAssistantMsg("done"),
		makeUserMsg("turn3"),
		makeAssistantMsg("done"),
		makeUserMsg("turn4"),
		makeAssistantMsg("done"),
		makeUserMsg(strings.Repeat("y", 120)),
	}
	state := NewState("s1")
	state.RequestCount = 4

	got, stats := Transform(msgs, state, cfg, tokenizer.New(), 1000)
	if stats.DedupCount != 1 {
		t.Fatalf("DedupCount = %d, want 1", stats.DedupCount)
	}
	if stats.ErrorPurgeCount != 1 {
		t.Fatalf("ErrorPurgeCount = %d, want 1", stats.ErrorPurgeCount)
	}
	foundID := false
	foundNudge := false
	for _, msg := range got {
		if strings.Contains(msg.StringContent(), "<dcp-id>m") {
			foundID = true
		}
		if strings.Contains(msg.StringContent(), "<dcp-reminder>") {
			foundNudge = true
		}
	}
	if !foundID {
		t.Fatal("expected message ids in output")
	}
	if !foundNudge {
		t.Fatal("expected nudge in output")
	}
}

func TestDCPIntegration_ToolCallResultPairIntegrity(t *testing.T) {
	msgs := []provider.Message{
		makeUserMsg("turn1"),
		makeAssistantWithToolCall("", "read_file", `{"path":"a.txt"}`, "tc1"),
		makeToolResult("tc1", "read_file", "A"),
		makeAssistantMsg("after tc1"),
		makeAssistantWithToolCall("", "exec", `{"cmd":"pwd"}`, "tc2"),
		makeToolResult("tc2", "exec", "ok"),
		makeAssistantMsg("after tc2"),
	}
	state := NewState("s1")
	cfg := DefaultConfig()
	cfg.Deduplication.Enabled = false
	cfg.PurgeErrors.Enabled = false
	cfg.TurnProtection = 0

	first, _ := Transform(msgs, state, cfg, tokenizer.New(), 200000)
	if err := state.CreateBlock(CompressionBlock{
		ID:             state.AllocateBlockID(),
		Topic:          "first pair",
		Summary:        "compressed first tool sequence",
		StartRef:       extractDCPID(first[1].StringContent()),
		EndRef:         extractDCPID(first[2].StringContent()),
		AnchorMsgIndex: 1,
	}); err != nil {
		t.Fatalf("CreateBlock: %v", err)
	}

	second, _ := Transform(msgs, state, cfg, tokenizer.New(), 200000)
	assertToolPairsIntact(t, second)
}

func assertToolPairsIntact(t *testing.T, messages []provider.Message) {
	t.Helper()

	seenCalls := make(map[string]bool)
	for i, msg := range messages {
		if msg.Role == "assistant" {
			for _, call := range msg.ToolCalls {
				seenCalls[call.ID] = true
			}
			continue
		}
		if msg.Role == "tool" && msg.ToolCallID != "" && !seenCalls[msg.ToolCallID] {
			t.Fatalf("orphaned tool result at index %d for toolCallID=%q", i, msg.ToolCallID)
		}
	}
}
