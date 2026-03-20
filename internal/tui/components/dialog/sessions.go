package dialog

import (
	"fmt"

	tea "charm.land/bubbletea/v2"
	"github.com/Nomadcxx/nanobot-go/internal/tui/client"
)

type SessionChosenMsg struct {
	Key string
}

type SessionNewMsg struct{}

type SessionResetMsg struct {
	Key string
}

type CloseDialogMsg struct{}

type SessionsModel struct {
	selectorState
}

func NewSessions(sessions []client.SessionInfo, current string) SessionsModel {
	options := make([]selectorOption, 0, len(sessions))
	for _, session := range sessions {
		label := session.Key
		if session.Key == current {
			label += " (current)"
		}
		description := session.UpdatedAt
		if description == "" {
			description = session.Preview
		}
		options = append(options, selectorOption{
			Value:       session.Key,
			Label:       label,
			Description: description,
		})
	}
	return SessionsModel{selectorState: newSelectorState("Sessions", options, "No sessions", "Enter=switch  Tab=switch  n=new  d=reset  Esc=close")}
}

func (m SessionsModel) Update(msg tea.Msg) (SessionsModel, tea.Cmd) {
	if key, ok := msg.(tea.KeyMsg); ok {
		switch key.String() {
		case "n":
			return m, func() tea.Msg { return SessionNewMsg{} }
		case "d":
			current, ok := m.current()
			if !ok {
				return m, nil
			}
			return m, func() tea.Msg { return SessionResetMsg{Key: current.Value} }
		}
	}
	switch m.update(msg) {
	case selectorActionClose:
		return m, func() tea.Msg { return CloseDialogMsg{} }
	case selectorActionSelect:
		current, ok := m.current()
		if !ok {
			return m, nil
		}
		return m, func() tea.Msg { return SessionChosenMsg{Key: current.Value} }
	}
	return m, nil
}

func (m SessionsModel) View() string { return m.view() }

func (m SessionsModel) String() string {
	return fmt.Sprintf("SessionsDialog(%d)", len(m.filtered))
}
