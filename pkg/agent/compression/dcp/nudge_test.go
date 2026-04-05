package dcp

import (
	"strings"
	"testing"

	"github.com/Nomadcxx/smolbot/pkg/provider"
	"github.com/Nomadcxx/smolbot/pkg/tokenizer"
)

func TestNudge_CriticalTier(t *testing.T) {
	msgs := []provider.Message{makeUserMsg(strings.Repeat("x", 400))}
	state := NewState("s1")
	state.RequestCount = 5
	cfg := DefaultConfig()
	cfg.Nudge.MinContextLimit = 10
	cfg.Nudge.MaxContextLimit = 20
	cfg.Nudge.NudgeFrequency = 5

	if got := InjectNudges(msgs, state, cfg, tokenizer.New(), 100); got != 1 {
		t.Fatalf("InjectNudges() = %d, want 1", got)
	}
	if !strings.Contains(msgs[0].StringContent(), "CRITICAL") {
		t.Fatalf("critical nudge missing: %q", msgs[0].StringContent())
	}
}

func TestNudge_TurnTier(t *testing.T) {
	msgs := []provider.Message{makeUserMsg(strings.Repeat("x", 80))}
	state := NewState("s1")
	state.RequestCount = 5
	cfg := DefaultConfig()
	cfg.Nudge.MinContextLimit = 10
	cfg.Nudge.MaxContextLimit = 1000000
	cfg.Nudge.NudgeFrequency = 5

	if got := InjectNudges(msgs, state, cfg, tokenizer.New(), 1000); got != 1 {
		t.Fatalf("InjectNudges() = %d, want 1", got)
	}
	if !strings.Contains(msgs[0].StringContent(), "Context is growing") {
		t.Fatalf("turn nudge missing: %q", msgs[0].StringContent())
	}
}

func TestNudge_FrequencyControl(t *testing.T) {
	msgs := []provider.Message{makeUserMsg(strings.Repeat("x", 80))}
	state := NewState("s1")
	state.RequestCount = 3
	cfg := DefaultConfig()
	cfg.Nudge.MinContextLimit = 10
	cfg.Nudge.NudgeFrequency = 5

	if got := InjectNudges(msgs, state, cfg, tokenizer.New(), 1000); got != 0 {
		t.Fatalf("InjectNudges() = %d, want 0", got)
	}
}
