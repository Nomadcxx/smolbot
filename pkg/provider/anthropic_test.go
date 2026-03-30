package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestAnthropicProviderChat(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/messages" {
			t.Fatalf("path = %q, want /v1/messages", r.URL.Path)
		}
		if r.Header.Get("x-api-key") != "test-key" {
			t.Fatalf("x-api-key missing")
		}
		if r.Header.Get("anthropic-version") == "" {
			t.Fatalf("anthropic-version missing")
		}

		resp := map[string]any{
			"id":   "msg_123",
			"type": "message",
			"role": "assistant",
			"content": []map[string]any{
				{"type": "text", "text": "Hello from Anthropic!"},
			},
			"stop_reason": "end_turn",
			"usage": map[string]any{
				"input_tokens":  10,
				"output_tokens": 5,
			},
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	p := NewAnthropicProvider("test-key", server.URL)
	p.sleep = func(context.Context, int) error { return nil }

	resp, err := p.Chat(context.Background(), ChatRequest{
		Model:    "claude-sonnet-4-20250514",
		Messages: []Message{{Role: "user", Content: "Hello"}},
	})
	if err != nil {
		t.Fatalf("Chat: %v", err)
	}
	if resp.Content != "Hello from Anthropic!" {
		t.Fatalf("content = %q, want Hello from Anthropic!", resp.Content)
	}
	if resp.FinishReason != "stop" {
		t.Fatalf("finish_reason = %q, want stop", resp.FinishReason)
	}
}

func TestAnthropicProviderChatWithToolUseAndThinking(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]any{
			"id":   "msg_456",
			"type": "message",
			"role": "assistant",
			"content": []map[string]any{
				{"type": "thinking", "thinking": "Need to inspect files"},
				{"type": "tool_use", "id": "toolu_1", "name": "exec", "input": map[string]any{"command": "ls"}},
			},
			"stop_reason": "tool_use",
			"usage":       map[string]any{"input_tokens": 20, "output_tokens": 30},
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	p := NewAnthropicProvider("test-key", server.URL)
	p.sleep = func(context.Context, int) error { return nil }

	resp, err := p.Chat(context.Background(), ChatRequest{
		Model:    "claude-sonnet-4-20250514",
		Messages: []Message{{Role: "user", Content: "list files"}},
		Tools:    []ToolDef{{Name: "exec", Description: "run"}},
	})
	if err != nil {
		t.Fatalf("Chat: %v", err)
	}
	if len(resp.ToolCalls) != 1 {
		t.Fatalf("tool_calls len = %d, want 1", len(resp.ToolCalls))
	}
	if resp.ToolCalls[0].Function.Name != "exec" {
		t.Fatalf("tool name = %q, want exec", resp.ToolCalls[0].Function.Name)
	}
	if resp.FinishReason != "tool_calls" {
		t.Fatalf("finish_reason = %q, want tool_calls", resp.FinishReason)
	}
	if resp.ReasoningContent != "Need to inspect files" {
		t.Fatalf("reasoning = %q, want preserved thinking", resp.ReasoningContent)
	}
	if len(resp.ThinkingBlocks) != 1 {
		t.Fatalf("thinking_blocks len = %d, want 1", len(resp.ThinkingBlocks))
	}
}

func TestAnthropicProviderRetriesAndImageFallback(t *testing.T) {
	attempts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req map[string]any
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		attempts++
		body, _ := json.Marshal(req)
		if attempts == 1 {
			http.Error(w, `{"error":{"message":"images are not supported"}}`, http.StatusBadRequest)
			return
		}
		if !strings.Contains(string(body), "[image omitted]") {
			t.Fatalf("second request should omit image blocks: %s", string(body))
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"type":        "message",
			"role":        "assistant",
			"content":     []map[string]any{{"type": "text", "text": "ok"}},
			"stop_reason": "end_turn",
			"usage":       map[string]any{"input_tokens": 1, "output_tokens": 1},
		})
	}))
	defer server.Close()

	p := NewAnthropicProvider("test-key", server.URL)
	p.sleep = func(context.Context, int) error { return nil }

	resp, err := p.Chat(context.Background(), ChatRequest{
		Model: "claude-sonnet-4-20250514",
		Messages: []Message{
			{
				Role: "user",
				Content: []ContentBlock{
					{Type: "text", Text: "describe"},
					{Type: "image_url", ImageURL: &ImageURL{URL: "https://example.com/image.png"}},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("Chat: %v", err)
	}
	if attempts != 2 {
		t.Fatalf("attempts = %d, want 2", attempts)
	}
	if resp.Content != "ok" {
		t.Fatalf("content = %q, want ok", resp.Content)
	}
}

func TestAnthropicProviderInjectsPromptCachingHooks(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req map[string]any
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		body, _ := json.Marshal(req)
		payload := string(body)
		if !strings.Contains(payload, `"cache_control":{"type":"ephemeral"}`) {
			t.Fatalf("request missing cache_control hook: %s", payload)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"type":        "message",
			"role":        "assistant",
			"content":     []map[string]any{{"type": "text", "text": "cached"}},
			"stop_reason": "end_turn",
			"usage":       map[string]any{"input_tokens": 1, "output_tokens": 1},
		})
	}))
	defer server.Close()

	p := NewAnthropicProvider("test-key", server.URL)
	p.sleep = func(context.Context, int) error { return nil }

	_, err := p.Chat(context.Background(), ChatRequest{
		Model: "claude-sonnet-4-20250514",
		Messages: []Message{
			{Role: "system", Content: "system instructions"},
			{Role: "user", Content: "hello"},
		},
		Tools: []ToolDef{
			{Name: "tool_one", Description: "first"},
			{Name: "tool_two", Description: "second"},
		},
	})
	if err != nil {
		t.Fatalf("Chat: %v", err)
	}
}

