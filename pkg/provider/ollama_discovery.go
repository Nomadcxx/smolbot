package provider

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/Nomadcxx/nanobot-go/pkg/config"
)

// OllamaClient provides model discovery for Ollama servers
type OllamaClient struct {
	baseURL string
	client  *http.Client
}

// OllamaModel represents a model from Ollama's /api/tags endpoint
type OllamaModel struct {
	Name       string `json:"name"`
	Model      string `json:"model"`
	ModifiedAt string `json:"modified_at"`
	Size       int64  `json:"size"`
	Digest     string `json:"digest"`
	Details    struct {
		ParentModel       string   `json:"parent_model"`
		Format            string   `json:"format"`
		Family            string   `json:"family"`
		Families          []string `json:"families"`
		ParameterSize     string   `json:"parameter_size"`
		QuantizationLevel string   `json:"quantization_level"`
	} `json:"details"`
}

// NewOllamaClient creates a client for Ollama model discovery
func NewOllamaClient(baseURL string) *OllamaClient {
	if baseURL == "" {
		baseURL = "http://localhost:11434"
	}
	return &OllamaClient{
		baseURL: baseURL,
		client:  &http.Client{Timeout: 10 * time.Second},
	}
}

// ListModels fetches all available models from Ollama
func (c *OllamaClient) ListModels() ([]OllamaModel, error) {
	resp, err := c.client.Get(c.baseURL + "/api/tags")
	if err != nil {
		return nil, fmt.Errorf("fetch models from ollama: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("ollama returned status %d", resp.StatusCode)
	}

	var result struct {
		Models []OllamaModel `json:"models"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode ollama response: %w", err)
	}

	return result.Models, nil
}

// ModelInfo represents a model for the TUI
type ModelInfo struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Provider string `json:"provider"`
}

// GetAvailableModels returns all available models for the current provider configuration
func GetAvailableModels(cfg *config.Config) ([]ModelInfo, error) {
	provider := cfg.Agents.Defaults.Provider
	
	switch provider {
	case "ollama":
		return getOllamaModels(cfg)
	default:
		// For other providers, return just the current model
		// Future: implement discovery for OpenRouter, etc.
		return []ModelInfo{{
			ID:       cfg.Agents.Defaults.Model,
			Name:     cfg.Agents.Defaults.Model,
			Provider: provider,
		}}, nil
	}
}

func getOllamaModels(cfg *config.Config) ([]ModelInfo, error) {
	baseURL := cfg.Providers["ollama"].APIBase
	if baseURL == "" {
		baseURL = "http://localhost:11434"
	}
	
	client := NewOllamaClient(baseURL)
	ollamaModels, err := client.ListModels()
	if err != nil {
		// If Ollama is not available, return just the current model
		return []ModelInfo{{
			ID:       cfg.Agents.Defaults.Model,
			Name:     cfg.Agents.Defaults.Model,
			Provider: "ollama",
		}}, nil
	}

	models := make([]ModelInfo, len(ollamaModels))
	for i, m := range ollamaModels {
		models[i] = ModelInfo{
			ID:       m.Name,
			Name:     m.Name,
			Provider: "ollama",
		}
	}
	return models, nil
}
