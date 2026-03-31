package tool

import (
	"context"
	"encoding/json"
	"slices"
	"strings"
	"testing"
)

func TestRegistryDefinitionsSorted(t *testing.T) {
	reg := NewRegistry()
	reg.Register(&stubTool{name: "zeta"})
	reg.Register(&stubTool{name: "alpha"})

	defs := reg.Definitions()
	names := []string{defs[0].Name, defs[1].Name}
	if !slices.Equal(names, []string{"alpha", "zeta"}) {
		t.Fatalf("definitions = %v, want sorted names", names)
	}
}

func TestRegistryExecute(t *testing.T) {
	reg := NewRegistry()
	reg.Register(&stubTool{name: "echo", result: &Result{Output: "ok"}})

	result, err := reg.Execute(context.Background(), "echo", json.RawMessage(`{}`), ToolContext{})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if result.Output != "ok" {
		t.Fatalf("output = %q, want ok", result.Output)
	}
}

func TestRegistryUnknownTool(t *testing.T) {
	reg := NewRegistry()
	_, err := reg.Execute(context.Background(), "missing", json.RawMessage(`{}`), ToolContext{})
	if err == nil {
		t.Fatal("expected unknown tool error")
	}
}

func TestRegistryAppendsRetryHintOnToolError(t *testing.T) {
	reg := NewRegistry()
	reg.Register(&stubTool{
		name:   "failing",
		result: &Result{Error: "boom"},
	})

	result, err := reg.Execute(context.Background(), "failing", json.RawMessage(`{}`), ToolContext{})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !strings.Contains(result.Error, "try a different approach") {
		t.Fatalf("result error missing retry hint: %q", result.Error)
	}
}

func TestWithToolContextRoundTrip(t *testing.T) {
	want := ToolContext{
		SessionKey: "sess-1",
		Channel:    "whatsapp",
		ChatID:     "chat-123",
	}
	ctx := WithToolContext(context.Background(), want)
	got, ok := ContextToolContext(ctx)
	if !ok {
		t.Fatal("ContextToolContext returned ok=false, want true")
	}
	if got != want {
		t.Fatalf("ContextToolContext() = %+v, want %+v", got, want)
	}
}

func TestContextToolContextMissingReturnsZeroValue(t *testing.T) {
	_, ok := ContextToolContext(context.Background())
	if ok {
		t.Fatal("ContextToolContext on empty context returned ok=true, want false")
	}
}

type stubTool struct {
	name   string
	result *Result
}

func (s *stubTool) Name() string        { return s.name }
func (s *stubTool) Description() string { return s.name }
func (s *stubTool) Parameters() map[string]any {
	return map[string]any{"type": "object"}
}
func (s *stubTool) Execute(context.Context, json.RawMessage, ToolContext) (*Result, error) {
	return s.result, nil
}
