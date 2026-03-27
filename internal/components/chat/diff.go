package chat

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	lipgloss "charm.land/lipgloss/v2"
	"github.com/Nomadcxx/smolbot/internal/theme"
)

// DiffLineKind classifies a parsed unified diff line.
type DiffLineKind int

const (
	DiffLineContext DiffLineKind = iota
	DiffLineAdded
	DiffLineRemoved
	DiffLineFileHeader
	DiffLineHunkHeader
	DiffLineMeta
)

// DiffLine is a structured representation of a unified diff line.
type DiffLine struct {
	Kind    DiffLineKind
	OldLine int
	NewLine int
	Text    string
}

// ParsedDiff keeps diff lines in source order so renderers can choose a layout.
type ParsedDiff struct {
	Lines []DiffLine
}

var hunkHeaderRE = regexp.MustCompile(`^@@ -(\d+)(?:,(\d+))? \+(\d+)(?:,(\d+))? @@`)

// ParseUnifiedDiff converts a unified diff string into a structured line slice.
func ParseUnifiedDiff(input string) ParsedDiff {
	lines := strings.Split(input, "\n")
	parsed := ParsedDiff{Lines: make([]DiffLine, 0, len(lines))}
	oldLine := 0
	newLine := 0

	for _, raw := range lines {
		switch {
		case strings.HasPrefix(raw, "@@"):
			parsed.Lines = append(parsed.Lines, DiffLine{Kind: DiffLineHunkHeader, Text: raw})
			if matches := hunkHeaderRE.FindStringSubmatch(raw); len(matches) == 5 {
				oldLine = atoiOrZero(matches[1])
				newLine = atoiOrZero(matches[3])
			}
		case isDiffFileHeader(raw):
			parsed.Lines = append(parsed.Lines, DiffLine{Kind: DiffLineFileHeader, Text: raw})
		case strings.HasPrefix(raw, "\\ No newline at end of file"):
			parsed.Lines = append(parsed.Lines, DiffLine{Kind: DiffLineMeta, Text: raw})
		case len(raw) > 0 && raw[0] == ' ':
			parsed.Lines = append(parsed.Lines, DiffLine{Kind: DiffLineContext, OldLine: oldLine, NewLine: newLine, Text: raw[1:]})
			oldLine++
			newLine++
		case len(raw) > 0 && raw[0] == '-':
			parsed.Lines = append(parsed.Lines, DiffLine{Kind: DiffLineRemoved, OldLine: oldLine, Text: raw[1:]})
			oldLine++
		case len(raw) > 0 && raw[0] == '+':
			parsed.Lines = append(parsed.Lines, DiffLine{Kind: DiffLineAdded, NewLine: newLine, Text: raw[1:]})
			newLine++
		case raw == "":
			parsed.Lines = append(parsed.Lines, DiffLine{Kind: DiffLineContext, OldLine: oldLine, NewLine: newLine, Text: ""})
		default:
			parsed.Lines = append(parsed.Lines, DiffLine{Kind: DiffLineMeta, Text: raw})
		}
	}

	return parsed
}

// RenderDiff parses and renders a unified diff string using a width-aware layout.
func RenderDiff(input string, width int, t *theme.Theme) string {
	return RenderParsedDiff(ParseUnifiedDiff(input), width, t)
}

// RenderParsedDiff renders a parsed diff, switching layouts at the width threshold.
func RenderParsedDiff(parsed ParsedDiff, width int, t *theme.Theme) string {
	if t == nil {
		return renderPlainDiff(parsed)
	}
	if width >= 120 {
		return renderWideDiff(parsed, width, t)
	}
	return renderUnifiedDiff(parsed, width, t)
}

func renderPlainDiff(parsed ParsedDiff) string {
	lines := make([]string, 0, len(parsed.Lines))
	for _, line := range parsed.Lines {
		switch line.Kind {
		case DiffLineAdded:
			lines = append(lines, "+ "+line.Text)
		case DiffLineRemoved:
			lines = append(lines, "- "+line.Text)
		case DiffLineContext:
			lines = append(lines, "  "+line.Text)
		default:
			lines = append(lines, line.Text)
		}
	}
	return strings.Join(lines, "\n")
}

