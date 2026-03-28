package provider

import (
	"context"
	"testing"

	"github.com/Nomadcxx/smolbot/pkg/config"
)

type mockProvider struct {
	name string
}

func (m *mockProvider) Chat(_ context.Context, _ ChatRequest) (*Response, error) {
	return &Response{Content: "mock"}, nil
}

func (m *mockProvider) ChatStream(_ context.Context, _ ChatRequest) (*Stream, error) {
	return nil, nil
}

func (m *mockProvider) Name() string { return m.name }

func TestRegistryExplicitProvider(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Agents.Defaults.Provider = "anthropic"
	cfg.Providers["anthropic"] = config.ProviderConfig{APIKey: "sk-ant-xxx"}

	r := NewRegistry(&cfg)
	r.RegisterFactory("anthropic", func(_ config.ProviderConfig) Provider {
		return &mockProvider{name: "anthropic"}
	})

	p, err := r.ForModel("any-model-name")
	if err != nil {
		t.Fatalf("ForModel: %v", err)
	}
	if p.Name() != "anthropic" {
		t.Fatalf("provider = %q, want anthropic", p.Name())
	}
}

func TestRegistryPrefixMatching(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Providers["anthropic"] = config.ProviderConfig{APIKey: "sk-ant-xxx"}
	cfg.Providers["openai"] = config.ProviderConfig{APIKey: "sk-xxx"}

	r := NewRegistry(&cfg)
	r.RegisterFactory("anthropic", func(_ config.ProviderConfig) Provider {
		return &mockProvider{name: "anthropic"}
	})
	r.RegisterFactory("openai", func(_ config.ProviderConfig) Provider {
		return &mockProvider{name: "openai"}
	})

	tests := []struct {
		model    string
		wantName string
	}{
		{model: "claude-sonnet-4-20250514", wantName: "anthropic"},
		{model: "anthropic/claude-3", wantName: "anthropic"},
		{model: "gpt-4o", wantName: "openai"},
		{model: "llama-3.1-70b", wantName: "openai"},
	}

	for _, tt := range tests {
		p, err := r.ForModel(tt.model)
		if err != nil {
			t.Fatalf("ForModel(%q): %v", tt.model, err)
		}
		if p.Name() != tt.wantName {
			t.Fatalf("ForModel(%q) = %q, want %q", tt.model, p.Name(), tt.wantName)
		}
	}
}

func TestRegistryExplicitModelOverridesFallbackProvider(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Agents.Defaults.Provider = "openai"

	r := NewRegistry(&cfg)
	r.RegisterFactory("anthropic", func(_ config.ProviderConfig) Provider {
		return &mockProvider{name: "anthropic"}
	})
	r.RegisterFactory("openai", func(_ config.ProviderConfig) Provider {
		return &mockProvider{name: "openai"}
	})

	p, err := r.ForModel("claude-sonnet-4-20250514")
	if err != nil {
		t.Fatalf("ForModel: %v", err)
	}
	if p.Name() != "anthropic" {
		t.Fatalf("provider = %q, want anthropic override for explicit claude model", p.Name())
	}
}

func TestRegistryCaching(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Providers["openai"] = config.ProviderConfig{APIKey: "sk-xxx"}

	callCount := 0
	r := NewRegistry(&cfg)
	r.RegisterFactory("openai", func(_ config.ProviderConfig) Provider {
		callCount++
		return &mockProvider{name: "openai"}
	})

	_, _ = r.ForModel("gpt-4o")
	_, _ = r.ForModel("gpt-4o")
	_, _ = r.ForModel("gpt-3.5-turbo")

	if callCount != 1 {
		t.Fatalf("factory called %d times, want 1", callCount)
	}
}

func TestRegistryCustomOpenAICompatibleProvider(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Providers["kilo"] = config.ProviderConfig{APIKey: "sk-kilo", APIBase: "https://kilo.example/v1"}

	r := NewRegistry(&cfg)
	r.RegisterFactory("openai", func(pc config.ProviderConfig) Provider {
		return &mockProvider{name: pc.APIBase}
	})

	p, err := r.ForModel("kilo/my-model")
	if err != nil {
		t.Fatalf("ForModel: %v", err)
	}
	if p.Name() != "https://kilo.example/v1" {
		t.Fatalf("provider = %q, want apiBase-backed custom provider", p.Name())
	}
}

func TestRegistryCustomProvidersHaveDistinctCacheKeys(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Providers["kilo"] = config.ProviderConfig{APIKey: "a", APIBase: "https://kilo.example/v1"}
	cfg.Providers["nova"] = config.ProviderConfig{APIKey: "b", APIBase: "https://nova.example/v1"}

	callCount := 0
	r := NewRegistry(&cfg)
	r.RegisterFactory("openai", func(pc config.ProviderConfig) Provider {
		callCount++
		return &mockProvider{name: pc.APIBase}
	})

	p1, err := r.ForModel("kilo/model-a")
	if err != nil {
		t.Fatalf("kilo: %v", err)
	}
	p2, err := r.ForModel("nova/model-b")
	if err != nil {
		t.Fatalf("nova: %v", err)
	}
	if p1.Name() == p2.Name() {
		t.Fatalf("custom providers collapsed to same cache entry: %q", p1.Name())
	}
	if callCount != 2 {
		t.Fatalf("factory called %d times, want 2", callCount)
	}
}

func TestRegistryDefaultFactories(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Providers["anthropic"] = config.ProviderConfig{APIKey: "sk-ant-test"}
	cfg.Providers["openai"] = config.ProviderConfig{APIKey: "sk-openai-test"}
	cfg.Providers["azure_openai"] = config.ProviderConfig{APIKey: "azure-test", APIBase: "https://test.openai.azure.com"}
	cfg.Providers["openrouter"] = config.ProviderConfig{APIKey: "or-test", APIBase: "https://openrouter.ai/api/v1"}

	r := NewRegistryWithDefaults(&cfg)

	p1, err := r.ForModel("claude-sonnet-4-20250514")
	if err != nil {
		t.Fatalf("anthropic: %v", err)
	}
	if p1.Name() != "anthropic" {
		t.Fatalf("claude -> %q, want anthropic", p1.Name())
	}

	p2, err := r.ForModel("gpt-4o")
	if err != nil {
		t.Fatalf("openai: %v", err)
	}
	if p2.Name() != "openai" {
		t.Fatalf("gpt -> %q, want openai", p2.Name())
	}

	p3, err := r.ForModel("azure/gpt-4o")
	if err != nil {
		t.Fatalf("azure: %v", err)
	}
	if p3.Name() != "azure_openai" {
		t.Fatalf("azure -> %q, want azure_openai", p3.Name())
	}

	p4, err := r.ForModel("openrouter/auto")
	if err != nil {
		t.Fatalf("openrouter: %v", err)
	}
	if p4.Name() != "openrouter" {
		t.Fatalf("openrouter -> %q, want openrouter", p4.Name())
	}
}
