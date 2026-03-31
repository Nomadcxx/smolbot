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

type ConfigureProviderMsg struct {
	ProviderID string
	APIKey     string
	APIBase    string
}

type RemoveProviderMsg struct {
	ProviderID string
}

type SwitchProviderMsg struct {
	ProviderID string
}

type providerMode int

const (
	providerModeBrowse     providerMode = iota
	providerModeConfigure
	providerModeConfirming
)

type providerMetaEntry struct {
	DisplayName string
	Description string
}

var providerMeta = map[string]providerMetaEntry{
	"anthropic":    {"Anthropic", "Claude models — console.anthropic.com"},
	"openai":       {"OpenAI", "GPT & o-series — platform.openai.com"},
	"gemini":       {"Google Gemini", "Gemini models — aistudio.google.com"},
	"groq":         {"Groq", "Fast inference — console.groq.com"},
	"deepseek":     {"DeepSeek", "DeepSeek models — platform.deepseek.com"},
	"minimax":      {"MiniMax", "MiniMax models — platform.minimax.io"},
	"ollama":       {"Ollama", "Local models — no API key needed"},
}

type ProviderInfo struct {
	Name        string
	Type        string
	APIBase     string
	HasAuth     bool
	IsOAuth     bool
	IsActive    bool
	IsPartial   bool
	Description string
}

type providerRenderRow struct {
	kind      string
	section   string
	label     string
	value     string
	hasAuth   bool
	isOAuth   bool
	isActive  bool
	isPartial bool
}

type ProvidersModel struct {
	rows           []providerRenderRow
	activeProvider string
	activeModel    string
	termWidth      int

	mode           providerMode
	cursor         int
	selectableRows []int

	configTarget   string
	configAPIKey   string
	configAPIBase  string
	configFocused  int
	configError    string
	configWorking  bool

	confirmTarget string
}

func NewProviders(info []ProviderInfo, activeProvider, activeModel string) ProvidersModel {
	rows := buildProviderRows(info, activeProvider, activeModel)
	selectable := make([]int, 0)
	for i, row := range rows {
		if row.kind == "provider" {
			selectable = append(selectable, i)
		}
	}
	cursor := 0
	for i, rowIdx := range selectable {
		if rows[rowIdx].isActive {
			cursor = i
			break
		}
	}
	return ProvidersModel{
		rows:           rows,
		activeProvider: activeProvider,
		activeModel:    activeModel,
		selectableRows: selectable,
		cursor:         cursor,
	}
}

func NewProvidersFromData(models []client.ModelInfo, current string, status client.StatusPayload, cfg *cfgpkg.Config) ProvidersModel {
	currentModel := firstNonEmptyString(current, status.Model, "")
	activeProvider := providerNameForModel(models, currentModel)
	info := buildProviderInfoList(models, currentModel, activeProvider, cfg)
	rows := buildProviderRows(info, activeProvider, currentModel)

	selectable := make([]int, 0)
	for i, row := range rows {
		if row.kind == "provider" {
			selectable = append(selectable, i)
		}
	}

	cursor := 0
	for i, rowIdx := range selectable {
		if rows[rowIdx].isActive {
			cursor = i
			break
		}
	}

	return ProvidersModel{
		rows:           rows,
		activeProvider: activeProvider,
		activeModel:    currentModel,
		termWidth:      0,
		mode:           providerModeBrowse,
		cursor:         cursor,
		selectableRows: selectable,
	}
}

