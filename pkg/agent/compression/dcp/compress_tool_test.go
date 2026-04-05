package dcp

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/Nomadcxx/smolbot/pkg/provider"
	"github.com/Nomadcxx/smolbot/pkg/tokenizer"
	"github.com/Nomadcxx/smolbot/pkg/tool"
)

func TestCompressTool_SingleRange(t *testing.T) {
	sm, err := NewStateManager(makeInMemoryDB(t))
	if err != nil {
		t.Fatalf("NewStateManager: %v", err)
	}
	state := NewState("s1")
	state.MessageIDs.ByRef["m0001"] = 0
	state.MessageIDs.ByRef["m0003"] = 2
	if err := sm.Save(state); err != nil {
		t.Fatalf("Save: %v", err)
	}

	ct := NewCompressTool(sm, DefaultConfig(), tokenizer.New())
	args, _ := json.Marshal(map[string]any{
		"topic": "topic",
		"ranges": []map[string]any{{
			"start_id": "m0001",
			"end_id":   "m0003",
			"summary":  "summary",
		}},
	})

	result, err := ct.Execute(context.Background(), args, tool.ToolContext{SessionKey: "s1"})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if result.Error != "" {
		t.Fatalf("Execute() error result = %q", result.Error)
	}
	updated, err := sm.Load("s1")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(updated.Blocks) != 1 {
		t.Fatalf("blocks = %d, want 1", len(updated.Blocks))
	}
}

func TestCompressTool_InvalidMessageID(t *testing.T) {
	sm, err := NewStateManager(makeInMemoryDB(t))
	if err != nil {
		t.Fatalf("NewStateManager: %v", err)
	}
	if err := sm.Save(NewState("s1")); err != nil {
		t.Fatalf("Save: %v", err)
	}
	ct := NewCompressTool(sm, DefaultConfig(), tokenizer.New())
	args, _ := json.Marshal(map[string]any{
		"topic": "topic",
		"ranges": []map[string]any{{
			"start_id": "m9999",
			"end_id":   "m9999",
			"summary":  "summary",
		}},
	})

	result, err := ct.Execute(context.Background(), args, tool.ToolContext{SessionKey: "s1"})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !strings.Contains(result.Error, "invalid message id") {
		t.Fatalf("result error = %q, want invalid message id", result.Error)
	}
}

func TestCompressTool_MultipleRanges(t *testing.T) {
	sm, err := NewStateManager(makeInMemoryDB(t))
	if err != nil {
		t.Fatalf("NewStateManager: %v", err)
	}
	state := NewState("s1")
	for i, ref := range []string{"m0001", "m0002", "m0003", "m0004"} {
		state.MessageIDs.ByRef[ref] = i
	}
	if err := sm.Save(state); err != nil {
		t.Fatalf("Save: %v", err)
	}
	ct := NewCompressTool(sm, DefaultConfig(), tokenizer.New())
	ct.messages = []provider.Message{
		makeUserMsg("one"),
		makeAssistantMsg("two"),
		makeUserMsg("three"),
		makeAssistantMsg("four"),
	}
	args, _ := json.Marshal(map[string]any{
		"topic": "topic",
		"ranges": []map[string]any{
			{
				"start_id": "m0001",
				"end_id":   "m0002",
				"summary":  "first",
			},
			{
				"start_id": "m0003",
				"end_id":   "m0004",
				"summary":  "second",
			},
		},
	})

	result, err := ct.Execute(context.Background(), args, tool.ToolContext{SessionKey: "s1"})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if result.Error != "" {
		t.Fatalf("Execute() error result = %q", result.Error)
	}
	updated, err := sm.Load("s1")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(updated.Blocks) != 2 {
		t.Fatalf("blocks = %d, want 2", len(updated.Blocks))
	}
}

func TestCompressTool_OverlappingRanges(t *testing.T) {
	sm, err := NewStateManager(makeInMemoryDB(t))
	if err != nil {
		t.Fatalf("NewStateManager: %v", err)
	}
	state := NewState("s1")
	for i, ref := range []string{"m0001", "m0002", "m0003"} {
		state.MessageIDs.ByRef[ref] = i
	}
	if err := sm.Save(state); err != nil {
		t.Fatalf("Save: %v", err)
	}
	ct := NewCompressTool(sm, DefaultConfig(), tokenizer.New())
	ct.messages = []provider.Message{
		makeUserMsg("one"),
		makeAssistantMsg("two"),
		makeUserMsg("three"),
	}
	args, _ := json.Marshal(map[string]any{
		"topic": "topic",
		"ranges": []map[string]any{
			{
				"start_id": "m0001",
				"end_id":   "m0002",
				"summary":  "first",
			},
			{
				"start_id": "m0002",
				"end_id":   "m0003",
				"summary":  "second",
			},
		},
	})

	result, err := ct.Execute(context.Background(), args, tool.ToolContext{SessionKey: "s1"})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !strings.Contains(strings.ToLower(result.Error), "overlap") {
		t.Fatalf("result error = %q, want overlap error", result.Error)
	}
}

func TestCompressTool_ProtectedMessage(t *testing.T) {
	sm, err := NewStateManager(makeInMemoryDB(t))
	if err != nil {
		t.Fatalf("NewStateManager: %v", err)
	}
	state := NewState("s1")
	state.MessageIDs.ByRef["m0001"] = 0
	state.MessageIDs.ByMsgIndex[0] = "m0001"
	state.ProtectedIndexes[0] = true
	if err := sm.Save(state); err != nil {
		t.Fatalf("Save: %v", err)
	}
	ct := NewCompressTool(sm, DefaultConfig(), tokenizer.New())
	ct.messages = []provider.Message{
		makeAssistantWithToolCall("", "write_file", `{"path":"a.txt"}`, "tc1"),
	}
	args, _ := json.Marshal(map[string]any{
		"topic": "topic",
		"ranges": []map[string]any{{
			"start_id": "m0001",
			"end_id":   "m0001",
			"summary":  "summary",
		}},
	})

	result, err := ct.Execute(context.Background(), args, tool.ToolContext{SessionKey: "s1"})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !strings.Contains(result.Error, "protected") {
		t.Fatalf("result error = %q, want protected", result.Error)
	}
}

