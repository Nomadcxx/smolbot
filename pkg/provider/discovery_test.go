package provider

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Nomadcxx/smolbot/pkg/config"
)

func TestGetAvailableModelsUsesLiveOllamaDiscovery(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/tags" {
			t.Fatalf("unexpected path %q", r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"models": []map[string]any{
				{
					"name":  "qwen3:8b",
					"model": "qwen3:8b",
					"details": map[string]any{
						"family":             "qwen3",
						"parameter_size":     "8B",
						"quantization_level": "Q4_K_M",
					},
				},
				{
					"name":  "llama3.2",
					"model": "llama3.2",
					"details": map[string]any{
						"family":         "llama",
						"parameter_size": "3B",
					},
				},
			},
		})
	}))
	defer server.Close()

	cfg := &config.Config{}
	cfg.Agents.Defaults.Model = "qwen3:8b"
	cfg.Agents.Defaults.Provider = "ollama"
	cfg.Providers = map[string]config.ProviderConfig{
		"ollama": {APIBase: server.URL},
	}

	models, err := GetAvailableModels(cfg)
	if err != nil {
		t.Fatalf("GetAvailableModels: %v", err)
	}
	if len(models) != 2 {
		t.Fatalf("len(models) = %d, want 2", len(models))
	}
	got, ok := findModel(models, "ollama", "qwen3:8b")
	if !ok {
		t.Fatalf("expected qwen3:8b in ollama catalog, got %#v", models)
	}
	if got.ID != "qwen3:8b" {
		t.Fatalf("ollama id = %q, want qwen3:8b", got.ID)
	}
	if got.Name != "qwen3:8b" {
		t.Fatalf("ollama name = %q, want qwen3:8b", got.Name)
	}
	if got.Source != "ollama.live" {
		t.Fatalf("ollama source = %q, want ollama.live", got.Source)
	}
	if got.Capability != "local" {
		t.Fatalf("ollama capability = %q, want local", got.Capability)
	}
	if got.Description == "" {
		t.Fatal("expected ollama description to be populated from live metadata")
	}
}

func TestGetAvailableModelsIncludesConfiguredCompatibleProviders(t *testing.T) {
	cfg := &config.Config{}
	cfg.Agents.Defaults.Model = "gpt-5"
	cfg.Agents.Defaults.Provider = "openai"
	cfg.Providers = map[string]config.ProviderConfig{
		"openai":     {APIKey: "sk-openai"},
		"openrouter": {APIKey: "sk-openrouter", APIBase: "https://openrouter.ai/api/v1"},
		"kilo":       {APIKey: "sk-kilo", APIBase: "https://kilo.example/v1"},
	}

	models, err := GetAvailableModels(cfg)
	if err != nil {
		t.Fatalf("GetAvailableModels: %v", err)
	}

	byProvider := indexModelsByProvider(t, models)
	if len(byProvider) != 3 {
		t.Fatalf("len(byProvider) = %d, want 3 (%#v)", len(byProvider), models)
	}

	openAI := byProvider["openai"]
	if openAI.ID != "gpt-5" {
		t.Fatalf("openai id = %q, want gpt-5", openAI.ID)
	}
	if openAI.Name != "OpenAI" {
		t.Fatalf("openai name = %q, want OpenAI", openAI.Name)
	}
	if openAI.Source != "config" {
		t.Fatalf("openai source = %q, want config", openAI.Source)
	}
	if openAI.Capability != "openai-compatible" {
		t.Fatalf("openai capability = %q, want openai-compatible", openAI.Capability)
	}
	if openAI.Description == "" {
		t.Fatal("expected openai description to be populated")
	}

	openRouter := byProvider["openrouter"]
	if openRouter.ID != "openrouter/gpt-5" {
		t.Fatalf("openrouter id = %q, want openrouter/gpt-5", openRouter.ID)
	}
	if openRouter.Name != "OpenRouter" {
		t.Fatalf("openrouter name = %q, want OpenRouter", openRouter.Name)
	}

	custom := byProvider["kilo"]
	if custom.ID != "kilo/gpt-5" {
		t.Fatalf("kilo id = %q, want kilo/gpt-5", custom.ID)
	}
	if custom.Name != "Kilo" {
		t.Fatalf("kilo name = %q, want Kilo", custom.Name)
	}
}

func TestGetAvailableModelsFallsBackWhenOllamaDiscoveryUnavailable(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "unavailable", http.StatusBadGateway)
	}))
	defer server.Close()

	cfg := &config.Config{}
	cfg.Agents.Defaults.Model = "qwen3:8b"
	cfg.Agents.Defaults.Provider = "ollama"
	cfg.Providers = map[string]config.ProviderConfig{
		"ollama":     {APIBase: server.URL},
		"openrouter": {APIKey: "sk-openrouter", APIBase: "https://openrouter.ai/api/v1"},
	}

	models, err := GetAvailableModels(cfg)
	if err != nil {
		t.Fatalf("GetAvailableModels: %v", err)
	}

	byProvider := indexModelsByProvider(t, models)
	ollama := byProvider["ollama"]
	if ollama.ID != "qwen3:8b" {
		t.Fatalf("ollama id = %q, want qwen3:8b", ollama.ID)
	}
	if ollama.Source != "fallback" {
		t.Fatalf("ollama source = %q, want fallback", ollama.Source)
	}
	if ollama.Description == "" {
		t.Fatal("expected ollama fallback description")
	}

	if _, ok := byProvider["openrouter"]; !ok {
		t.Fatalf("expected configured openrouter entry in fallback catalog, got %#v", models)
	}
}

func indexModelsByProvider(t *testing.T, models []ModelInfo) map[string]ModelInfo {
	t.Helper()

	byProvider := make(map[string]ModelInfo, len(models))
	for _, model := range models {
		if _, exists := byProvider[model.Provider]; exists {
			continue
		}
		byProvider[model.Provider] = model
	}
	return byProvider
}

func findModel(models []ModelInfo, providerID, modelID string) (ModelInfo, bool) {
	for _, model := range models {
		if model.Provider == providerID && model.ID == modelID {
			return model, true
		}
	}
	return ModelInfo{}, false
}
