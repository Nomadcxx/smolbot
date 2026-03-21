package agent

import (
	"context"
	"errors"
	"testing"

	"github.com/Nomadcxx/smolbot/pkg/provider"
)

func TestEvaluator(t *testing.T) {
	t.Run("deliver true", func(t *testing.T) {
		evaluator := NewEvaluator(fakeDecisionProvider{decision: "deliver=true"})
		if !evaluator.ShouldDeliver(context.Background(), "important update") {
			t.Fatal("expected deliver=true to approve delivery")
		}
	})

	t.Run("deliver false", func(t *testing.T) {
		evaluator := NewEvaluator(fakeDecisionProvider{decision: "deliver=false"})
		if evaluator.ShouldDeliver(context.Background(), "routine update") {
			t.Fatal("expected deliver=false to suppress delivery")
		}
	})

	t.Run("fails open on provider failure", func(t *testing.T) {
		evaluator := NewEvaluator(fakeDecisionProvider{err: errors.New("boom")})
		if !evaluator.ShouldDeliver(context.Background(), "anything") {
			t.Fatal("expected provider failure to fail open")
		}
	})

	t.Run("provider decision provider uses provider chat", func(t *testing.T) {
		fakeProvider := &fakeDecisionChatProvider{
			resp: &provider.Response{Content: "deliver=false"},
		}
		decisionProvider := ProviderDecisionProvider{
			Provider:     fakeProvider,
			Model:        "gpt-test",
			SystemPrompt: "Decide carefully.",
		}
		value, err := decisionProvider.Decide(context.Background(), "background output")
		if err != nil {
			t.Fatalf("Decide: %v", err)
		}
		if value != "deliver=false" {
			t.Fatalf("unexpected decision %q", value)
		}
		if len(fakeProvider.requests) != 1 {
			t.Fatalf("expected one provider request, got %d", len(fakeProvider.requests))
		}
		if got := fakeProvider.requests[0].Messages[1].StringContent(); got != "background output" {
			t.Fatalf("unexpected decision prompt content %q", got)
		}
	})
}

type fakeDecisionProvider struct {
	decision string
	err      error
}

func (f fakeDecisionProvider) Decide(context.Context, string) (string, error) {
	return f.decision, f.err
}

type fakeDecisionChatProvider struct {
	requests []provider.ChatRequest
	resp     *provider.Response
	err      error
}

func (f *fakeDecisionChatProvider) Chat(_ context.Context, req provider.ChatRequest) (*provider.Response, error) {
	f.requests = append(f.requests, req)
	if f.err != nil {
		return nil, f.err
	}
	return f.resp, nil
}

func (f *fakeDecisionChatProvider) ChatStream(context.Context, provider.ChatRequest) (*provider.Stream, error) {
	return nil, errors.New("unexpected ChatStream call")
}

func (f *fakeDecisionChatProvider) Name() string { return "openai" }
