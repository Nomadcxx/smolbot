// cmd/installer/theme.go
package main

import "github.com/charmbracelet/lipgloss"

// Theme colors - matching nanobot-tui Catppuccin theme
type ThemeColors struct {
	BgBase       string
	BgElevated   string
	Primary      string
	Secondary    string
	Accent       string
	FgPrimary    string
	FgSecondary  string
	FgMuted      string
	ErrorColor   string
	WarningColor string
	SuccessColor string
}

// Catppuccin Mocha theme (matches nanobot-tui)
var defaultTheme = ThemeColors{
	BgBase:       "#1e1e2e",
	BgElevated:   "#313244",
	Primary:      "#cba6f7",
	Secondary:    "#89b4fa",
	Accent:       "#f38ba8",
	FgPrimary:    "#cdd6f4",
	FgSecondary:  "#a6adc8",
	FgMuted:      "#6c7086",
	ErrorColor:   "#f38ba8",
	WarningColor: "#fab387",
	SuccessColor: "#a6e3a1",
}

// Lipgloss styles
var (
	headerStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color(defaultTheme.Primary)).
		Bold(true).
		MarginBottom(1)

	titleStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color(defaultTheme.Secondary)).
		Bold(true)

	checkMark = lipgloss.NewStyle().
		Foreground(lipgloss.Color(defaultTheme.SuccessColor)).
		SetString("[✓]")

	failMark = lipgloss.NewStyle().
		Foreground(lipgloss.Color(defaultTheme.ErrorColor)).
		SetString("[✗]")

	skipMark = lipgloss.NewStyle().
		Foreground(lipgloss.Color(defaultTheme.WarningColor)).
		SetString("[-]")

	descriptionStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color(defaultTheme.FgSecondary))

	mutedStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color(defaultTheme.FgMuted))

	selectedStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color(defaultTheme.Primary)).
		Bold(true)

	inputStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color(defaultTheme.FgPrimary)).
		Background(lipgloss.Color(defaultTheme.BgElevated)).
		Padding(0, 1)

	inputFocusedStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color(defaultTheme.Primary)).
		Background(lipgloss.Color(defaultTheme.BgElevated)).
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color(defaultTheme.Primary)).
		Padding(0, 1)

	errorStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color(defaultTheme.ErrorColor)).
		Bold(true)

	successStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color(defaultTheme.SuccessColor)).
		Bold(true)

	warningStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color(defaultTheme.WarningColor))

	boxStyle = lipgloss.NewStyle().
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(defaultTheme.BgElevated)).
		Padding(1)
)

// SMOLBOT ASCII art
func getSmolbotArt() string {
	return `
   _____ __  ______  ____        ____  ____  ________    
  / ___//  |/  / _ )/ __ \____ _/ __ )/ __ \/ ____/ /    
  \__ \/ /|_/ / _  / /_/ / __ / /_/ / / / / __/ / /     
 ___/ / /  / / __/ / _, _/ /_/ / _, _/ /_/ / /___/ /___  
/____/_/  /_/_/   /_/ |_|\__, /_/ |_/_____/_____/_____/  
                        /____/                            
`
}
