package session

import (
	"testing"

	"github.com/Nomadcxx/smolbot/pkg/provider"
)

func TestGetHistory_BasicRetrieval(t *testing.T) {
	store := newHistoryTestStore(t)
	defer store.Close()

	if err := store.SaveMessages("s1", []provider.Message{
		{Role: "user", Content: "Hello"},
		{Role: "assistant", Content: "Hi!"},
		{Role: "user", Content: "How are you?"},
		{Role: "assistant", Content: "Good!"},
	}); err != nil {
		t.Fatalf("SaveMessages: %v", err)
	}

	msgs, err := store.GetHistory("s1", 500)
	if err != nil {
		t.Fatalf("GetHistory: %v", err)
	}
	if len(msgs) != 4 {
		t.Fatalf("history len = %d, want 4", len(msgs))
	}
}

func TestGetHistory_ExcludesConsolidated(t *testing.T) {
	store := newHistoryTestStore(t)
	defer store.Close()

	if err := store.SaveMessages("s1", []provider.Message{
		{Role: "user", Content: "Old message"},
		{Role: "assistant", Content: "Old reply"},
		{Role: "user", Content: "New message"},
		{Role: "assistant", Content: "New reply"},
	}); err != nil {
		t.Fatalf("SaveMessages: %v", err)
	}

	if err := store.MarkConsolidated("s1", 2); err != nil {
		t.Fatalf("MarkConsolidated: %v", err)
	}

	msgs, err := store.GetHistory("s1", 500)
	if err != nil {
		t.Fatalf("GetHistory: %v", err)
	}
	if len(msgs) != 2 {
		t.Fatalf("history len = %d, want 2", len(msgs))
	}
	if content, ok := msgs[0].Content.(string); !ok || content != "New message" {
		t.Fatalf("first content = %#v, want %q", msgs[0].Content, "New message")
	}
}

func TestGetHistory_LegalBoundary_OrphanToolResult(t *testing.T) {
	store := newHistoryTestStore(t)
	defer store.Close()

	if err := store.SaveMessages("s1", []provider.Message{
		{Role: "tool", Content: "orphan output", ToolCallID: "tc_missing", Name: "exec"},
		{Role: "user", Content: "Do something"},
		{
			Role:    "assistant",
			Content: "",
			ToolCalls: []provider.ToolCall{
				{ID: "tc1", Function: provider.FunctionCall{Name: "exec", Arguments: `{}`}},
			},
		},
		{Role: "tool", Content: "result", ToolCallID: "tc1", Name: "exec"},
		{Role: "assistant", Content: "Done!"},
	}); err != nil {
		t.Fatalf("SaveMessages: %v", err)
	}

	msgs, err := store.GetHistory("s1", 500)
	if err != nil {
		t.Fatalf("GetHistory: %v", err)
	}
	if len(msgs) != 4 {
		t.Fatalf("history len = %d, want 4", len(msgs))
	}
	if msgs[0].Role != "user" {
		t.Fatalf("first role = %q, want user", msgs[0].Role)
	}
}

func TestGetHistory_DropsLeadingNonUser(t *testing.T) {
	store := newHistoryTestStore(t)
	defer store.Close()

	if err := store.SaveMessages("s1", []provider.Message{
		{Role: "assistant", Content: "Stale assistant"},
		{Role: "user", Content: "Real start"},
		{Role: "assistant", Content: "Reply"},
	}); err != nil {
		t.Fatalf("SaveMessages: %v", err)
	}

	msgs, err := store.GetHistory("s1", 500)
	if err != nil {
		t.Fatalf("GetHistory: %v", err)
	}
	if len(msgs) != 2 {
		t.Fatalf("history len = %d, want 2", len(msgs))
	}
	if msgs[0].Role != "user" {
		t.Fatalf("first role = %q, want user", msgs[0].Role)
	}
}

func newHistoryTestStore(t *testing.T) *Store {
	t.Helper()
	store, err := NewStore(":memory:")
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	if _, err := store.GetOrCreateSession("s1"); err != nil {
		t.Fatalf("GetOrCreateSession: %v", err)
	}
	return store
}