func buildProviderInfoList(models []client.ModelInfo, currentModel, activeProvider string, cfg *cfgpkg.Config) []ProviderInfo {
	configuredProviders := make(map[string]cfgpkg.ProviderConfig)
	if cfg != nil {
		for name, pc := range cfg.Providers {
			configuredProviders[name] = pc
		}
	}

	seen := make(map[string]bool)
	var infoList []ProviderInfo

	if activeProvider != "" {
		seen[activeProvider] = true
		pc := configuredProviders[activeProvider]
		meta := providerMeta[activeProvider]
		infoList = append(infoList, ProviderInfo{
			Name:        activeProvider,
			Type:        providerTypeName(activeProvider),
			APIBase:     pc.APIBase,
			HasAuth:     pc.APIKey != "" || pc.AuthType == "oauth",
			IsOAuth:     pc.AuthType == "oauth",
			IsActive:    true,
			Description: meta.Description,
		})
	}

	knownIDs := make([]string, 0, len(providerMeta))
	for id := range providerMeta {
		knownIDs = append(knownIDs, id)
	}
	sort.Strings(knownIDs)
	for _, name := range knownIDs {
		if seen[name] {
			continue
		}
		seen[name] = true
		pc := configuredProviders[name]
		meta := providerMeta[name]
		hasAuth := pc.APIKey != "" || pc.AuthType == "oauth"
		isOAuth := pc.AuthType == "oauth"
		isPartial := !hasAuth && pc.APIBase == ""
		infoList = append(infoList, ProviderInfo{
			Name:        name,
			Type:        providerTypeName(name),
			APIBase:     pc.APIBase,
			HasAuth:     hasAuth,
			IsOAuth:     isOAuth,
			IsPartial:   isPartial,
			Description: meta.Description,
		})
	}

	extras := make([]string, 0)
	for name := range configuredProviders {
		if !seen[name] {
			extras = append(extras, name)
		}
	}
	sort.Strings(extras)
	for _, name := range extras {
		pc := configuredProviders[name]
		hasAuth := pc.APIKey != "" || pc.AuthType == "oauth"
		infoList = append(infoList, ProviderInfo{
			Name:    name,
			Type:    providerTypeName(name),
			APIBase: pc.APIBase,
			HasAuth: hasAuth,
			IsOAuth: pc.AuthType == "oauth",
		})
	}

	return infoList
}

func providerTypeName(providerID string) string {
	switch providerID {
	case "anthropic":
		return "Anthropic"
	case "openai":
		return "OpenAI Compatible"
	case "azure_openai":
		return "Azure OpenAI"
	case "ollama":
		return "Ollama"
	case "openrouter":
		return "OpenRouter"
	case "deepseek":
		return "DeepSeek"
	case "groq":
		return "Groq"
	case "gemini":
		return "Google Gemini"
	case "minimax":
		return "MiniMax"
	case "minimax-portal":
		return "MiniMax OAuth"
	}
	return "OpenAI Compatible"
}

func buildProviderRows(info []ProviderInfo, activeProvider, activeModel string) []providerRenderRow {
	rows := []providerRenderRow{}

	hasActiveSection := false
	for _, p := range info {
		if p.IsActive {
			hasActiveSection = true
			break
		}
	}

	if hasActiveSection {
		rows = append(rows, providerRenderRow{
			kind:    "section",
			section: "Active",
		})
		for _, p := range info {
			if !p.IsActive {
				continue
			}
			rows = append(rows, providerRenderRow{
				kind:     "provider",
				label:    "Provider",
				value:    p.Name,
				hasAuth:  p.HasAuth,
				isOAuth:  p.IsOAuth,
				isActive: true,
			})
			rows = append(rows, providerRenderRow{
				kind:  "info",
				label: "Type",
				value: p.Type,
			})
			rows = append(rows, providerRenderRow{
				kind:  "info",
				label: "Model",
				value: activeModel,
			})
			if p.APIBase != "" {
				rows = append(rows, providerRenderRow{
					kind:  "info",
					label: "API Base",
					value: p.APIBase,
				})
			}
			authStatus := "Not configured"
			if p.IsOAuth {
				authStatus = "OAuth"
			} else if p.HasAuth {
				authStatus = "API Key"
			}
			rows = append(rows, providerRenderRow{
				kind:  "info",
				label: "Auth",
				value: authStatus,
			})
		}
	}

	configuredProviders := []ProviderInfo{}
	unconfiguredProviders := []ProviderInfo{}
	for _, p := range info {
		if p.IsActive {
			continue
		}
		if p.HasAuth || p.APIBase != "" {
			configuredProviders = append(configuredProviders, p)
		} else {
			unconfiguredProviders = append(unconfiguredProviders, p)
		}
	}

	if len(configuredProviders) > 0 {
		rows = append(rows, providerRenderRow{
			kind:    "section",
			section: "Configured",
		})
		for _, p := range configuredProviders {
			authStatus := "Not configured"
			if p.IsOAuth {
				authStatus = "OAuth"
			} else if p.HasAuth {
				authStatus = "API Key"
			}
			rows = append(rows, providerRenderRow{
				kind:      "provider",
				label:     p.Name,
				value:     p.Type,
				hasAuth:   p.HasAuth,
				isOAuth:   p.IsOAuth,
				isPartial: p.IsPartial,
			})
			if p.APIBase != "" {
				rows = append(rows, providerRenderRow{
					kind:  "info",
					label: "API Base",
					value: p.APIBase,
				})
			}
			rows = append(rows, providerRenderRow{
				kind:  "info",
				label: "Auth",
				value: authStatus,
			})
		}
	}

	if len(unconfiguredProviders) > 0 {
		rows = append(rows, providerRenderRow{
			kind:    "section",
			section: "Not Configured",
		})
		for _, p := range unconfiguredProviders {
			rows = append(rows, providerRenderRow{
				kind:      "provider",
				label:     p.Name,
				value:     p.Type,
				isPartial: true,
			})
		}
	}

	return rows
}

