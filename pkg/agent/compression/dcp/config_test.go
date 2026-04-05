package dcp

import "testing"

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if !cfg.Deduplication.Enabled {
		t.Fatalf("Deduplication.Enabled = false, want true")
	}
	if !cfg.PurgeErrors.Enabled {
		t.Fatalf("PurgeErrors.Enabled = false, want true")
	}
	if cfg.TurnProtection != 4 {
		t.Fatalf("TurnProtection = %d, want 4", cfg.TurnProtection)
	}
	if !cfg.CompressTool.Enabled {
		t.Fatalf("CompressTool.Enabled = false, want true")
	}
	if cfg.Nudge.MinContextLimit != 50000 {
		t.Fatalf("MinContextLimit = %d, want 50000", cfg.Nudge.MinContextLimit)
	}
	if cfg.Nudge.MaxContextLimit != 100000 {
		t.Fatalf("MaxContextLimit = %d, want 100000", cfg.Nudge.MaxContextLimit)
	}
	if cfg.Nudge.NudgeFrequency != 5 {
		t.Fatalf("NudgeFrequency = %d, want 5", cfg.Nudge.NudgeFrequency)
	}
	if cfg.Nudge.IterationNudgeThreshold != 15 {
		t.Fatalf("IterationNudgeThreshold = %d, want 15", cfg.Nudge.IterationNudgeThreshold)
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate(): %v", err)
	}
}

func TestConfigValidation(t *testing.T) {
	t.Run("negative turn protection", func(t *testing.T) {
		cfg := DefaultConfig()
		cfg.TurnProtection = -1
		if err := cfg.Validate(); err == nil {
			t.Fatal("Validate() error = nil, want error")
		}
	})

	t.Run("invalid purge threshold", func(t *testing.T) {
		cfg := DefaultConfig()
		cfg.PurgeErrors.TurnThreshold = 0
		if err := cfg.Validate(); err == nil {
			t.Fatal("Validate() error = nil, want error")
		}
	})

	t.Run("invalid nudge limits", func(t *testing.T) {
		cfg := DefaultConfig()
		cfg.Nudge.MinContextLimit = 200
		cfg.Nudge.MaxContextLimit = 100
		if err := cfg.Validate(); err == nil {
			t.Fatal("Validate() error = nil, want error")
		}
	})
}
