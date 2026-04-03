package sidebar

import (
	"image/color"
	"strings"

	lipgloss "charm.land/lipgloss/v2"
	"github.com/Nomadcxx/smolbot/internal/client"
	"github.com/Nomadcxx/smolbot/internal/theme"
	"github.com/charmbracelet/x/ansi"
)

const DefaultWidth = 30

const minItemsPerSection = 1

type Model struct {
	width     int
	height    int
	visible   bool
	sidebarBg color.Color

	session  SessionSection
	context  ContextSection
	usage    UsageSection
	channels ChannelsSection
	mcps     MCPsSection
	cron     CronSection
}

func New() Model {
	return Model{
		width:   DefaultWidth,
		visible: true,
	}
}

func (m *Model) SetSize(width, height int) {
	if width > 0 {
		m.width = width
	}
	if height > 0 {
		m.height = height
	}
}

func (m Model) Width() int {
	if m.width <= 0 {
		return DefaultWidth
	}
	return m.width
}

func (m Model) Visible() bool {
	return m.visible
}

func (m *Model) Toggle() {
	m.visible = !m.visible
}

func (m *Model) SetVisible(v bool) {
	m.visible = v
}

func (m *Model) SetSidebarBg(bg color.Color) {
	m.sidebarBg = bg
}

func (m *Model) SetSession(session string) {
	m.session.sessionKey = session
}

func (m *Model) SetCWD(cwd string) {
	m.session.cwd = cwd
}

func (m *Model) SetModel(model string) {
	m.session.model = model
}

func (m *Model) SetUsage(usage client.UsageInfo) {
	m.context.usage = usage
}

func (m *Model) SetPersistedUsage(summary *client.UsageSummary) {
	m.usage.summary = summary
}

func (m *Model) SetCompression(info *client.CompressionInfo) {
	m.context.compression = info
}

func (m *Model) SetChannels(channels []ChannelEntry) {
	m.channels.channels = append([]ChannelEntry(nil), channels...)
}

func (m *Model) SetMCPs(servers []MCPEntry) {
	m.mcps.servers = append([]MCPEntry(nil), servers...)
}

func (m *Model) SetCronJobs(jobs []client.CronJob) {
	m.cron.jobs = append([]client.CronJob(nil), jobs...)
}

func (m Model) View() string {
	if !m.visible {
		return ""
	}
	t := theme.Current()
	limits := m.getDynamicLimits()

	// Content width accounts for left padding
	contentWidth := m.width - 1
	if contentWidth < 10 {
		contentWidth = m.width
	}

	sections := []string{
		renderSectionBlock(m.session, contentWidth, 0, t),
		renderSectionBlock(m.context, contentWidth, 0, t),
		renderSectionBlock(m.usage, contentWidth, 0, t),
		renderSectionBlock(m.channels, contentWidth, limits["CHANNELS"], t),
		renderSectionBlock(m.mcps, contentWidth, limits["MCPS"], t),
		renderSectionBlock(m.cron, contentWidth, limits["SCHEDULED"], t),
	}
	blocks := filterEmpty(sections)

	// Style wraps each block to fill full sidebar width with background
	blockStyle := lipgloss.NewStyle().Width(m.width).PaddingLeft(1)
	if m.sidebarBg != nil {
		blockStyle = blockStyle.Background(m.sidebarBg)
	}

	// Separator between sections: an empty line with background fill
	gapStyle := lipgloss.NewStyle().Width(m.width)
	if m.sidebarBg != nil {
		gapStyle = gapStyle.Background(m.sidebarBg)
	}
	gap := gapStyle.Render("")

	if m.height <= 0 {
		wrapped := make([]string, 0, len(blocks))
		for _, block := range blocks {
			wrapped = append(wrapped, blockStyle.Render(block))
		}
		return strings.Join(wrapped, "\n"+gap+"\n")
	}

	rendered := make([]string, 0, len(blocks))
	remaining := m.height
	for _, block := range blocks {
		if remaining <= 0 {
			break
		}
		block = trimToHeight(block, remaining)
		if strings.TrimSpace(block) == "" {
			break
		}
		wrapped := blockStyle.Render(block)
		rendered = append(rendered, wrapped)
		remaining -= lipgloss.Height(wrapped)
		if remaining > 1 {
			remaining -= 1 // gap line
		}
	}
	return strings.Join(rendered, "\n"+gap+"\n")
}

func (m Model) CompactView() string {
	t := theme.Current()
	sections := []Section{
		m.session,
		m.context,
		m.usage,
		m.channels,
		m.mcps,
		m.cron,
	}
	visible := filterSections(sections)
	if len(visible) == 0 {
		return ""
	}

	sectionWidth := m.width
	if len(visible) > 0 && m.width > 0 {
		sectionWidth = max(10, (m.width-len(visible)-1)/len(visible))
	}

	cols := make([]string, 0, len(visible))
	for _, s := range visible {
		block := renderCompactBlock(s, sectionWidth, t)
		cols = append(cols, lipgloss.NewStyle().Width(sectionWidth).Render(block))
	}
	return lipgloss.JoinHorizontal(lipgloss.Top, cols...)
}