func firstNonEmptyString(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func providerNameForModel(models []client.ModelInfo, current string) string {
	for _, model := range models {
		if model.ID == current {
			return model.Provider
		}
	}
	return ""
}

func (m ProvidersModel) Update(msg tea.Msg) (ProvidersModel, tea.Cmd) {
	switch m.mode {
	case providerModeBrowse:
		return m.updateBrowse(msg)
	case providerModeConfigure:
		return m.updateConfigure(msg)
	case providerModeConfirming:
		return m.updateConfirm(msg)
	}
	return m, nil
}

func (m ProvidersModel) updateBrowse(msg tea.Msg) (ProvidersModel, tea.Cmd) {
	key, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}
	switch key.String() {
	case "up", "k", "ctrl+p":
		if len(m.selectableRows) > 0 {
			m.cursor = (m.cursor - 1 + len(m.selectableRows)) % len(m.selectableRows)
		}
	case "down", "j", "ctrl+n":
		if len(m.selectableRows) > 0 {
			m.cursor = (m.cursor + 1) % len(m.selectableRows)
		}
	case "enter":
		return m.handleBrowseEnter()
	case "d":
		return m.handleBrowseDelete()
	case "esc":
		return m, func() tea.Msg { return CloseDialogMsg{} }
	}
	return m, nil
}

func (m ProvidersModel) handleBrowseEnter() (ProvidersModel, tea.Cmd) {
	if len(m.selectableRows) == 0 {
		return m, nil
	}
	rowIdx := m.selectableRows[m.cursor]
	row := m.rows[rowIdx]
	name := providerIDForRow(row)
	if row.isActive {
		return m, func() tea.Msg { return SwitchProviderMsg{ProviderID: m.activeProvider} }
	}
	if row.hasAuth || row.isOAuth {
		return m, func() tea.Msg { return SwitchProviderMsg{ProviderID: name} }
	}
	m.mode = providerModeConfigure
	m.configTarget = name
	m.configAPIKey = ""
	m.configAPIBase = defaultAPIBase(name)
	m.configFocused = 0
	m.configError = ""
	m.configWorking = false
	return m, nil
}

func (m ProvidersModel) handleBrowseDelete() (ProvidersModel, tea.Cmd) {
	if len(m.selectableRows) == 0 {
		return m, nil
	}
	rowIdx := m.selectableRows[m.cursor]
	row := m.rows[rowIdx]
	if row.isActive {
		return m, nil
	}
	name := providerIDForRow(row)
	if !row.hasAuth && !row.isOAuth {
		return m, nil
	}
	m.mode = providerModeConfirming
	m.confirmTarget = name
	return m, nil
}

func (m ProvidersModel) updateConfigure(msg tea.Msg) (ProvidersModel, tea.Cmd) {
	key, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}
	switch key.String() {
	case "esc":
		m.mode = providerModeBrowse
		m.configTarget = ""
		m.configAPIKey = ""
		m.configAPIBase = ""
		m.configError = ""
		m.configWorking = false
	case "tab", "shift+tab":
		if m.configTarget == "openai" {
			m.configFocused = 1 - m.configFocused
		}
	case "backspace":
		if m.configFocused == 0 && len(m.configAPIKey) > 0 {
			runes := []rune(m.configAPIKey)
			m.configAPIKey = string(runes[:len(runes)-1])
		} else if m.configFocused == 1 && len(m.configAPIBase) > 0 {
			runes := []rune(m.configAPIBase)
			m.configAPIBase = string(runes[:len(runes)-1])
		}
		m.configError = ""
	case "enter":
		if m.configWorking {
			return m, nil
		}
		if strings.TrimSpace(m.configAPIKey) == "" {
			m.configError = "API key is required"
			return m, nil
		}
		m.configWorking = true
		providerID := m.configTarget
		apiKey := strings.TrimSpace(m.configAPIKey)
		apiBase := strings.TrimSpace(m.configAPIBase)
		return m, func() tea.Msg {
			return ConfigureProviderMsg{
				ProviderID: providerID,
				APIKey:     apiKey,
				APIBase:    apiBase,
			}
		}
	default:
		if r := []rune(key.String()); len(r) == 1 && r[0] >= 32 {
			if m.configFocused == 0 {
				m.configAPIKey += string(r)
			} else {
				m.configAPIBase += string(r)
			}
			m.configError = ""
		}
	}
	return m, nil
}

