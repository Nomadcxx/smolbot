package agent

import (
	"context"
	"errors"
	"testing"

	"github.com/Nomadcxx/nanobot-go/pkg/provider"
)

func TestEvaluator_StructuredDecision(t *testing.T) {
	t.Run("deliver=false in malformed JSON-like string is rejected", func(t *testing.T) {
		evaluator := NewEvaluator(fakeDecisionProvider{decision: `{"decision":"deliver","value":false}`})
		if evaluator.ShouldDeliver(context.Background(), "anything") {
			t.Fatal("expected malformed decision to be rejected")
		}
	})

	t.Run("deliver=true in malformed JSON-like string is accepted", func(t *testing.T) {
		evaluator := NewEvaluator(fakeDecisionProvider{decision: `{"decision":"deliver","value":true}`})
		if !evaluator.ShouldDeliver(context.Background(), "anything") {
			t.Fatal("expected valid structured deliver=true to be accepted")
		}
	})

	t.Run("malformed free text defaults closed instead of open", func(t *testing.T) {
		evaluator := NewEvaluator(fakeDecisionProvider{decision: "maybe run? who knows"})
		if evaluator.ShouldDeliver(context.Background(), "anything") {
			t.Fatal("expected malformed free text to fail closed")
		}
	})

	t.Run("garbage output from provider fails closed", func(t *testing.T) {
		evaluator := NewEvaluator(fakeDecisionProvider{decision: "¥←≈¬˚∆˙"})
		if evaluator.ShouldDeliver(context.Background(), "anything") {
			t.Fatal("expected garbage to fail closed")
		}
	})

	t.Run("empty output fails closed", func(t *testing.T) {
		evaluator := NewEvaluator(fakeDecisionProvider{decision: ""})
		if evaluator.ShouldDeliver(context.Background(), "anything") {
			t.Fatal("expected empty decision to fail closed")
		}
	})

	t.Run("whitespace-only output fails closed", func(t *testing.T) {
		evaluator := NewEvaluator(fakeDecisionProvider{decision: "   \n\t  "})
		if evaluator.ShouldDeliver(context.Background(), "anything") {
			t.Fatal("expected whitespace-only to fail closed")
		}
	})

	t.Run("provider error still fails open", func(t *testing.T) {
		evaluator := NewEvaluator(fakeDecisionProvider{err: errors.New("network failure")})
		if !evaluator.ShouldDeliver(context.Background(), "anything") {
			t.Fatal("expected provider error to fail open")
		}
	})

	t.Run("structured JSON decision format accepted", func(t *testing.T) {
		evaluator := NewEvaluator(fakeDecisionProvider{decision: `{"decision":"deliver","value":true}`})
		if !evaluator.ShouldDeliver(context.Background(), "important update") {
			t.Fatal("expected structured deliver=true")
		}
	})

	t.Run("structured JSON skip decision suppresses delivery", func(t *testing.T) {
		evaluator := NewEvaluator(fakeDecisionProvider{decision: `{"decision":"deliver","value":false}`})
		if evaluator.ShouldDeliver(context.Background(), "routine") {
			t.Fatal("expected structured deliver=false to suppress")
		}
	})
}

func TestProviderDecisionProvider_StructuredDecide(t *testing.T) {
	t.Run("rejects malformed provider response", func(t *testing.T) {
		fakeProvider := &fakeDecisionChatProvider{
			resp: &provider.Response{Content: "this is not structured JSON at all"},
		}
		p := ProviderDecisionProvider{
			Provider:     fakeProvider,
			Model:        "claude-test",
			SystemPrompt: "Decide whether to deliver.",
		}
		evaluator := NewEvaluator(p)
		if evaluator.ShouldDeliver(context.Background(), "background update") {
			t.Fatal("expected malformed provider output to fail closed")
		}
	})

	t.Run("accepts valid structured response", func(t *testing.T) {
		fakeProvider := &fakeDecisionChatProvider{
			resp: &provider.Response{Content: `{"decision":"deliver","value":true}`},
		}
		p := ProviderDecisionProvider{
			Provider:     fakeProvider,
			Model:        "claude-test",
			SystemPrompt: "Decide.",
		}
		evaluator := NewEvaluator(p)
		if !evaluator.ShouldDeliver(context.Background(), "important") {
			t.Fatal("expected structured deliver=true")
		}
	})

	t.Run("rejects garbage structured response", func(t *testing.T) {
		fakeProvider := &fakeDecisionChatProvider{
			resp: &provider.Response{Content: `{"decision":"deliver","value":"yes"}`},
		}
		p := ProviderDecisionProvider{
			Provider:     fakeProvider,
			Model:        "claude-test",
			SystemPrompt: "Decide.",
		}
		evaluator := NewEvaluator(p)
		if evaluator.ShouldDeliver(context.Background(), "anything") {
			t.Fatal("expected non-boolean value to fail closed")
		}
	})
}
