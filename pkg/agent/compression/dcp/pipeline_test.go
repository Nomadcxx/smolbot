package dcp

import (
	"reflect"
	"strings"
	"testing"

	"github.com/Nomadcxx/smolbot/pkg/provider"
	"github.com/Nomadcxx/smolbot/pkg/tokenizer"
)

func TestDeepCopyMessages(t *testing.T) {
	original := []provider.Message{
		makeUserMsg("hello"),
		{
			Role:    "assistant",
			Content: []provider.ContentBlock{{Type: "text", Text: "block"}},
			ToolCalls: []provider.ToolCall{{
				ID: "tc1",
				Function: provider.FunctionCall{Name: "read_file", Arguments: `{"path":"a.txt"}`},
			}},
			ThinkingBlocks: []provider.ThinkingBlock{{Type: "thinking", Content: "secret"}},
		},
	}

	copyMsgs := deepCopyMessages(original)
	copyMsgs[0].Content = "changed"
	copyMsgs[1].Content = []provider.ContentBlock{{Type: "text", Text: "changed block"}}
	copyMsgs[1].ToolCalls[0].Function.Arguments = `{"path":"b.txt"}`
	copyMsgs[1].ThinkingBlocks[0].Content = "changed"

	if got := original[0].StringContent(); got != "hello" {
		t.Fatalf("original[0] = %q, want hello", got)
	}
	blocks, ok := original[1].Content.([]provider.ContentBlock)
	if !ok || len(blocks) != 1 || blocks[0].Text != "block" {
		t.Fatalf("original content blocks mutated: %#v", original[1].Content)
	}
	if got := original[1].ToolCalls[0].Function.Arguments; got != `{"path":"a.txt"}` {
		t.Fatalf("original tool args = %q, want unchanged", got)
	}
	if got := original[1].ThinkingBlocks[0].Content; got != "secret" {
		t.Fatalf("original thinking block = %q, want secret", got)
	}
}

func TestTransformPassthrough(t *testing.T) {
	input := []provider.Message{
		makeUserMsg("hello"),
		makeAssistantMsg("world"),
	}
	state := NewState("s1")
	cfg := DefaultConfig()
	cfg.Deduplication.Enabled = false
	cfg.PurgeErrors.Enabled = false
	cfg.CompressTool.Enabled = false

	got, _ := Transform(input, state, cfg, tokenizer.New(), 200000)
	if !reflect.DeepEqual(got, input) {
		t.Fatalf("Transform() mismatch:\n got: %#v\nwant: %#v", got, input)
	}
}

func TestTransformStatsPopulated(t *testing.T) {
	input := []provider.Message{
		makeUserMsg("hello"),
		makeAssistantMsg("world"),
	}
	state := NewState("s1")
	cfg := DefaultConfig()
	cfg.Deduplication.Enabled = false
	cfg.PurgeErrors.Enabled = false
	cfg.CompressTool.Enabled = false

	_, stats := Transform(input, state, cfg, tokenizer.New(), 200000)
	if stats.MessagesIn != 2 {
		t.Fatalf("MessagesIn = %d, want 2", stats.MessagesIn)
	}
	if stats.MessagesOut != 2 {
		t.Fatalf("MessagesOut = %d, want 2", stats.MessagesOut)
	}
	if stats.TokensEstimated <= 0 {
		t.Fatalf("TokensEstimated = %d, want > 0", stats.TokensEstimated)
	}
}

