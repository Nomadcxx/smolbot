package heartbeat

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/Nomadcxx/smolbot/pkg/agent"
	"github.com/Nomadcxx/smolbot/pkg/provider"
)

type Decider interface {
	Decide(ctx context.Context) (string, error)
}

type ModelSetter interface {
	SetModel(model string)
}

type Processor interface {
	ProcessDirect(ctx context.Context, req agent.Request, cb agent.EventCallback) (string, error)
}

type Evaluator interface {
	ShouldDeliver(ctx context.Context, content string) bool
}

type Router interface {
	Route(ctx context.Context, channel, chatID, content string) error
}

type ServiceDeps struct {
	Decider   Decider
	Processor Processor
	Evaluator Evaluator
	Router    Router
	Channel   string
	ChatID    string
}

type Service struct {
	decider   Decider
	processor Processor
	evaluator Evaluator
	router    Router
	channel   string
	chatID    string
	model     string
}

type ProviderDecider struct {
	Provider     provider.Provider
	Model        string
	SystemPrompt string
	MaxTokens    int
}

func (d *ProviderDecider) SetModel(model string) {
	d.Model = model
}

func (s *Service) EffectiveModel() string {
	return s.model
}

func (s *Service) SetActiveModel(model string) {
	s.model = model
	if pd, ok := s.decider.(ModelSetter); ok {
		pd.SetModel(model)
	}
	if ev, ok := s.evaluator.(ModelSetter); ok {
		ev.SetModel(model)
	}
}

func NewService(deps ServiceDeps) *Service {
	return &Service{
		decider:   deps.Decider,
		processor: deps.Processor,
		evaluator: deps.Evaluator,
		router:    deps.Router,
		channel:   deps.Channel,
		chatID:    deps.ChatID,
	}
}

func (d ProviderDecider) Decide(ctx context.Context) (string, error) {
	if d.Provider == nil {
		return "", fmt.Errorf("heartbeat decision provider unavailable")
	}
	maxTokens := d.MaxTokens
	if maxTokens <= 0 {
		maxTokens = 32
	}
	resp, err := d.Provider.Chat(ctx, provider.ChatRequest{
		Model: d.Model,
		Messages: []provider.Message{
			{Role: "system", Content: d.SystemPrompt},
			{Role: "user", Content: `Decide whether heartbeat should act now. Reply with exactly "run" or "skip".`},
		},
		MaxTokens:   maxTokens,
		Temperature: 0,
	})
	if err != nil {
		return "", err
	}
	return strings.ToLower(strings.TrimSpace(resp.Content)), nil
}

func (s *Service) RunOnce(ctx context.Context) error {
	decision := "skip"
	if s.decider != nil {
		value, err := s.decider.Decide(ctx)
		if err != nil {
			return nil
		}
		decision = strings.ToLower(strings.TrimSpace(value))
	}
	var structured struct {
		Action string `json:"action"`
	}
	if err := json.Unmarshal([]byte(decision), &structured); err == nil {
		if structured.Action == "run" {
			decision = "run"
		} else {
			decision = "skip"
		}
	} else {
		if decision != "run" && decision != "skip" {
			decision = "skip"
		}
	}
	if decision != "run" || s.processor == nil {
		return nil
	}

	result, err := s.processor.ProcessDirect(ctx, agent.Request{
		Content:    "heartbeat",
		SessionKey: "heartbeat",
		Channel:    s.channel,
		ChatID:     s.chatID,
	}, nil)
	if err != nil {
		return err
	}
	if s.router == nil {
		return nil
	}
	if s.evaluator != nil && !s.evaluator.ShouldDeliver(ctx, result) {
		return nil
	}
	return s.router.Route(ctx, s.channel, s.chatID, result)
}
