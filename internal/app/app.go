package app

import (
	"fmt"

	"github.com/Nomadcxx/smolbot/internal/client"
)

type Config struct {
	Host       string
	Port       int
	Theme      string
	Session    string
	MCPServers []client.MCPServerInfo
}

type App struct {
	Config         Config
	Theme          string
	Session        string
	Model          string
	SidebarVisible bool
	Connected      bool
	State          State
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
		Config:         cfg,
		Theme:          theme,
		Session:        session,
		Model:          state.LastModel,
		SidebarVisible: state.SidebarVisible == nil || *state.SidebarVisible,
		State:          state,
	}
}

func (a *App) WSURL() string {
	return fmt.Sprintf("ws://%s:%d/ws", a.Config.Host, a.Config.Port)
}
