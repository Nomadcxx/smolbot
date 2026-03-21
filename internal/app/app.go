package app

import "fmt"

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

func New(cfg Config) *App {
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