func TestTransform_DedupAndPurge(t *testing.T) {
	cfg := DefaultConfig()
	input := []provider.Message{
		makeUserMsg("turn1"),
		makeAssistantWithToolCall("", "read_file", `{"path":"a.txt"}`, "tc1"),
		makeToolResult("tc1", "read_file", "A"),
		makeAssistantWithToolCall("", "read_file", `{"path":"a.txt"}`, "tc2"),
		makeToolResult("tc2", "read_file", "A"),
		makeAssistantWithToolCall("", "exec", `{"cmd":"bad"}`, "tc3"),
		makeToolError("tc3", "exec", "Error: boom"),
		makeUserMsg("turn2"),
		makeAssistantMsg("a"),
		makeUserMsg("turn3"),
		makeAssistantMsg("b"),
		makeUserMsg("turn4"),
		makeAssistantMsg("c"),
		makeUserMsg("turn5"),
		makeAssistantMsg("d"),
	}

	got, stats := Transform(input, NewState("s1"), cfg, tokenizer.New(), 200000)
	if stats.DedupCount != 1 {
		t.Fatalf("DedupCount = %d, want 1", stats.DedupCount)
	}
	if stats.ErrorPurgeCount != 1 {
		t.Fatalf("ErrorPurgeCount = %d, want 1", stats.ErrorPurgeCount)
	}
	if !strings.Contains(got[2].StringContent(), DedupPlaceholder) {
		t.Fatalf("dedup placeholder missing: %q", got[2].StringContent())
	}
	if got[5].ToolCalls[0].Function.Arguments != ErrorInputPlaceholder {
		t.Fatalf("purged args = %q, want placeholder", got[5].ToolCalls[0].Function.Arguments)
	}
}

func TestTransform_DedupDisabled(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Deduplication.Enabled = false
	input := []provider.Message{
		makeAssistantWithToolCall("", "read_file", `{"path":"a.txt"}`, "tc1"),
		makeToolResult("tc1", "read_file", "A"),
		makeAssistantWithToolCall("", "read_file", `{"path":"a.txt"}`, "tc2"),
		makeToolResult("tc2", "read_file", "A"),
	}

	got, stats := Transform(input, NewState("s1"), cfg, tokenizer.New(), 200000)
	if stats.DedupCount != 0 {
		t.Fatalf("DedupCount = %d, want 0", stats.DedupCount)
	}
	if strings.TrimSpace(StripDCPTags(got[1].StringContent())) != "A" {
		t.Fatalf("tool result = %q, want unchanged", got[1].StringContent())
	}
}

func TestTransformStableRefsAfterPrune(t *testing.T) {
	msgs := []provider.Message{
		{Role: "system", Content: "sys"},
		makeUserMsg("u1"),
		makeAssistantMsg("a1"),
		makeUserMsg("u2"),
		makeAssistantMsg("a2"),
		makeUserMsg("u3"),
		makeAssistantMsg("a3"),
	}
	state := NewState("s1")
	cfg := DefaultConfig()
	cfg.Deduplication.Enabled = false
	cfg.PurgeErrors.Enabled = false
	cfg.CompressTool.Enabled = true
	cfg.TurnProtection = 0

	first, _ := Transform(msgs, state, cfg, tokenizer.New(), 200000)
	refBefore := extractDCPID(first[5].StringContent())
	if refBefore == "" {
		t.Fatal("missing ref before prune")
	}

	id := state.AllocateBlockID()
	if err := state.CreateBlock(CompressionBlock{
		ID:             id,
		Topic:          "middle",
		Summary:        "compressed",
		StartRef:       extractDCPID(first[2].StringContent()),
		EndRef:         extractDCPID(first[4].StringContent()),
		AnchorMsgIndex: 2,
	}); err != nil {
		t.Fatalf("CreateBlock: %v", err)
	}

	second, _ := Transform(msgs, state, cfg, tokenizer.New(), 200000)
	var refAfter string
	for _, msg := range second {
		if strings.TrimSpace(StripDCPTags(msg.StringContent())) == "u3" {
			refAfter = extractDCPID(msg.StringContent())
			break
		}
	}
	if refAfter == "" {
		t.Fatal("missing surviving message after prune")
	}
	if refAfter != refBefore {
		t.Fatalf("stable ref mismatch: got %q want %q", refAfter, refBefore)
	}
}
