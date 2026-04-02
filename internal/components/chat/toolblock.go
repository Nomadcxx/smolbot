package chat

import (
	"fmt"
	"image/color"
	"strings"

	lipgloss "charm.land/lipgloss/v2"
	"github.com/Nomadcxx/smolbot/internal/theme"
)

// ToolBlockState describes the lifecycle state of a rendered tool block.
type ToolBlockState int

const (
	ToolBlockRunning ToolBlockState = iota
	ToolBlockDone
	ToolBlockError
)

// ToolBlockOpts configures a reusable tool artifact container.
type ToolBlockOpts struct {
	Title        string
	Content      string
	State        ToolBlockState
	Width        int
	SpinnerFrame int
}

func (s ToolBlockState) String() string {
	switch s {
	case ToolBlockRunning:
		return "running"
	case ToolBlockDone:
		return "done"
	case ToolBlockError:
		return "error"
	default:
		return "unknown"
	}
}

// RenderToolBlock renders a left-accented tool artifact container with state-specific chrome.
func RenderToolBlock(opts ToolBlockOpts, t *theme.Theme) string {
	if t == nil {
		return renderPlainToolBlock(opts)
	}

	title := strings.TrimSpace(opts.Title)
	if title == "" {
		title = "Tool"
	}

	accent := toolBlockAccent(opts.State, t)
	statusGlyph := toolBlockGlyph(opts.State, opts.SpinnerFrame)
	badge := lipgloss.NewStyle().
		Background(accent).
		Foreground(t.Background).
		Bold(true).
		Padding(0, 1).
		Render(statusGlyph)

	titleText := lipgloss.NewStyle().
		Foreground(t.ToolName).
		Bold(true).
		Render(title)

	headerLine := badge + " " + titleText

	bodyText := strings.TrimRight(opts.Content, "\n")
	if strings.TrimSpace(bodyText) == "" && opts.State == ToolBlockRunning {
		bodyText = "running..."
	}

	innerWidth := cappedWidth(opts.Width)
	header := lipgloss.NewStyle().
		Background(subtleWash(accent)).
		Width(innerWidth).
		Padding(0, 1).
		Render(headerLine)

	body := lipgloss.NewStyle().
		Foreground(t.Text).
		Width(innerWidth).
		Padding(0, 1).
		Render(bodyText)

	content := lipgloss.JoinVertical(lipgloss.Left, header, body)
	style := lipgloss.NewStyle().
		Border(lipgloss.ThickBorder(), false, false, false, true).
		BorderForeground(accent).
		Padding(0, 0)
	if opts.Width > 4 {
		style = style.Width(opts.Width - 2)
	}
	return style.Render(content)
}

func renderPlainToolBlock(opts ToolBlockOpts) string {
	title := strings.TrimSpace(opts.Title)
	if title == "" {
		title = "Tool"
	}
	body := strings.TrimRight(opts.Content, "\n")
	if body == "" && opts.State == ToolBlockRunning {
		body = "running..."
	}
	if body == "" {
		return title
	}
	return title + "\n" + body
}

func toolBlockAccent(state ToolBlockState, t *theme.Theme) color.Color {
	switch state {
	case ToolBlockRunning:
		if t.ToolStateRunning != nil {
			return t.ToolStateRunning
		}
		return t.Warning
	case ToolBlockDone:
		if t.ToolStateDone != nil {
			return t.ToolStateDone
		}
		return t.Success
	case ToolBlockError:
		if t.ToolStateError != nil {
			return t.ToolStateError
		}
		return t.Error
	default:
		return t.ToolBorder
	}
}

func toolBlockGlyph(state ToolBlockState, spinnerFrame int) string {
	switch state {
	case ToolBlockRunning:
		frames := []string{"◐", "◓", "◑", "◒"}
		if len(frames) == 0 {
			return "•"
		}
		if spinnerFrame < 0 {
			spinnerFrame = 0
		}
		return frames[spinnerFrame%len(frames)]
	case ToolBlockDone:
		return "✓"
	case ToolBlockError:
		return "✗"
	default:
		return "•"
	}
}

// --- Phase 5: Collapsed Group Renderer ---

// RenderToolGroup renders a collapsed group as a compact summary line.
// When verbose=true returns "" signalling the caller should render tools individually.
func RenderToolGroup(group *ToolGroup, t *theme.Theme, width int, verbose bool) string {
return RenderToolGroupWithSpinner(group, t, width, verbose, 0)
}

// RenderToolGroupWithSpinner renders with an animated spinner for active groups.
func RenderToolGroupWithSpinner(group *ToolGroup, t *theme.Theme, width int, verbose bool, spinnerFrame int) string {
if verbose {
return ""
}
if t == nil {
return renderPlainToolGroup(group)
}

// Build indicator
indicator, indicatorColor := groupIndicator(group, t, spinnerFrame)
indicatorText := lipgloss.NewStyle().Foreground(indicatorColor).Render(indicator)

// Build summary text
var summary string
if group.IsActive {
summary = group.ActiveSummary()
} else {
summary = group.Summary()
}
if len(summary) > 0 {
runes := []rune(summary)
summary = strings.ToUpper(string(runes[:1])) + string(runes[1:])
}

// Append error count suffix
if group.HasError && group.ErrorCount() > 0 {
errorSuffix := fmt.Sprintf(" (%d failed)", group.ErrorCount())
summary += lipgloss.NewStyle().Foreground(t.Error).Render(errorSuffix)
}

line := indicatorText + " " + summary

// Expansion hint (only if space permits)
hint := lipgloss.NewStyle().Foreground(t.TextMuted).Render("  (Ctrl+O to expand)")
if width > 0 && lipgloss.Width(line)+lipgloss.Width(hint) < width-4 {
line += hint
}

// Phase 10: hint line for active operation
if group.IsActive {
if current := group.CurrentOperationForHint(); current != nil {
hintTitle := toolDisplayTitle(*current)
hintLine := lipgloss.NewStyle().
Foreground(t.TextMuted).
PaddingLeft(2).
Render("⎿ " + hintTitle)
line = line + "\n" + hintLine
}
}

return line
}

// groupIndicator returns the status glyph and its color.
func groupIndicator(group *ToolGroup, t *theme.Theme, spinnerFrame int) (string, color.Color) {
if group.HasError {
return "✗", t.Error
}
if group.IsActive {
frames := []string{"◐", "◓", "◑", "◒"}
glyph := frames[spinnerFrame%len(frames)]
return glyph, t.Warning
}
return "✓", t.Success
}

func renderPlainToolGroup(group *ToolGroup) string {
prefix := "[✓] "
if group.HasError {
prefix = "[!] "
} else if group.IsActive {
prefix = "[.] "
}

var summary string
if group.IsActive {
summary = group.ActiveSummary()
} else {
summary = group.Summary()
}
return prefix + summary
}
