package status

import (
	"fmt"
	"strings"

	lipgloss "charm.land/lipgloss/v2"
	"github.com/Nomadcxx/smolbot/internal/app"
	"github.com/Nomadcxx/smolbot/internal/client"
	"github.com/Nomadcxx/smolbot/internal/theme"
)

type FooterModel struct {
	app               *app.App
	width             int
	metadata          string
	usage             client.UsageInfo
	compression       *client.CompressionInfo
	compacting        bool
	compactFrame      int
	modelText         string
	modelWidth        int
	sessionText       string
	sessionWidth      int
	metadataText      string
	metadataWidth     int
	usageFullWidth    int
	usageCompactWidth int

	// Tool activity tracking
	toolsRunning      int
	toolsDone         int
	toolsError        int
	toolActivityText  string
	toolActivityWidth int

	// Queue tracking
	queueCount     int
	queueText      string
	queueTextWidth int

	// Scroll position tracking
	scrollText      string
	scrollTextWidth int

	// View cache
	cachedView string
	cacheDirty bool
}

func NewFooter(a *app.App) FooterModel {
	model := FooterModel{app: a}
	if a != nil {
		model.SetModel(a.Model)
		model.SetSession(a.Session)
	}
	return model
}

func (m *FooterModel) SetWidth(w int) {
	m.width = w
	m.cacheDirty = true
}

func (m *FooterModel) SetMetadata(value string) {
	m.metadata = value
	m.metadataText = strings.TrimSpace(value)
	m.metadataWidth = lipgloss.Width(m.metadataText)
	m.cacheDirty = true
}

func (m *FooterModel) SetModel(value string) {
	m.modelText = "model " + footerValue(value, "connecting...")
	m.modelWidth = lipgloss.Width(m.modelText)
	m.cacheDirty = true
}

func (m *FooterModel) SetSession(value string) {
	m.sessionText = "session " + footerValue(value, "none")
	m.sessionWidth = lipgloss.Width(m.sessionText)
	m.cacheDirty = true
}

func (m *FooterModel) SetUsage(value client.UsageInfo) {
	m.usage = value
	m.syncUsageLayout()
	m.cacheDirty = true
}

func (m *FooterModel) SetCompression(info *client.CompressionInfo) {
	m.compression = info
	m.cacheDirty = true
}

func (m *FooterModel) SetCompacting(v bool) {
	m.compacting = v
	if !v {
		m.compactFrame = 0
	}
	m.cacheDirty = true
}

func (m *FooterModel) IsCompacting() bool {
	return m.compacting
}

func (m *FooterModel) SetCompactionFrame(frame int) {
	m.compactFrame = frame
	m.cacheDirty = true
}

// SetToolCounts sets all tool activity counters at once.
func (m *FooterModel) SetToolCounts(running, done, errored int) {
	m.toolsRunning = running
	m.toolsDone = done
	m.toolsError = errored
	m.syncToolActivity()
	m.cacheDirty = true
}

// IncrementToolRunning increments the running tool count.
func (m *FooterModel) IncrementToolRunning() {
	m.toolsRunning++
	m.syncToolActivity()
	m.cacheDirty = true
}

// ToolDone moves one tool from running to done.
func (m *FooterModel) ToolDone() {
	if m.toolsRunning > 0 {
		m.toolsRunning--
	}
	m.toolsDone++
	m.syncToolActivity()
	m.cacheDirty = true
}

// ToolError moves one tool from running to error.
func (m *FooterModel) ToolError() {
	if m.toolsRunning > 0 {
		m.toolsRunning--
	}
	m.toolsError++
	m.syncToolActivity()
	m.cacheDirty = true
}

// ResetToolCounts clears all tool counters (called at start of each turn).
func (m *FooterModel) ResetToolCounts() {
	m.toolsRunning = 0
	m.toolsDone = 0
	m.toolsError = 0
	m.toolActivityText = ""
	m.toolActivityWidth = 0
	m.cacheDirty = true
}

// SetQueueCount updates the queued message count display.
func (m *FooterModel) SetQueueCount(n int) {
	m.queueCount = n
	t := theme.Current()
	if n <= 0 || t == nil {
		m.queueText = ""
		m.queueTextWidth = 0
	} else {
		arrows := "▶"
		if n > 1 {
			arrows = strings.Repeat("▶", min(n, 5))
		}
		m.queueText = lipgloss.NewStyle().Foreground(t.Warning).Bold(true).
			Render(fmt.Sprintf("%s %d queued", arrows, n))
		m.queueTextWidth = lipgloss.Width(m.queueText)
	}
	m.cacheDirty = true
}

