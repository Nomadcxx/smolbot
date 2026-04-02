package chat

import (
	"fmt"
	"image/color"
	"strings"

	lipgloss "charm.land/lipgloss/v2"
	"github.com/Nomadcxx/smolbot/internal/format"
	"github.com/Nomadcxx/smolbot/internal/theme"
)

// AgentProgress tracks a single sub-agent's execution state for display.
type AgentProgress struct {
	ID           string
	Type         string // "explore", "code-review", "general-purpose", etc.
	Description  string
	IsResolved   bool
	IsError      bool
	IsAsync      bool   // launched in background mode
	ToolUseCount int
	TokenCount   int
	LastToolInfo string // current operation hint (e.g., "Reading 3 files...")
}

// TreeChar returns the tree branch character for the agent's position in a list.
func TreeChar(isLast bool) string {
	if isLast {
		return "└─"
	}
	return "├─"
}

// RenderAgentProgressLine renders a single agent as one tree branch line.
// Format: "├─ [type] description · N tool uses · 1.2K tokens"
//
//	"   ⇿ Reading src/index.ts"
func RenderAgentProgressLine(ap AgentProgress, t *theme.Theme, isLast bool) string {
	if t == nil {
		return fmt.Sprintf("%s [%s] %s", TreeChar(isLast), ap.Type, ap.Description)
	}

	treeChar := lipgloss.NewStyle().Foreground(t.TextMuted).Render(TreeChar(isLast))

	// Agent type badge
	agentColor := theme.GetAgentThemeColor(t, theme.GetAgentColor(ap.Type))
	if agentColor == nil {
		agentColor = t.Info
	}
	typeBadge := lipgloss.NewStyle().
		Background(agentColor).
		Foreground(t.Text).
		Padding(0, 1).
		Bold(true).
		Render(ap.Type)

	// Status text
	var statusText string
	switch {
	case ap.IsAsync && ap.IsResolved:
		statusText = "Running in the background"
	case ap.IsResolved && ap.IsError:
		statusText = "Failed"
	case ap.IsResolved:
		statusText = "Done"
	case ap.LastToolInfo != "":
		statusText = ap.LastToolInfo
	default:
		statusText = "Initialising…"
	}

	// Metrics line
	metrics := fmt.Sprintf("%d tool uses · %s tokens",
		ap.ToolUseCount,
		format.FormatTokens(ap.TokenCount),
	)
	metricsStyled := lipgloss.NewStyle().Foreground(t.TextMuted).Render(metrics)

	descPart := ap.Description
	if descPart == "" {
		descPart = ap.Type
	}

	line1 := fmt.Sprintf("%s %s %s · %s", treeChar, typeBadge, descPart, metricsStyled)
	statusLine := lipgloss.NewStyle().
		Foreground(t.TextMuted).
		PaddingLeft(3).
		Render("⇿ " + statusText)

	return line1 + "\n" + statusLine
}

// --- Grouped Agent Container ---

// GroupedAgentView renders multiple agents as a single collapsible unit.
type GroupedAgentView struct {
	Agents        []AgentProgress
	AllSameType   bool
	AnyUnresolved bool
	AnyError      bool
	AllAsync      bool
}

// NewGroupedAgentView creates a GroupedAgentView from a slice of agents and
// computes its derived boolean flags.
func NewGroupedAgentView(agents []AgentProgress) GroupedAgentView {
	g := GroupedAgentView{Agents: agents}
	if len(agents) == 0 {
		return g
	}

	firstType := agents[0].Type
	g.AllSameType = true
	g.AllAsync = true

	for _, ap := range agents {
		if ap.Type != firstType {
			g.AllSameType = false
		}
		if !ap.IsAsync {
			g.AllAsync = false
		}
		if !ap.IsResolved {
			g.AnyUnresolved = true
		}
		if ap.IsError {
			g.AnyError = true
		}
	}
	return g
}

// Summarize returns a natural-language group summary line.
func (g GroupedAgentView) Summarize() string {
	count := len(g.Agents)
	switch {
	case g.AllAsync:
		return fmt.Sprintf("%d background agents launched", count)
	case g.AnyUnresolved && g.AllSameType && count > 1:
		return fmt.Sprintf("Running %d %s agents…", count, g.Agents[0].Type)
	case g.AnyUnresolved:
		return fmt.Sprintf("Running %d agents…", count)
	case count == 1:
		return "1 agent finished"
	case g.AllSameType:
		return fmt.Sprintf("%d %s agents finished", count, g.Agents[0].Type)
	default:
		return fmt.Sprintf("%d agents finished", count)
	}
}

// Render renders the full grouped agent view with optional expansion.
func (g GroupedAgentView) Render(t *theme.Theme, spinnerFrame int, expanded bool) string {
	if t == nil {
		return g.Summarize()
	}

	var indicator string
	var indicatorColor color.Color
	switch {
	case g.AnyError:
		indicator = "✗"
		indicatorColor = t.Error
	case g.AnyUnresolved:
		frames := []string{"◐", "◓", "◑", "◒"}
		indicator = frames[spinnerFrame%len(frames)]
		indicatorColor = t.Warning
	default:
		indicator = "✓"
		indicatorColor = t.Success
	}

	indicatorStyled := lipgloss.NewStyle().Foreground(indicatorColor).Render(indicator)
	summary := g.Summarize()

	var b strings.Builder
	b.WriteString(indicatorStyled + " " + summary)

	if !g.AllAsync && !expanded {
		hint := lipgloss.NewStyle().Foreground(t.TextMuted).Render("  (Ctrl+O to expand)")
		b.WriteString(hint)
	}

	if !expanded {
		return b.String()
	}

	// Expanded: show each agent's progress line.
	b.WriteString("\n")
	for i, ap := range g.Agents {
		isLast := i == len(g.Agents)-1
		b.WriteString(RenderAgentProgressLine(ap, t, isLast))
		if !isLast {
			b.WriteString("\n")
		}
	}
	return b.String()
}

// --- Async / Teammate Detection ---

// DetectAsyncAgent returns true when the spawn tool's input/output indicates
// the agent was launched in background mode.
func DetectAsyncAgent(input, output map[string]interface{}) bool {
	if runInBg, ok := input["run_in_background"].(bool); ok && runInBg {
		return true
	}
	if status, ok := output["status"].(string); ok {
		return status == "async_launched" || status == "remote_launched"
	}
	return false
}

// --- Render Context ---

// RenderContext tracks nesting state to suppress redundant UI chrome.
type RenderContext struct {
	InSubAgent    bool
	InVirtualList bool
	NestingLevel  int
}

// ShouldShowExpandHint returns true when "(Ctrl+O to expand)" is appropriate.
// The hint is suppressed inside sub-agent output or virtual list contexts.
func (rc RenderContext) ShouldShowExpandHint() bool {
	return !rc.InSubAgent && !rc.InVirtualList
}
