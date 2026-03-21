package dialog

import (
	"reflect"
	"strings"

	tea "charm.land/bubbletea/v2"
	lipgloss "charm.land/lipgloss/v2"
	"github.com/Nomadcxx/smolbot/internal/client"
	"github.com/Nomadcxx/smolbot/internal/theme"
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
		case "up", "k", "ctrl+p":
			if m.cursor > 0 {
				m.cursor--
			}
			return m, nil
		case "down", "j", "ctrl+n":
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

	start, end := visibleBounds(len(m.filtered), m.cursor)
	if start > 0 {
		lines = append(lines, lipgloss.NewStyle().Foreground(t.TextMuted).Render("▲ more above"))
	}
	for i := start; i < end; i++ {
		model := m.filtered[i]
		prefix := "  "
		if i == m.cursor {
			prefix = "› "
		}
		label := model.Name
		if label == "" {
			label = model.ID
		}
		descParts := []string{}
		if model.ID != label {
			descParts = append(descParts, model.ID)
		}
		if model.Provider != "" {
			descParts = append(descParts, model.Provider)
		}
		if extra := optionalModelDescription(model); extra != "" {
			descParts = append(descParts, extra)
		}
		if model.ID == m.current {
			label += lipgloss.NewStyle().Foreground(t.Success).Bold(true).Render(" current")
		}
		row := prefix + label
		if len(descParts) > 0 {
			row += lipgloss.NewStyle().Foreground(t.TextMuted).Render("  " + strings.Join(descParts, " • "))
		}
		if i == m.cursor {
			row = lipgloss.NewStyle().Foreground(t.Primary).Bold(true).Render(row)
		}
		lines = append(lines, row)
	}
	if len(m.filtered) == 0 {
		lines = append(lines, lipgloss.NewStyle().Foreground(t.TextMuted).Italic(true).Render("No models"))
	}
	if end < len(m.filtered) {
		lines = append(lines, lipgloss.NewStyle().Foreground(t.TextMuted).Render("▼ more below"))
	}
	lines = append(lines, "", lipgloss.NewStyle().Foreground(t.TextMuted).Render("Enter switch • Esc close"))
	return lipgloss.NewStyle().
		Background(t.Panel).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(t.BorderFocus).
		Padding(1, 2).
		Width(64).
		Render(strings.Join(lines, "\n"))
}

func (m *ModelsModel) applyFilter() {
	m.filtered = m.filtered[:0]
	for _, model := range m.models {
		if matchesQuery(m.filter, model.ID, model.Name, model.Provider, optionalModelDescription(model)) {
			m.filtered = append(m.filtered, model)
		}
	}
	if m.cursor >= len(m.filtered) {
		m.cursor = max(0, len(m.filtered)-1)
	}
}

func optionalModelDescription(value any) string {
	v := reflect.ValueOf(value)
	if !v.IsValid() {
		return ""
	}
	if v.Kind() == reflect.Pointer {
		if v.IsNil() {
			return ""
		}
		v = v.Elem()
	}
	if v.Kind() != reflect.Struct {
		return ""
	}
	field := v.FieldByName("Description")
	if !field.IsValid() || field.Kind() != reflect.String {
		return ""
	}
	return field.String()
}
