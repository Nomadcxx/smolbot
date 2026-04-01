package dialog

import (
	"sort"
	"strings"

	tea "charm.land/bubbletea/v2"
	lipgloss "charm.land/lipgloss/v2"
	"github.com/Nomadcxx/smolbot/internal/client"
	"github.com/Nomadcxx/smolbot/internal/theme"
	cfgpkg "github.com/Nomadcxx/smolbot/pkg/config"
)

type OAuthProviderFilter struct {
	MinimaxPortalIsOAuth bool
}

func NewModels(providerConfig *cfgpkg.Config, models []client.ModelInfo, current string) ModelsModel {
	return NewModelsWithState(providerConfig, models, current, nil, nil)
}

func NewModelsWithState(providerConfig *cfgpkg.Config, models []client.ModelInfo, current string, favorites []string, recents []string) ModelsModel {
	m := ModelsModel{
		models:    models,
		current:   current,
		favorites: append([]string(nil), favorites...),
		recents:   append([]string(nil), recents...),
	}
	for _, model := range models {
		if model.ID == current {
			m.currentProvider = model.Provider
			break
		}
	}
	m.oauthFilter = buildOAuthFilter(providerConfig)
	m.applyFilter()
	if idx := m.indexOfSelectableID(current); idx >= 0 {
		m.cursor = idx
	}
	return m
}

func buildOAuthFilter(cfg *cfgpkg.Config) OAuthProviderFilter {
	if cfg == nil || cfg.Providers == nil {
		return OAuthProviderFilter{}
	}
	portal, ok := cfg.Providers["minimax-portal"]
	if !ok {
		return OAuthProviderFilter{}
	}
	return OAuthProviderFilter{MinimaxPortalIsOAuth: portal.AuthType == "oauth"}
}

type ModelChosenMsg struct {
	ID string
}

type ModelFavoriteToggledMsg struct {
	ID string
}

type ModelRemovedFromRecentsMsg struct {
	ID string
}

type RequestProviderAddMsg struct{}

type ModelsModel struct {
	models          []client.ModelInfo
	rows            []modelRenderRow
	selectable      []int
	filter          string
	cursor          int
	current         string
	currentProvider string
	pending         string
	oauthFilter     OAuthProviderFilter
	termWidth       int
	favorites       []string
	recents         []string
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
		case "ctrl+f":
			if focused, ok := m.focusedModel(); ok {
				id := focused.ID
				return m, func() tea.Msg { return ModelFavoriteToggledMsg{ID: id} }
			}
			return m, nil
		case "ctrl+x":
			if focused, ok := m.focusedModel(); ok {
				id := focused.ID
				return m, func() tea.Msg { return ModelRemovedFromRecentsMsg{ID: id} }
			}
			return m, nil
		case "ctrl+a":
			return m, func() tea.Msg { return RequestProviderAddMsg{} }
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
	lines = append(lines, "", lipgloss.NewStyle().Foreground(t.TextMuted).Render("Type filter • Ctrl+F fav • Ctrl+X remove • Ctrl+A add provider • Space mark • Enter save • Esc close"))
	return lipgloss.NewStyle().
		Background(t.Panel).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(t.BorderFocus).
		Padding(1, 2).
		Width(dialogWidth(m.termWidth, 64)).
		Render(strings.Join(lines, "\n"))
}

func (m *ModelsModel) applyFilter() {
	focusedID := m.focusedModelID()
	filtered := make([]client.ModelInfo, 0, len(m.models))
	for _, model := range m.models {
		if !matchesQuery(m.filter, model.ID, model.Name, model.Provider, optionalModelDescription(model)) {
			continue
		}
		if m.oauthFilter.MinimaxPortalIsOAuth && model.Provider == "minimax" {
			continue
		}
		filtered = append(filtered, model)
	}
	m.rows = buildModelRows(filtered, m.currentProvider, m.favorites, m.recents)
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
	kind       string
	model      client.ModelInfo
	group      string
	current    bool
	focused    bool
	pending    bool
	isFavorite bool
}

