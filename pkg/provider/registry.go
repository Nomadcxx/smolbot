package provider

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"

	"github.com/Nomadcxx/smolbot/pkg/config"
)

type ProviderFactory func(config.ProviderConfig) Provider

type Registry struct {
	cfg           *config.Config
	store         config.OAuthTokenStore
	factories     map[string]ProviderFactory
	cache         map[string]Provider
	oauthFactories map[string]func(cfg OAuthConfig) OAuthProvider
	mu            sync.Mutex
}

func NewRegistry(cfg *config.Config) *Registry {
	return &Registry{
		cfg:           cfg,
		factories:     make(map[string]ProviderFactory),
		cache:         make(map[string]Provider),
		oauthFactories: make(map[string]func(cfg OAuthConfig) OAuthProvider),
	}
}

func NewRegistryWithDefaults(cfg *config.Config) *Registry {
	return NewRegistryWithOAuthStore(cfg, nil)
}

func NewRegistryWithOAuthStore(cfg *config.Config, store config.OAuthTokenStore) *Registry {
	r := NewRegistry(cfg)
	r.store = store

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
		"gemini", "moonshot", "aihubmix", "siliconflow",
		"volcengine", "byteplus", "dashscope", "zhipu",
	}
	for _, name := range openAICompatible {
		providerName := name
		r.RegisterFactory(providerName, func(pc config.ProviderConfig) Provider {
			return NewOpenAIProvider(providerName, pc.APIKey, pc.APIBase, pc.ExtraHeaders)
		})
	}

	r.RegisterFactory("minimax", func(pc config.ProviderConfig) Provider {
		apiBase := pc.APIBase
		if apiBase == "" {
			apiBase = "https://api.minimax.io/v1"
		}
		return NewOpenAIProvider("minimax", pc.APIKey, apiBase, pc.ExtraHeaders)
	})

	r.RegisterOAuthFactory("minimax-portal", func(cfg OAuthConfig) OAuthProvider {
		return NewMiniMaxOAuthProvider("minimax-portal", WithMiniMaxOAuthBaseURL(cfg.BaseURL))
	})

	return r
}

func (r *Registry) RegisterFactory(name string, factory ProviderFactory) {
	r.factories[name] = factory
}

func (r *Registry) RegisterOAuthFactory(name string, factory func(cfg OAuthConfig) OAuthProvider) {
	r.oauthFactories[name] = factory
}

func (r *Registry) ForModel(model string) (Provider, error) {
	return r.ForModelWithCtx(context.Background(), model)
}

func (r *Registry) ForModelWithCtx(ctx context.Context, model string) (Provider, error) {
	resolved := r.resolveProvider(model)

	r.mu.Lock()
	defer r.mu.Unlock()

	cacheKey := resolved.cacheKey
	if resolved.isOAuth {
		cacheKey = fmt.Sprintf("%s:%s", resolved.factoryKey, resolved.profileID)
	}

	if provider, ok := r.cache[cacheKey]; ok {
		if oauth, ok := provider.(OAuthProvider); ok && oauth.AuthType() == AuthTypeOAuth {
			if token := oauth.GetToken(); token != nil && token.IsExpired() {
				if _, err := oauth.RefreshToken(ctx); err == nil {
					if r.store != nil {
						r.store.Save(oauth.Name(), oauth.GetToken().ProfileID, &config.TokenStoreEntry{
							AccessToken:  oauth.GetToken().AccessToken,
							RefreshToken: oauth.GetToken().RefreshToken,
							ExpiresAt:    oauth.GetToken().ExpiresAt,
							TokenType:    oauth.GetToken().TokenType,
							Scope:        oauth.GetToken().Scope,
							ProviderID:   oauth.GetToken().ProviderID,
							ProfileID:   oauth.GetToken().ProfileID,
						})
					}
				}
			}
		}
		return provider, nil
	}

	if resolved.isOAuth {
		if factory, ok := r.oauthFactories[resolved.factoryKey]; ok {
			oauthProvider := factory(resolved.oauthConfig)
			if r.store != nil {
				profileID := resolved.profileID
				if profileID == "" {
					profileID = "default"
				}
				if entry, ok, _ := r.store.Load(oauthProvider.Name(), profileID); ok {
					oauthProvider.SetToken(&TokenInfo{
						AccessToken:  entry.AccessToken,
						RefreshToken: entry.RefreshToken,
						ExpiresAt:    entry.ExpiresAt,
						TokenType:    entry.TokenType,
						Scope:        entry.Scope,
						ProviderID:   entry.ProviderID,
						ProfileID:    entry.ProfileID,
					})
				}
			}
			r.cache[cacheKey] = oauthProvider
			return oauthProvider, nil
		}
		return nil, fmt.Errorf("no OAuth factory for %q", resolved.factoryKey)
	}

	factory, ok := r.factories[resolved.factoryKey]
	if !ok && resolved.factoryKey != "openai" {
		factory, ok = r.factories["openai"]
	}
	if !ok {
		return nil, fmt.Errorf("no provider factory for %q", resolved.factoryKey)
	}

	provider := factory(resolved.providerConfig)
	r.cache[cacheKey] = provider
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
	fallback := ""
	if r.cfg != nil {
		fallback = r.cfg.Agents.Defaults.Provider
	}
	name := detectProviderName(model, fallback, r.cfg.Providers, r.factories, r.oauthFactories)

	providerConfig, hasConfig := r.cfg.Providers[name]
	factoryKey := name
	cacheKey := name

	if hasConfig && providerConfig.AuthType == "oauth" {
		return resolvedProvider{
			cacheKey:    fmt.Sprintf("%s:%s", factoryKey, providerConfig.ProfileID),
			factoryKey:  name,
			isOAuth:     true,
			oauthConfig: OAuthConfig{BaseURL: providerConfig.APIBase},
			profileID:   providerConfig.ProfileID,
		}
	}

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
	isOAuth        bool
	oauthConfig    OAuthConfig
	profileID      string
}

func detectProviderName(model, fallback string, providers map[string]config.ProviderConfig, factories map[string]ProviderFactory, oauthFactories map[string]func(cfg OAuthConfig) OAuthProvider) string {
	lowerModel := strings.ToLower(strings.TrimSpace(model))

	if strings.HasPrefix(lowerModel, "claude-") || strings.HasPrefix(lowerModel, "anthropic/") {
		return "anthropic"
	}
	if strings.HasPrefix(lowerModel, "gpt-") || strings.HasPrefix(lowerModel, "openai/") {
		return "openai"
	}
	if strings.HasPrefix(lowerModel, "azure/") {
		return "azure_openai"
	}

	names := make([]string, 0, len(factories))
	for name := range factories {
		if name == "anthropic" || name == "azure_openai" || name == "openai" {
			continue
		}
		names = append(names, name)
	}
	for name := range providers {
		if name == "anthropic" || name == "azure_openai" || name == "openai" {
			continue
		}
		if _, ok := factories[name]; ok {
			continue
		}
		names = append(names, name)
	}
	for name := range oauthFactories {
		if _, ok := factories[name]; ok {
			continue
		}
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		lowerName := strings.ToLower(name)
		if strings.HasPrefix(lowerModel, lowerName+"/") {
			return name
		}
	}
	for _, name := range names {
		lowerName := strings.ToLower(name)
		if strings.Contains(lowerModel, lowerName) {
			return name
		}
	}

	if strings.TrimSpace(fallback) != "" {
		return fallback
	}

	return "openai"
}
