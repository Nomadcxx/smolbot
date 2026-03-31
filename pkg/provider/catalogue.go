package provider

import (
	"sort"
	"strings"

	"github.com/Nomadcxx/smolbot/pkg/config"
)

type CatalogueEntry struct {
	ID         string
	Name       string
	Capability string
}

var providerCatalogue = map[string][]CatalogueEntry{
	"anthropic": {
		{ID: "claude-opus-4-5", Name: "Claude Opus 4.5", Capability: "chat"},
		{ID: "claude-sonnet-4-5-20251001", Name: "Claude Sonnet 4.5", Capability: "chat"},
		{ID: "claude-haiku-4-5-20251001", Name: "Claude Haiku 4.5", Capability: "chat"},
		{ID: "claude-opus-4", Name: "Claude Opus 4", Capability: "chat"},
		{ID: "claude-sonnet-4", Name: "Claude Sonnet 4", Capability: "chat"},
		{ID: "claude-3-5-sonnet-20241022", Name: "Claude 3.5 Sonnet", Capability: "chat"},
		{ID: "claude-3-5-haiku-20241022", Name: "Claude 3.5 Haiku", Capability: "chat"},
	},
	"openai": {
		{ID: "gpt-4o", Name: "GPT-4o", Capability: "chat"},
		{ID: "gpt-4o-mini", Name: "GPT-4o mini", Capability: "chat"},
		{ID: "gpt-4-turbo", Name: "GPT-4 Turbo", Capability: "chat"},
		{ID: "o1", Name: "o1", Capability: "reasoning"},
		{ID: "o1-mini", Name: "o1-mini", Capability: "reasoning"},
		{ID: "o3-mini", Name: "o3-mini", Capability: "reasoning"},
		{ID: "o3", Name: "o3", Capability: "reasoning"},
	},
	"gemini": {
		{ID: "gemini-2.5-pro-preview-03-25", Name: "Gemini 2.5 Pro", Capability: "chat"},
		{ID: "gemini-2.0-flash", Name: "Gemini 2.0 Flash", Capability: "chat"},
		{ID: "gemini-2.0-flash-lite", Name: "Gemini 2.0 Flash Lite", Capability: "chat"},
		{ID: "gemini-1.5-pro-002", Name: "Gemini 1.5 Pro", Capability: "chat"},
		{ID: "gemini-1.5-flash-002", Name: "Gemini 1.5 Flash", Capability: "chat"},
	},
	"groq": {
		{ID: "llama-3.3-70b-versatile", Name: "Llama 3.3 70B", Capability: "chat"},
		{ID: "llama-3.1-8b-instant", Name: "Llama 3.1 8B", Capability: "chat"},
		{ID: "mixtral-8x7b-32768", Name: "Mixtral 8x7B", Capability: "chat"},
		{ID: "gemma2-9b-it", Name: "Gemma 2 9B", Capability: "chat"},
	},
	"deepseek": {
		{ID: "deepseek-chat", Name: "DeepSeek Chat", Capability: "chat"},
		{ID: "deepseek-reasoner", Name: "DeepSeek Reasoner", Capability: "reasoning"},
	},
	"minimax": {
		{ID: "MiniMax-Text-01", Name: "MiniMax Text-01", Capability: "chat"},
		{ID: "abab6.5s-chat", Name: "ABAB 6.5s", Capability: "chat"},
	},
}

func CatalogueModels(providerID string) []CatalogueEntry {
	return providerCatalogue[providerID]
}

func KnownProviderIDs() []string {
	ids := make([]string, 0, len(providerCatalogue))
	for id := range providerCatalogue {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

func isCatalogueProvider(providerID string) bool {
	_, ok := providerCatalogue[providerID]
	return ok
}

func isConfiguredWithAPIKey(pc config.ProviderConfig) bool {
	if strings.TrimSpace(pc.APIKey) == "" {
		return false
	}
	if pc.AuthType == "oauth" {
		return false
	}
	return true
}
