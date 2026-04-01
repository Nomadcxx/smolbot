package app

import (
	"encoding/json"
	"os"
	"path/filepath"
)

const MaxRecents = 10

type State struct {
	Theme          string   `json:"theme"`
	LastSession    string   `json:"lastSession"`
	LastModel      string   `json:"lastModel"`
	SidebarVisible *bool    `json:"sidebarVisible,omitempty"`
	Favorites      []string `json:"favorites,omitempty"`
	Recents        []string `json:"recents,omitempty"`
}

func (s *State) ToggleFavorite(modelID string) bool {
	for i, id := range s.Favorites {
		if id == modelID {
			s.Favorites = append(s.Favorites[:i], s.Favorites[i+1:]...)
			return false
		}
	}
	s.Favorites = append(s.Favorites, modelID)
	return true
}

func (s *State) IsFavorite(modelID string) bool {
	for _, id := range s.Favorites {
		if id == modelID {
			return true
		}
	}
	return false
}

func (s *State) AddRecent(modelID string) {
	s.RemoveRecent(modelID)
	s.Recents = append([]string{modelID}, s.Recents...)
	if len(s.Recents) > MaxRecents {
		s.Recents = s.Recents[:MaxRecents]
	}
}

func (s *State) RemoveRecent(modelID string) {
	out := s.Recents[:0]
	for _, id := range s.Recents {
		if id != modelID {
			out = append(out, id)
		}
	}
	s.Recents = out
}

func (s *State) RecentModelIDs() []string {
	cp := make([]string, len(s.Recents))
	copy(cp, s.Recents)
	return cp
}

func (s *State) FavoriteModelIDs() []string {
	cp := make([]string, len(s.Favorites))
	copy(cp, s.Favorites)
	return cp
}

func statePath() string {
	dir, err := os.UserConfigDir()
	if err != nil {
		return filepath.Join(".", "state.json")
	}
	return filepath.Join(dir, "smolbot-tui", "state.json")
}

func LoadState() State {
	defaultSidebarVisible := true
	data, err := os.ReadFile(statePath())
	if err != nil {
		return State{Theme: "nord", LastSession: "tui:main", SidebarVisible: &defaultSidebarVisible}
	}

	var state State
	if err := json.Unmarshal(data, &state); err != nil {
		return State{Theme: "nord", LastSession: "tui:main", SidebarVisible: &defaultSidebarVisible}
	}
	if state.Theme == "" {
		state.Theme = "nord"
	}
	if state.LastSession == "" {
		state.LastSession = "tui:main"
	}
	if state.SidebarVisible == nil {
		state.SidebarVisible = &defaultSidebarVisible
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
