package dialog

import (
	"strings"

	tea "charm.land/bubbletea/v2"
	lipgloss "charm.land/lipgloss/v2"
	"github.com/Nomadcxx/nanobot-go/internal/client"
	"github.com/Nomadcxx/nanobot-go/internal/theme"
)

type ModelChosenMsg struct {
	ID string
}

type ModelsModel struct {
	models   []client.ModelInfo
	filtered []client.ModelInfo
	filter   string
	cursor   int
	current  string
}

func NewModels(models []client.ModelInfo, current string) ModelsModel {
	m := ModelsModel{models: models, current: current}
	m.applyFilter()
	return m
}

func (m ModelsModel) Update(msg tea.Msg) (ModelsModel, tea.Cmd) {
	if key, ok := msg.(tea.KeyMsg); ok {
		switch key.String() {
		case "esc":
			return m, func() tea.Msg { return CloseDialogMsg{} }
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
			return m, nil
		case "down", "j":
			if m.cursor < len(m.filtered)-1 {
				m.cursor++
			}
			return m, nil
		case "enter":
			if len(m.filtered) == 0 {
				return m, nil
			}
			id := m.filtered[m.cursor].ID
			return m, func() tea.Msg { return ModelChosenMsg{ID: id} }
		case "backspace":
			if len(m.filter) > 0 {
				m.filter = m.filter[:len(m.filter)-1]
				m.applyFilter()
			}
			return m, nil
		default:
			k := key.String()
			if len(k) == 1 && k >= " " {
				m.filter += k
				m.applyFilter()
			}
			return m, nil
		}
	}
	return m, nil
}

func (m ModelsModel) View() string {
	t := theme.Current()
	if t == nil {
		return "models"
	}

	lines := []string{
		lipgloss.NewStyle().Foreground(t.Primary).Bold(true).Render("Models"),
		lipgloss.NewStyle().Foreground(t.TextMuted).Render("Filter: " + m.filter),
		"",
	}
	for i, model := range m.filtered {
		prefix := "  "
		if i == m.cursor {
			prefix = "› "
		}
		label := model.Name
		if label == "" {
			label = model.ID
		}
		if model.ID == m.current {
			label += " (current)"
		}
		lines = append(lines, prefix+label)
	}
	if len(m.filtered) == 0 {
		lines = append(lines, "  No models")
	}
	lines = append(lines, "", "Enter=switch  Esc=close")
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(t.BorderFocus).
		Padding(1, 2).
		Width(48).
		Render(strings.Join(lines, "\n"))
}

func (m *ModelsModel) applyFilter() {
	m.filtered = m.filtered[:0]
	needle := strings.ToLower(m.filter)
	for _, model := range m.models {
		label := strings.ToLower(model.ID + " " + model.Name)
		if needle == "" || strings.Contains(label, needle) {
			m.filtered = append(m.filtered, model)
		}
	}
	if m.cursor >= len(m.filtered) {
		m.cursor = max(0, len(m.filtered)-1)
	}
}