// SetScrollPosition updates the scroll position indicator.
// current is the top visible line (0-based), total is total lines, visible is viewport height.
func (m *FooterModel) SetScrollPosition(current, total, visible int) {
	if total <= visible || total <= 0 {
		if m.scrollText != "" {
			m.scrollText = ""
			m.scrollTextWidth = 0
			m.cacheDirty = true
		}
		return
	}
	t := theme.Current()
	if t == nil {
		return
	}
	bottom := current + visible
	if bottom > total {
		bottom = total
	}
	text := fmt.Sprintf("↕ %d/%d", bottom, total)
	if text != m.scrollText {
		m.scrollText = lipgloss.NewStyle().Foreground(t.TextMuted).Render(text)
		m.scrollTextWidth = lipgloss.Width(m.scrollText)
		m.cacheDirty = true
	}
}

func (m *FooterModel) syncToolActivity() {
	t := theme.Current()
	if t == nil {
		m.toolActivityText = ""
		m.toolActivityWidth = 0
		return
	}

	var parts []string

	if m.toolsDone > 0 {
		parts = append(parts, lipgloss.NewStyle().Foreground(t.Success).Render(fmt.Sprintf("✓%d", m.toolsDone)))
	}
	if m.toolsRunning > 0 {
		parts = append(parts, lipgloss.NewStyle().Foreground(t.Warning).Render(fmt.Sprintf("●%d", m.toolsRunning)))
	}
	if m.toolsError > 0 {
		parts = append(parts, lipgloss.NewStyle().Foreground(t.Error).Render(fmt.Sprintf("✗%d", m.toolsError)))
	}

	if len(parts) == 0 {
		m.toolActivityText = ""
		m.toolActivityWidth = 0
		return
	}
	m.toolActivityText = strings.Join(parts, " ")
	m.toolActivityWidth = lipgloss.Width(m.toolActivityText)
}

func (m *FooterModel) syncUsageLayout() {
	full, compact := m.usageWidthStrings()
	m.usageFullWidth = lipgloss.Width(full)
	m.usageCompactWidth = lipgloss.Width(compact)
}

func (m *FooterModel) usageWidthStrings() (string, string) {
	if m.usage.ContextWindow <= 0 || m.usage.TotalTokens <= 0 {
		return "", ""
	}
	percentage := (float64(m.usage.TotalTokens) / float64(m.usage.ContextWindow)) * 100
	percentText := fmt.Sprintf("%d%%", int(percentage+0.5))
	warning := ""
	switch {
	case percentage >= 95:
		warning = " ⚠ /compact"
	case percentage >= 90:
		warning = " ⚠"
	}
	return percentText + warning + " " + fmt.Sprintf("(%s/%s)", formatUsageTokens(m.usage.TotalTokens), formatUsageTokens(m.usage.ContextWindow)),
		percentText + warning + " " + fmt.Sprintf("(%s)", formatUsageTokens(m.usage.TotalTokens))
}

func (m FooterModel) renderCompression(t *theme.Theme) string {
	if m.compacting {
		frames := []string{"⣾", "⣽", "⣻", "⢿", "⡿", "⣟", "⣯", "⣷"}
		frame := frames[m.compactFrame%len(frames)]
		return lipgloss.NewStyle().
			Foreground(t.Primary).
			Bold(true).
			Render(frame + " compacting...")
	}
	if m.compression == nil || !m.compression.Enabled {
		return ""
	}

	pct := m.compression.ReductionPercent

	// Choose style based on reduction percentage (inspired by nanocoder)
	var style lipgloss.Style
	indicator := "↓" // Down arrow indicates compression

	switch {
	case pct >= 60: // Heavy compression
		style = lipgloss.NewStyle().
			Foreground(t.CompressionWarning).
			Bold(true)
		indicator += fmt.Sprintf("%.0f%%", pct)
	case pct >= 30: // Moderate compression
		style = lipgloss.NewStyle().
			Foreground(t.CompressionSuccess)
		indicator += fmt.Sprintf("%.0f%%", pct)
	default: // Light compression
		style = lipgloss.NewStyle().
			Foreground(t.CompressionActive)
		indicator += fmt.Sprintf("%.0f%%", pct)
	}

	return style.Render(indicator)
}

