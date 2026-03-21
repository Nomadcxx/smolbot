package app

import (
	"encoding/json"
	"os"
	"path/filepath"
)

type State struct {
	Theme       string `json:"theme"`
	LastSession string `json:"lastSession"`
	LastModel   string `json:"lastModel"`
}

func statePath() string {
	dir, err := os.UserConfigDir()
	if err != nil {
		return filepath.Join(".", "state.json")
	}
	return filepath.Join(dir, "smolbot-tui", "state.json")
}

func LoadState() State {
	data, err := os.ReadFile(statePath())
	if err != nil {
		return State{Theme: "nord", LastSession: "tui:main"}
	}

	var state State
	if err := json.Unmarshal(data, &state); err != nil {
		return State{Theme: "nord", LastSession: "tui:main"}
	}
	if state.Theme == "" {
		state.Theme = "nord"
	}
	if state.LastSession == "" {
		state.LastSession = "tui:main"
	}
	return state
}

func SaveState(state State) error {
	path := statePath()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}