func renderUnifiedDiff(parsed ParsedDiff, width int, t *theme.Theme) string {
	numWidth := maxDiffLineNumberWidth(parsed)
	lines := make([]string, 0, len(parsed.Lines))

	for _, line := range parsed.Lines {
		switch line.Kind {
		case DiffLineFileHeader:
			lines = append(lines, lipgloss.NewStyle().Foreground(t.DiffLineNumber).Bold(true).Render(line.Text))
		case DiffLineHunkHeader:
			lines = append(lines, lipgloss.NewStyle().Foreground(t.DiffContext).Bold(true).Render(line.Text))
		case DiffLineMeta:
			lines = append(lines, lipgloss.NewStyle().Foreground(t.DiffLineNumber).Italic(true).Render(line.Text))
		case DiffLineContext, DiffLineAdded, DiffLineRemoved:
			lines = append(lines, renderUnifiedContentLine(line, numWidth, t, width))
		}
	}

	return strings.Join(lines, "\n")
}

func renderUnifiedContentLine(line DiffLine, numWidth int, t *theme.Theme, width int) string {
	sign := " "
	ink := t.DiffContext
	bg := t.DiffContextBg
	switch line.Kind {
	case DiffLineAdded:
		sign = "+"
		ink = t.DiffAdded
		bg = t.DiffAddedBg
	case DiffLineRemoved:
		sign = "-"
		ink = t.DiffRemoved
		bg = t.DiffRemovedBg
	}

	oldNo := formatLineNumber(line.OldLine, numWidth)
	newNo := formatLineNumber(line.NewLine, numWidth)
	numberStyle := lipgloss.NewStyle().Foreground(t.DiffLineNumber).Background(bg)
	textStyle := lipgloss.NewStyle().Foreground(ink).Background(bg)

	content := strings.Join([]string{
		numberStyle.Render(oldNo),
		numberStyle.Render(newNo),
		lipgloss.NewStyle().Foreground(ink).Background(bg).Bold(true).Render(sign),
		textStyle.Render(line.Text),
	}, " ")

	if width <= 4 {
		return content
	}
	return lipgloss.NewStyle().Background(bg).Render(padRightStyled(content, width-2))
}

type diffRow struct {
	left  *DiffLine
	right *DiffLine
	text  string
	kind  DiffLineKind
}

func renderWideDiff(parsed ParsedDiff, width int, t *theme.Theme) string {
	numWidth := maxDiffLineNumberWidth(parsed)
	rows := buildDiffRows(parsed)
	leftWidth := max(24, (width-3)/2)
	rightWidth := max(24, width-leftWidth-3)
	sep := lipgloss.NewStyle().Foreground(t.TextMuted).Render(" │ ")

	rendered := make([]string, 0, len(rows))
	for _, row := range rows {
		if row.text != "" {
			switch row.kind {
			case DiffLineFileHeader:
				rendered = append(rendered, lipgloss.NewStyle().Foreground(t.DiffLineNumber).Bold(true).Render(row.text))
			case DiffLineHunkHeader:
				rendered = append(rendered, lipgloss.NewStyle().Foreground(t.DiffContext).Bold(true).Render(row.text))
			case DiffLineMeta:
				rendered = append(rendered, lipgloss.NewStyle().Foreground(t.DiffLineNumber).Italic(true).Render(row.text))
			}
			continue
		}

		left := renderDiffCell(row.left, true, leftWidth, numWidth, t)
		right := renderDiffCell(row.right, false, rightWidth, numWidth, t)
		rendered = append(rendered, left+sep+right)
	}

	return strings.Join(rendered, "\n")
}

