package agent

import (
	"context"
	"reflect"
	"strings"
	"testing"

	"github.com/Nomadcxx/smolbot/pkg/agent/compression/dcp"
	"github.com/Nomadcxx/smolbot/pkg/provider"
)

func TestLoopDCPModePassthrough(t *testing.T) {
	fakeProvider := &fakeStreamProvider{
		streams: []fakeStreamScript{{
			deltas: []*provider.StreamDelta{
				{Content: "done"},
				{FinishReason: stringPtr("stop")},
			},
		}},
	}
	loop, store, _ := newTestAgentLoop(t, fakeProvider)
	defer store.Close()
	loop.config.Agents.Defaults.Compression.Engine = "dcp"
	loop.config.Agents.Defaults.Compression.DCP.Deduplication.Enabled = false
	loop.config.Agents.Defaults.Compression.DCP.PurgeErrors.Enabled = false
	loop.config.Agents.Defaults.Compression.DCP.CompressTool.Enabled = false

	if err := store.SaveMessages("s1", []provider.Message{
		{Role: "user", Content: "hello"},
		{Role: "assistant", Content: "world"},
	}); err != nil {
		t.Fatalf("SaveMessages: %v", err)
	}

	resp, err := loop.ProcessDirect(context.Background(), Request{
		Content:    "next",
		SessionKey: "s1",
	}, nil)
	if err != nil {
		t.Fatalf("ProcessDirect: %v", err)
	}
	if resp != "done" {
		t.Fatalf("response = %q, want done", resp)
	}
	if len(fakeProvider.requests) != 1 {
		t.Fatalf("provider requests = %d, want 1", len(fakeProvider.requests))
	}
	got := fakeProvider.requests[0].Messages
	if len(got) < 4 {
		t.Fatalf("request messages = %+v, want system+history+user", got)
	}
	if !reflect.DeepEqual(got[1:3], []provider.Message{
		{Role: "user", Content: "hello"},
		{Role: "assistant", Content: "world"},
	}) {
		t.Fatalf("history changed in passthrough mode: %+v", got)
	}
}

func TestLoopDCPModeDedupLeavesStoredHistoryUntouched(t *testing.T) {
	fakeProvider := &fakeStreamProvider{
		streams: []fakeStreamScript{{
			deltas: []*provider.StreamDelta{
				{Content: "done"},
				{FinishReason: stringPtr("stop")},
			},
		}},
	}
	loop, store, _ := newTestAgentLoop(t, fakeProvider)
	defer store.Close()
	loop.config.Agents.Defaults.Compression.Engine = "dcp"
	loop.config.Agents.Defaults.Compression.DCP.PurgeErrors.Enabled = false
	loop.config.Agents.Defaults.Compression.DCP.CompressTool.Enabled = false

	if err := store.SaveMessages("s1", []provider.Message{
		{Role: "user", Content: "one"},
		{Role: "assistant", ToolCalls: []provider.ToolCall{{ID: "tc1", Function: provider.FunctionCall{Name: "read_file", Arguments: `{"path":"a.txt"}`}}}},
		{Role: "tool", ToolCallID: "tc1", Name: "read_file", Content: "A"},
		{Role: "assistant", ToolCalls: []provider.ToolCall{{ID: "tc2", Function: provider.FunctionCall{Name: "read_file", Arguments: `{"path":"a.txt"}`}}}},
		{Role: "tool", ToolCallID: "tc2", Name: "read_file", Content: "A"},
	}); err != nil {
		t.Fatalf("SaveMessages: %v", err)
	}

	if _, err := loop.ProcessDirect(context.Background(), Request{
		Content:    "next",
		SessionKey: "s1",
	}, nil); err != nil {
		t.Fatalf("ProcessDirect: %v", err)
	}

	reqMsgs := fakeProvider.requests[0].Messages
	foundPlaceholder := false
	for _, msg := range reqMsgs {
		if strings.Contains(msg.StringContent(), dcp.DedupPlaceholder) {
			foundPlaceholder = true
		}
	}
	if !foundPlaceholder {
		t.Fatalf("request messages missing dedup placeholder: %+v", reqMsgs)
	}

	history, err := store.GetHistory("s1", 500)
	if err != nil {
		t.Fatalf("GetHistory: %v", err)
	}
	if history[2].StringContent() != "A" {
		t.Fatalf("stored history mutated: %q", history[2].StringContent())
	}
}

