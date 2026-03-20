package tui

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

type Config struct {
	Host    string
	Port    int
	Theme   string
	Session string
}

type App struct {
	Config    Config
	Theme     string
	Session   string
	Model     string
	Connected bool
}

type State struct {
	Theme       string `json:"theme"`
	LastSession string `json:"lastSession"`
	LastModel   string `json:"lastModel"`
}

type RuntimeStatus struct {
	Model            string
	Provider         string
	UptimeSeconds    int
	Channels         []string
	ConnectedClients int
}

func NewApp(cfg Config) *App {
	state := LoadState()

	session := cfg.Session
	if session == "" {
		session = state.LastSession
	}
	if session == "" {
		session = "tui:main"
	}

	theme := cfg.Theme
	if theme == "" {
		theme = state.Theme
	}
	if theme == "" {
		theme = "nord"
	}

	return &App{
		Config:  cfg,
		Theme:   theme,
		Session: session,
		Model:   state.LastModel,
	}
}

func (a *App) WSURL() string {
	return fmt.Sprintf("ws://%s:%d/ws", a.Config.Host, a.Config.Port)
}

func statePath() string {
	dir, err := os.UserConfigDir()
	if err != nil {
		return filepath.Join(".", "state.json")
	}
	return filepath.Join(dir, "nanobot-tui", "state.json")
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