func (m ProvidersModel) updateConfirm(msg tea.Msg) (ProvidersModel, tea.Cmd) {
	key, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}
	switch key.String() {
	case "y", "enter":
		target := m.confirmTarget
		m.mode = providerModeBrowse
		m.confirmTarget = ""
		return m, func() tea.Msg { return RemoveProviderMsg{ProviderID: target} }
	case "n", "esc":
		m.mode = providerModeBrowse
		m.confirmTarget = ""
	}
	return m, nil
}

func (m ProvidersModel) WithConfigureResult(err error) ProvidersModel {
	m.configWorking = false
	if err != nil {
		m.configError = err.Error()
		return m
	}
	m.mode = providerModeBrowse
	m.configTarget = ""
	m.configAPIKey = ""
	m.configAPIBase = ""
	m.configError = ""
	return m
}

func providerIDForRow(row providerRenderRow) string {
	if row.isActive {
		return row.value
	}
	return row.label
}

func defaultAPIBase(providerID string) string {
	switch providerID {
	case "openai":
		return "https://api.openai.com/v1"
	default:
		return ""
	}
}

func maskAPIKey(key string) string {
	runes := []rune(key)
	if len(runes) <= 8 {
		return strings.Repeat("*", len(runes))
	}
	return strings.Repeat("*", len(runes)-4) + string(runes[len(runes)-4:])
}

func (m ProvidersModel) View() string {
	t := theme.Current()
	if t == nil {
		return "providers"
	}
	var content string
	switch m.mode {
	case providerModeBrowse:
		content = m.viewBrowse(t)
	case providerModeConfigure:
		content = m.viewConfigure(t)
	case providerModeConfirming:
		content = m.viewConfirm(t)
	}
	return lipgloss.NewStyle().
		Background(t.Panel).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(t.BorderFocus).
		Padding(1, 2).
		Width(dialogWidth(m.termWidth, 72)).
		Render(content)
}

func (m ProvidersModel) viewBrowse(t *theme.Theme) string {
	lines := []string{
		lipgloss.NewStyle().Foreground(t.Primary).Bold(true).Render("Providers"),
		"",
	}
	if len(m.rows) == 0 {
		lines = append(lines, lipgloss.NewStyle().Foreground(t.TextMuted).Italic(true).Render("No providers found"))
	} else {
		selectedRowIdx := -1
		if len(m.selectableRows) > 0 && m.cursor < len(m.selectableRows) {
			selectedRowIdx = m.selectableRows[m.cursor]
		}
		for i, row := range m.rows {
			isCursor := i == selectedRowIdx
			lines = append(lines, m.renderRowBrowse(row, isCursor, t)...)
		}
	}
	lines = append(lines, "",
		lipgloss.NewStyle().Foreground(t.TextMuted).Render("↑/↓ navigate • Enter select/configure • d remove • Esc close"),
	)
	return strings.Join(lines, "\n")
}

func (m ProvidersModel) renderRowBrowse(row providerRenderRow, isCursor bool, t *theme.Theme) []string {
	switch row.kind {
	case "section":
		style := lipgloss.NewStyle().Foreground(t.Primary).Bold(true)
		return []string{"", style.Render(row.section)}
	case "provider":
		prefix := "  "
		if isCursor {
			prefix = "> "
		}
		name := row.label
		if row.isActive {
			name = row.value + " (active)"
		}
		var style lipgloss.Style
		switch {
		case row.isActive:
			style = lipgloss.NewStyle().Foreground(t.Success).Bold(true)
		case row.isPartial || (!row.hasAuth && !row.isOAuth):
			style = lipgloss.NewStyle().Foreground(t.TextMuted)
		default:
			style = lipgloss.NewStyle().Foreground(t.Text)
		}
		hint := ""
		if isCursor && !row.isActive && !row.hasAuth && !row.isOAuth {
			hint = "  " + lipgloss.NewStyle().Foreground(t.TextMuted).Italic(true).Render("Press Enter to configure")
		}
		return []string{prefix + style.Render(name) + hint}
	default:
		labelStyle := lipgloss.NewStyle().Foreground(t.TextMuted)
		valueStyle := lipgloss.NewStyle().Foreground(t.Text)
		return []string{"  " + labelStyle.Render(row.label+":") + " " + valueStyle.Render(row.value)}
	}
}

