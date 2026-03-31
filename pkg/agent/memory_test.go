package agent

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/Nomadcxx/smolbot/pkg/provider"
	"github.com/Nomadcxx/smolbot/pkg/session"
	"github.com/Nomadcxx/smolbot/pkg/tokenizer"
)

func TestMaybeConsolidateSkipsWhenUnderThreshold(t *testing.T) {
	workspace := prepareMemoryWorkspace(t)
	store := newMemoryStore(t)
	defer store.Close()

	if err := store.SaveMessages("s1", []provider.Message{
		{Role: "user", Content: "hi"},
		{Role: "assistant", Content: "hello"},
	}); err != nil {
		t.Fatalf("SaveMessages: %v", err)
	}

	fake := &fakeMemoryProvider{}
	consolidator := NewMemoryConsolidator(fake, store, tokenizer.New(), workspace, 1000)

	if err := consolidator.MaybeConsolidate(context.Background(), "s1"); err != nil {
		t.Fatalf("MaybeConsolidate: %v", err)
	}
	if fake.calls != 0 {
		t.Fatalf("provider called %d times, want 0", fake.calls)
	}
}

func TestFindBoundaryDoesNotSplitToolCallGroup(t *testing.T) {
	messages := []session.StoredMessage{
		{ID: 1, Message: provider.Message{Role: "user", Content: "first"}},
		{ID: 2, Message: provider.Message{Role: "assistant", ToolCalls: []provider.ToolCall{{ID: "tc1", Function: provider.FunctionCall{Name: "exec", Arguments: "{}"}}}}},
		{ID: 3, Message: provider.Message{Role: "tool", ToolCallID: "tc1", Content: "result"}},
		{ID: 4, Message: provider.Message{Role: "assistant", Content: "done"}},
		{ID: 5, Message: provider.Message{Role: "user", Content: "second"}},
	}

	idx, upToID := findConsolidationBoundary(messages)
	if idx != 3 {
		t.Fatalf("boundary index = %d, want 3", idx)
	}
	if upToID != 4 {
		t.Fatalf("boundary upToID = %d, want 4", upToID)
	}
}

func TestNormalizeSaveMemoryArgs(t *testing.T) {
	tests := []struct {
		name        string
		input       any
		wantHistory string
		wantMemory  string
	}{
		{
			name:        "map",
			input:       map[string]any{"history_entry": "history one", "memory_update": "memory one"},
			wantHistory: "history one",
			wantMemory:  "memory one",
		},
		{
			name:        "json string",
			input:       `{"history_entry":"history two","memory_update":"memory two"}`,
			wantHistory: "history two",
			wantMemory:  "memory two",
		},
		{
			name:        "mixed list",
			input:       []any{"history three", map[string]any{"memory_update": "memory three"}},
			wantHistory: "history three",
			wantMemory:  "memory three",
		},
	}

	for _, tt := range tests {
		historyEntry, memoryUpdate := normalizeSaveMemoryArgs(tt.input)
		if historyEntry != tt.wantHistory || memoryUpdate != tt.wantMemory {
			t.Fatalf("%s => (%q, %q), want (%q, %q)", tt.name, historyEntry, memoryUpdate, tt.wantHistory, tt.wantMemory)
		}
	}
}

func TestRawArchiveFallbackAfterThreeFailures(t *testing.T) {
	workspace := prepareMemoryWorkspace(t)
	store := newMemoryStore(t)
	defer store.Close()

	long := strings.Repeat("context ", 40)
	if err := store.SaveMessages("s1", []provider.Message{
		{Role: "user", Content: long},
		{Role: "assistant", Content: long},
	}); err != nil {
		t.Fatalf("SaveMessages: %v", err)
	}

	fake := &fakeMemoryProvider{err: errors.New("provider failed")}
	consolidator := NewMemoryConsolidator(fake, store, tokenizer.New(), workspace, 10)

	for i := 0; i < 3; i++ {
		_ = consolidator.MaybeConsolidate(context.Background(), "s1")
	}

	historyData, err := os.ReadFile(filepath.Join(workspace, "memory", "HISTORY.md"))
	if err != nil {
		t.Fatalf("read HISTORY.md: %v", err)
	}
	if !strings.Contains(string(historyData), "RAW ARCHIVE") {
		t.Fatalf("history missing raw archive fallback: %q", string(historyData))
	}
	count, err := store.CountUnconsolidated("s1")
	if err != nil {
		t.Fatalf("CountUnconsolidated: %v", err)
	}
	if count != 0 {
		t.Fatalf("unconsolidated count = %d, want 0", count)
	}
}

func TestMaybeConsolidateRepeatsUpToThresholdOrFiveRounds(t *testing.T) {
	workspace := prepareMemoryWorkspace(t)
	store := newMemoryStore(t)
	defer store.Close()

	var msgs []provider.Message
	for i := 0; i < 8; i++ {
		msgs = append(msgs,
			provider.Message{Role: "user", Content: strings.Repeat("u", 120)},
			provider.Message{Role: "assistant", Content: strings.Repeat("a", 120)},
		)
	}
	if err := store.SaveMessages("s1", msgs); err != nil {
		t.Fatalf("SaveMessages: %v", err)
	}

	fake := &fakeMemoryProvider{
		responses: []*provider.Response{
			saveMemoryResponse("history one", "memory one"),
			saveMemoryResponse("history two", "memory two"),
			saveMemoryResponse("history three", "memory three"),
			saveMemoryResponse("history four", "memory four"),
			saveMemoryResponse("history five", "memory five"),
		},
	}
	consolidator := NewMemoryConsolidator(fake, store, tokenizer.New(), workspace, 200)

	if err := consolidator.MaybeConsolidate(context.Background(), "s1"); err != nil {
		t.Fatalf("MaybeConsolidate: %v", err)
	}
	if fake.calls < 2 {
		t.Fatalf("provider calls = %d, want multiple rounds", fake.calls)
	}
	if fake.calls > 5 {
		t.Fatalf("provider calls = %d, want <= 5", fake.calls)
	}
}

