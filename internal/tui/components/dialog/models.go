package dialog

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/Nomadcxx/nanobot-go/internal/tui/client"
)

type ModelChosenMsg struct {
	ID string
}

type ModelsModel struct {
	selectorState
}

func NewModels(models []client.ModelInfo, current string) ModelsModel {
	options := make([]selectorOption, 0, len(models))
	for _, model := range models {
		label := model.Name
		if label == "" {
			label = model.ID
		}
		if model.ID == current {
			label += " (current)"
		}
		description := fmt.Sprintf("%s via %s", model.ID, model.Provider)
		description = strings.TrimSuffix(strings.TrimSpace(description), " via")
		options = append(options, selectorOption{
			Value:       model.ID,
			Label:       label,
			Description: description,
		})
	}
	return ModelsModel{selectorState: newSelectorState("Models", options, "No models", "Enter=switch  Tab=switch  Esc=close")}
}

func (m ModelsModel) Update(msg tea.Msg) (ModelsModel, tea.Cmd) {
	switch m.update(msg) {
	case selectorActionClose:
		return m, func() tea.Msg { return CloseDialogMsg{} }
	case selectorActionSelect:
		current, ok := m.current()
		if !ok {
			return m, nil
		}
		return m, func() tea.Msg { return ModelChosenMsg{ID: current.Value} }
	}
	return m, nil
}

func (m ModelsModel) View() string { return m.view() }
