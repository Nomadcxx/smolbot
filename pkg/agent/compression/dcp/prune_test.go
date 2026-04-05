package dcp

import (
	"strings"
	"testing"

	"github.com/Nomadcxx/smolbot/pkg/provider"
)

func TestApplyBlocks_SingleBlock(t *testing.T) {
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
	AssignMessageIDs(msgs, state, DefaultConfig())
	id := state.AllocateBlockID()
	if err := state.CreateBlock(CompressionBlock{
		ID:             id,
		Topic:          "work",
		Summary:        "done",
		StartRef:       "m0002",
		EndRef:         "m0005",
		AnchorMsgIndex: 2,
	}); err != nil {
		t.Fatalf("CreateBlock: %v", err)
	}

	got := ApplyCompressionBlocks(deepCopyMessages(msgs), state)
	if len(got) != 4 {
		t.Fatalf("len(got) = %d, want 4", len(got))
	}
	if !strings.Contains(got[2].StringContent(), "[Compressed: work]") {
		t.Fatalf("summary not injected: %q", got[2].StringContent())
	}
}

func TestApplyBlocks_NestedBlock(t *testing.T) {
	state := NewState("s1")
	state.Blocks[1] = &CompressionBlock{ID: 1, Summary: "inner", Active: false}
	state.Blocks[2] = &CompressionBlock{ID: 2, Summary: "outer (b1)", Active: true}
	got := ExpandBlockPlaceholders(state.Blocks[2].Summary, state.Blocks)
	if !strings.Contains(got, "inner") {
		t.Fatalf("ExpandBlockPlaceholders() = %q, want inner content", got)
	}
}

func TestWrapSummary(t *testing.T) {
	got := WrapSummary("topic", "summary", 3)
	if !strings.Contains(got, "[Compressed: topic]") || !strings.Contains(got, "<dcp-id>b3</dcp-id>") {
		t.Fatalf("WrapSummary() = %q", got)
	}
}

func TestApplyBlocksPreservesToolPairs(t *testing.T) {
	msgs := []provider.Message{
		makeUserMsg("u1"),
		makeAssistantWithToolCall("", "exec", `{"cmd":"ls"}`, "tc1"),
		makeToolResult("tc1", "exec", "ok"),
		makeAssistantMsg("after"),
	}
	state := NewState("s1")
	AssignMessageIDs(msgs, state, DefaultConfig())
	id := state.AllocateBlockID()
	err := state.CreateBlock(CompressionBlock{
		ID:             id,
		Topic:          "bad",
		Summary:        "summary",
		StartRef:       extractDCPID(msgs[1].StringContent()),
		EndRef:         extractDCPID(msgs[1].StringContent()),
		AnchorMsgIndex: 1,
	})
	if err == nil {
		t.Fatal("CreateBlock() error = nil, want pair-integrity error")
	}
}