func buildDiffRows(parsed ParsedDiff) []diffRow {
	rows := make([]diffRow, 0, len(parsed.Lines))
	for i := 0; i < len(parsed.Lines); i++ {
		line := parsed.Lines[i]
		switch line.Kind {
		case DiffLineFileHeader, DiffLineHunkHeader, DiffLineMeta:
			rows = append(rows, diffRow{text: line.Text, kind: line.Kind})
		case DiffLineContext:
			l := line
			r := line
			rows = append(rows, diffRow{left: &l, right: &r})
		case DiffLineRemoved:
			if i+1 < len(parsed.Lines) && parsed.Lines[i+1].Kind == DiffLineAdded {
				left := line
				right := parsed.Lines[i+1]
				rows = append(rows, diffRow{left: &left, right: &right})
				i++
				continue
			}
			left := line
			rows = append(rows, diffRow{left: &left})
		case DiffLineAdded:
			right := line
			rows = append(rows, diffRow{right: &right})
		}
	}
	return rows
}

func renderDiffCell(line *DiffLine, left bool, width, numWidth int, t *theme.Theme) string {
	if line == nil {
		return strings.Repeat(" ", width)
	}

	number := line.OldLine
	if !left {
		number = line.NewLine
	}
	sign := " "
	ink := t.DiffContext
	bg := t.DiffContextBg
	switch line.Kind {
	case DiffLineAdded:
		sign = "+"
		ink = t.DiffAdded
		bg = t.DiffAddedBg
	case DiffLineRemoved:
		sign = "-"
		ink = t.DiffRemoved
		bg = t.DiffRemovedBg
	}

	lineNo := formatLineNumber(number, numWidth)
	numberStyle := lipgloss.NewStyle().Foreground(t.DiffLineNumber).Background(bg)
	signStyle := lipgloss.NewStyle().Foreground(ink).Background(bg).Bold(true)
	textStyle := lipgloss.NewStyle().Foreground(ink).Background(bg)
	prefix := strings.Join([]string{
		numberStyle.Render(lineNo),
		signStyle.Render(sign),
	}, " ")
	prefix += " "

	available := width - lipgloss.Width(prefix)
	text := truncateToWidth(line.Text, available)
	cell := prefix + textStyle.Render(text)
	return lipgloss.NewStyle().Background(bg).Render(padRightStyled(cell, width))
}

func isDiffFileHeader(line string) bool {
	return strings.HasPrefix(line, "--- ") ||
		strings.HasPrefix(line, "+++ ") ||
		strings.HasPrefix(line, "diff --git ") ||
		strings.HasPrefix(line, "index ") ||
		strings.HasPrefix(line, "new file mode ") ||
		strings.HasPrefix(line, "deleted file mode ") ||
		strings.HasPrefix(line, "Binary files ")
}

func maxDiffLineNumberWidth(parsed ParsedDiff) int {
	maxLine := 0
	for _, line := range parsed.Lines {
		if line.OldLine > maxLine {
			maxLine = line.OldLine
		}
		if line.NewLine > maxLine {
			maxLine = line.NewLine
		}
	}
	if maxLine == 0 {
		return 1
	}
	return len(strconv.Itoa(maxLine))
}

func formatLineNumber(n, width int) string {
	if n <= 0 {
		return strings.Repeat(" ", width)
	}
	return fmt.Sprintf("%*d", width, n)
}

func truncateToWidth(s string, width int) string {
	if width <= 0 {
		return ""
	}
	if lipgloss.Width(s) <= width {
		return s
	}

	runes := []rune(s)
	for i := range runes {
		candidate := string(runes[:i+1])
		if lipgloss.Width(candidate) > width-1 {
			if i == 0 {
				return "…"
			}
			return string(runes[:i]) + "…"
		}
	}
	return s
}

func padRightStyled(s string, width int) string {
	if width <= 0 {
		return s
	}
	current := lipgloss.Width(s)
	if current >= width {
		return s
	}
	return s + strings.Repeat(" ", width-current)
}

func atoiOrZero(s string) int {
	if s == "" {
		return 0
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		return 0
	}
	return n
}