func TestLoopDCPStatePersistedAcrossRequests(t *testing.T) {
	fakeProvider := &fakeStreamProvider{
		streams: []fakeStreamScript{
			{deltas: []*provider.StreamDelta{{Content: "one"}, {FinishReason: stringPtr("stop")}}},
			{deltas: []*provider.StreamDelta{{Content: "two"}, {FinishReason: stringPtr("stop")}}},
		},
	}
	loop, store, _ := newTestAgentLoop(t, fakeProvider)
	defer store.Close()
	loop.config.Agents.Defaults.Compression.Engine = "dcp"
	loop.config.Agents.Defaults.Compression.DCP.Deduplication.Enabled = false
	loop.config.Agents.Defaults.Compression.DCP.PurgeErrors.Enabled = false
	loop.config.Agents.Defaults.Compression.DCP.CompressTool.Enabled = false

	for _, content := range []string{"a", "b"} {
		if _, err := loop.ProcessDirect(context.Background(), Request{
			Content:    content,
			SessionKey: "s1",
		}, nil); err != nil {
			t.Fatalf("ProcessDirect(%q): %v", content, err)
		}
	}

	state, err := loop.dcpStateManager.Load("s1")
	if err != nil {
		t.Fatalf("Load state: %v", err)
	}
	if state.RequestCount != 2 {
		t.Fatalf("RequestCount = %d, want 2", state.RequestCount)
	}
}

func TestLoopDCPRegistersCompressTool(t *testing.T) {
	fakeProvider := &fakeStreamProvider{
		streams: []fakeStreamScript{{
			deltas: []*provider.StreamDelta{
				{Content: "done"},
				{FinishReason: stringPtr("stop")},
			},
		}},
	}
	loop, store, _ := newTestAgentLoop(t, fakeProvider)
	defer store.Close()
	loop.config.Agents.Defaults.Compression.Engine = "dcp"
	loop.config.Agents.Defaults.Compression.DCP.Deduplication.Enabled = false
	loop.config.Agents.Defaults.Compression.DCP.PurgeErrors.Enabled = false
	loop.config.Agents.Defaults.Compression.DCP.CompressTool.Enabled = true

	if _, err := loop.ProcessDirect(context.Background(), Request{
		Content:    "hello",
		SessionKey: "s1",
	}, nil); err != nil {
		t.Fatalf("ProcessDirect: %v", err)
	}

	if len(fakeProvider.requests) != 1 {
		t.Fatalf("provider requests = %d, want 1", len(fakeProvider.requests))
	}
	found := false
	for _, def := range fakeProvider.requests[0].Tools {
		if def.Name == "compress" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("compress tool was not visible in DCP mode request")
	}
}

func TestLoopDefaultEngineIsLegacy(t *testing.T) {
	fakeProvider := &fakeStreamProvider{
		streams: []fakeStreamScript{{
			deltas: []*provider.StreamDelta{
				{Content: "done"},
				{FinishReason: stringPtr("stop")},
			},
		}},
	}
	loop, store, _ := newTestAgentLoop(t, fakeProvider)
	defer store.Close()
	loop.config.Agents.Defaults.Compression.Engine = ""
	loop.config.Agents.Defaults.ContextWindowTokens = 1
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

	if _, err := loop.ProcessDirect(context.Background(), Request{
		Content:    "next",
		SessionKey: "s1",
	}, nil); err != nil {
		t.Fatalf("ProcessDirect: %v", err)
	}

	history, err := store.GetHistory("s1", 500)
	if err != nil {
		t.Fatalf("GetHistory: %v", err)
	}
	if len(history) < 2 {
		t.Fatalf("history too short after processing: %+v", history)
	}
	if got := history[1].StringContent(); len(got) >= len(long) {
		t.Fatalf("legacy mode did not compact stored history: got len=%d want<%d", len(got), len(long))
	}
}