func (m ProvidersModel) viewConfigure(t *theme.Theme) string {
	meta := providerMeta[m.configTarget]
	displayName := meta.DisplayName
	if displayName == "" {
		displayName = m.configTarget
	}
	lines := []string{
		lipgloss.NewStyle().Foreground(t.Primary).Bold(true).Render("Configure " + displayName),
		"",
	}
	if meta.Description != "" {
		lines = append(lines, lipgloss.NewStyle().Foreground(t.TextMuted).Render(meta.Description), "")
	}

	keyLabel := lipgloss.NewStyle().Foreground(t.Text).Render("API Key: ")
	keyValue := maskAPIKey(m.configAPIKey)
	if m.configFocused == 0 {
		keyValue += "█"
	}
	keyStyle := lipgloss.NewStyle().Foreground(t.Primary)
	if m.configFocused == 0 {
		keyStyle = keyStyle.Bold(true)
	}
	lines = append(lines, keyLabel+keyStyle.Render(keyValue))

	if m.configTarget == "openai" {
		baseLabel := lipgloss.NewStyle().Foreground(t.Text).Render("API Base: ")
		baseValue := m.configAPIBase
		if m.configFocused == 1 {
			baseValue += "█"
		}
		baseStyle := lipgloss.NewStyle().Foreground(t.Primary)
		if m.configFocused == 1 {
			baseStyle = baseStyle.Bold(true)
		}
		lines = append(lines, baseLabel+baseStyle.Render(baseValue))
	}

	if m.configWorking {
		lines = append(lines, "", lipgloss.NewStyle().Foreground(t.TextMuted).Italic(true).Render("Configuring..."))
	} else if m.configError != "" {
		lines = append(lines, "", lipgloss.NewStyle().Foreground(t.Error).Render("✗ "+m.configError))
	}

	lines = append(lines, "",
		lipgloss.NewStyle().Foreground(t.TextMuted).Render("Enter submit • Esc cancel • Tab toggle field"),
	)
	return strings.Join(lines, "\n")
}

func (m ProvidersModel) viewConfirm(t *theme.Theme) string {
	meta := providerMeta[m.confirmTarget]
	displayName := meta.DisplayName
	if displayName == "" {
		displayName = m.confirmTarget
	}
	lines := []string{
		lipgloss.NewStyle().Foreground(t.Primary).Bold(true).Render("Remove " + displayName + "?"),
		"",
		lipgloss.NewStyle().Foreground(t.Text).Render("This will remove the API key for " + displayName + "."),
		"",
		lipgloss.NewStyle().Foreground(t.TextMuted).Render("y/Enter remove • n/Esc cancel"),
	}
	return strings.Join(lines, "\n")
}

func (m ProvidersModel) WithTerminalWidth(w int) ProvidersModel {
	m.termWidth = w
	return m
}

func (m ProvidersModel) renderRow(row providerRenderRow, t *theme.Theme) []string {
	switch row.kind {
	case "section":
		label := row.section
		style := lipgloss.NewStyle().Foreground(t.Primary).Bold(true)
		return []string{style.Render(label)}

	case "provider":
		label := row.label
		value := row.value
		if row.isActive {
			label += " (active)"
		}
		if row.isPartial {
			value += " incomplete"
		}

		providerStyle := lipgloss.NewStyle().Foreground(t.Primary).Bold(false)
		if row.isActive {
			providerStyle = providerStyle.Foreground(t.Success).Bold(true)
		} else if row.isPartial {
			providerStyle = providerStyle.Foreground(t.Warning)
		} else if row.isOAuth {
			providerStyle = providerStyle.Foreground(t.Primary).Bold(true)
		}

		valueStyle := lipgloss.NewStyle().Foreground(t.TextMuted)
		if row.hasAuth {
			valueStyle = valueStyle.Foreground(t.Success)
		}
		if row.isOAuth {
			valueStyle = valueStyle.Foreground(t.Primary)
		}

		return []string{providerStyle.Render(label), valueStyle.Render("  " + value)}

	default:
		labelStyle := lipgloss.NewStyle().Foreground(t.TextMuted)
		valueStyle := lipgloss.NewStyle().Foreground(t.Text)
		return []string{labelStyle.Render(row.label + ":"), valueStyle.Render(" " + row.value)}
	}
}