func (m *FooterModel) View() string {
	t := theme.Current()
	if t == nil {
		return ""
	}
	if m.app != nil {
		if next := "model " + footerValue(m.app.Model, "connecting..."); next != m.modelText {
			m.SetModel(m.app.Model)
		}
		if next := "session " + footerValue(m.app.Session, "none"); next != m.sessionText {
			m.SetSession(m.app.Session)
		}
	}
	if !m.cacheDirty && m.cachedView != "" {
		return m.cachedView
	}
	parts := []string{m.modelText, m.sessionText}
	leftWidth := 1 + m.modelWidth + 3 + m.sessionWidth
	if m.metadataText != "" {
		parts = append(parts, m.metadataText)
		leftWidth += 3 + m.metadataWidth
	}
	// Add tool activity if any tools were used this turn
	if m.toolActivityText != "" {
		parts = append(parts, m.toolActivityText)
		leftWidth += 3 + m.toolActivityWidth
	}
	// Add queue count if messages are queued
	if m.queueText != "" {
		parts = append(parts, m.queueText)
		leftWidth += 3 + m.queueTextWidth
	}
	// Add scroll position
	if m.scrollText != "" {
		parts = append(parts, m.scrollText)
		leftWidth += 3 + m.scrollTextWidth
	}
	// Add compression indicator if available
	if comp := m.renderCompression(t); comp != "" {
		parts = append(parts, comp)
		leftWidth += 3 + lipgloss.Width(comp)
	}
	left := " " + strings.Join(parts, " | ")
	right := m.renderUsage(t, false)
	rightWidth := m.usageFullWidth
	if right == "" {
		return m.cacheFooter(lipgloss.NewStyle().
			Width(m.width).
			Background(t.Panel).
			Foreground(t.TextMuted).
			Render(left))
	}

	if m.width > 0 && leftWidth+1+rightWidth > m.width {
		right = m.renderUsage(t, true)
		rightWidth = m.usageCompactWidth
	}
	if m.width > 0 && rightWidth+1 >= m.width {
		return m.cacheFooter(lipgloss.NewStyle().
			Width(m.width).
			Background(t.Panel).
			Foreground(t.TextMuted).
			Align(lipgloss.Right).
			Render(right))
	}

	if m.width > 0 {
		availableLeft := max(1, m.width-rightWidth-1)
		left = truncateFooterText(left, leftWidth, availableLeft)
		leftWidth = min(leftWidth, availableLeft)
	}
	gap := " "
	if m.width > 0 {
		gapWidth := m.width - leftWidth - rightWidth
		if gapWidth > 0 {
			gap = strings.Repeat(" ", gapWidth)
		}
	}
	return m.cacheFooter(lipgloss.NewStyle().
		Width(m.width).
		Background(t.Panel).
		Foreground(t.TextMuted).
		Render(left + gap + right))
}

func (m *FooterModel) cacheFooter(output string) string {
	m.cachedView = output
	m.cacheDirty = false
	return output
}

func footerValue(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}

func truncateFooterText(text string, currentWidth, width int) string {
	if width <= 0 || currentWidth <= width {
		return text
	}
	if width <= 1 {
		return text[:width]
	}
	runes := []rune(text)
	if len(runes) > width-1 {
		runes = runes[:width-1]
	}
	return string(runes) + "…"
}

func (m FooterModel) renderUsage(t *theme.Theme, compact bool) string {
	if m.usage.ContextWindow <= 0 || m.usage.TotalTokens <= 0 {
		return ""
	}

	percentage := (float64(m.usage.TotalTokens) / float64(m.usage.ContextWindow)) * 100

	// Color coding inspired by nanocoder (green → yellow → red)
	var percentStyle lipgloss.Style
	if percentage >= 90 {
		percentStyle = lipgloss.NewStyle().Foreground(t.TokenHighUsage).Bold(true)
	} else if percentage >= 80 {
		percentStyle = lipgloss.NewStyle().Foreground(t.Warning).Bold(true)
	} else if percentage >= 60 {
		percentStyle = lipgloss.NewStyle().Foreground(t.CompressionWarning)
	} else {
		percentStyle = lipgloss.NewStyle().Foreground(t.TextMuted)
	}

	percentText := percentStyle.Render(fmt.Sprintf("%d%%", int(percentage+0.5)))
	warning := ""
	switch {
	case percentage >= 95:
		warning = lipgloss.NewStyle().
			Foreground(t.TokenHighUsage).
			Bold(true).
			Blink(true).
			Render(" ⚠ /compact")
	case percentage >= 90:
		warning = lipgloss.NewStyle().
			Foreground(t.Warning).
			Bold(true).
			Render(" ⚠")
	}

	if compact {
		return percentText + warning + " " + lipgloss.NewStyle().
			Background(t.Panel).
			Foreground(t.TextMuted).
			Render("("+formatUsageTokens(m.usage.TotalTokens)+")")
	}

	return percentText + warning + " " + lipgloss.NewStyle().
		Background(t.Panel).
		Foreground(t.TextMuted).
		Render(fmt.Sprintf("(%s/%s)", formatUsageTokens(m.usage.TotalTokens), formatUsageTokens(m.usage.ContextWindow)))
}

func formatUsageTokens(value int) string {
	switch {
	case value >= 1_000_000:
		text := fmt.Sprintf("%.1fM", float64(value)/1_000_000)
		return strings.Replace(text, ".0M", "M", 1)
	case value >= 1_000:
		text := fmt.Sprintf("%.1fK", float64(value)/1_000)
		return strings.Replace(text, ".0K", "K", 1)
	default:
		return fmt.Sprintf("%d", value)
	}
}