func TestAnthropicProviderStreamIncludesThinkingAndToolUse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		flusher, ok := w.(http.Flusher)
		if !ok {
			t.Fatal("response writer is not a flusher")
		}

		events := []string{
			`data: {"type":"content_block_start","index":0,"content_block":{"type":"thinking"}}` + "\n\n",
			`data: {"type":"content_block_delta","index":0,"delta":{"type":"thinking_delta","thinking":"Need "}}` + "\n\n",
			`data: {"type":"content_block_start","index":1,"content_block":{"type":"tool_use","id":"toolu_1","name":"exec"}}` + "\n\n",
			`data: {"type":"content_block_delta","index":1,"delta":{"type":"input_json_delta","partial_json":"{\"command\":\""}}` + "\n\n",
			`data: {"type":"content_block_delta","index":1,"delta":{"type":"input_json_delta","partial_json":"ls\"}"}}` + "\n\n",
			`data: {"type":"content_block_delta","index":2,"delta":{"type":"text_delta","text":"done"}}` + "\n\n",
			`data: {"type":"message_delta","delta":{"stop_reason":"tool_use"},"usage":{"input_tokens":12,"output_tokens":8}}` + "\n\n",
			`data: {"type":"message_stop"}` + "\n\n",
		}
		for _, event := range events {
			fmt.Fprint(w, event)
			flusher.Flush()
		}
	}))
	defer server.Close()

	p := NewAnthropicProvider("test-key", server.URL)
	p.sleep = func(context.Context, int) error { return nil }

	stream, err := p.ChatStream(context.Background(), ChatRequest{
		Model:    "claude-sonnet-4-20250514",
		Messages: []Message{{Role: "user", Content: "hello"}},
	})
	if err != nil {
		t.Fatalf("ChatStream: %v", err)
	}

	resp, err := AccumulateStream(stream)
	if err != nil {
		t.Fatalf("AccumulateStream: %v", err)
	}
	if resp.ReasoningContent != "Need " {
		t.Fatalf("reasoning = %q, want %q", resp.ReasoningContent, "Need ")
	}
	if resp.Content != "done" {
		t.Fatalf("content = %q, want done", resp.Content)
	}
	if len(resp.ToolCalls) != 1 {
		t.Fatalf("tool_calls len = %d, want 1", len(resp.ToolCalls))
	}
	if resp.ToolCalls[0].Function.Arguments != `{"command":"ls"}` {
		t.Fatalf("arguments = %q, want %q", resp.ToolCalls[0].Function.Arguments, `{"command":"ls"}`)
	}
	if resp.FinishReason != "tool_calls" {
		t.Fatalf("finish_reason = %q, want tool_calls", resp.FinishReason)
	}
	if resp.Usage.TotalTokens != 20 {
		t.Fatalf("total_tokens = %d, want 20", resp.Usage.TotalTokens)
	}
}

func TestAnthropicProviderName(t *testing.T) {
	p := NewAnthropicProvider("key", "http://api.anthropic.com")
	if p.Name() != "anthropic" {
		t.Fatalf("Name = %q, want anthropic", p.Name())
	}
}

func TestAnthropicProviderStripsPrefixFromModel(t *testing.T) {
	var gotModel string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Model string `json:"model"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err == nil {
			gotModel = body.Model
		}
		resp := map[string]any{
			"id":          "msg-1",
			"type":        "message",
			"role":        "assistant",
			"model":       "claude-3-opus-20240229",
			"stop_reason": "end_turn",
			"content":     []map[string]any{{"type": "text", "text": "ok"}},
			"usage":       map[string]any{"input_tokens": 10, "output_tokens": 5},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	p := NewAnthropicProvider("test-key", server.URL)
	_, err := p.Chat(context.Background(), ChatRequest{
		Model:    "anthropic/claude-3-opus-20240229",
		Messages: []Message{{Role: "user", Content: "hi"}},
	})
	if err != nil {
		t.Fatalf("Chat: %v", err)
	}
	if gotModel != "claude-3-opus-20240229" {
		t.Fatalf("model sent to API = %q, want %q", gotModel, "claude-3-opus-20240229")
	}
}
