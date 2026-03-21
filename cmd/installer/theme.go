// cmd/installer/theme.go
package main

import "github.com/charmbracelet/lipgloss"

// Theme colors - matching nanobot-tui
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
}

var defaultTheme = ThemeColors{
	BgBase:       "#1a1a1a",
	BgElevated:   "#2a2a2a",
	Primary:      "#ffffff",
	Secondary:    "#cccccc",
	Accent:       "#ffffff",
	FgPrimary:    "#ffffff",
	FgSecondary:  "#cccccc",
	FgMuted:      "#666666",
	ErrorColor:   "#ff6b6b",
	WarningColor: "#ffa500",
}

// Lipgloss styles
var (
	headerStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color(defaultTheme.Primary)).
		Bold(true).
		MarginBottom(1)

	checkMark = lipgloss.NewStyle().
		Foreground(lipgloss.Color(defaultTheme.Accent)).
		SetString("[OK]")

	failMark = lipgloss.NewStyle().
		Foreground(lipgloss.Color(defaultTheme.ErrorColor)).
		SetString("[FAIL]")

	skipMark = lipgloss.NewStyle().
		Foreground(lipgloss.Color(defaultTheme.WarningColor)).
		SetString("[SKIP]")

	descriptionStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color(defaultTheme.FgSecondary))

	mutedStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color(defaultTheme.FgMuted))
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
