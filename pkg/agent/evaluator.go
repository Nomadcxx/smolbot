package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/Nomadcxx/nanobot-go/pkg/provider"
)

type DecisionProvider interface {
	Decide(ctx context.Context, content string) (string, error)
}

type Evaluator struct {
	provider DecisionProvider
}

type ProviderDecisionProvider struct {
	Provider     provider.Provider
	Model        string
	SystemPrompt string
	MaxTokens    int
}

func (p *ProviderDecisionProvider) SetModel(model string) {
	p.Model = model
}

func NewEvaluator(provider DecisionProvider) *Evaluator {
	return &Evaluator{provider: provider}
}

func (e *Evaluator) ShouldDeliver(ctx context.Context, content string) bool {
	if e == nil || e.provider == nil {
		return true
	}
	decision, err := e.provider.Decide(ctx, content)
	if err != nil {
		return true
	}
	normalized := strings.ToLower(strings.TrimSpace(decision))
	var structured struct {
		Decision string `json:"decision"`
		Value    bool   `json:"value"`
	}
	if err := json.Unmarshal([]byte(normalized), &structured); err == nil {
		if structured.Decision == "deliver" {
			return structured.Value
		}
	}
	if strings.Contains(normalized, "deliver=false") {
		return false
	}
	if strings.Contains(normalized, "deliver=true") {
		return true
	}
	return false
}

func (p ProviderDecisionProvider) Decide(ctx context.Context, content string) (string, error) {
	if p.Provider == nil {
		return "", fmt.Errorf("decision provider unavailable")
	}
	maxTokens := p.MaxTokens
	if maxTokens <= 0 {
		maxTokens = 64
	}
	resp, err := p.Provider.Chat(ctx, provider.ChatRequest{
		Model: p.Model,
		Messages: []provider.Message{
			{Role: "system", Content: p.SystemPrompt},
			{Role: "user", Content: content},
		},
		MaxTokens:   maxTokens,
		Temperature: 0,
	})
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(resp.Content), nil
}
