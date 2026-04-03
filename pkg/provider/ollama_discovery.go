package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/Nomadcxx/smolbot/pkg/config"
)

// OllamaClient provides model discovery for Ollama servers
type OllamaClient struct {
	baseURL string
	client  *http.Client
}

// OllamaContextWindow captures the detected context window and where it came from.
type OllamaContextWindow struct {
	Value  int
	Found  bool
	Source string
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
	baseURL = normalizeOllamaBaseURL(baseURL)
	return &OllamaClient{
		baseURL: baseURL,
		client:  &http.Client{Timeout: 10 * time.Second},
	}
}

func normalizeOllamaBaseURL(baseURL string) string {
	baseURL = strings.TrimSpace(strings.TrimRight(baseURL, "/"))
	if baseURL == "" {
		return "http://localhost:11434"
	}
	baseURL = strings.TrimSuffix(baseURL, "/v1")
	baseURL = strings.TrimSuffix(baseURL, "/api")
	baseURL = strings.TrimRight(baseURL, "/")
	if baseURL == "" {
		return "http://localhost:11434"
	}
	return baseURL
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

type ollamaPSResponse struct {
	Models []struct {
		Name          string `json:"name"`
		Model         string `json:"model"`
		ContextLength int    `json:"context_length"`
	} `json:"models"`
}

type ollamaShowResponse struct {
	Parameters string         `json:"parameters"`
	ModelInfo  map[string]any `json:"model_info"`
}

// ContextWindow returns the detected context window for a model.
func (c *OllamaClient) ContextWindow(ctx context.Context, model string) (OllamaContextWindow, error) {
	if found, ok := c.contextWindowFromPS(ctx, model); ok {
		return found, nil
	}
	if found, ok := c.contextWindowFromShow(ctx, model); ok {
		return found, nil
	}
	return OllamaContextWindow{}, nil
}

func (c *OllamaClient) contextWindowFromPS(ctx context.Context, model string) (OllamaContextWindow, bool) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/api/ps", nil)
	if err != nil {
		return OllamaContextWindow{}, false
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return OllamaContextWindow{}, false
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return OllamaContextWindow{}, false
	}

	var result ollamaPSResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return OllamaContextWindow{}, false
	}
	for _, entry := range result.Models {
		if entry.Name == model || entry.Model == model {
			if entry.ContextLength > 0 {
				return OllamaContextWindow{Value: entry.ContextLength, Found: true, Source: "ps"}, true
			}
		}
	}
	return OllamaContextWindow{}, false
}

func (c *OllamaClient) contextWindowFromShow(ctx context.Context, model string) (OllamaContextWindow, bool) {
	payload, err := json.Marshal(map[string]string{"model": model})
	if err != nil {
		return OllamaContextWindow{}, false
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/api/show", strings.NewReader(string(payload)))
	if err != nil {
		return OllamaContextWindow{}, false
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return OllamaContextWindow{}, false
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return OllamaContextWindow{}, false
	}

	var result ollamaShowResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return OllamaContextWindow{}, false
	}

	if value, ok := parseOllamaNumCtx(result.Parameters); ok {
		return OllamaContextWindow{Value: value, Found: true, Source: "show.parameters"}, true
	}
	if value, ok := findContextLength(result.ModelInfo); ok {
		return OllamaContextWindow{Value: value, Found: true, Source: "show.model_info"}, true
	}
	return OllamaContextWindow{}, false
}

func parseOllamaNumCtx(parameters string) (int, bool) {
	for _, line := range strings.Split(parameters, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 2 || fields[0] != "num_ctx" {
			continue
		}
		value, err := strconv.Atoi(fields[1])
		if err != nil {
			continue
		}
		return value, true
	}
	return 0, false
}

func findContextLength(value any) (int, bool) {
	switch typed := value.(type) {
	case map[string]any:
		for key, child := range typed {
			if strings.Contains(strings.ToLower(key), "context_length") {
				if value, ok := anyToInt(child); ok {
					return value, true
				}
			}
			if value, ok := findContextLength(child); ok {
				return value, true
			}
		}
	case []any:
		for _, child := range typed {
			if value, ok := findContextLength(child); ok {
				return value, true
			}
		}
	}
	return 0, false
}

func anyToInt(value any) (int, bool) {
	switch typed := value.(type) {
	case float64:
		return int(typed), true
	case float32:
		return int(typed), true
	case int:
		return typed, true
	case int32:
		return int(typed), true
	case int64:
		return int(typed), true
	case json.Number:
		n, err := typed.Int64()
		if err != nil {
			return 0, false
		}
		return int(n), true
	case string:
		n, err := strconv.Atoi(typed)
		if err != nil {
			return 0, false
		}
		return n, true
	default:
		return 0, false
	}
}

func discoverOllamaModels(cfg *config.Config) ([]ModelInfo, error) {
	baseURL := cfg.Providers["ollama"].APIBase
	if baseURL == "" {
		baseURL = "http://localhost:11434"
	}

	client := NewOllamaClient(baseURL)
	ollamaModels, err := client.ListModels()
	if err != nil {
		return nil, err
	}

	models := make([]ModelInfo, len(ollamaModels))
	for i, m := range ollamaModels {
		// Prefix with ollama/ for correct routing
		modelID := "ollama/" + m.Name
		models[i] = ModelInfo{
			ID:          modelID,
			Name:        m.Name,
			Provider:    "ollama",
			Description: ollamaModelDescription(m),
			Source:      "ollama.live",
			Capability:  "local",
			Selectable:  true,
		}
	}
	return models, nil
}

func ollamaModelDescription(model OllamaModel) string {
	parts := make([]string, 0, 3)
	if family := strings.TrimSpace(model.Details.Family); family != "" {
		parts = append(parts, family)
	}
	if size := strings.TrimSpace(model.Details.ParameterSize); size != "" {
		parts = append(parts, size)
	}
	if quantization := strings.TrimSpace(model.Details.QuantizationLevel); quantization != "" {
		parts = append(parts, quantization)
	}
	return strings.Join(parts, " • ")
}
