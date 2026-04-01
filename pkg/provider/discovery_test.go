package provider

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
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
	if !got.Selectable {
		t.Fatal("expected live ollama row to be selectable")
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

	// openai has a catalogue: expect selectable catalogue entries (not a single stub)
	gpt4o, ok := findModel(models, "openai", "gpt-4o")
	if !ok {
		t.Fatalf("expected openai catalogue model gpt-4o, got %#v", models)
	}
	if gpt4o.Source != "catalogue" {
		t.Fatalf("gpt-4o source = %q, want catalogue", gpt4o.Source)
	}
	if !gpt4o.Selectable {
		t.Fatal("expected catalogue model to be selectable")
	}
	if gpt4o.Capability != "chat" {
		t.Fatalf("gpt-4o capability = %q, want chat", gpt4o.Capability)
	}

	// openrouter has no catalogue: expect a single config stub (non-selectable)
	openRouter, ok := findModel(models, "openrouter", "openrouter")
	if !ok {
		t.Fatalf("expected provider-backed openrouter row, got %#v", models)
	}
	if openRouter.ID != "openrouter" {
		t.Fatalf("openrouter id = %q, want openrouter", openRouter.ID)
	}
	if openRouter.Name != "OpenRouter" {
		t.Fatalf("openrouter name = %q, want OpenRouter", openRouter.Name)
	}

	// kilo is a custom endpoint with no catalogue: expect a single config stub
	custom, ok := findModel(models, "kilo", "kilo")
	if !ok {
		t.Fatalf("expected provider-backed kilo row, got %#v", models)
	}
	if custom.ID != "kilo" {
		t.Fatalf("kilo id = %q, want kilo", custom.ID)
	}
	if custom.Name != "Kilo" {
		t.Fatalf("kilo name = %q, want Kilo", custom.Name)
	}

	// current model (gpt-5) is not in catalogue but is the active model: expect a fallback row
	current, ok := findModel(models, "openai", "gpt-5")
	if !ok {
		t.Fatalf("expected current openai model row, got %#v", models)
	}
	if current.Source != "fallback" {
		t.Fatalf("current source = %q, want fallback", current.Source)
	}
	if !current.Selectable {
		t.Fatal("expected current fallback row to stay selectable")
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
	if !ollama.Selectable {
		t.Fatal("expected ollama fallback row to be selectable")
	}
	if ollama.Description == "" {
		t.Fatal("expected ollama fallback description")
	}

	if _, ok := byProvider["openrouter"]; !ok {
		t.Fatalf("expected configured openrouter entry in fallback catalog, got %#v", models)
	}
}

func TestGetAvailableModelsDoesNotFabricateOllamaIDFromUnrelatedCurrentModel(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "unavailable", http.StatusBadGateway)
	}))
	defer server.Close()

	cfg := &config.Config{}
	cfg.Agents.Defaults.Model = "gpt-5"
	cfg.Agents.Defaults.Provider = "openai"
	cfg.Providers = map[string]config.ProviderConfig{
		"openai": {APIKey: "sk-openai"},
		"ollama": {APIBase: server.URL},
	}

	models, err := GetAvailableModels(cfg)
	if err != nil {
		t.Fatalf("GetAvailableModels: %v", err)
	}

	ollama, ok := findModel(models, "ollama", "ollama")
	if !ok {
		t.Fatalf("expected provider-backed ollama fallback row, got %#v", models)
	}
	if strings.Contains(ollama.ID, "gpt-5") {
		t.Fatalf("ollama id = %q, should not include unrelated current model", ollama.ID)
	}
	if ollama.Source != "fallback" {
		t.Fatalf("ollama source = %q, want fallback", ollama.Source)
	}
}

func TestGetAvailableModelsLegacyAzureConfigUsesConsistentDiscoveryID(t *testing.T) {
	cfg := &config.Config{}
	cfg.Agents.Defaults.Model = "gpt-5"
	cfg.Agents.Defaults.Provider = "openai"
	cfg.Providers = map[string]config.ProviderConfig{
		"openai": {APIKey: "sk-openai"},
		"azure":  {APIKey: "azure-key", APIBase: "https://legacy-azure.example/v1"},
	}

	models, err := GetAvailableModels(cfg)
	if err != nil {
		t.Fatalf("GetAvailableModels: %v", err)
	}

	azureRow, ok := findModel(models, "azure", "azure")
	if !ok {
		t.Fatalf("expected legacy azure discovery row, got %#v", models)
	}
	if azureRow.Source != "config" {
		t.Fatalf("azure source = %q, want config", azureRow.Source)
	}

	registry := NewRegistryWithDefaults(cfg)
	// Register "azure" as an alias for "azure_openai" since GetAvailableModels
	// returns "azure" as the model/provider ID for legacy azure configs.
	registry.RegisterFactory("azure", func(pc config.ProviderConfig) Provider {
		return NewOpenAIProvider("azure", pc.APIKey, pc.APIBase, pc.ExtraHeaders)
	})
	resolved, err := registry.ForModel(azureRow.ID)
	if err != nil {
		t.Fatalf("ForModel(%q): %v", azureRow.ID, err)
	}
	openAIProvider, ok := resolved.(*OpenAIProvider)
	if !ok {
		t.Fatalf("provider type = %T, want *OpenAIProvider", resolved)
	}
	if openAIProvider.baseURL != "https://legacy-azure.example/v1" {
		t.Fatalf("provider baseURL = %q, want legacy azure config base", openAIProvider.baseURL)
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

func TestCatalogueMetadataFlowsToModelInfo(t *testing.T) {
cfg := &config.Config{}
cfg.Providers = map[string]config.ProviderConfig{
"anthropic": {APIKey: "test-key"},
}

models, err := GetAvailableModels(cfg)
if err != nil {
t.Fatalf("GetAvailableModels: %v", err)
}

for _, m := range models {
if m.Provider != "anthropic" || m.Source != "catalogue" {
continue
}
if m.ReleaseDate == "" {
t.Errorf("model %q has empty ReleaseDate", m.ID)
}
if m.ContextWindow == 0 {
t.Errorf("model %q has zero ContextWindow", m.ID)
}
return
}
t.Fatal("no anthropic catalogue models found")
}