func TestConsolidateBatchFallsBackToAutoOnForcedToolChoiceRejection(t *testing.T) {
	workspace := prepareMemoryWorkspace(t)
	store := newMemoryStore(t)
	defer store.Close()

	long := strings.Repeat("context ", 40)
	if err := store.SaveMessages("s1", []provider.Message{
		{Role: "user", Content: long},
		{Role: "assistant", Content: long},
	}); err != nil {
		t.Fatalf("SaveMessages: %v", err)
	}

	refusalErr := errors.New("provider rejected forced tool_choice")
	fake := &fakeMemoryProvider{
		responses: []*provider.Response{
			{Content: "I cannot force save_memory tool", FinishReason: "stop"},
			{Content: "", ToolCalls: []provider.ToolCall{{
				ID:       "call1",
				Function: provider.FunctionCall{Name: "save_memory", Arguments: `{"history_entry":"h","memory_update":"m"}`},
			}}, FinishReason: "tool_calls"},
		},
		refusalErrors: []error{refusalErr, nil},
	}
	consolidator := NewMemoryConsolidator(fake, store, tokenizer.New(), workspace, 10)

	if err := consolidator.MaybeConsolidate(context.Background(), "s1"); err != nil {
		t.Fatalf("MaybeConsolidate: %v", err)
	}

	if fake.calls < 2 {
		t.Fatalf("expected at least 2 calls (first rejected forced, second with auto), got %d", fake.calls)
	}

	firstReq := fake.requests[0]
	if firstReq.ToolChoice != "save_memory" {
		t.Fatalf("first request ToolChoice = %v, want save_memory", firstReq.ToolChoice)
	}

	autoReq := fake.requests[1]
	if autoReq.ToolChoice != "auto" {
		t.Fatalf("second request ToolChoice = %v, want auto (fallback)", autoReq.ToolChoice)
	}
}

type fakeMemoryProvider struct {
	responses      []*provider.Response
	err            error
	calls          int
	lastReq        provider.ChatRequest
	requests       []provider.ChatRequest
	refusalErrors  []error
	refusalIndex   int
}

func (f *fakeMemoryProvider) Chat(_ context.Context, req provider.ChatRequest) (*provider.Response, error) {
	f.calls++
	f.lastReq = req
	f.requests = append(f.requests, req)
	if f.refusalIndex < len(f.refusalErrors) && f.refusalErrors[f.refusalIndex] != nil {
		f.refusalIndex++
		return nil, f.refusalErrors[f.refusalIndex-1]
	}
	if f.err != nil {
		return nil, f.err
	}
	if len(f.responses) == 0 {
		return saveMemoryResponse("history", "memory"), nil
	}
	resp := f.responses[0]
	f.responses = f.responses[1:]
	return resp, nil
}

func (f *fakeMemoryProvider) ChatStream(context.Context, provider.ChatRequest) (*provider.Stream, error) {
	return nil, errors.New("not used")
}

func (f *fakeMemoryProvider) Name() string { return "openai" }

func (f *fakeMemoryProvider) lastRequest() provider.ChatRequest {
	return f.lastReq
}

var _ provider.Provider = (*fakeMemoryProvider)(nil)

func saveMemoryResponse(historyEntry, memoryUpdate string) *provider.Response {
	return &provider.Response{
		ToolCalls: []provider.ToolCall{
			{
				ID: "save_memory",
				Function: provider.FunctionCall{
					Name:      "save_memory",
					Arguments: `{"history_entry":"` + historyEntry + `","memory_update":"` + memoryUpdate + `"}`,
				},
			},
		},
		FinishReason: "tool_calls",
	}
}

func prepareMemoryWorkspace(t *testing.T) string {
	t.Helper()
	workspace := t.TempDir()
	for _, rel := range []string{
		filepath.Join("memory", "MEMORY.md"),
		filepath.Join("memory", "HISTORY.md"),
	} {
		path := filepath.Join(workspace, rel)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatalf("mkdir %q: %v", path, err)
		}
		if err := os.WriteFile(path, []byte(""), 0o644); err != nil {
			t.Fatalf("write %q: %v", path, err)
		}
	}
	return workspace
}

func newMemoryStore(t *testing.T) *session.Store {
	t.Helper()
	store, err := session.NewStore(":memory:")
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	if _, err := store.GetOrCreateSession("s1"); err != nil {
		t.Fatalf("GetOrCreateSession: %v", err)
	}
	return store
}

func TestSessionLocksConcurrentAccessRace(t *testing.T) {
	mem := NewMemoryConsolidator(nil, newMemoryStore(t), nil, t.TempDir(), 128000)
	ctx := context.Background()
	const goroutines = 10
	const iterations = 50
	var wg sync.WaitGroup
	wg.Add(goroutines * iterations)
	for i := 0; i < goroutines; i++ {
		go func() {
			for j := 0; j < iterations; j++ {
				key := fmt.Sprintf("session-%d", j%5)
				mem.MaybeConsolidate(ctx, key)
				wg.Done()
			}
		}()
	}
	wg.Wait()
}
