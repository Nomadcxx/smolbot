package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAzureProviderChat(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("api-key") != "azure-key" {
			t.Fatalf("api-key = %q, want azure-key", r.Header.Get("api-key"))
		}
		if r.URL.Path != "/openai/deployments/gpt-4o/chat/completions" {
			t.Fatalf("path = %q", r.URL.Path)
		}
		if r.URL.Query().Get("api-version") == "" {
			t.Fatal("missing api-version query parameter")
		}

		resp := map[string]any{
			"choices": []map[string]any{
				{
					"finish_reason": "stop",
					"message":       map[string]any{"role": "assistant", "content": "Hello from Azure!"},
				},
			},
			"usage": map[string]any{"prompt_tokens": 10, "completion_tokens": 5, "total_tokens": 15},
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	p := NewAzureProvider("azure-key", server.URL)
	p.sleep = func(context.Context, int) error { return nil }

	resp, err := p.Chat(context.Background(), ChatRequest{
		Model:    "gpt-4o",
		Messages: []Message{{Role: "user", Content: "Hello"}},
	})
	if err != nil {
		t.Fatalf("Chat: %v", err)
	}
	if resp.Content != "Hello from Azure!" {
		t.Fatalf("content = %q, want Hello from Azure!", resp.Content)
	}
}

func TestAzureProviderUsesMaxCompletionTokensAndSuppressesTemperatureForReasoningModels(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req map[string]any
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if _, ok := req["max_completion_tokens"]; !ok {
			t.Fatalf("request missing max_completion_tokens: %#v", req)
		}
		if _, ok := req["temperature"]; ok {
			t.Fatalf("temperature should be suppressed for reasoning model: %#v", req)
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

	p := NewAzureProvider("azure-key", server.URL)
	p.sleep = func(context.Context, int) error { return nil }

	_, err := p.Chat(context.Background(), ChatRequest{
		Model:       "gpt-5",
		Messages:    []Message{{Role: "user", Content: "Hello"}},
		MaxTokens:   512,
		Temperature: 0.7,
	})
	if err != nil {
		t.Fatalf("Chat: %v", err)
	}
}

func TestAzureProviderRetriesTransientFailures(t *testing.T) {
	attempts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if attempts < 3 {
			http.Error(w, `{"error":{"message":"capacity exceeded"}}`, http.StatusInternalServerError)
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

	p := NewAzureProvider("azure-key", server.URL)
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

func TestAzureProviderName(t *testing.T) {
	p := NewAzureProvider("key", "https://mydeployment.openai.azure.com")
	if p.Name() != "azure_openai" {
		t.Fatalf("Name = %q, want azure_openai", p.Name())
	}
}

func TestAzureStreamingReturnsUsage(t *testing.T) {
	streamChunks := []string{
		`data: {"id":"1","choices":[{"index":0,"delta":{"role":"assistant","content":"hi"},"finish_reason":null}]}`,
		`data: {"id":"1","choices":[{"index":0,"delta":{},"finish_reason":"stop"}]}`,
		`data: {"id":"1","choices":[],"usage":{"prompt_tokens":10,"completion_tokens":3,"total_tokens":13}}`,
		`data: [DONE]`,
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		for _, chunk := range streamChunks {
			fmt.Fprintln(w, chunk)
		}
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}
	}))
	defer server.Close()

	p := NewAzureProvider("key", server.URL)
	p.sleep = func(context.Context, int) error { return nil }

	stream, err := p.ChatStream(context.Background(), ChatRequest{
		Model:    "gpt-4",
		Messages: []Message{{Role: "user", Content: "hi"}},
	})
	if err != nil {
		t.Fatalf("ChatStream: %v", err)
	}
	defer stream.Close()

	var lastUsage *Usage
	for {
		delta, err := stream.Recv()
		if err != nil {
			break
		}
		if delta != nil && delta.Usage != nil {
			lastUsage = delta.Usage
		}
	}
	if lastUsage == nil {
		t.Fatal("expected usage in stream, got none — empty-choices chunk was skipped")
	}
	if lastUsage.PromptTokens != 10 || lastUsage.CompletionTokens != 3 {
		t.Fatalf("usage = %+v, want {10 3 13}", lastUsage)
	}
}

func TestAzureStreamingRequestIncludesStreamOptions(t *testing.T) {
	var gotBody map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &gotBody)
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprintln(w, "data: {\"choices\":[{\"delta\":{\"content\":\"x\"}}]}")
		fmt.Fprintln(w, "data: [DONE]")
	}))
	defer server.Close()

	p := NewAzureProvider("key", server.URL)
	p.sleep = func(context.Context, int) error { return nil }

	stream, _ := p.ChatStream(context.Background(), ChatRequest{
		Model:    "gpt-4",
		Messages: []Message{{Role: "user", Content: "hi"}},
	})
	stream.Close()

	if gotBody == nil {
		t.Fatal("no request body captured")
	}
	so, ok := gotBody["stream_options"].(map[string]any)
	if !ok {
		t.Fatal("stream_options not present in request body")
	}
	if so["include_usage"] != true {
		t.Fatalf("stream_options.include_usage = %v, want true", so["include_usage"])
	}
}