func TestNewClearsDCPState(t *testing.T) {
	fakeProvider := &fakeStreamProvider{
		streams: []fakeStreamScript{{
			deltas: []*provider.StreamDelta{
				{Content: "done"},
				{FinishReason: stringPtr("stop")},
			},
		}},
	}
	loop, store, _ := newTestAgentLoop(t, fakeProvider)
	defer store.Close()
	loop.config.Agents.Defaults.Compression.Engine = "dcp"
	loop.config.Agents.Defaults.Compression.DCP.Deduplication.Enabled = false
	loop.config.Agents.Defaults.Compression.DCP.PurgeErrors.Enabled = false
	loop.config.Agents.Defaults.Compression.DCP.CompressTool.Enabled = false

	if _, err := loop.ProcessDirect(context.Background(), Request{
		Content:    "hello",
		SessionKey: "s1",
	}, nil); err != nil {
		t.Fatalf("ProcessDirect: %v", err)
	}
	if loop.dcpStateManager == nil {
		t.Fatal("expected dcpStateManager to be initialized")
	}
	if _, err := loop.dcpStateManager.Load("s1"); err != nil {
		t.Fatalf("Load before /new: %v", err)
	}

	if _, err := loop.ProcessDirect(context.Background(), Request{
		Content:    "/new",
		SessionKey: "s1",
	}, nil); err != nil {
		t.Fatalf("/new: %v", err)
	}

	state, err := loop.dcpStateManager.Load("s1")
	if err != nil {
		t.Fatalf("Load after /new: %v", err)
	}
	if state.RequestCount != 0 || len(state.Blocks) != 0 || len(state.MessageIDs.ByRef) != 0 {
		t.Fatalf("state after /new = %+v, want empty", state)
	}
}

func TestCompressToolDoesNotLeakIntoLegacyMode(t *testing.T) {
	fakeProvider := &fakeStreamProvider{
		streams: []fakeStreamScript{
			{deltas: []*provider.StreamDelta{{Content: "dcp"}, {FinishReason: stringPtr("stop")}}},
			{deltas: []*provider.StreamDelta{{Content: "legacy"}, {FinishReason: stringPtr("stop")}}},
		},
	}
	loop, store, _ := newTestAgentLoop(t, fakeProvider)
	defer store.Close()

	loop.config.Agents.Defaults.Compression.Engine = "dcp"
	loop.config.Agents.Defaults.Compression.DCP.Deduplication.Enabled = false
	loop.config.Agents.Defaults.Compression.DCP.PurgeErrors.Enabled = false
	loop.config.Agents.Defaults.Compression.DCP.CompressTool.Enabled = true
	if _, err := loop.ProcessDirect(context.Background(), Request{
		Content:    "hello",
		SessionKey: "s1",
	}, nil); err != nil {
		t.Fatalf("dcp ProcessDirect: %v", err)
	}

	loop.config.Agents.Defaults.Compression.Engine = "legacy"
	if _, err := loop.ProcessDirect(context.Background(), Request{
		Content:    "hello again",
		SessionKey: "s2",
	}, nil); err != nil {
		t.Fatalf("legacy ProcessDirect: %v", err)
	}

	matches := loop.tools.SearchDeferredTools("compress context")
	for _, match := range matches {
		if match.Name() == "compress" {
			t.Fatal("compress tool leaked into legacy mode")
		}
	}
}