func buildModelRows(models []client.ModelInfo, currentProvider string, favorites []string, recents []string) []modelRenderRow {
	favSet := make(map[string]bool, len(favorites))
	for _, id := range favorites {
		favSet[id] = true
	}

	modelByID := make(map[string]client.ModelInfo, len(models))
	for _, m := range models {
		modelByID[m.ID] = m
	}

	shown := make(map[string]bool)
	rows := make([]modelRenderRow, 0, len(models)+4)

	// Favorites section
	favRows := make([]modelRenderRow, 0)
	for _, id := range favorites {
		m, ok := modelByID[id]
		if !ok {
			continue
		}
		kind := "info"
		if isSelectableModel(m) {
			kind = "model"
		}
		favRows = append(favRows, modelRenderRow{kind: kind, model: m, group: "Favorites", isFavorite: true})
		shown[id] = true
	}
	if len(favRows) > 0 {
		rows = append(rows, modelRenderRow{kind: "header", group: "Favorites"})
		rows = append(rows, favRows...)
	}

	// Recents section (exclude those already shown in favorites)
	recentRows := make([]modelRenderRow, 0)
	for _, id := range recents {
		if shown[id] {
			continue
		}
		m, ok := modelByID[id]
		if !ok {
			continue
		}
		kind := "info"
		if isSelectableModel(m) {
			kind = "model"
		}
		recentRows = append(recentRows, modelRenderRow{kind: kind, model: m, group: "Recent", isFavorite: favSet[id]})
		shown[id] = true
	}
	if len(recentRows) > 0 {
		rows = append(rows, modelRenderRow{kind: "header", group: "Recent"})
		rows = append(rows, recentRows...)
	}

	// Provider groups — skip models already shown
	grouped := make(map[string][]client.ModelInfo)
	order := make([]string, 0)
	for _, model := range models {
		if shown[model.ID] {
			continue
		}
		group := model.Provider
		if group == "" {
			group = "unknown"
		}
		if _, ok := grouped[group]; !ok {
			order = append(order, group)
		}
		grouped[group] = append(grouped[group], model)
	}
	for _, group := range order {
		// Sort within each provider group: Free first, newer release date first, alpha fallback.
		sort.SliceStable(grouped[group], func(i, j int) bool {
			a, b := grouped[group][i], grouped[group][j]
			if a.IsFree != b.IsFree {
				return a.IsFree
			}
			if a.ReleaseDate != b.ReleaseDate {
				return a.ReleaseDate > b.ReleaseDate
			}
			return a.Name < b.Name
		})
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
				kind:       kind,
				model:      model,
				group:      group,
				isFavorite: favSet[model.ID],
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

func (m ModelsModel) WithFavorites(favorites []string) ModelsModel {
	m.favorites = append([]string(nil), favorites...)
	m.applyFilter()
	return m
}

func (m ModelsModel) WithRecents(recents []string) ModelsModel {
	m.recents = append([]string(nil), recents...)
	m.applyFilter()
	return m
}

func (r modelRenderRow) render(t *theme.Theme) string {
	switch r.kind {
	case "header":
		var label string
		style := lipgloss.NewStyle().Foreground(t.Primary).Bold(true)
		switch r.group {
		case "Favorites":
			label = style.Foreground(t.Warning).Render("★ Favorites")
		case "Recent":
			label = style.Foreground(t.TextMuted).Render("⏱ Recent")
		default:
			displayName := ProviderDisplayName(r.group)
			if r.current {
				style = style.Foreground(t.Success)
			}
			label = style.Render(displayName)
			if r.current {
				label += lipgloss.NewStyle().Foreground(t.Success).Render(" (current)")
			}
		}
		return label
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
		if model.ReleaseDate != "" {
			descParts = append(descParts, lipgloss.NewStyle().Foreground(t.TextMuted).Render(model.ReleaseDate))
		}
		if model.IsFree {
			descParts = append(descParts, lipgloss.NewStyle().Foreground(t.Success).Bold(true).Render("Free"))
		}
		if r.isFavorite {
			descParts = append(descParts, lipgloss.NewStyle().Foreground(t.Warning).Render("★"))
		}
		if r.pending {
			label += lipgloss.NewStyle().Foreground(t.Primary).Bold(true).Render(" pending")
		}
		if r.kind == "info" {
			label += lipgloss.NewStyle().Foreground(t.TextMuted).Render(" info")
		}
		cursorChar := " "
		if r.focused {
			cursorChar = lipgloss.NewStyle().Foreground(t.Primary).Bold(true).Render(">")
		}
		dotChar := " "
		if r.current {
			dotChar = lipgloss.NewStyle().Foreground(t.Success).Bold(true).Render("●")
		}
		prefix := cursorChar + " " + dotChar + " "
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

func optionalModelDescription(model client.ModelInfo) string {
	return model.Description
}

func (m ModelsModel) WithTerminalWidth(w int) ModelsModel {
	m.termWidth = w
	return m
}

