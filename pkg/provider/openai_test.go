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

func TestOpenAIProviderChat(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" {
			t.Fatalf("path = %q, want /v1/chat/completions", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer test-key" {
			t.Fatalf("authorization = %q", r.Header.Get("Authorization"))
		}

		resp := map[string]any{
			"id":    "chatcmpl-123",
			"model": "gpt-4o",
			"choices": []map[string]any{
				{
					"index":         0,
					"finish_reason": "stop",
					"message": map[string]any{
						"role":    "assistant",
						"content": "Hello from mock!",
					},
				},
			},
			"usage": map[string]any{
				"prompt_tokens":     10,
				"completion_tokens": 5,
				"total_tokens":      15,
			},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	p := NewOpenAIProvider("openai", "test-key", server.URL+"/v1", nil)
	p.sleep = func(context.Context, int) error { return nil }

	resp, err := p.Chat(context.Background(), ChatRequest{
		Model:       "gpt-4o",
		Messages:    []Message{{Role: "user", Content: "Hello"}},
		MaxTokens:   100,
		Temperature: 0.7,
	})
	if err != nil {
		t.Fatalf("Chat: %v", err)
	}
	if resp.Content != "Hello from mock!" {
		t.Fatalf("content = %q, want Hello from mock!", resp.Content)
	}
	if resp.FinishReason != "stop" {
		t.Fatalf("finish_reason = %q, want stop", resp.FinishReason)
	}
	if resp.Usage.TotalTokens != 15 {
		t.Fatalf("total_tokens = %d, want 15", resp.Usage.TotalTokens)
	}
}

func TestOpenAIProviderChatWithToolCalls(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]any{
			"id": "chatcmpl-456",
			"choices": []map[string]any{
				{
					"index":         0,
					"finish_reason": "tool_calls",
					"message": map[string]any{
						"role":    "assistant",
						"content": "",
						"tool_calls": []map[string]any{
							{
								"id":   "tc1",
								"type": "function",
								"function": map[string]any{
									"name":      "exec",
									"arguments": `{"command":"ls -la"}`,
								},
							},
						},
					},
				},
			},
			"usage": map[string]any{"prompt_tokens": 10, "completion_tokens": 20, "total_tokens": 30},
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	p := NewOpenAIProvider("openai", "test-key", server.URL+"/v1", nil)
	p.sleep = func(context.Context, int) error { return nil }

	resp, err := p.Chat(context.Background(), ChatRequest{
		Model:    "gpt-4o",
		Messages: []Message{{Role: "user", Content: "list files"}},
		Tools:    []ToolDef{{Name: "exec", Description: "run command", Parameters: map[string]any{"type": "object"}}},
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
}

func TestOpenAIProviderRetriesTransientFailures(t *testing.T) {
	attempts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if attempts < 3 {
			http.Error(w, `{"error":{"message":"rate limit exceeded"}}`, http.StatusTooManyRequests)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{
				{
					"finish_reason": "stop",
					"message":       map[string]any{"role": "assistant", "content": "ok"},
				},
			},
			"usage": map[string]any{"prompt_tokens": 1, "completion_tokens": 1, "total_tokens": 2},
		})
	}))
	defer server.Close()

	p := NewOpenAIProvider("openai", "test-key", server.URL+"/v1", nil)
	p.sleep = func(context.Context, int) error { return nil }

	resp, err := p.Chat(context.Background(), ChatRequest{
		Model:    "gpt-4o",
		Messages: []Message{{Role: "user", Content: "hello"}},
	})
	if err != nil {
		t.Fatalf("Chat: %v", err)
	}
	if attempts != 3 {
		t.Fatalf("attempts = %d, want 3", attempts)
	}
	if resp.Content != "ok" {
		t.Fatalf("content = %q, want ok", resp.Content)
	}
}

