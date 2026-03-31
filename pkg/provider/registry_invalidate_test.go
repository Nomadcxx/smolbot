package provider_test

import (
	"testing"

	"github.com/Nomadcxx/smolbot/pkg/config"
	"github.com/Nomadcxx/smolbot/pkg/provider"
)

func TestRegistry_Invalidate_ClearsCache(t *testing.T) {
	cfg := &config.Config{
		Agents:    config.AgentsConfig{Defaults: config.AgentDefaults{Model: "gpt-4o", Provider: "openai"}},
		Providers: map[string]config.ProviderConfig{"openai": {APIKey: "k1"}},
	}
	r := provider.NewRegistryWithDefaults(cfg)
	p1, err := r.ForModel("gpt-4o")
	if err != nil {
		t.Fatal(err)
	}
	r.Invalidate("openai")
	p2, err := r.ForModel("gpt-4o")
	if err != nil {
		t.Fatal(err)
	}
	if p1 == p2 {
		t.Fatal("expected new provider instance after Invalidate")
	}
}

func TestRegistry_UpdateProviderConfig_ReflectsNewKey(t *testing.T) {
	cfg := &config.Config{
		Agents:    config.AgentsConfig{Defaults: config.AgentDefaults{Model: "gpt-4o", Provider: "openai"}},
		Providers: map[string]config.ProviderConfig{"openai": {APIKey: "k1"}},
	}
	r := provider.NewRegistryWithDefaults(cfg)
	r.UpdateProviderConfig("openai", config.ProviderConfig{APIKey: "k2"})
	if cfg.Providers["openai"].APIKey != "k2" {
		t.Fatalf("expected APIKey=k2, got %q", cfg.Providers["openai"].APIKey)
	}
}

func TestRegistry_RemoveProviderConfig_RemovesEntry(t *testing.T) {
	cfg := &config.Config{
		Providers: map[string]config.ProviderConfig{"openai": {APIKey: "k1"}},
	}
	r := provider.NewRegistryWithDefaults(cfg)
	r.RemoveProviderConfig("openai")
	if _, ok := cfg.Providers["openai"]; ok {
		t.Fatal("expected openai to be removed from config.Providers")
	}
}
