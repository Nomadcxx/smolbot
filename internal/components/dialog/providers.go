package dialog

import (
	"strings"

	tea "charm.land/bubbletea/v2"
	lipgloss "charm.land/lipgloss/v2"
	"github.com/Nomadcxx/smolbot/internal/client"
	"github.com/Nomadcxx/smolbot/internal/theme"
	cfgpkg "github.com/Nomadcxx/smolbot/pkg/config"
)

type ProviderInfo struct {
	Name      string
	Type      string
	APIBase   string
	HasAuth   bool
	IsOAuth   bool
	IsActive  bool
	IsPartial bool
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
}

func NewProviders(info []ProviderInfo, activeProvider, activeModel string) ProvidersModel {
	rows := buildProviderRows(info, activeProvider, activeModel)
	return ProvidersModel{
		rows:           rows,
		activeProvider: activeProvider,
		activeModel:    activeModel,
	}
}

func NewProvidersFromData(models []client.ModelInfo, current string, status client.StatusPayload, cfg *cfgpkg.Config) ProvidersModel {
	currentModel := firstNonEmptyString(current, status.Model, "")
	activeProvider := providerNameForModel(models, currentModel)

	info := buildProviderInfoList(models, currentModel, activeProvider, cfg)

	rows := buildProviderRows(info, activeProvider, currentModel)
	return ProvidersModel{
		rows:           rows,
		activeProvider: activeProvider,
		activeModel:    currentModel,
	}
}

func buildProviderInfoList(models []client.ModelInfo, currentModel, activeProvider string, cfg *cfgpkg.Config) []ProviderInfo {
	infoList := []ProviderInfo{}

	configuredProviders := make(map[string]cfgpkg.ProviderConfig)
	if cfg != nil {
		for name, pc := range cfg.Providers {
			configuredProviders[name] = pc
		}
	}

	providerTypes := map[string]string{
		"openai":          "OpenAI Compatible",
		"anthropic":       "Anthropic",
		"azure_openai":    "Azure OpenAI",
		"ollama":          "Ollama",
		"deepseek":        "DeepSeek",
		"groq":            "Groq",
		"minimax":         "MiniMax",
		"minimax-portal":  "MiniMax OAuth",
		"gemini":          "Google Gemini",
		"moonshot":        "Moonshot",
		"openrouter":      "OpenRouter",
		"vllm":            "vLLM",
	}

	if activeProvider != "" {
		hasAuth := false
		apiBase := ""
		isOAuth := false
		providerType := providerTypes[activeProvider]
		if providerType == "" {
			providerType = "OpenAI Compatible"
		}
		if pc, ok := configuredProviders[activeProvider]; ok {
			hasAuth = pc.APIKey != "" || pc.AuthType == "oauth"
			apiBase = pc.APIBase
			isOAuth = pc.AuthType == "oauth"
		}
		infoList = append(infoList, ProviderInfo{
			Name:     activeProvider,
			Type:     providerType,
			APIBase:  apiBase,
			HasAuth:  hasAuth,
			IsOAuth:  isOAuth,
			IsActive: true,
		})
	}

	for name, pc := range configuredProviders {
		if name == activeProvider {
			continue
		}
		providerType := providerTypes[name]
		if providerType == "" {
			providerType = "OpenAI Compatible"
		}
		hasAuth := pc.APIKey != "" || pc.AuthType == "oauth"
		isOAuth := pc.AuthType == "oauth"
		isPartial := !hasAuth && pc.APIBase == ""
		infoList = append(infoList, ProviderInfo{
			Name:      name,
			Type:      providerType,
			APIBase:   pc.APIBase,
			HasAuth:   hasAuth,
			IsOAuth:   isOAuth,
			IsActive:  false,
			IsPartial: isPartial,
		})
	}

	return infoList
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
				hasAuth:  p.HasAuth,
				isOAuth:  p.IsOAuth,
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
	if key, ok := msg.(tea.KeyMsg); ok && key.String() == "esc" {
		return m, func() tea.Msg { return CloseDialogMsg{} }
	}
	return m, nil
}

func (m ProvidersModel) View() string {
	t := theme.Current()
	if t == nil {
		return "providers"
	}

	lines := []string{
		lipgloss.NewStyle().Foreground(t.Primary).Bold(true).Render("Providers"),
		"",
	}

	if len(m.rows) == 0 {
		lines = append(lines, lipgloss.NewStyle().Foreground(t.TextMuted).Italic(true).Render("No provider information available"))
	} else {
		for _, row := range m.rows {
			lines = append(lines, m.renderRow(row, t)...)
		}
	}

	lines = append(lines, "", lipgloss.NewStyle().Foreground(t.TextMuted).Render("Esc close"))
	return lipgloss.NewStyle().
		Background(t.Panel).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(t.BorderFocus).
		Padding(1, 2).
		Width(dialogWidth(m.termWidth, 72)).
		Render(strings.Join(lines, "\n"))
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
