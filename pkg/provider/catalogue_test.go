package provider_test

import (
	"testing"

	"github.com/Nomadcxx/smolbot/pkg/config"
	"github.com/Nomadcxx/smolbot/pkg/provider"
)

func TestCatalogueModels_AnthropicReturnsEntries(t *testing.T) {
	entries := provider.CatalogueModels("anthropic")
	if len(entries) == 0 {
		t.Fatal("expected anthropic catalogue entries, got none")
	}
	for _, e := range entries {
		if e.ID == "" || e.Name == "" {
			t.Errorf("entry has empty ID or Name: %+v", e)
		}
	}
}

func TestCatalogueModels_UnknownProviderReturnsNil(t *testing.T) {
	if provider.CatalogueModels("nonexistent") != nil {
		t.Fatal("expected nil for unknown provider")
	}
}

func TestGetAvailableModels_AnthropicCatalogueWhenConfigured(t *testing.T) {
	cfg := &config.Config{
		Providers: map[string]config.ProviderConfig{
			"anthropic": {APIKey: "sk-ant-test"},
		},
	}
	models, err := provider.GetAvailableModels(cfg)
	if err != nil {
		t.Fatal(err)
	}
	var found bool
	for _, m := range models {
		if m.Provider == "anthropic" && m.Source == "catalogue" && m.Selectable {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected selectable anthropic catalogue model, got: %+v", models)
	}
}

func TestGetAvailableModels_AnthropicCatalogueAbsentWhenNotConfigured(t *testing.T) {
	cfg := &config.Config{
		Providers: map[string]config.ProviderConfig{},
	}
	models, err := provider.GetAvailableModels(cfg)
	if err != nil {
		t.Fatal(err)
	}
	for _, m := range models {
		if m.Provider == "anthropic" && m.Source == "catalogue" {
			t.Fatalf("unexpected anthropic catalogue model when not configured: %+v", m)
		}
	}
}

func TestGetAvailableModels_GroqCatalogueWhenConfigured(t *testing.T) {
	cfg := &config.Config{
		Providers: map[string]config.ProviderConfig{
			"groq": {APIKey: "gsk_test"},
		},
	}
	models, _ := provider.GetAvailableModels(cfg)
	var count int
	for _, m := range models {
		if m.Provider == "groq" && m.Source == "catalogue" {
			count++
		}
	}
	if count == 0 {
		t.Fatal("expected groq catalogue models")
	}
}

func TestGetAvailableModels_DeepSeekCatalogueWhenConfigured(t *testing.T) {
	cfg := &config.Config{
		Providers: map[string]config.ProviderConfig{
			"deepseek": {APIKey: "ds_test"},
		},
	}
	models, _ := provider.GetAvailableModels(cfg)
	var count int
	for _, m := range models {
		if m.Provider == "deepseek" && m.Source == "catalogue" {
			count++
		}
	}
	if count == 0 {
		t.Fatal("expected deepseek catalogue models")
	}
}

func TestKnownProviderIDs_IsSorted(t *testing.T) {
	ids := provider.KnownProviderIDs()
	for i := 1; i < len(ids); i++ {
		if ids[i] < ids[i-1] {
			t.Fatalf("KnownProviderIDs not sorted: %v", ids)
		}
	}
}