func TestOpenAIProviderRetriesWithoutImagesOnImageRejection(t *testing.T) {
	attempts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req map[string]any
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		attempts++
		messagesJSON, _ := json.Marshal(req["messages"])
		if attempts == 1 {
			if !strings.Contains(string(messagesJSON), "image_url") {
				t.Fatalf("first request should contain image block: %s", string(messagesJSON))
			}
			http.Error(w, `{"error":{"message":"image inputs are not supported by this model"}}`, http.StatusBadRequest)
			return
		}
		if !strings.Contains(string(messagesJSON), "[image omitted]") {
			t.Fatalf("second request should replace images with [image omitted]: %s", string(messagesJSON))
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{
				{
					"finish_reason": "stop",
					"message":       map[string]any{"role": "assistant", "content": "retried"},
				},
			},
			"usage": map[string]any{"prompt_tokens": 1, "completion_tokens": 1, "total_tokens": 2},
		})
	}))
	defer server.Close()

	p := NewOpenAIProvider("openai", "test-key", server.URL+"/v1", nil)
	p.sleep = func(context.Context, int) error { return nil }

	resp, err := p.Chat(context.Background(), ChatRequest{
		Model: "gpt-4o",
		Messages: []Message{
			{
				Role: "user",
				Content: []ContentBlock{
					{Type: "text", Text: "Describe this image"},
					{Type: "image_url", ImageURL: &ImageURL{URL: "https://example.com/image.png", Detail: "auto"}},
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
	if resp.Content != "retried" {
		t.Fatalf("content = %q, want retried", resp.Content)
	}
}

func TestOpenAIProviderInjectsPromptCachingHooks(t *testing.T) {
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
			"choices": []map[string]any{
				{
					"finish_reason": "stop",
					"message":       map[string]any{"role": "assistant", "content": "cached"},
				},
			},
			"usage": map[string]any{"prompt_tokens": 1, "completion_tokens": 1, "total_tokens": 2},
		})
	}))
	defer server.Close()

	p := NewOpenAIProvider("openrouter", "test-key", server.URL+"/v1", nil)
	p.sleep = func(context.Context, int) error { return nil }

	_, err := p.Chat(context.Background(), ChatRequest{
		Model: "openrouter/auto",
		Messages: []Message{
			{Role: "system", Content: "System instructions"},
			{Role: "user", Content: "Hello"},
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

func TestOpenAIProviderStreamAccumulatesPartialToolCalls(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req map[string]any
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		streamOptions, ok := req["stream_options"].(map[string]any)
		if !ok || streamOptions["include_usage"] != true {
			t.Fatalf("expected stream_options.include_usage=true, got %#v", req["stream_options"])
		}

		w.Header().Set("Content-Type", "text/event-stream")
		flusher, ok := w.(http.Flusher)
		if !ok {
			t.Fatal("response writer is not a flusher")
		}

		events := []string{
			`data: {"choices":[{"delta":{"content":"Hello "}}]}` + "\n\n",
			`data: {"choices":[{"delta":{"tool_calls":[{"index":0,"id":"call_1","function":{"name":"exec","arguments":"{\"command\":\""}}]}}]}` + "\n\n",
			`data: {"choices":[],"usage":{"prompt_tokens":12,"completion_tokens":7,"total_tokens":19}}` + "\n\n",
			`data: {"choices":[{"delta":{"tool_calls":[{"index":0,"function":{"arguments":"ls\"}"}}]},"finish_reason":"tool_calls"}]}` + "\n\n",
			"data: [DONE]\n\n",
		}
		for _, event := range events {
			fmt.Fprint(w, event)
			flusher.Flush()
		}
	}))
	defer server.Close()

	p := NewOpenAIProvider("openai", "test-key", server.URL+"/v1", nil)
	p.sleep = func(context.Context, int) error { return nil }

	stream, err := p.ChatStream(context.Background(), ChatRequest{
		Model:    "gpt-4o",
		Messages: []Message{{Role: "user", Content: "hello"}},
	})
	if err != nil {
		t.Fatalf("ChatStream: %v", err)
	}

	resp, err := AccumulateStream(stream)
	if err != nil {
		t.Fatalf("AccumulateStream: %v", err)
	}
	if resp.Content != "Hello " {
		t.Fatalf("content = %q, want %q", resp.Content, "Hello ")
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
	if resp.Usage.TotalTokens != 19 {
		t.Fatalf("total_tokens = %d, want 19", resp.Usage.TotalTokens)
	}
}

func TestOpenAIProviderStreamFallsBackWhenUsageOptionsUnsupported(t *testing.T) {
	attempts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		var req map[string]any
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if attempts == 1 {
			streamOptions, ok := req["stream_options"].(map[string]any)
			if !ok || streamOptions["include_usage"] != true {
				t.Fatalf("expected first request to include usage options, got %#v", req["stream_options"])
			}
			http.Error(w, `{"error":{"message":"unknown parameter: stream_options.include_usage"}}`, http.StatusBadRequest)
			return
		}
		if _, ok := req["stream_options"]; ok {
			t.Fatalf("expected fallback request to omit stream_options, got %#v", req["stream_options"])
		}

		w.Header().Set("Content-Type", "text/event-stream")
		flusher, ok := w.(http.Flusher)
		if !ok {
			t.Fatal("response writer is not a flusher")
		}
		events := []string{
			`data: {"choices":[{"delta":{"content":"ok"},"finish_reason":"stop"}]}` + "\n\n",
			"data: [DONE]\n\n",
		}
		for _, event := range events {
			fmt.Fprint(w, event)
			flusher.Flush()
		}
	}))
	defer server.Close()

	p := NewOpenAIProvider("moonshot", "test-key", server.URL+"/v1", nil)
	p.sleep = func(context.Context, int) error { return nil }

	stream, err := p.ChatStream(context.Background(), ChatRequest{
		Model:    "kimi-k2.5:cloud",
		Messages: []Message{{Role: "user", Content: "hello"}},
	})
	if err != nil {
		t.Fatalf("ChatStream: %v", err)
	}

	resp, err := AccumulateStream(stream)
	if err != nil {
		t.Fatalf("AccumulateStream: %v", err)
	}
	if attempts != 2 {
		t.Fatalf("attempts = %d, want 2", attempts)
	}
	if resp.Content != "ok" {
		t.Fatalf("content = %q, want ok", resp.Content)
	}
}

func TestOpenAIProviderName(t *testing.T) {
	p := NewOpenAIProvider("ollama", "", "http://localhost:11434/v1", nil)
	if p.Name() != "ollama" {
		t.Fatalf("Name = %q, want ollama", p.Name())
	}
}