func TestCompressTool_ConcurrencySafe(t *testing.T) {
	ct := NewCompressTool(nil, DefaultConfig(), tokenizer.New())
	if ct.IsConcurrencySafe() {
		t.Fatal("compress tool should not be concurrency safe")
	}
}

func TestCompressTool_DeferredKeywords(t *testing.T) {
	ct := NewCompressTool(nil, DefaultConfig(), tokenizer.New())
	keywords := strings.Join(ct.DeferredKeywords(), " ")
	for _, word := range []string{"compress", "context"} {
		if !strings.Contains(keywords, word) {
			t.Fatalf("keywords = %q, want %q", keywords, word)
		}
	}
}

func TestCompressToolRejectsBrokenToolPairRange(t *testing.T) {
	sm, err := NewStateManager(makeInMemoryDB(t))
	if err != nil {
		t.Fatalf("NewStateManager: %v", err)
	}
	state := NewState("s1")
	state.MessageIDs.ByRef["m0001"] = 0
	state.MessageIDs.ByRef["m0002"] = 1
	if err := sm.Save(state); err != nil {
		t.Fatalf("Save: %v", err)
	}
	ct := NewCompressTool(sm, DefaultConfig(), tokenizer.New())
	ct.messages = []provider.Message{
		makeAssistantWithToolCall("", "exec", `{"cmd":"ls"}`, "tc1"),
		makeToolResult("tc1", "exec", "ok"),
	}
	args, _ := json.Marshal(map[string]any{
		"topic": "topic",
		"ranges": []map[string]any{{
			"start_id": "m0001",
			"end_id":   "m0001",
			"summary":  "summary",
		}},
	})

	result, err := ct.Execute(context.Background(), args, tool.ToolContext{SessionKey: "s1"})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !strings.Contains(strings.ToLower(result.Error), "pair") {
		t.Fatalf("result error = %q, want pair-integrity error", result.Error)
	}
}

func TestCompressToolComputesTokenSavings(t *testing.T) {
	sm, err := NewStateManager(makeInMemoryDB(t))
	if err != nil {
		t.Fatalf("NewStateManager: %v", err)
	}
	state := NewState("s1")
	state.MessageIDs.ByRef["m0001"] = 0
	state.MessageIDs.ByRef["m0002"] = 1
	if err := sm.Save(state); err != nil {
		t.Fatalf("Save: %v", err)
	}
	ct := NewCompressTool(sm, DefaultConfig(), tokenizer.New())
	ct.messages = []provider.Message{
		makeUserMsg(strings.Repeat("x", 200)),
		makeAssistantMsg(strings.Repeat("y", 200)),
	}
	args, _ := json.Marshal(map[string]any{
		"topic": "topic",
		"ranges": []map[string]any{{
			"start_id": "m0001",
			"end_id":   "m0002",
			"summary":  "short",
		}},
	})

	result, err := ct.Execute(context.Background(), args, tool.ToolContext{SessionKey: "s1"})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if result.Error != "" {
		t.Fatalf("Execute() error result = %q", result.Error)
	}
	updated, err := sm.Load("s1")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	var saved int
	for _, block := range updated.Blocks {
		saved = block.TokensSaved
	}
	if saved <= 0 {
		t.Fatalf("TokensSaved = %d, want > 0", saved)
	}
}

func TestCompressTool_NestedCompression(t *testing.T) {
	sm, err := NewStateManager(makeInMemoryDB(t))
	if err != nil {
		t.Fatalf("NewStateManager: %v", err)
	}
	state := NewState("s1")
	for i, ref := range []string{"m0001", "m0002", "m0003"} {
		state.MessageIDs.ByRef[ref] = i
	}
	state.Blocks[1] = &CompressionBlock{
		ID:             1,
		Active:         true,
		Topic:          "inner",
		Summary:        "inner summary",
		StartRef:       "m0002",
		EndRef:         "m0003",
		AnchorMsgIndex: 1,
	}
	state.NextBlockID = 2
	if err := sm.Save(state); err != nil {
		t.Fatalf("Save: %v", err)
	}
	ct := NewCompressTool(sm, DefaultConfig(), tokenizer.New())
	ct.messages = []provider.Message{
		makeUserMsg("one"),
		makeAssistantMsg("two"),
		makeUserMsg("three"),
	}
	args, _ := json.Marshal(map[string]any{
		"topic": "outer",
		"ranges": []map[string]any{{
			"start_id": "m0001",
			"end_id":   "b1",
			"summary":  "outer summary",
		}},
	})

	result, err := ct.Execute(context.Background(), args, tool.ToolContext{SessionKey: "s1"})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if result.Error != "" {
		t.Fatalf("Execute() error result = %q", result.Error)
	}
	updated, err := sm.Load("s1")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if updated.Blocks[1].Active {
		t.Fatal("inner block should be deactivated after nested compression")
	}
	if updated.Blocks[1].ConsumedBy != 2 {
		t.Fatalf("ConsumedBy = %d, want 2", updated.Blocks[1].ConsumedBy)
	}
	if len(updated.Blocks[2].ConsumedBlocks) != 1 || updated.Blocks[2].ConsumedBlocks[0] != 1 {
		t.Fatalf("ConsumedBlocks = %+v, want [1]", updated.Blocks[2].ConsumedBlocks)
	}
}
