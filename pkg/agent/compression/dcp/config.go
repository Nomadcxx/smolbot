package dcp

import "fmt"

type Config struct {
	Deduplication  DeduplicationConfig `json:"deduplication"`
	PurgeErrors    PurgeErrorsConfig   `json:"purgeErrors"`
	TurnProtection int                 `json:"turnProtection"`
	CompressTool   CompressToolConfig  `json:"compressTool"`
	Nudge          NudgeConfig         `json:"nudge"`
	ProtectedTools []string            `json:"protectedTools"`
}

type DeduplicationConfig struct {
	Enabled        bool     `json:"enabled"`
	ProtectedTools []string `json:"protectedTools"`
}

type PurgeErrorsConfig struct {
	Enabled        bool     `json:"enabled"`
	TurnThreshold  int      `json:"turnThreshold"`
	ProtectedTools []string `json:"protectedTools"`
}

type CompressToolConfig struct {
	Enabled         bool     `json:"enabled"`
	ProtectedTools  []string `json:"protectedTools"`
	ProtectUserMsgs bool     `json:"protectUserMessages"`
}

type NudgeConfig struct {
	MinContextLimit         int `json:"minContextLimit"`
	MaxContextLimit         int `json:"maxContextLimit"`
	NudgeFrequency          int `json:"nudgeFrequency"`
	IterationNudgeThreshold int `json:"iterationNudgeThreshold"`
}

func DefaultConfig() Config {
	return Config{
		Deduplication: DeduplicationConfig{
			Enabled: true,
		},
		PurgeErrors: PurgeErrorsConfig{
			Enabled:       true,
			TurnThreshold: 4,
		},
		TurnProtection: 4,
		CompressTool: CompressToolConfig{
			Enabled: true,
		},
		Nudge: NudgeConfig{
			MinContextLimit:         50000,
			MaxContextLimit:         100000,
			NudgeFrequency:          5,
			IterationNudgeThreshold: 15,
		},
	}
}

func (c Config) Validate() error {
	if c.TurnProtection < 0 {
		return fmt.Errorf("turnProtection must be >= 0")
	}
	if c.PurgeErrors.TurnThreshold <= 0 {
		return fmt.Errorf("purgeErrors.turnThreshold must be > 0")
	}
	if c.Nudge.MinContextLimit <= 0 {
		return fmt.Errorf("nudge.minContextLimit must be > 0")
	}
	if c.Nudge.MaxContextLimit <= 0 {
		return fmt.Errorf("nudge.maxContextLimit must be > 0")
	}
	if c.Nudge.MinContextLimit > c.Nudge.MaxContextLimit {
		return fmt.Errorf("nudge.minContextLimit must be <= nudge.maxContextLimit")
	}
	if c.Nudge.NudgeFrequency <= 0 {
		return fmt.Errorf("nudge.nudgeFrequency must be > 0")
	}
	if c.Nudge.IterationNudgeThreshold <= 0 {
		return fmt.Errorf("nudge.iterationNudgeThreshold must be > 0")
	}
	return nil
}
