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
	models          []client.ModelInfo
	filtered        []client.ModelInfo
	filter          string
	cursor          int
	current         string
	currentProvider string
}

func NewModels(models []client.ModelInfo, current string) ModelsModel {
	m := ModelsModel{models: models, current: current}
	for _, model := range models {
		if model.ID == current {
			m.currentProvider = model.Provider
			break
		}
	}
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

	rows := m.renderRows(t)
	if len(rows) == 0 {
		lines = append(lines, lipgloss.NewStyle().Foreground(t.TextMuted).Italic(true).Render("No models"))
	} else {
		cursorRow := m.cursorRow(rows)
		start, end := visibleBounds(len(rows), cursorRow)
		if start > 0 {
			lines = append(lines, lipgloss.NewStyle().Foreground(t.TextMuted).Render("▲ more above"))
		}
		for i := start; i < end; i++ {
			lines = append(lines, rows[i].render(t))
		}
		if end < len(rows) {
			lines = append(lines, lipgloss.NewStyle().Foreground(t.TextMuted).Render("▼ more below"))
		}
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

type modelRenderRow struct {
	kind    string
	model   client.ModelInfo
	group   string
	current bool
	active  bool
}

func (m ModelsModel) renderRows(t *theme.Theme) []modelRenderRow {
	if len(m.filtered) == 0 {
		return nil
	}
	grouped := make(map[string][]client.ModelInfo)
	order := make([]string, 0)
	for _, model := range m.filtered {
		group := model.Provider
		if group == "" {
			group = "unknown"
		}
		if _, ok := grouped[group]; !ok {
			order = append(order, group)
		}
		grouped[group] = append(grouped[group], model)
	}

	rows := make([]modelRenderRow, 0, len(m.filtered)+len(order))
	for _, group := range order {
		rows = append(rows, modelRenderRow{
			kind:    "header",
			group:   group,
			current: group == m.currentProvider,
		})
		for _, model := range grouped[group] {
			rows = append(rows, modelRenderRow{
				kind:   "model",
				model:  model,
				group:  group,
				active: model.ID == m.current,
			})
		}
	}
	return rows
}

func (m ModelsModel) cursorRow(rows []modelRenderRow) int {
	modelIdx := 0
	for i, row := range rows {
		if row.kind != "model" {
			continue
		}
		if modelIdx == m.cursor {
			return i
		}
		modelIdx++
	}
	return 0
}

func (r modelRenderRow) render(t *theme.Theme) string {
	switch r.kind {
	case "header":
		label := "Provider: " + r.group
		if r.current {
			label += " (current)"
		}
		style := lipgloss.NewStyle().Foreground(t.Primary).Bold(true)
		if r.current {
			style = style.Foreground(t.Success)
		}
		return style.Render(label)
	default:
		model := r.model
		label := model.Name
		if label == "" {
			label = model.ID
		}
		if model.Provider != "" {
			label = "[" + model.Provider + "] " + label
		}
		descParts := []string{}
		if model.ID != label {
			descParts = append(descParts, model.ID)
		}
		if extra := optionalModelDescription(model); extra != "" {
			descParts = append(descParts, extra)
		}
		if r.active {
			label += lipgloss.NewStyle().Foreground(t.Success).Bold(true).Render(" current")
		}
		row := "  " + label
		if len(descParts) > 0 {
			row += lipgloss.NewStyle().Foreground(t.TextMuted).Render("  " + strings.Join(descParts, " • "))
		}
		return row
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
