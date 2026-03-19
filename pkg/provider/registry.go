package provider

import (
	"fmt"
	"strings"
	"sync"

	"github.com/Nomadcxx/nanobot-go/pkg/config"
)

type ProviderFactory func(config.ProviderConfig) Provider

type Registry struct {
	cfg       *config.Config
	factories map[string]ProviderFactory
	cache     map[string]Provider
	mu        sync.Mutex
}

func NewRegistry(cfg *config.Config) *Registry {
	return &Registry{
		cfg:       cfg,
		factories: make(map[string]ProviderFactory),
		cache:     make(map[string]Provider),
	}
}

func NewRegistryWithDefaults(cfg *config.Config) *Registry {
	r := NewRegistry(cfg)

	r.RegisterFactory("openai", func(pc config.ProviderConfig) Provider {
		return NewOpenAIProvider("openai", pc.APIKey, pc.APIBase, pc.ExtraHeaders)
	})
	r.RegisterFactory("anthropic", func(pc config.ProviderConfig) Provider {
		return NewAnthropicProvider(pc.APIKey, pc.APIBase)
	})
	r.RegisterFactory("azure_openai", func(pc config.ProviderConfig) Provider {
		return NewAzureProvider(pc.APIKey, pc.APIBase)
	})

	openAICompatible := []string{
		"deepseek", "groq", "ollama", "openrouter", "vllm",
		"minimax", "gemini", "moonshot", "aihubmix", "siliconflow",
		"volcengine", "byteplus", "dashscope", "zhipu",
	}
	for _, name := range openAICompatible {
		providerName := name
		r.RegisterFactory(providerName, func(pc config.ProviderConfig) Provider {
			return NewOpenAIProvider(providerName, pc.APIKey, pc.APIBase, pc.ExtraHeaders)
		})
	}

	return r
}

func (r *Registry) RegisterFactory(name string, factory ProviderFactory) {
	r.factories[name] = factory
}

func (r *Registry) ForModel(model string) (Provider, error) {
	resolved := r.resolveProvider(model)

	r.mu.Lock()
	defer r.mu.Unlock()

	if provider, ok := r.cache[resolved.cacheKey]; ok {
		return provider, nil
	}

	factory, ok := r.factories[resolved.factoryKey]
	if !ok && resolved.factoryKey != "openai" {
		factory, ok = r.factories["openai"]
	}
	if !ok {
		return nil, fmt.Errorf("no provider factory for %q", resolved.factoryKey)
	}

	provider := factory(resolved.providerConfig)
	r.cache[resolved.cacheKey] = provider
	return provider, nil
}

func (r *Registry) SetModel(model string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.cfg.Agents.Defaults.Model = model
}

func (r *Registry) CurrentModel() string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.cfg.Agents.Defaults.Model
}

func (r *Registry) resolveProvider(model string) resolvedProvider {
	name := r.cfg.Agents.Defaults.Provider
	if name == "" {
		name = detectProviderName(model, r.cfg.Providers)
	}

	providerConfig, hasConfig := r.cfg.Providers[name]
	factoryKey := name
	cacheKey := name

	switch name {
	case "anthropic", "azure_openai":
	case "openai":
	default:
		if !hasConfig {
			providerConfig = config.ProviderConfig{}
		}
		if _, ok := r.factories[name]; !ok {
			factoryKey = "openai"
		}
		if cacheKey == "" {
			cacheKey = factoryKey
		}
		return resolvedProvider{
			cacheKey:       cacheKey,
			factoryKey:     factoryKey,
			providerConfig: providerConfig,
		}
	}

	if !hasConfig {
		providerConfig = r.cfg.Providers[factoryKey]
	}

	return resolvedProvider{
		cacheKey:       cacheKey,
		factoryKey:     factoryKey,
		providerConfig: providerConfig,
	}
}

type resolvedProvider struct {
	cacheKey       string
	factoryKey     string
	providerConfig config.ProviderConfig
}

func detectProviderName(model string, providers map[string]config.ProviderConfig) string {
	lowerModel := strings.ToLower(model)

	if strings.HasPrefix(lowerModel, "claude-") || strings.HasPrefix(lowerModel, "anthropic/") {
		return "anthropic"
	}
	if strings.HasPrefix(lowerModel, "azure/") {
		return "azure_openai"
	}

	for name := range providers {
		lowerName := strings.ToLower(name)
		if name == "anthropic" || name == "azure_openai" || name == "openai" {
			continue
		}
		if strings.HasPrefix(lowerModel, lowerName+"/") || strings.Contains(lowerModel, lowerName) {
			return name
		}
	}

	return "openai"
}
