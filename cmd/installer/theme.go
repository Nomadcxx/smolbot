// cmd/installer/theme.go
package main

import "github.com/charmbracelet/lipgloss"

// Theme colors - Monochrome (sysc-family style)
var (
	BgBase       = lipgloss.Color("#1a1a1a")
	BgElevated   = lipgloss.Color("#2a2a2a")
	Primary      = lipgloss.Color("#ffffff")
	Secondary    = lipgloss.Color("#cccccc")
	Accent       = lipgloss.Color("#ffffff")
	FgPrimary    = lipgloss.Color("#ffffff")
	FgSecondary  = lipgloss.Color("#cccccc")
	FgMuted      = lipgloss.Color("#666666")
	ErrorColor   = lipgloss.Color("#ffffff")
	WarningColor = lipgloss.Color("#888888")
	SuccessColor = lipgloss.Color("#ffffff")
)

// Styles
var (
	checkMark   = lipgloss.NewStyle().Foreground(SuccessColor).SetString("[OK]")
	failMark    = lipgloss.NewStyle().Foreground(ErrorColor).SetString("[FAIL]")
	skipMark    = lipgloss.NewStyle().Foreground(WarningColor).SetString("[SKIP]")
	headerStyle = lipgloss.NewStyle().Foreground(Primary).Bold(true)
)

// SMOLBOT ASCII header
var asciiHeaderLines = []string{
	"   _____ __  ______  ____        ____  ____  ________    ",
	"  / ___//  |/  / _ )/ __ \\____ _/ __ )/ __ \\/ ____/ /    ",
	"  \\__ \\/ /|_/ / _  / /_/ / __ / /_/ / / / / __/ / /     ",
	" ___/ / /  / / __/ / _, _/ /_/ / _, _/ /_/ / /___/ /___  ",
	"/____/_/  /_/_/   /_/ |_|\\__, /_/ |_/_____/_____/_____/  ",
	"                        /____/                            ",
}

// Ticker messages
var tickerMessages = []string{
	"A terminal-based AI assistant that runs on your own hardware",
	"Chat with local or cloud AI models",
	"Manage persistent sessions and automate tasks",
}
