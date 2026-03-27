package chat

import (
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
