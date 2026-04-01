package provider

import (
	"sort"
	"strings"

	"github.com/Nomadcxx/smolbot/pkg/config"
)

type CatalogueEntry struct {
	ID            string
	Name          string
	Capability    string
	ReleaseDate   string
	IsFree        bool
	ContextWindow int
}

var providerCatalogue = map[string][]CatalogueEntry{
	"anthropic": {
		{ID: "claude-opus-4-5", Name: "Claude Opus 4.5", Capability: "chat", ReleaseDate: "2025-07"},
		{ID: "claude-sonnet-4-5-20251001", Name: "Claude Sonnet 4.5", Capability: "chat", ReleaseDate: "2025-10"},
		{ID: "claude-haiku-4-5-20251001", Name: "Claude Haiku 4.5", Capability: "chat", ReleaseDate: "2025-10"},
		{ID: "claude-opus-4", Name: "Claude Opus 4", Capability: "chat", ReleaseDate: "2025-05"},
		{ID: "claude-sonnet-4", Name: "Claude Sonnet 4", Capability: "chat", ReleaseDate: "2025-05"},
		{ID: "claude-3-5-sonnet-20241022", Name: "Claude 3.5 Sonnet", Capability: "chat", ReleaseDate: "2024-10", ContextWindow: 200000},
		{ID: "claude-3-5-haiku-20241022", Name: "Claude 3.5 Haiku", Capability: "chat", ReleaseDate: "2024-10", ContextWindow: 200000},
	},
	"openai": {
		{ID: "gpt-4o", Name: "GPT-4o", Capability: "chat", ReleaseDate: "2024-05", ContextWindow: 128000},
		{ID: "gpt-4o-mini", Name: "GPT-4o mini", Capability: "chat", ReleaseDate: "2024-07", ContextWindow: 128000},
		{ID: "gpt-4-turbo", Name: "GPT-4 Turbo", Capability: "chat", ReleaseDate: "2024-04", ContextWindow: 128000},
		{ID: "o1", Name: "o1", Capability: "reasoning", ReleaseDate: "2024-09", ContextWindow: 200000},
		{ID: "o1-mini", Name: "o1-mini", Capability: "reasoning", ReleaseDate: "2024-09", ContextWindow: 128000},
		{ID: "o3-mini", Name: "o3-mini", Capability: "reasoning", ReleaseDate: "2025-01", ContextWindow: 200000},
		{ID: "o3", Name: "o3", Capability: "reasoning", ReleaseDate: "2025-04", ContextWindow: 200000},
	},
	"gemini": {
		{ID: "gemini-2.5-pro-preview-03-25", Name: "Gemini 2.5 Pro", Capability: "chat", ReleaseDate: "2025-03", ContextWindow: 1000000},
		{ID: "gemini-2.0-flash", Name: "Gemini 2.0 Flash", Capability: "chat", ReleaseDate: "2025-01", ContextWindow: 1000000},
		{ID: "gemini-2.0-flash-lite", Name: "Gemini 2.0 Flash Lite", Capability: "chat", ReleaseDate: "2025-02", ContextWindow: 1000000, IsFree: true},
		{ID: "gemini-1.5-pro-002", Name: "Gemini 1.5 Pro", Capability: "chat", ReleaseDate: "2024-09", ContextWindow: 2000000},
		{ID: "gemini-1.5-flash-002", Name: "Gemini 1.5 Flash", Capability: "chat", ReleaseDate: "2024-09", ContextWindow: 1000000, IsFree: true},
	},
	"groq": {
		{ID: "llama-3.3-70b-versatile", Name: "Llama 3.3 70B", Capability: "chat", ReleaseDate: "2024-12", ContextWindow: 128000, IsFree: true},
		{ID: "llama-3.1-8b-instant", Name: "Llama 3.1 8B", Capability: "chat", ReleaseDate: "2024-07", ContextWindow: 128000, IsFree: true},
		{ID: "mixtral-8x7b-32768", Name: "Mixtral 8x7B", Capability: "chat", ReleaseDate: "2024-01", ContextWindow: 32768, IsFree: true},
		{ID: "gemma2-9b-it", Name: "Gemma 2 9B", Capability: "chat", ReleaseDate: "2024-06", ContextWindow: 8192, IsFree: true},
	},
	"deepseek": {
		{ID: "deepseek-chat", Name: "DeepSeek Chat", Capability: "chat", ReleaseDate: "2025-01", ContextWindow: 64000},
		{ID: "deepseek-reasoner", Name: "DeepSeek Reasoner", Capability: "reasoning", ReleaseDate: "2025-01", ContextWindow: 64000},
	},
	"minimax": {
		{ID: "MiniMax-Text-01", Name: "MiniMax Text-01", Capability: "chat", ReleaseDate: "2025-01", ContextWindow: 1000000},
		{ID: "abab6.5s-chat", Name: "ABAB 6.5s", Capability: "chat", ReleaseDate: "2024-06", ContextWindow: 245000},
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
