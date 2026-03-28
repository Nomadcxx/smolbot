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
	rows            []modelRenderRow
	selectable      []int
	filter          string
	cursor          int
	current         string
	currentProvider string
	pending         string
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
	if idx := m.indexOfSelectableID(current); idx >= 0 {
		m.cursor = idx
	}
	return m
}

func (m ModelsModel) Update(msg tea.Msg) (ModelsModel, tea.Cmd) {
	if key, ok := msg.(tea.KeyMsg); ok {
		switch key.String() {
		case "esc":
			return m, func() tea.Msg { return CloseDialogMsg{} }
		case "up", "k", "ctrl+p":
			if len(m.selectable) > 0 && m.cursor > 0 {
				m.cursor--
			}
			return m, nil
		case "down", "j", "ctrl+n":
			if len(m.selectable) > 0 && m.cursor < len(m.selectable)-1 {
				m.cursor++
			}
			return m, nil
		case " ", "space":
			if focused, ok := m.focusedModel(); ok {
				m.pending = focused.ID
			}
			return m, nil
		case "enter":
			if m.pending != "" {
				id := m.pending
				return m, func() tea.Msg { return ModelChosenMsg{ID: id} }
			}
			focused, ok := m.focusedModel()
			if !ok {
				return m, nil
			}
			id := focused.ID
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

	rows := m.renderRows()
	if len(rows) == 0 {
		lines = append(lines, lipgloss.NewStyle().Foreground(t.TextMuted).Italic(true).Render("No models"))
	} else {
		cursorRow := m.focusedRowIndex()
		if cursorRow < 0 {
			cursorRow = 0
		}
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
	lines = append(lines, "", lipgloss.NewStyle().Foreground(t.TextMuted).Render("Type filter • Space mark • Enter save • Esc close"))
	return lipgloss.NewStyle().
		Background(t.Panel).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(t.BorderFocus).
		Padding(1, 2).
		Width(64).
		Render(strings.Join(lines, "\n"))
}

func (m *ModelsModel) applyFilter() {
	focusedID := m.focusedModelID()
	filtered := make([]client.ModelInfo, 0, len(m.models))
	for _, model := range m.models {
		if matchesQuery(m.filter, model.ID, model.Name, model.Provider, optionalModelDescription(model)) {
			filtered = append(filtered, model)
		}
	}
	m.rows = buildModelRows(filtered, m.currentProvider)
	m.selectable = m.selectable[:0]
	for i, row := range m.rows {
		if row.kind == "model" {
			m.selectable = append(m.selectable, i)
		}
	}
	if m.pending != "" && m.indexOfSelectableID(m.pending) == -1 {
		m.pending = ""
	}
	if len(m.selectable) == 0 {
		m.cursor = 0
		return
	}
	if idx := m.indexOfSelectableID(focusedID); idx >= 0 {
		m.cursor = idx
		return
	}
	if idx := m.indexOfSelectableID(m.current); idx >= 0 {
		m.cursor = idx
		return
	}
	if m.cursor >= len(m.selectable) {
		m.cursor = len(m.selectable) - 1
	}
}

type modelRenderRow struct {
	kind    string
	model   client.ModelInfo
	group   string
	current bool
	focused bool
	pending bool
}

func buildModelRows(models []client.ModelInfo, currentProvider string) []modelRenderRow {
	grouped := make(map[string][]client.ModelInfo)
	order := make([]string, 0)
	for _, model := range models {
		group := model.Provider
		if group == "" {
			group = "unknown"
		}
		if _, ok := grouped[group]; !ok {
			order = append(order, group)
		}
		grouped[group] = append(grouped[group], model)
	}

	rows := make([]modelRenderRow, 0, len(models)+len(order))
	for _, group := range order {
		rows = append(rows, modelRenderRow{
			kind:    "header",
			group:   group,
			current: group == currentProvider,
		})
		for _, model := range grouped[group] {
			kind := "info"
			if isSelectableModel(model) {
				kind = "model"
			}
			rows = append(rows, modelRenderRow{
				kind:  kind,
				model: model,
				group: group,
			})
		}
	}
	return rows
}

func (m ModelsModel) renderRows() []modelRenderRow {
	if len(m.rows) == 0 {
		return nil
	}
	rows := make([]modelRenderRow, len(m.rows))
	copy(rows, m.rows)
	focusedRow := m.focusedRowIndex()
	for i := range rows {
		if rows[i].kind == "header" {
			continue
		}
		rows[i].current = rows[i].model.ID == m.current
		rows[i].pending = rows[i].model.ID != "" && rows[i].model.ID == m.pending
		rows[i].focused = i == focusedRow
	}
	return rows
}

func (m ModelsModel) focusedRowIndex() int {
	if len(m.selectable) == 0 || m.cursor < 0 || m.cursor >= len(m.selectable) {
		return -1
	}
	return m.selectable[m.cursor]
}

func (m ModelsModel) focusedModelID() string {
	if focused, ok := m.focusedModel(); ok {
		return focused.ID
	}
	return ""
}

func (m ModelsModel) focusedModel() (client.ModelInfo, bool) {
	rowIndex := m.focusedRowIndex()
	if rowIndex < 0 || rowIndex >= len(m.rows) {
		return client.ModelInfo{}, false
	}
	row := m.rows[rowIndex]
	if row.kind != "model" {
		return client.ModelInfo{}, false
	}
	return row.model, true
}

func (m ModelsModel) indexOfSelectableID(id string) int {
	if id == "" {
		return -1
	}
	for idx, rowIndex := range m.selectable {
		if rowIndex >= 0 && rowIndex < len(m.rows) && m.rows[rowIndex].model.ID == id {
			return idx
		}
	}
	return -1
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
		label := strings.TrimSpace(model.Name)
		if label == "" {
			label = strings.TrimSpace(model.ID)
		}
		descParts := []string{}
		if r.kind == "info" {
			label = strings.TrimSpace(label + " configured provider")
		}
		if model.ID != "" && model.ID != label {
			descParts = append(descParts, model.ID)
		}
		if extra := optionalModelDescription(model); extra != "" {
			descParts = append(descParts, extra)
		}
		if r.current {
			label += lipgloss.NewStyle().Foreground(t.Success).Bold(true).Render(" current")
		}
		if r.pending {
			label += lipgloss.NewStyle().Foreground(t.Primary).Bold(true).Render(" pending")
		}
		if r.kind == "info" {
			label += lipgloss.NewStyle().Foreground(t.TextMuted).Render(" info")
		}
		prefix := "  "
		if r.focused {
			prefix = lipgloss.NewStyle().Foreground(t.Primary).Bold(true).Render("> ")
		}
		row := prefix + label
		if len(descParts) > 0 {
			row += lipgloss.NewStyle().Foreground(t.TextMuted).Render("  " + strings.Join(descParts, " • "))
		}
		if r.kind == "info" {
			row = lipgloss.NewStyle().Foreground(t.TextMuted).Render(row)
		}
		return row
	}
}

func isSelectableModel(model client.ModelInfo) bool {
	if model.Selectable {
		return true
	}
	return model.Source != "config"
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
