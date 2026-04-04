package provider

import (
	"fmt"
	"sort"
	"strings"

	"github.com/Nomadcxx/smolbot/pkg/config"
)

// ModelInfo represents a model row returned to the client.
type ModelInfo struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	Provider      string `json:"provider"`
	Description   string `json:"description,omitempty"`
	Source        string `json:"source,omitempty"`
	Capability    string `json:"capability,omitempty"`
	Selectable    bool   `json:"selectable"`
	ReleaseDate   string `json:"releaseDate,omitempty"`
	IsFree        bool   `json:"isFree,omitempty"`
	ContextWindow int    `json:"contextWindow,omitempty"`
}

// GetAvailableModels returns a provider-aware model catalog for the current configuration.
func GetAvailableModels(cfg *config.Config) ([]ModelInfo, error) {
	if cfg == nil {
		return nil, nil
	}

	models := make([]ModelInfo, 0)
	seen := make(map[string]struct{})

	if shouldDiscoverOllama(cfg) {
		ollamaModels, err := discoverOllamaModels(cfg)
		if err != nil {
			models = appendUniqueModel(models, seen, fallbackModel(cfg, "ollama", "Ollama live discovery unavailable"))
		} else {
			for _, model := range ollamaModels {
				models = appendUniqueModel(models, seen, model)
			}
		}
	}

	for _, providerID := range KnownProviderIDs() {
		pc, ok := cfg.Providers[providerID]
		if !ok {
			continue
		}
		// API-key providers need a key set; OAuth providers are always available.
		if pc.AuthType != "oauth" && strings.TrimSpace(pc.APIKey) == "" {
			continue
		}
		for _, entry := range CatalogueModels(providerID) {
			models = appendUniqueModel(models, seen, ModelInfo{
				ID:            entry.ID,
				Name:          entry.Name,
				Provider:      providerID,
				Capability:    entry.Capability,
				Source:        "catalogue",
				Selectable:    true,
				ReleaseDate:   entry.ReleaseDate,
				IsFree:        entry.IsFree,
				ContextWindow: entry.ContextWindow,
			})
		}
	}

	for _, providerID := range configuredCompatibleProviders(cfg) {
		models = appendUniqueModel(models, seen, configuredProviderModel(cfg, providerID))
	}

	currentProvider := strings.TrimSpace(cfg.Agents.Defaults.Provider)
	currentModel := strings.TrimSpace(cfg.Agents.Defaults.Model)
	if currentProvider != "" && currentModel != "" {
		models = appendUniqueModel(models, seen, fallbackModel(cfg, currentProvider, "Current configured model"))
	}

	sort.SliceStable(models, func(i, j int) bool {
		if models[i].Provider != models[j].Provider {
			return models[i].Provider < models[j].Provider
		}
		return models[i].ID < models[j].ID
	})

	return models, nil
}

func shouldDiscoverOllama(cfg *config.Config) bool {
	if cfg == nil {
		return false
	}
	if strings.TrimSpace(cfg.Agents.Defaults.Provider) == "ollama" {
		return true
	}
	_, ok := cfg.Providers["ollama"]
	return ok
}

func configuredCompatibleProviders(cfg *config.Config) []string {
	if cfg == nil {
		return nil
	}

	providers := make([]string, 0, len(cfg.Providers))
	for providerID := range cfg.Providers {
		if !isConfiguredCompatibleProvider(providerID) {
			continue
		}
		providers = append(providers, providerID)
	}
	sort.Strings(providers)
	return providers
}

func isConfiguredCompatibleProvider(providerID string) bool {
	switch strings.TrimSpace(providerID) {
	case "", "ollama":
		return false
	default:
		if len(CatalogueModels(providerID)) > 0 {
			return false
		}
		return true
	}
}

func configuredProviderModel(cfg *config.Config, providerID string) ModelInfo {
	modelID := strings.TrimSpace(providerID)
	description := configuredProviderDescription(providerID, cfg.Providers[providerID])

	capability := "openai-compatible"
	if providerID == "azure_openai" {
		capability = "azure-openai"
	}

	return ModelInfo{
		ID:          modelID,
		Name:        providerDisplayName(providerID),
		Provider:    providerID,
		Description: description,
		Source:      "config",
		Capability:  capability,
		Selectable:  false,
	}
}

func fallbackModel(cfg *config.Config, providerID, reason string) ModelInfo {
	modelID := fallbackModelID(cfg, providerID)
	name := displayModelName(modelID)
	if name == "" {
		name = providerDisplayName(providerID)
	}

	description := reason
	if strings.TrimSpace(reason) == "" {
		description = "Current configured model"
	}

	capability := ""
	switch providerID {
	case "ollama":
		capability = "local"
	case "azure_openai":
		capability = "azure-openai"
	case "anthropic":
		capability = "anthropic"
	default:
		capability = "openai-compatible"
	}

	return ModelInfo{
		ID:          modelID,
		Name:        name,
		Provider:    providerID,
		Description: description,
		Source:      "fallback",
		Capability:  capability,
		Selectable:  true,
	}
}

func appendUniqueModel(models []ModelInfo, seen map[string]struct{}, model ModelInfo) []ModelInfo {
	if strings.TrimSpace(model.Provider) == "" || strings.TrimSpace(model.ID) == "" {
		return models
	}
	key := model.Provider + "\x00" + model.ID
	if _, ok := seen[key]; ok {
		return models
	}
	seen[key] = struct{}{}
	return append(models, model)
}

func fallbackModelID(cfg *config.Config, providerID string) string {
	if cfg == nil {
		return providerID
	}

	currentProvider := strings.TrimSpace(cfg.Agents.Defaults.Provider)
	currentModel := strings.TrimSpace(cfg.Agents.Defaults.Model)
	if providerID == currentProvider && currentModel != "" {
		return currentModel
	}
	return providerID
}

func displayModelName(modelID string) string {
	modelID = strings.TrimSpace(modelID)
	if modelID == "" {
		return ""
	}
	for _, prefix := range []string{"openai", "openrouter", "azure", "ollama", "groq", "deepseek", "minimax", "gemini", "moonshot", "aihubmix", "siliconflow", "volcengine", "byteplus", "dashscope", "zhipu", "vllm"} {
		if strings.HasPrefix(strings.ToLower(modelID), prefix+"/") {
			return modelID[len(prefix)+1:]
		}
	}
	return modelID
}

func providerDisplayName(providerID string) string {
	switch providerID {
	case "openai":
		return "OpenAI"
	case "azure_openai":
		return "Azure OpenAI"
	case "openrouter":
		return "OpenRouter"
	case "ollama":
		return "Ollama"
	}

	parts := strings.FieldsFunc(providerID, func(r rune) bool {
		return r == '_' || r == '-'
	})
	for i, part := range parts {
		if part == "" {
			continue
		}
		parts[i] = strings.ToUpper(part[:1]) + part[1:]
	}
	return strings.Join(parts, " ")
}

func configuredProviderDescription(providerID string, providerCfg config.ProviderConfig) string {
	baseURL := strings.TrimSpace(providerCfg.APIBase)
	if baseURL == "" {
		return fmt.Sprintf("Configured %s endpoint", providerDisplayName(providerID))
	}
	return fmt.Sprintf("Configured %s endpoint at %s", providerDisplayName(providerID), baseURL)
}
