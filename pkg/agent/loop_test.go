package agent

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Nomadcxx/smolbot/pkg/config"
	"github.com/Nomadcxx/smolbot/pkg/provider"
	"github.com/Nomadcxx/smolbot/pkg/session"
	"github.com/Nomadcxx/smolbot/pkg/skill"
	"github.com/Nomadcxx/smolbot/pkg/tokenizer"
	"github.com/Nomadcxx/smolbot/pkg/tool"
	"github.com/Nomadcxx/smolbot/pkg/usage"
)

func TestAgentLoopHelpAndNew(t *testing.T) {
	loop, store, fakeMemory := newTestAgentLoop(t, &fakeStreamProvider{})
	defer store.Close()

	if err := store.SaveMessages("s1", []provider.Message{
		{Role: "user", Content: "old"},
		{Role: "assistant", Content: "reply"},
	}); err != nil {
		t.Fatalf("SaveMessages: %v", err)
	}

	resp, err := loop.ProcessDirect(context.Background(), Request{
		Content:    "/help",
		SessionKey: "s1",
	}, nil)
	if err != nil {
		t.Fatalf("/help: %v", err)
	}
	if !strings.Contains(resp, "/new") || !strings.Contains(resp, "/stop") {
		t.Fatalf("help text missing commands: %q", resp)
	}

	resp, err = loop.ProcessDirect(context.Background(), Request{
		Content:    "/new",
		SessionKey: "s1",
	}, nil)
	if err != nil {
		t.Fatalf("/new: %v", err)
	}
	if !strings.Contains(resp, "new session") {
		t.Fatalf("unexpected /new response: %q", resp)
	}
	if fakeMemory.calls != 1 {
		t.Fatalf("memory consolidator calls = %d, want 1", fakeMemory.calls)
	}
	count, err := store.CountUnconsolidated("s1")
	if err != nil {
		t.Fatalf("CountUnconsolidated: %v", err)
	}
	if count != 0 {
		t.Fatalf("session not cleared, unconsolidated count = %d", count)
	}
}

