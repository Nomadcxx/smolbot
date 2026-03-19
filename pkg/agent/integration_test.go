package agent

import (
	"context"
	"testing"

	"github.com/Nomadcxx/nanobot-go/pkg/provider"
	"github.com/Nomadcxx/nanobot-go/pkg/tool"
)

func TestAgentLoopIntegration(t *testing.T) {
	fakeProvider := &fakeStreamProvider{
		streams: []fakeStreamScript{
			{
				deltas: []*provider.StreamDelta{
					{
						ToolCalls: []provider.ToolCall{{
							ID:    "call1",
							Index: 0,
							Function: provider.FunctionCall{
								Name:      "echo",
								Arguments: `{"text":"hello"}`,
							},
						}},
						FinishReason: stringPtr("tool_calls"),
					},
				},
			},
			{
				deltas: []*provider.StreamDelta{
					{Content: "all done"},
					{FinishReason: stringPtr("stop")},
				},
			},
		},
	}

	echoTool := &fakeTool{
		name: "echo",
		result: &tool.Result{
			Content: "tool output",
		},
	}

	loop, store, _ := newTestAgentLoop(t, fakeProvider, echoTool)
	defer store.Close()

	var eventTypes []EventType
	resp, err := loop.ProcessDirect(context.Background(), Request{
		Content:    "run tool",
		SessionKey: "integration",
		Channel:    "gateway",
		ChatID:     "ws-client-1",
	}, func(ev Event) {
		eventTypes = append(eventTypes, ev.Type)
	})
	if err != nil {
		t.Fatalf("ProcessDirect: %v", err)
	}
	if resp != "all done" {
		t.Fatalf("response = %q, want all done", resp)
	}

	wantOrder := []EventType{EventToolHint, EventToolStart, EventToolDone, EventProgress, EventDone}
	if len(eventTypes) != len(wantOrder) {
		t.Fatalf("event count = %d, want %d (%v)", len(eventTypes), len(wantOrder), eventTypes)
	}
	for i, want := range wantOrder {
		if eventTypes[i] != want {
			t.Fatalf("event[%d] = %q, want %q (%v)", i, eventTypes[i], want, eventTypes)
		}
	}

	history, err := store.GetHistory("integration", 500)
	if err != nil {
		t.Fatalf("GetHistory: %v", err)
	}
	if len(history) != 4 {
		t.Fatalf("history len = %d, want 4", len(history))
	}
	if history[1].Role != "assistant" || len(history[1].ToolCalls) != 1 {
		t.Fatalf("assistant tool call not preserved: %+v", history[1])
	}
	if history[2].Role != "tool" || history[2].ToolCallID != history[1].ToolCalls[0].ID {
		t.Fatalf("tool boundary invalid: assistant=%+v tool=%+v", history[1], history[2])
	}
	if history[3].Role != "assistant" || history[3].StringContent() != "all done" {
		t.Fatalf("final assistant message invalid: %+v", history[3])
	}
}