func (m Model) getDynamicLimits() map[string]int {
	limits := map[string]int{
		"CHANNELS":  0,
		"MCPS":      0,
		"SCHEDULED": 0,
	}
	if m.height <= 0 {
		limits["CHANNELS"] = min(m.channels.ItemCount(), 8)
		limits["MCPS"] = min(m.mcps.ItemCount(), 8)
		limits["SCHEDULED"] = min(m.cron.ItemCount(), 6)
		return limits
	}

	type variableSection struct {
		title   string
		count   int
		cost    int
		ceiling int
	}
	var sections []variableSection
	if m.channels.ItemCount() > 0 {
		sections = append(sections, variableSection{title: "CHANNELS", count: m.channels.ItemCount(), cost: 1, ceiling: 8})
	}
	if m.mcps.ItemCount() > 0 {
		sections = append(sections, variableSection{title: "MCPS", count: m.mcps.ItemCount(), cost: 1, ceiling: 8})
	}
	if m.cron.ItemCount() > 0 {
		sections = append(sections, variableSection{title: "SCHEDULED", count: m.cron.ItemCount(), cost: 2, ceiling: 6})
	}

	fixedBlocks := []string{
		renderSectionBlock(m.session, m.width, 0, theme.Current()),
		renderSectionBlock(m.context, m.width, 0, theme.Current()),
		renderSectionBlock(m.usage, m.width, 0, theme.Current()),
	}
	if m.channels.ItemCount() == 0 {
		fixedBlocks = append(fixedBlocks, renderSectionBlock(m.channels, m.width, 0, theme.Current()))
	}
	if m.mcps.ItemCount() == 0 {
		fixedBlocks = append(fixedBlocks, renderSectionBlock(m.mcps, m.width, 0, theme.Current()))
	}
	if m.cron.ItemCount() == 0 {
		fixedBlocks = append(fixedBlocks, renderSectionBlock(m.cron, m.width, 0, theme.Current()))
	}

	fixedHeight := 0
	for _, block := range filterEmpty(fixedBlocks) {
		fixedHeight += lipgloss.Height(block)
	}
	totalSections := len(filterEmpty(fixedBlocks)) + len(sections)
	if totalSections > 1 {
		fixedHeight += (totalSections - 1) * 2
	}

	remaining := m.height - fixedHeight
	if remaining <= 0 || len(sections) == 0 {
		for _, section := range sections {
			limits[section.title] = min(section.count, minItemsPerSection)
		}
		return limits
	}

	perSection := remaining / len(sections)
	if perSection <= 0 {
		perSection = 1
	}
	for _, section := range sections {
		maxItems := min(section.count, section.ceiling)
		lines := min(perSection, maxItems*section.cost)
		items := max(minItemsPerSection, lines/section.cost)
		if items > maxItems {
			items = maxItems
		}
		limits[section.title] = items
		remaining -= items * section.cost
	}

	for _, title := range []string{"CHANNELS", "MCPS", "SCHEDULED"} {
		cost := 1
		ceiling := 8
		count := 0
		if title == "MCPS" {
			count = m.mcps.ItemCount()
		} else if title == "SCHEDULED" {
			count = m.cron.ItemCount()
			cost = 2
			ceiling = 6
		} else {
			count = m.channels.ItemCount()
		}
		if count == 0 {
			continue
		}
		maxItems := min(count, ceiling)
		for remaining >= cost && limits[title] < maxItems {
			limits[title]++
			remaining -= cost
		}
	}

	return limits
}

func renderPanelBlock(content string, width int, t *theme.Theme) string {
	content = strings.TrimRight(content, "\n")
	if content == "" {
		return ""
	}
	if width <= 0 {
		width = DefaultWidth
	}
	lines := strings.Split(content, "\n")
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		if lipgloss.Width(line) > width {
			line = ansi.Cut(line, 0, width-1) + "…"
		}
		if t != nil {
			line = lipgloss.NewStyle().Foreground(t.Text).Render(line)
		}
		out = append(out, line)
	}
	return strings.Join(out, "\n")
}

func renderSectionBlock(s Section, width, maxItems int, t *theme.Theme) string {
	header := renderSectionHeader(s.Title(), width, t)
	body := s.Render(width, maxItems, t)
	if header == "" {
		return body
	}
	if body == "" {
		return header
	}
	return header + "\n" + body
}

func renderCompactBlock(s Section, width int, t *theme.Theme) string {
	header := renderSectionHeader(s.Title(), width, t)
	body := s.Render(width, 3, t)
	if body == "" {
		return header
	}
	return header + "\n" + body
}

func filterEmpty(values []string) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			out = append(out, value)
		}
	}
	return out
}

func filterSections(values []Section) []Section {
	out := make([]Section, 0, len(values))
	for _, value := range values {
		if value != nil {
			out = append(out, value)
		}
	}
	return out
}

func trimToHeight(content string, height int) string {
	if height <= 0 || content == "" {
		return ""
	}
	lines := strings.Split(content, "\n")
	if len(lines) <= height {
		return content
	}
	return strings.Join(lines[:height], "\n")
}