func TestAgentLoopStopCancelsActiveSession(t *testing.T) {
	fakeProvider := &fakeStreamProvider{
		streams: []fakeStreamScript{{blockUntilCancel: true}},
	}
	loop, store, _ := newTestAgentLoop(t, fakeProvider)
	defer store.Close()
	cancelled := false
	loop.tools.SetCancelSession(func(sessionKey string) {
		if sessionKey == "s1" {
			cancelled = true
		}
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan error, 1)
	go func() {
		_, err := loop.ProcessDirect(ctx, Request{
			Content:    "run",
			SessionKey: "s1",
		}, nil)
		done <- err
	}()

	waitUntil(t, func() bool { return fakeProvider.activeStreamCount() == 1 })

	resp, err := loop.ProcessDirect(context.Background(), Request{
		Content:    "/stop",
		SessionKey: "s1",
	}, nil)
	if err != nil {
		t.Fatalf("/stop: %v", err)
	}
	if !strings.Contains(resp, "stopped") {
		t.Fatalf("unexpected /stop response: %q", resp)
	}
	if !cancelled {
		t.Fatalf("tool session cancellation was not invoked")
	}

	select {
	case err := <-done:
		if err == nil {
			t.Fatalf("expected cancelled run to return an error")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("active session did not cancel")
	}
}

func TestAgentLoopSanitizesOutboundMessagesAndPersistsNormalizedTurn(t *testing.T) {
	fakeProvider := &fakeStreamProvider{
		streams: []fakeStreamScript{{
			deltas: []*provider.StreamDelta{
				{Content: "<think>secret</think>Visible answer"},
				{FinishReason: stringPtr("stop")},
			},
		}},
	}
	loop, store, _ := newTestAgentLoop(t, fakeProvider)
	defer store.Close()

	longID := "call_very_long_id_that_exceeds_nine_chars"
	if err := store.SaveMessages("s1", []provider.Message{
		{Role: "user", Content: "earlier"},
		{
			Role: "assistant",
			ToolCalls: []provider.ToolCall{
				{ID: longID, Function: provider.FunctionCall{Name: "exec", Arguments: `{}`}},
			},
		},
		{Role: "tool", Content: "", ToolCallID: longID, Name: "exec"},
	}); err != nil {
		t.Fatalf("SaveMessages: %v", err)
	}

	var events []Event
	resp, err := loop.ProcessDirect(context.Background(), Request{
		Content:    "describe image",
		SessionKey: "s1",
		Channel:    "gateway",
		ChatID:     "ws-client-1",
		Media: []MediaAttachment{{
			Data:     []byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n'},
			MimeType: "image/png",
		}},
	}, func(ev Event) {
		events = append(events, ev)
	})
	if err != nil {
		t.Fatalf("ProcessDirect: %v", err)
	}
	if resp != "Visible answer" {
		t.Fatalf("response = %q, want Visible answer", resp)
	}
	if len(fakeProvider.requests) == 0 {
		t.Fatalf("provider did not receive any requests")
	}

	reqMessages := fakeProvider.requests[0].Messages
	if len(reqMessages) < 3 {
		t.Fatalf("provider request missing expected history and user messages: %+v", reqMessages)
	}
	foundSanitizedToolCall := false
	for _, msg := range reqMessages {
		if len(msg.ToolCalls) == 0 {
			continue
		}
		foundSanitizedToolCall = true
		if msg.ToolCalls[0].ID == longID {
			t.Fatalf("tool call id was not sanitized")
		}
	}
	if !foundSanitizedToolCall {
		t.Fatalf("outbound request missing expected tool call history: %+v", reqMessages)
	}
	if reqMessages[len(reqMessages)-1].StringContent() == "describe image" {
		t.Fatalf("runtime context prefix was not applied to outbound user message")
	}

	history, err := store.GetHistory("s1", 500)
	if err != nil {
		t.Fatalf("GetHistory: %v", err)
	}
	lastUser := history[len(history)-2]
	lastAssistant := history[len(history)-1]
	if strings.Contains(lastUser.StringContent(), "[Runtime Context") {
		t.Fatalf("runtime context prefix leaked into saved history: %q", lastUser.StringContent())
	}
	if !strings.Contains(lastUser.StringContent(), "[image]") {
		t.Fatalf("saved user message missing image placeholder: %q", lastUser.StringContent())
	}
	if strings.Contains(lastAssistant.StringContent(), "<think>") {
		t.Fatalf("saved assistant message still contains think block: %q", lastAssistant.StringContent())
	}
	if len(events) == 0 || events[len(events)-1].Type != EventDone {
		t.Fatalf("expected final done event, got %+v", events)
	}
}

func TestAgentLoopMessageSuppressesFinalResponseForSameTarget(t *testing.T) {
	fakeProvider := &fakeStreamProvider{
		streams: []fakeStreamScript{
			{
				deltas: []*provider.StreamDelta{
					{
						ToolCalls: []provider.ToolCall{{
							ID:    "call1",
							Index: 0,
							Function: provider.FunctionCall{
								Name:      "message",
								Arguments: `{"channel":"gateway","chat_id":"ws-client-1","content":"hello"}`,
							},
						}},
						FinishReason: stringPtr("tool_calls"),
					},
				},
			},
			{
				deltas: []*provider.StreamDelta{
					{Content: "final "},
					{Content: "reply"},
					{FinishReason: stringPtr("stop")},
				},
			},
		},
	}

	loop, store, _ := newTestAgentLoop(t, fakeProvider, tool.NewMessageTool())
	defer store.Close()

	var events []Event
	resp, err := loop.ProcessDirect(context.Background(), Request{
		Content:    "send message",
		SessionKey: "s1",
		Channel:    "gateway",
		ChatID:     "ws-client-1",
	}, func(ev Event) {
		events = append(events, ev)
	})
	if err != nil {
		t.Fatalf("ProcessDirect: %v", err)
	}
	if resp != "" {
		t.Fatalf("response = %q, want suppression", resp)
	}

	var toolDone Event
	found := false
	for _, ev := range events {
		if ev.Type == EventToolDone {
			toolDone = ev
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("missing tool.done event")
	}
	if delivered, _ := toolDone.Data["deliveredToRequestTarget"].(bool); !delivered {
		t.Fatalf("tool.done missing deliveredToRequestTarget metadata: %+v", toolDone.Data)
	}
	for _, ev := range events {
		if ev.Type == EventProgress || ev.Type == EventThinking || ev.Type == EventDone {
			t.Fatalf("unexpected assistant event leaked through suppression: %+v", ev)
		}
	}

	history, err := store.GetHistory("s1", 500)
	if err != nil {
		t.Fatalf("GetHistory: %v", err)
	}
	var toolMsg provider.Message
	for _, msg := range history {
		if msg.Role == "tool" {
			toolMsg = msg
			break
		}
	}
	if toolMsg.StringContent() != "message sent" {
		t.Fatalf("tool result = %q, want message sent", toolMsg.StringContent())
	}
}

func TestAgentLoopSkipsPersistenceOnProviderErrorAndRejectsBusySession(t *testing.T) {
	fakeProvider := &fakeStreamProvider{
		streams: []fakeStreamScript{{blockUntilCancel: true}},
	}
	loop, store, _ := newTestAgentLoop(t, fakeProvider)
	defer store.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() {
		_, err := loop.ProcessDirect(ctx, Request{Content: "first", SessionKey: "s1"}, nil)
		done <- err
	}()
	waitUntil(t, func() bool { return fakeProvider.activeStreamCount() == 1 })

	_, err := loop.ProcessDirect(context.Background(), Request{Content: "second", SessionKey: "s1"}, nil)
	if err == nil || !strings.Contains(err.Error(), "busy") {
		t.Fatalf("expected busy error, got %v", err)
	}
	cancel()
	<-done

	errorProvider := &fakeStreamProvider{
		streams: []fakeStreamScript{{
			deltas: []*provider.StreamDelta{
				{FinishReason: stringPtr("error")},
			},
		}},
	}
	loop, store, _ = newTestAgentLoop(t, errorProvider)
	defer store.Close()

	_, err = loop.ProcessDirect(context.Background(), Request{Content: "boom", SessionKey: "s2"}, nil)
	if err == nil {
		t.Fatalf("expected provider error")
	}
	count, err := store.CountUnconsolidated("s2")
	if err != nil {
		t.Fatalf("CountUnconsolidated: %v", err)
	}
	if count != 0 {
		t.Fatalf("error turn should not persist, count = %d", count)
	}
}

func TestAgentLoopRespectsRequestOverridesForDelegatedChildRuns(t *testing.T) {
	fakeProvider := &fakeStreamProvider{
		streams: []fakeStreamScript{{
			deltas: []*provider.StreamDelta{
				{Content: "delegated response"},
				{FinishReason: stringPtr("stop")},
			},
		}},
	}

	spawnTool := &fakeTool{name: "spawn", result: &tool.Result{Output: "nope"}}
	taskTool := &fakeTool{name: "task", result: &tool.Result{Output: "nope"}}
	readTool := &fakeTool{name: "read_file", result: &tool.Result{Output: "ok"}}
	loop, store, _ := newTestAgentLoop(t, fakeProvider, spawnTool, taskTool, readTool)
	defer store.Close()

	_, err := loop.ProcessDirect(context.Background(), Request{
		Content:         "run delegated child",
		SessionKey:      "child-session",
		Model:           "gpt-5.4-mini",
		ReasoningEffort: "high",
		MaxIterations:   2,
		DisabledTools:   []string{"spawn", "task"},
	}, nil)
	if err != nil {
		t.Fatalf("ProcessDirect: %v", err)
	}
	if len(fakeProvider.requests) != 1 {
		t.Fatalf("expected one provider request, got %d", len(fakeProvider.requests))
	}
	req := fakeProvider.requests[0]
	if req.Model != "gpt-5.4-mini" || req.ReasoningEffort != "high" {
		t.Fatalf("expected per-request overrides, got %#v", req)
	}
	if len(req.Tools) != 1 || req.Tools[0].Name != "read_file" {
		t.Fatalf("expected disabled tools to be excluded, got %#v", req.Tools)
	}
}

func TestAgentLoopPassesRoutingDepsAndUsesToolOutput(t *testing.T) {
	fakeProvider := &fakeStreamProvider{
		streams: []fakeStreamScript{
			{
				deltas: []*provider.StreamDelta{
					{
						ToolCalls: []provider.ToolCall{{
							ID:    "call1",
							Index: 0,
							Function: provider.FunctionCall{
								Name:      "capture",
								Arguments: `{}`,
							},
						}},
						FinishReason: stringPtr("tool_calls"),
					},
				},
			},
			{
				deltas: []*provider.StreamDelta{
					{Content: "done"},
					{FinishReason: stringPtr("stop")},
				},
			},
		},
	}

	captureTool := &capturingTool{
		name:   "capture",
		result: &tool.Result{Output: "tool output"},
	}
	loop, store, _ := newTestAgentLoop(t, fakeProvider, captureTool)
	defer store.Close()

	var events []Event
	_, err := loop.ProcessDirect(context.Background(), Request{
		Content:    "run tool",
		SessionKey: "s1",
		Channel:    "gateway",
		ChatID:     "chat-1",
	}, func(ev Event) {
		events = append(events, ev)
	})
	if err != nil {
		t.Fatalf("ProcessDirect: %v", err)
	}

	if captureTool.lastContext.MessageRouter == nil {
		t.Fatal("expected message router in tool context")
	}
	if captureTool.lastContext.Spawner == nil {
		t.Fatal("expected spawner in tool context")
	}
	if captureTool.lastContext.Workspace == "" {
		t.Fatal("expected workspace in tool context")
	}

	history, err := store.GetHistory("s1", 100)
	if err != nil {
		t.Fatalf("GetHistory: %v", err)
	}
	foundTool := false
	for _, msg := range history {
		if msg.Role == "tool" {
			foundTool = true
			if msg.StringContent() != "tool output" {
				t.Fatalf("tool message content = %q, want tool output", msg.StringContent())
			}
		}
	}
	if !foundTool {
		t.Fatal("expected tool message in history")
	}

	for _, ev := range events {
		if ev.Type != EventToolDone {
			continue
		}
		if got, _ := ev.Data["output"].(string); got != "tool output" {
			t.Fatalf("tool.done output = %q, want tool output", got)
		}
		if got, _ := ev.Data["error"].(string); got != "" {
			t.Fatalf("tool.done error = %q, want empty", got)
		}
		return
	}
	t.Fatal("expected tool.done event")
}

func TestAgentLoopEmitsToolErrorsToTranscriptAndEvents(t *testing.T) {
	fakeProvider := &fakeStreamProvider{
		streams: []fakeStreamScript{
			{
				deltas: []*provider.StreamDelta{
					{
						ToolCalls: []provider.ToolCall{{
							ID:    "call1",
							Index: 0,
							Function: provider.FunctionCall{
								Name:      "capture",
								Arguments: `{}`,
							},
						}},
						FinishReason: stringPtr("tool_calls"),
					},
				},
			},
			{
				deltas: []*provider.StreamDelta{
					{Content: "done"},
					{FinishReason: stringPtr("stop")},
				},
			},
		},
	}

	captureTool := &capturingTool{
		name:   "capture",
		result: &tool.Result{Error: "permission denied"},
	}
	loop, store, _ := newTestAgentLoop(t, fakeProvider, captureTool)
	defer store.Close()

	var events []Event
	_, err := loop.ProcessDirect(context.Background(), Request{
		Content:    "run tool",
		SessionKey: "s1",
	}, func(ev Event) {
		events = append(events, ev)
	})
	if err != nil {
		t.Fatalf("ProcessDirect: %v", err)
	}

	history, err := store.GetHistory("s1", 100)
	if err != nil {
		t.Fatalf("GetHistory: %v", err)
	}
	for _, msg := range history {
		if msg.Role != "tool" {
			continue
		}
		if !strings.Contains(msg.StringContent(), "permission denied") {
			t.Fatalf("tool message content = %q, want to contain permission denied", msg.StringContent())
		}
		goto foundHistory
	}
	t.Fatal("expected tool message in history")

foundHistory:
	for _, ev := range events {
		if ev.Type != EventToolDone {
			continue
		}
		if got, _ := ev.Data["error"].(string); !strings.Contains(got, "permission denied") {
			t.Fatalf("tool.done error = %q, want to contain permission denied", got)
		}
		if got, _ := ev.Data["output"].(string); got != "" {
			t.Fatalf("tool.done output = %q, want empty", got)
		}
		return
	}
	t.Fatal("expected tool.done event")
}

func TestAgentLoopEmitsUsageEventsFromStreamDeltas(t *testing.T) {
	fakeProvider := &fakeStreamProvider{
		streams: []fakeStreamScript{{
			deltas: []*provider.StreamDelta{
				{Content: "done"},
				{Usage: &provider.Usage{PromptTokens: 12, CompletionTokens: 8, TotalTokens: 20}},
				{FinishReason: stringPtr("stop")},
			},
		}},
	}

	loop, store, _ := newTestAgentLoop(t, fakeProvider)
	defer store.Close()

	var events []Event
	resp, err := loop.ProcessDirect(context.Background(), Request{
		Content:    "hello",
		SessionKey: "s1",
	}, func(ev Event) {
		events = append(events, ev)
	})
	if err != nil {
		t.Fatalf("ProcessDirect: %v", err)
	}
	if resp != "done" {
		t.Fatalf("response = %q, want done", resp)
	}

	foundUsage := false
	for _, ev := range events {
		if ev.Type != EventUsage {
			continue
		}
		foundUsage = true
		if got, _ := ev.Data["totalTokens"].(int); got != 20 {
			t.Fatalf("usage totalTokens = %v, want 20", ev.Data["totalTokens"])
		}
		if got, _ := ev.Data["promptTokens"].(int); got != 12 {
			t.Fatalf("usage promptTokens = %v, want 12", ev.Data["promptTokens"])
		}
		if got, _ := ev.Data["completionTokens"].(int); got != 8 {
			t.Fatalf("usage completionTokens = %v, want 8", ev.Data["completionTokens"])
		}
	}
	if !foundUsage {
		t.Fatalf("expected usage event, got %+v", events)
	}
}

func TestAgentLoopPersistsReportedUsage(t *testing.T) {
	usageStore, err := usage.NewStore(":memory:")
	if err != nil {
		t.Fatalf("usage.NewStore: %v", err)
	}
	defer usageStore.Close()

	loop, store, _ := newTestAgentLoopWithUsage(t, &fakeStreamProvider{
		streams: []fakeStreamScript{{
			deltas: []*provider.StreamDelta{
				{Content: "done"},
				{Usage: &provider.Usage{PromptTokens: 12, CompletionTokens: 8, TotalTokens: 20}},
				{FinishReason: stringPtr("stop")},
			},
		}},
	}, usageStore)
	defer store.Close()

	resp, err := loop.ProcessDirect(context.Background(), Request{
		Content:    "hello",
		SessionKey: "s1",
	}, nil)
	if err != nil {
		t.Fatalf("ProcessDirect: %v", err)
	}
	if resp != "done" {
		t.Fatalf("response = %q, want done", resp)
	}

	records, err := usageStore.ListUsageRecords("s1")
	if err != nil {
		t.Fatalf("ListUsageRecords: %v", err)
	}
	if len(records) != 1 {
		t.Fatalf("usage records = %d, want 1", len(records))
	}
	if records[0].UsageSource != "reported" {
		t.Fatalf("usage source = %q, want reported", records[0].UsageSource)
	}
	if records[0].TotalTokens != 20 {
		t.Fatalf("total tokens = %d, want 20", records[0].TotalTokens)
	}
}

func TestAgentLoopPersistsEstimatedUsageWhenProviderOmitsUsage(t *testing.T) {
	usageStore, err := usage.NewStore(":memory:")
	if err != nil {
		t.Fatalf("usage.NewStore: %v", err)
	}
	defer usageStore.Close()

	loop, store, _ := newTestAgentLoopWithUsage(t, &fakeStreamProvider{
		streams: []fakeStreamScript{{
			deltas: []*provider.StreamDelta{
				{Content: "hello world"},
				{FinishReason: stringPtr("stop")},
			},
		}},
	}, usageStore)
	defer store.Close()

	resp, err := loop.ProcessDirect(context.Background(), Request{
		Content:    "hello",
		SessionKey: "s1",
	}, nil)
	if err != nil {
		t.Fatalf("ProcessDirect: %v", err)
	}
	if resp != "hello world" {
		t.Fatalf("response = %q, want hello world", resp)
	}

	records, err := usageStore.ListUsageRecords("s1")
	if err != nil {
		t.Fatalf("ListUsageRecords: %v", err)
	}
	if len(records) != 1 {
		t.Fatalf("usage records = %d, want 1", len(records))
	}
	if records[0].UsageSource != "estimated" {
		t.Fatalf("usage source = %q, want estimated", records[0].UsageSource)
	}
	if records[0].TotalTokens <= 0 {
		t.Fatalf("total tokens = %d, want > 0", records[0].TotalTokens)
	}
}

func TestAgentLoopPersistsUsageAgainstRequestModelEvenIfConfigChangesMidRun(t *testing.T) {
	usageStore, err := usage.NewStore(":memory:")
	if err != nil {
		t.Fatalf("usage.NewStore: %v", err)
	}
	defer usageStore.Close()

	providerWithMutation := &mutatingModelProvider{}
	loop, store, _ := newTestAgentLoopWithUsage(t, providerWithMutation, usageStore)
	defer store.Close()
	providerWithMutation.onRequest = func() {
		loop.SetActiveModel("gpt-4.1")
	}

	resp, err := loop.ProcessDirect(context.Background(), Request{
		Content:    "hello",
		SessionKey: "s1",
	}, nil)
	if err != nil {
		t.Fatalf("ProcessDirect: %v", err)
	}
	if resp != "done" {
		t.Fatalf("response = %q, want done", resp)
	}

	records, err := usageStore.ListUsageRecords("s1")
	if err != nil {
		t.Fatalf("ListUsageRecords: %v", err)
	}
	if len(records) != 1 {
		t.Fatalf("usage records = %d, want 1", len(records))
	}
	if records[0].ModelName != "gpt-4o" {
		t.Fatalf("model name = %q, want original request model gpt-4o", records[0].ModelName)
	}
	if providerWithMutation.lastModel != "gpt-4o" {
		t.Fatalf("provider request model = %q, want gpt-4o", providerWithMutation.lastModel)
	}
}

func TestAgentLoopCompactNowRewritesSessionHistory(t *testing.T) {
	loop, store, _ := newTestAgentLoop(t, &fakeStreamProvider{})
	defer store.Close()

	long := strings.Repeat("This is a long message that should be compacted. ", 80)
	if err := store.SaveMessages("s1", []provider.Message{
		{Role: "system", Content: "system prompt"},
		{Role: "user", Content: long},
		{Role: "assistant", Content: long},
		{Role: "user", Content: "recent question"},
		{Role: "assistant", Content: "recent answer"},
	}); err != nil {
		t.Fatalf("SaveMessages: %v", err)
	}

	original, compressed, pct, err := loop.CompactNow(context.Background(), "s1")
	if err != nil {
		t.Fatalf("CompactNow: %v", err)
	}
	if original == 0 || compressed == 0 || pct <= 0 {
		t.Fatalf("expected compaction stats, got original=%d compressed=%d pct=%f", original, compressed, pct)
	}

	history, err := store.GetHistory("s1", 500)
	if err != nil {
		t.Fatalf("GetHistory: %v", err)
	}
	if len(history) >= 5 {
		t.Fatalf("expected compacted history to be no longer than original, got %d", len(history))
	}
	if got := history[1].StringContent(); len(got) >= len(long) {
		t.Fatalf("expected compacted message to be shorter, got len=%d want<%d", len(got), len(long))
	}
}

type fakeStreamProvider struct {
	mu       sync.Mutex
	streams  []fakeStreamScript
	requests []provider.ChatRequest
	active   int
}

type fakeStreamScript struct {
	deltas           []*provider.StreamDelta
	blockUntilCancel bool
}

func (f *fakeStreamProvider) Chat(context.Context, provider.ChatRequest) (*provider.Response, error) {
	return nil, errors.New("not used")
}

func (f *fakeStreamProvider) ChatStream(ctx context.Context, req provider.ChatRequest) (*provider.Stream, error) {
	f.mu.Lock()
	f.requests = append(f.requests, req)
	if len(f.streams) == 0 {
		f.mu.Unlock()
		return nil, errors.New("no scripted stream")
	}
	script := f.streams[0]
	f.streams = f.streams[1:]
	f.active++
	f.mu.Unlock()

	idx := 0
	return provider.NewStream(func() (*provider.StreamDelta, error) {
		if script.blockUntilCancel {
			<-ctx.Done()
			return nil, ctx.Err()
		}
		if idx >= len(script.deltas) {
			return nil, io.EOF
		}
		delta := script.deltas[idx]
		idx++
		return delta, nil
	}, func() error {
		f.mu.Lock()
		defer f.mu.Unlock()
		f.active--
		return nil
	}), nil
}

func (f *fakeStreamProvider) Name() string { return "openai" }

func (f *fakeStreamProvider) activeStreamCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.active
}

type mutatingModelProvider struct {
	onRequest func()
	lastModel string
}

func (m *mutatingModelProvider) Chat(context.Context, provider.ChatRequest) (*provider.Response, error) {
	return nil, errors.New("not used")
}

func (m *mutatingModelProvider) ChatStream(_ context.Context, req provider.ChatRequest) (*provider.Stream, error) {
	m.lastModel = req.Model
	if m.onRequest != nil {
		m.onRequest()
	}
	idx := 0
	deltas := []*provider.StreamDelta{
		{Content: "done"},
		{Usage: &provider.Usage{PromptTokens: 12, CompletionTokens: 8, TotalTokens: 20}},
		{FinishReason: stringPtr("stop")},
	}
	return provider.NewStream(func() (*provider.StreamDelta, error) {
		if idx >= len(deltas) {
			return nil, io.EOF
		}
		delta := deltas[idx]
		idx++
		return delta, nil
	}, func() error { return nil }), nil
}

func (m *mutatingModelProvider) Name() string { return "openai" }

type fakeTool struct {
	name   string
	result *tool.Result
	err    error
}

func (f *fakeTool) Name() string        { return f.name }
func (f *fakeTool) Description() string { return f.name + " tool" }
func (f *fakeTool) Parameters() map[string]any {
	return map[string]any{"type": "object"}
}
func (f *fakeTool) Execute(context.Context, json.RawMessage, tool.ToolContext) (*tool.Result, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.result, nil
}

type capturingTool struct {
	name        string
	result      *tool.Result
	lastContext tool.ToolContext
}

func (f *capturingTool) Name() string        { return f.name }
func (f *capturingTool) Description() string { return f.name + " tool" }
func (f *capturingTool) Parameters() map[string]any {
	return map[string]any{"type": "object"}
}
func (f *capturingTool) Execute(_ context.Context, _ json.RawMessage, tctx tool.ToolContext) (*tool.Result, error) {
	f.lastContext = tctx
	return f.result, nil
}

type fakeMessageRouter struct{}

func (fakeMessageRouter) Route(context.Context, string, string, string) error { return nil }

type fakeSpawner struct{}

func (fakeSpawner) Spawn(context.Context, tool.SpawnRequest) (*tool.SpawnResult, error) {
	return &tool.SpawnResult{ID: "spawned", SessionKey: "spawned", Name: "Bernoulli"}, nil
}

func (fakeSpawner) ProcessDirect(context.Context, tool.SpawnRequest) (string, error) {
	return "spawned", nil
}

func (fakeSpawner) Wait(context.Context, tool.WaitRequest) (*tool.WaitResult, error) {
	return &tool.WaitResult{}, nil
}

type fakeLoopMemory struct {
	calls int
}

func (f *fakeLoopMemory) MaybeConsolidate(context.Context, string) error {
	f.calls++
	return nil
}

func newTestAgentLoop(t *testing.T, p provider.Provider, tools ...tool.Tool) (*AgentLoop, *session.Store, *fakeLoopMemory) {
	return newTestAgentLoopWithUsage(t, p, nil, tools...)
}

func newTestAgentLoopWithUsage(t *testing.T, p provider.Provider, recorder usage.Recorder, tools ...tool.Tool) (*AgentLoop, *session.Store, *fakeLoopMemory) {
	t.Helper()

	workspace := t.TempDir()
	if err := SyncWorkspaceTemplates(workspace); err != nil {
		t.Fatalf("SyncWorkspaceTemplates: %v", err)
	}
	paths := config.NewPaths(t.TempDir())
	paths.SetWorkspace(workspace)
	reg, err := skill.NewRegistry(paths)
	if err != nil {
		t.Fatalf("NewRegistry: %v", err)
	}
	store, err := session.NewStore(":memory:")
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	toolRegistry := tool.NewRegistry()
	for _, registered := range tools {
		toolRegistry.Register(registered)
	}

	cfg := config.DefaultConfig()
	cfg.Agents.Defaults.Model = "gpt-4o"
	cfg.Agents.Defaults.ContextWindowTokens = 1

	fakeMemory := &fakeLoopMemory{}
	loop := NewAgentLoop(LoopDeps{
		Provider:      p,
		Tools:         toolRegistry,
		Sessions:      store,
		UsageRecorder: recorder,
		Config:        &cfg,
		Skills:        reg,
		Tokenizer:     tokenizer.New(),
		Memory:        fakeMemory,
		Workspace:     workspace,
		MessageRouter: fakeMessageRouter{},
		Spawner:       fakeSpawner{},
	})
	return loop, store, fakeMemory
}

func waitUntil(t *testing.T, fn func() bool) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if fn() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("condition not met before timeout")
}

func stringPtr(s string) *string { return &s }
