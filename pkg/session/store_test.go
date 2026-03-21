package session

import (
	"testing"

	"github.com/Nomadcxx/smolbot/pkg/provider"
)

func TestNewStore_CreatesSchema(t *testing.T) {
	store, err := NewStore(":memory:")
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	defer store.Close()

	var count int
	if err := store.db.QueryRow("SELECT COUNT(*) FROM sessions").Scan(&count); err != nil {
		t.Fatalf("query sessions: %v", err)
	}
	if err := store.db.QueryRow("SELECT COUNT(*) FROM messages").Scan(&count); err != nil {
		t.Fatalf("query messages: %v", err)
	}
}

func TestGetOrCreateSession(t *testing.T) {
	store, err := NewStore(":memory:")
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	defer store.Close()

	s1, err := store.GetOrCreateSession("test:chat1")
	if err != nil {
		t.Fatalf("GetOrCreateSession: %v", err)
	}
	if s1.Key != "test:chat1" {
		t.Fatalf("session key = %q, want test:chat1", s1.Key)
	}

	s2, err := store.GetOrCreateSession("test:chat1")
	if err != nil {
		t.Fatalf("GetOrCreateSession second call: %v", err)
	}
	if s2.Key != s1.Key {
		t.Fatalf("second session key = %q, want %q", s2.Key, s1.Key)
	}
}

func TestSaveAndRetrieveMessages(t *testing.T) {
	store, err := NewStore(":memory:")
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	defer store.Close()

	if _, err := store.GetOrCreateSession("s1"); err != nil {
		t.Fatalf("GetOrCreateSession: %v", err)
	}

	msgs := []provider.Message{
		{Role: "user", Content: "Hello"},
		{Role: "assistant", Content: "Hi there!"},
		{
			Role:    "assistant",
			Content: "",
			ToolCalls: []provider.ToolCall{
				{ID: "tc1", Function: provider.FunctionCall{Name: "exec", Arguments: `{}`}},
			},
		},
		{Role: "tool", Content: "output", ToolCallID: "tc1", Name: "exec"},
	}
	if err := store.SaveMessages("s1", msgs); err != nil {
		t.Fatalf("SaveMessages: %v", err)
	}

	var count int
	if err := store.db.QueryRow("SELECT COUNT(*) FROM messages WHERE session_key = 's1'").Scan(&count); err != nil {
		t.Fatalf("count messages: %v", err)
	}
	if count != 4 {
		t.Fatalf("message count = %d, want 4", count)
	}
}

func TestSaveMessages_SkipsEmptyAssistant(t *testing.T) {
	store, err := NewStore(":memory:")
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	defer store.Close()

	if _, err := store.GetOrCreateSession("s1"); err != nil {
		t.Fatalf("GetOrCreateSession: %v", err)
	}

	msgs := []provider.Message{
		{Role: "user", Content: "Hello"},
		{Role: "assistant", Content: "", ToolCalls: nil},
		{Role: "assistant", Content: "Real response"},
	}
	if err := store.SaveMessages("s1", msgs); err != nil {
		t.Fatalf("SaveMessages: %v", err)
	}

	var count int
	if err := store.db.QueryRow("SELECT COUNT(*) FROM messages WHERE session_key = 's1'").Scan(&count); err != nil {
		t.Fatalf("count messages: %v", err)
	}
	if count != 2 {
		t.Fatalf("message count = %d, want 2", count)
	}
}

func TestClearSession(t *testing.T) {
	store, err := NewStore(":memory:")
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	defer store.Close()

	if _, err := store.GetOrCreateSession("s1"); err != nil {
		t.Fatalf("GetOrCreateSession: %v", err)
	}
	if err := store.SaveMessages("s1", []provider.Message{{Role: "user", Content: "Hi"}}); err != nil {
		t.Fatalf("SaveMessages: %v", err)
	}
	if err := store.ClearSession("s1"); err != nil {
		t.Fatalf("ClearSession: %v", err)
	}

	var count int
	if err := store.db.QueryRow("SELECT COUNT(*) FROM messages WHERE session_key = 's1'").Scan(&count); err != nil {
		t.Fatalf("count messages: %v", err)
	}
	if count != 0 {
		t.Fatalf("message count after clear = %d, want 0", count)
	}
}

func TestListSessions(t *testing.T) {
	store, err := NewStore(":memory:")
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	defer store.Close()

	if _, err := store.GetOrCreateSession("s1"); err != nil {
		t.Fatalf("GetOrCreateSession(s1): %v", err)
	}
	if _, err := store.GetOrCreateSession("s2"); err != nil {
		t.Fatalf("GetOrCreateSession(s2): %v", err)
	}

	sessions, err := store.ListSessions()
	if err != nil {
		t.Fatalf("ListSessions: %v", err)
	}
	if len(sessions) != 2 {
		t.Fatalf("session count = %d, want 2", len(sessions))
	}
}
