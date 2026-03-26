package provider

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestOllamaClientContextWindowFromPs(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/ps":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"models": []map[string]any{
					{
						"name":           "gemma3",
						"model":          "gemma3",
						"context_length": 4096,
					},
				},
			})
		case "/api/show":
			t.Fatalf("did not expect /api/show fallback when /api/ps matched")
		default:
			t.Fatalf("unexpected path %q", r.URL.Path)
		}
	}))
	defer server.Close()

	client := NewOllamaClient(server.URL)
	got, err := client.ContextWindow(context.Background(), "gemma3")
	if err != nil {
		t.Fatalf("ContextWindow: %v", err)
	}
	if !got.Found {
		t.Fatal("ContextWindow returned no value")
	}
	if got.Value != 4096 {
		t.Fatalf("Value = %d, want 4096", got.Value)
	}
	if got.Source != "ps" {
		t.Fatalf("Source = %q, want ps", got.Source)
	}
}

func TestOllamaClientContextWindowFallsBackToShowParameters(t *testing.T) {
	seenShow := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/ps":
			_ = json.NewEncoder(w).Encode(map[string]any{"models": []any{}})
		case "/api/show":
			seenShow = true
			var req map[string]any
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				t.Fatalf("decode show request: %v", err)
			}
			if req["model"] != "gemma3" {
				t.Fatalf("show model = %#v, want gemma3", req["model"])
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"parameters": "temperature 0.7\nnum_ctx 2048\nrepeat_penalty 1.1",
			})
		default:
			t.Fatalf("unexpected path %q", r.URL.Path)
		}
	}))
	defer server.Close()

	client := NewOllamaClient(server.URL)
	got, err := client.ContextWindow(context.Background(), "gemma3")
	if err != nil {
		t.Fatalf("ContextWindow: %v", err)
	}
	if !seenShow {
		t.Fatal("expected /api/show fallback to run")
	}
	if !got.Found {
		t.Fatal("ContextWindow returned no value")
	}
	if got.Value != 2048 {
		t.Fatalf("Value = %d, want 2048", got.Value)
	}
	if got.Source != "show.parameters" {
		t.Fatalf("Source = %q, want show.parameters", got.Source)
	}
}

func TestOllamaClientContextWindowFallsBackToShowModelInfo(t *testing.T) {
	seenShow := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/ps":
			_ = json.NewEncoder(w).Encode(map[string]any{"models": []any{}})
		case "/api/show":
			seenShow = true
			_ = json.NewEncoder(w).Encode(map[string]any{
				"model_info": map[string]any{
					"llama.context_length": 131072,
					"other_field":          "ignore",
				},
			})
		default:
			t.Fatalf("unexpected path %q", r.URL.Path)
		}
	}))
	defer server.Close()

	client := NewOllamaClient(server.URL)
	got, err := client.ContextWindow(context.Background(), "llama3.1")
	if err != nil {
		t.Fatalf("ContextWindow: %v", err)
	}
	if !seenShow {
		t.Fatal("expected /api/show fallback to run")
	}
	if !got.Found {
		t.Fatal("ContextWindow returned no value")
	}
	if got.Value != 131072 {
		t.Fatalf("Value = %d, want 131072", got.Value)
	}
	if got.Source != "show.model_info" {
		t.Fatalf("Source = %q, want show.model_info", got.Source)
	}
}

func TestOllamaClientContextWindowUnavailable(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	}))
	defer server.Close()

	client := NewOllamaClient(server.URL)
	got, err := client.ContextWindow(context.Background(), "missing-model")
	if err != nil {
		t.Fatalf("ContextWindow: %v", err)
	}
	if got.Found {
		t.Fatalf("ContextWindow found value %#v, want none", got)
	}
	if got.Value != 0 {
		t.Fatalf("Value = %d, want 0", got.Value)
	}
}
