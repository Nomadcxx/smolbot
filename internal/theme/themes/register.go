package themes

import (
	"fmt"
	"image/color"
	"strconv"

	lipgloss "charm.land/lipgloss/v2"
	"github.com/Nomadcxx/smolbot/internal/theme"
)

type themeOption func(*theme.Theme)

func register(name string, colors [15]string, opts ...themeOption) {
	t := &theme.Theme{
		Name:                      name,
		Background:                lipgloss.Color(colors[0]),
		Panel:                     lipgloss.Color(colors[1]),
		Element:                   lipgloss.Color(colors[2]),
		Border:                    lipgloss.Color(colors[3]),
		BorderFocus:               lipgloss.Color(colors[4]),
		Primary:                   lipgloss.Color(colors[5]),
		Secondary:                 lipgloss.Color(colors[6]),
		Accent:                    lipgloss.Color(colors[7]),
		Text:                      lipgloss.Color(colors[8]),
		TextMuted:                 lipgloss.Color(colors[9]),
		Error:                     lipgloss.Color(colors[10]),
		Warning:                   lipgloss.Color(colors[11]),
		Success:                   lipgloss.Color(colors[12]),
		Info:                      lipgloss.Color(colors[13]),
		ToolBorder:                lipgloss.Color(colors[14]),
		ToolName:                  lipgloss.Color(colors[5]),
		TranscriptUserAccent:      lipgloss.Color(colors[5]),
		TranscriptAssistantAccent: lipgloss.Color(colors[6]),
		TranscriptThinking:        lipgloss.Color(colors[9]),
		TranscriptStreaming:       lipgloss.Color(colors[13]),
		TranscriptError:           lipgloss.Color(colors[10]),
		MarkdownHeading:           lipgloss.Color(colors[5]),
		MarkdownLink:              lipgloss.Color(colors[7]),
		MarkdownCode:              lipgloss.Color(colors[5]),
		SyntaxKeyword:             lipgloss.Color(colors[5]),
		SyntaxString:              lipgloss.Color(colors[6]),
		SyntaxComment:             lipgloss.Color(colors[9]),
		ToolStateRunning:          lipgloss.Color(colors[11]),
		ToolStateDone:             lipgloss.Color(colors[12]),
		ToolStateError:            lipgloss.Color(colors[10]),
		ToolArtifactBorder:        lipgloss.Color(colors[14]),
		ToolArtifactHeader:        lipgloss.Color(darkenHex(colors[5], 0.18)),
		ToolArtifactBody:          lipgloss.Color(darkenHex(colors[3], 0.55)),
		CompressionActive:         lipgloss.Color(colors[12]), // Success color
		CompressionSuccess:        lipgloss.Color(colors[7]),  // Accent
		CompressionWarning:        lipgloss.Color(colors[11]), // Warning
		TokenHighUsage:            lipgloss.Color(colors[10]), // Error
		// Agent colors derived from base palette — provide 8 distinct identities.
		AgentBlue:   lipgloss.Color(colors[13]), // Info
		AgentGreen:  lipgloss.Color(colors[12]), // Success
		AgentYellow: lipgloss.Color(colors[11]), // Warning
		AgentRed:    lipgloss.Color(colors[10]), // Error
		AgentPurple: lipgloss.Color(colors[7]),  // Accent
		AgentOrange: lipgloss.Color(darkenHex(colors[11], 0.85)),
		AgentPink:   lipgloss.Color(darkenHex(colors[10], 0.70)),
		AgentCyan:   lipgloss.Color(darkenHex(colors[13], 0.80)),
	}

	for _, opt := range opts {
		opt(t)
	}

	if t.DiffAdded == nil {
		t.DiffAdded = t.Success
	}
	if t.DiffRemoved == nil {
		t.DiffRemoved = t.Error
	}
	if t.DiffContext == nil {
		t.DiffContext = t.TextMuted
	}
	if t.DiffContextBg == nil {
		t.DiffContextBg = t.Panel
	}
	if t.DiffAddedBg == nil {
		t.DiffAddedBg = darkenColor(t.DiffAdded, 0.85)
	}
	if t.DiffRemovedBg == nil {
		t.DiffRemovedBg = darkenColor(t.DiffRemoved, 0.85)
	}
	if t.DiffHighlightAdded == nil {
		t.DiffHighlightAdded = t.DiffAdded
	}
	if t.DiffHighlightRemoved == nil {
		t.DiffHighlightRemoved = t.DiffRemoved
	}
	if t.DiffLineNumber == nil {
		t.DiffLineNumber = t.TextMuted
	}

	theme.Register(t)
}

func darkenColor(value color.Color, factor float64) color.Color {
	if value == nil {
		return nil
	}
	r, g, b, _ := value.RGBA()
	return lipgloss.Color(fmt.Sprintf("#%02X%02X%02X",
		uint8(float64(r>>8)*factor+0.5),
		uint8(float64(g>>8)*factor+0.5),
		uint8(float64(b>>8)*factor+0.5),
	))
}

func darkenHex(hex string, factor float64) string {
	if len(hex) != 7 || hex[0] != '#' {
		return "#000000"
	}
	r := darkenChannel(hex[1:3], factor)
	g := darkenChannel(hex[3:5], factor)
	b := darkenChannel(hex[5:7], factor)
	return fmt.Sprintf("#%02X%02X%02X", r, g, b)
}

func darkenChannel(pair string, factor float64) uint8 {
	value, err := strconv.ParseUint(pair, 16, 8)
	if err != nil {
		return 0
	}
	return uint8(float64(value)*factor + 0.5)
}
