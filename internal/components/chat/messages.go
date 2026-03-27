package chat

import (
	"image/color"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	viewport "charm.land/bubbles/v2/viewport"
	glamour "charm.land/glamour/v2"
	"charm.land/glamour/v2/ansi"
	lipgloss "charm.land/lipgloss/v2"
	"github.com/Nomadcxx/smolbot/internal/theme"
	uv "github.com/charmbracelet/ultraviolet"
	xansi "github.com/charmbracelet/x/ansi"
)

type ChatMessage struct {
	Role     string
	Content  string
	Duration time.Duration
}

type messageCache struct {
	role      string
	content   string
	duration  time.Duration
	width     int
	signature string
	rendered  string
}

type MessagesModel struct {
	messages      []ChatMessage
	tools         []ToolCall
	width         int
	height        int
	progress      string
	thinking      string
	thinkingStart time.Time
	viewport      viewport.Model
	rendered      string
	dirty         bool
	renderer      *glamour.TermRenderer
	rendererWidth int
	rendererStyle string
	expandedTools map[string]bool
	cache         []messageCache
	messageBody   string
	plainLines    []string
	plainOffsets  []int
	selection     selectionRange
}

type selectionPoint struct {
	line int
	col  int
}

type selectionRange struct {
	active bool
	anchor selectionPoint
	focus  selectionPoint
}

func NewMessages() MessagesModel {
	vp := viewport.New()
	vp.SoftWrap = true
	vp.FillHeight = true
	return MessagesModel{
		viewport:      vp,
		dirty:         true,
		expandedTools: make(map[string]bool),
	}
}

func (m *MessagesModel) SetSize(w, h int) {
	if m.width != w {
		m.cache = nil
		m.messageBody = ""
	}
	m.width = w
	m.height = h
	m.viewport.SetWidth(max(1, w))
	m.viewport.SetHeight(max(1, h))
	m.dirty = true
}

func (m *MessagesModel) AppendUser(content string) {
	m.messages = append(m.messages, ChatMessage{Role: "user", Content: content})
	m.progress = ""
	m.thinking = ""
	if len(m.cache) != len(m.messages)-1 {
		m.messageBody = ""
	}
	m.sync(true)
}

func (m *MessagesModel) AppendAssistant(content string) {
	m.messages = append(m.messages, ChatMessage{Role: "assistant", Content: content})
	m.progress = ""
	m.thinking = ""
	m.tools = nil
	if len(m.cache) != len(m.messages)-1 {
		m.messageBody = ""
	}
	m.sync(true)
}

func (m *MessagesModel) AppendError(content string) {
	m.messages = append(m.messages, ChatMessage{Role: "error", Content: content})
	m.progress = ""
	m.thinking = ""
	if len(m.cache) != len(m.messages)-1 {
		m.messageBody = ""
	}
	m.sync(true)
}

func (m *MessagesModel) AppendSystem(content string) {
	if strings.TrimSpace(content) == "" {
		return
	}
	m.messages = append(m.messages, ChatMessage{Role: "system", Content: content})
	m.progress = ""
	m.thinking = ""
	if len(m.cache) != len(m.messages)-1 {
		m.messageBody = ""
	}
	m.sync(m.viewport.AtBottom())
}

func (m *MessagesModel) AppendThinking(content string) {
	if strings.TrimSpace(content) == "" {
		return
	}
	dur := time.Duration(0)
	if !m.thinkingStart.IsZero() {
		dur = time.Since(m.thinkingStart)
		m.thinkingStart = time.Time{}
	}
	m.messages = append(m.messages, ChatMessage{Role: "thinking", Content: content, Duration: dur})
	if len(m.cache) != len(m.messages)-1 {
		m.messageBody = ""
	}
	m.sync(m.viewport.AtBottom())
}

func (m *MessagesModel) SetProgress(content string) {
	m.progress = content
	m.sync(m.viewport.AtBottom())
}

// SetThinking sets ephemeral in-progress thinking text shown during an active
// streaming run. It is cleared when the run completes. For finalized thinking
// content that should persist in the transcript, use AppendThinking instead.
func (m *MessagesModel) SetThinking(content string) {
	if m.thinking == "" && content != "" {
		m.thinkingStart = time.Now()
	}
	m.thinking = content
	m.sync(m.viewport.AtBottom())
}

func (m *MessagesModel) GetThinking() string {
	return m.thinking
}

func (m *MessagesModel) GetProgress() string {
	return m.progress
}

func (m *MessagesModel) ReplaceHistory(history []ChatMessage) {
	m.messages = append([]ChatMessage(nil), history...)
	m.tools = nil
	m.progress = ""
	m.thinking = ""
	m.cache = nil
	m.messageBody = ""
	m.clearSelection()
	m.sync(true)
}

func (m *MessagesModel) StartTool(id, name, input string) {
	m.tools = append(m.tools, ToolCall{ID: id, Name: name, Input: input, Status: "running"})
	m.sync(m.viewport.AtBottom())
}

func (m *MessagesModel) FinishTool(id, name, status, output string) {
	for i := len(m.tools) - 1; i >= 0; i-- {
		if m.tools[i].ID == id {
			m.tools[i].Status = status
			m.tools[i].Output = output
			m.sync(m.viewport.AtBottom())
			return
		}
	}
	for i := len(m.tools) - 1; i >= 0; i-- {
		if m.tools[i].Name == name && m.tools[i].Status == "running" {
			m.tools[i].Status = status
			m.tools[i].Output = output
			m.sync(m.viewport.AtBottom())
			return
		}
	}
	m.tools = append(m.tools, ToolCall{ID: id, Name: name, Status: status, Output: output})
	m.sync(m.viewport.AtBottom())
}

func (m *MessagesModel) ToggleToolExpand(index int) {
	key := strconv.Itoa(index)
	m.expandedTools[key] = !m.expandedTools[key]
	m.dirty = true
}

func (m *MessagesModel) ScrollToBottom() {
	m.sync(true)
}

func (m *MessagesModel) Height() int {
	return m.height
}

func (m *MessagesModel) HandleMouseDown(x, y int) bool {
	point, ok := m.selectionPointAt(x, y)
	if !ok {
		return false
	}
	m.selection = selectionRange{
		active: true,
		anchor: point,
		focus:  point,
	}
	m.dirty = true
	return true
}

func (m *MessagesModel) HandleMouseDrag(x, y int) bool {
	if !m.selection.active {
		return false
	}
	point, ok := m.selectionPointAt(x, y)
	if !ok {
		return false
	}
	m.selection.focus = point
	m.dirty = true
	return true
}

func (m *MessagesModel) HandleMouseUp(x, y int) bool {
	if !m.selection.active {
		return false
	}
	point, ok := m.selectionPointAt(x, y)
	if ok {
		m.selection.focus = point
	}
	m.dirty = true
	return m.HasSelection()
}

func (m *MessagesModel) HasSelection() bool {
	if !m.selection.active || len(m.plainLines) == 0 {
		return false
	}
	start, end := m.normalizedSelection()
	return start.line != end.line || start.col != end.col
}

func (m *MessagesModel) SelectedText() string {
	if !m.HasSelection() {
		return ""
	}
	start, end := m.normalizedSelection()
	if start.line < 0 || end.line < 0 || start.line >= len(m.plainLines) || end.line >= len(m.plainLines) {
		return ""
	}

	var out strings.Builder
	for line := start.line; line <= end.line; line++ {
		if line > start.line {
			out.WriteByte('\n')
		}
		text := m.plainLines[line]
		from := 0
		to := runeCount(text)
		if line == start.line {
			from = clampInt(start.col, 0, to)
		}
		if line == end.line {
			to = clampInt(end.col, 0, to)
		}
		if from < to {
			out.WriteString(sliceRunes(text, from, to))
		}
	}
	return out.String()
}

func (m *MessagesModel) ClearSelection() {
	m.clearSelection()
	m.dirty = true
}

func (m *MessagesModel) clearSelection() {
	m.selection = selectionRange{}
}

func (m *MessagesModel) thinkingDuration() time.Duration {
	if m.thinkingStart.IsZero() {
		return 0
	}
	return time.Since(m.thinkingStart)
}

func (m *MessagesModel) selectionPointAt(x, y int) (selectionPoint, bool) {
	if len(m.plainLines) == 0 {
		return selectionPoint{}, false
	}
	line := m.viewport.YOffset() + max(0, y)
	if line < 0 || line >= len(m.plainLines) {
		return selectionPoint{}, false
	}
	text := m.plainLines[line]
	offset := 0
	if line < len(m.plainOffsets) {
		offset = m.plainOffsets[line]
	}
	col := clampInt(max(0, x-offset), 0, runeCount(text))
	return selectionPoint{line: line, col: col}, true
}

func (m *MessagesModel) normalizedSelection() (selectionPoint, selectionPoint) {
	start := m.selection.anchor
	end := m.selection.focus
	if end.line < start.line || (end.line == start.line && end.col < start.col) {
		start, end = end, start
	}
	return start, end
}

func (m *MessagesModel) ViewportOffset() int {
	return m.viewport.YOffset()
}

func (m *MessagesModel) HasContentAbove() bool {
	return m.viewport.YOffset() > 0
}

func (m *MessagesModel) InvalidateTheme() {
	m.renderer = nil
	m.cache = nil
	m.messageBody = ""
	m.dirty = true
}

func (m MessagesModel) LastAssistantContent() string {
	for i := len(m.messages) - 1; i >= 0; i-- {
		if m.messages[i].Role == "assistant" && strings.TrimSpace(m.messages[i].Content) != "" {
			return m.messages[i].Content
		}
	}
	return ""
}

func (m *MessagesModel) IsDirty() bool {
	return m.dirty
}

func (m *MessagesModel) HandleKey(key string) {
	switch key {
	case "pgup":
		m.viewport.PageUp()
	case "pgdown":
		m.viewport.PageDown()
	case "home":
		m.viewport.GotoTop()
	case "end", "ctrl+l":
		m.viewport.GotoBottom()
	}
}

func (m *MessagesModel) sync(follow bool) {
	offset := m.viewport.YOffset()
	m.rendered, m.plainLines, m.plainOffsets = m.renderTranscript()
	m.viewport.SetContent(m.rendered)
	if follow {
		m.viewport.GotoBottom()
	} else {
		m.viewport.SetYOffset(offset)
	}
	m.dirty = false
}

func (m *MessagesModel) renderTranscript() (string, []string, []int) {
	t := theme.Current()
	if t == nil {
		return "", nil, nil
	}

	blocks := make([]string, 0, len(m.messages)+len(m.tools)+2)
	signature := transcriptRenderSignature()
	for i, msg := range m.messages {
		blocks = append(blocks, m.renderCachedMessage(i, msg, t, signature))
	}
	if m.progress != "" {
		blocks = append(blocks, renderRoleBlock("ASSISTANT", m.renderAssistant(m.progress), t.TranscriptStreaming, m.width))
	}
	if m.thinking != "" {
		duration := m.thinkingDuration()
		blocks = append(blocks, renderThinkingBlock(m.thinking, duration, t.TranscriptThinking, m.width))
	}
	for i, tool := range m.tools {
		expanded := m.expandedTools[strconv.Itoa(i)]
		blocks = append(blocks, renderToolCall(tool, m.width, expanded))
	}

	rendered := strings.Join(blocks, "\n\n")
	lines, offsets := visibleTranscriptLines(rendered)
	if m.selection.active && len(lines) > 0 {
		start, end := m.normalizedSelection()
		rendered = highlightRenderedTranscript(rendered, offsets, start, end)
		lines, offsets = visibleTranscriptLines(rendered)
	}
	return rendered, lines, offsets
}

func visibleTranscriptLines(rendered string) ([]string, []int) {
	if rendered == "" {
		return nil, nil
	}
	rawLines := strings.Split(strings.ReplaceAll(rendered, "\r\n", "\n"), "\n")
	lines := make([]string, len(rawLines))
	offsets := make([]int, len(rawLines))
	for i, line := range rawLines {
		stripped := strings.TrimRight(xansi.Strip(line), " ")
		offset := transcriptLineOffset(stripped)
		offsets[i] = offset
		lines[i] = strings.TrimRight(sliceRunes(stripped, offset, runeCount(stripped)), " ")
	}
	return lines, offsets
}

func highlightRenderedTranscript(rendered string, offsets []int, start, end selectionPoint) string {
	lines, computedOffsets := visibleTranscriptLines(rendered)
	if len(lines) == 0 {
		return rendered
	}
	if len(offsets) != len(lines) {
		offsets = computedOffsets
	}

	width := 0
	for _, line := range lines {
		width = max(width, lipgloss.Width(line))
	}
	if width == 0 {
		return rendered
	}

	buf := uv.NewScreenBuffer(width, len(lines))
	uv.NewStyledString(rendered).Draw(&buf, uv.Rect(0, 0, width, len(lines)))

	for y := start.line; y <= end.line && y < len(lines); y++ {
		if y < 0 {
			continue
		}
		lineWidth := lipgloss.Width(lines[y])
		colStart := 0
		if y == start.line {
			colStart = clampInt(start.col, 0, lineWidth)
		}
		colEnd := lineWidth
		if y == end.line {
			colEnd = clampInt(end.col, 0, lineWidth)
		}
		if colStart >= colEnd {
			continue
		}
		lineOffset := 0
		if y < len(offsets) {
			lineOffset = offsets[y]
		}
		for x := colStart; x < colEnd; x++ {
			cell := buf.Line(y).At(x + lineOffset)
			if cell == nil || cell.Content == "" || cell.Content == " " {
				continue
			}
			applySelectionHighlight(cell)
		}
	}
	return buf.Render()
}

func transcriptLineOffset(line string) int {
	switch {
	case strings.HasPrefix(line, "┃  "), strings.HasPrefix(line, "│  "):
		return 3
	case strings.HasPrefix(line, "┃ "), strings.HasPrefix(line, "│ "), strings.HasPrefix(line, "▌ "):
		return 2
	default:
		return 0
	}
}

func applySelectionHighlight(cell *uv.Cell) {
	t := theme.Current()
	if t == nil || cell == nil {
		return
	}
	cell.Style.Fg = t.Background
	cell.Style.Bg = t.Accent
	cell.Style.Attrs |= uv.AttrBold
}

func plainMessageBlock(msg ChatMessage, width int) []string {
	switch msg.Role {
	case "user":
		return plainRoleBlock("USER", msg.Content, width)
	case "assistant":
		return plainRoleBlock("ASSISTANT", msg.Content, width)
	case "system":
		return plainSystemBlock(msg.Content, width)
	case "error":
		return plainErrorBlock(msg.Content, width)
	case "thinking":
		return plainThinkingBlock(msg.Content, msg.Duration, width)
	default:
		return wrapPlainText(msg.Content, max(20, width-2))
	}
}

func plainRoleBlock(label, body string, width int) []string {
	lines := []string{label}
	lines = append(lines, wrapPlainText(body, max(20, width-2))...)
	return trimTrailingEmpty(lines)
}

func plainSystemBlock(body string, width int) []string {
	lines := []string{"SYSTEM"}
	lines = append(lines, wrapPlainText(body, max(20, width-2))...)
	return trimTrailingEmpty(lines)
}

func plainErrorBlock(body string, width int) []string {
	lines := []string{"ERROR"}
	lines = append(lines, wrapPlainText(body, max(20, width-2))...)
	return trimTrailingEmpty(lines)
}

func plainThinkingBlock(body string, dur time.Duration, width int) []string {
	lines := []string{"THINKING"}
	lines = append(lines, wrapPlainText(body, max(20, width-2))...)
	if dur > 0 {
		lines = append(lines, "Thought for "+formatDuration(dur))
	}
	return trimTrailingEmpty(lines)
}

func plainToolBlock(tc ToolCall, expanded bool, width int) []string {
	lines := []string{toolDisplayTitle(tc)}
	input := toolInputSummary(tc.Name, tc.Input)
	if strings.TrimSpace(input) != "" {
		lines = append(lines, "INPUT")
		lines = append(lines, wrapPlainText(input, max(20, width-2))...)
	}
	output := toolOutputSummary(tc.Status, tc.Output, expanded)
	if strings.TrimSpace(output) != "" {
		lines = append(lines, toolOutputLabel(tc.Name, tc.Status))
		lines = append(lines, wrapPlainText(output, max(20, width-2))...)
	} else if strings.EqualFold(strings.TrimSpace(tc.Status), "running") {
		lines = append(lines, "STATUS")
		lines = append(lines, "running...")
	}
	return trimTrailingEmpty(lines)
}

func wrapPlainText(body string, width int) []string {
	body = strings.ReplaceAll(body, "\r\n", "\n")
	if width <= 0 {
		return strings.Split(body, "\n")
	}

	paras := strings.Split(body, "\n")
	lines := make([]string, 0, len(paras))
	for _, para := range paras {
		if para == "" {
			lines = append(lines, "")
			continue
		}
		lines = append(lines, wrapParagraph(para, width)...)
	}
	return trimTrailingEmpty(lines)
}

func wrapParagraph(text string, width int) []string {
	words := strings.Fields(text)
	if len(words) == 0 {
		return []string{""}
	}

	lines := make([]string, 0, len(words))
	current := words[0]
	for _, word := range words[1:] {
		candidate := current + " " + word
		if displayWidth(candidate) <= width {
			current = candidate
			continue
		}
		lines = append(lines, current)
		if displayWidth(word) > width {
			chunks := hardWrapWord(word, width)
			if len(chunks) > 0 {
				current = chunks[len(chunks)-1]
				if len(chunks) > 1 {
					lines = append(lines, chunks[:len(chunks)-1]...)
				}
				continue
			}
		}
		current = word
	}
	lines = append(lines, current)
	return lines
}

func hardWrapWord(word string, width int) []string {
	if width <= 0 {
		return []string{word}
	}
	runes := []rune(word)
	if len(runes) == 0 {
		return []string{""}
	}
	chunks := make([]string, 0, (len(runes)/width)+1)
	for len(runes) > 0 {
		end := minInt(len(runes), width)
		chunks = append(chunks, string(runes[:end]))
		runes = runes[end:]
	}
	return chunks
}

func trimTrailingEmpty(lines []string) []string {
	for len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	return lines
}

func displayWidth(value string) int {
	return lipgloss.Width(value)
}

func clampInt(value, minValue, maxValue int) int {
	if value < minValue {
		return minValue
	}
	if value > maxValue {
		return maxValue
	}
	return value
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func runeCount(value string) int {
	return utf8.RuneCountInString(value)
}

func sliceRunes(value string, from, to int) string {
	if from < 0 {
		from = 0
	}
	runes := []rune(value)
	if from > len(runes) {
		from = len(runes)
	}
	if to > len(runes) {
		to = len(runes)
	}
	if from >= to {
		return ""
	}
	return string(runes[from:to])
}

type selectionSpan struct {
	start int
	end   int
}

func selectionSpanForLine(text string, line int, start, end selectionPoint) (selectionSpan, bool) {
	if line < start.line || line > end.line {
		return selectionSpan{}, false
	}
	from := 0
	to := runeCount(text)
	if line == start.line {
		from = clampInt(start.col, 0, to)
	}
	if line == end.line {
		to = clampInt(end.col, 0, to)
	}
	if from == to {
		return selectionSpan{}, false
	}
	return selectionSpan{start: from, end: to}, true
}

func renderSelectableRoleBlock(label string, bodyLines []string, accent color.Color, width int, selections map[int]selectionSpan) string {
	t := theme.Current()
	if t == nil {
		return label + "\n" + strings.Join(bodyLines, "\n")
	}

	innerWidth := cappedWidth(width)
	headerText := renderSelectableText(label, selections[0], t.Background, true)
	badge := lipgloss.NewStyle().
		Background(accent).
		Foreground(t.Background).
		Bold(true).
		Padding(0, 1).
		Render(headerText)
	header := lipgloss.NewStyle().
		Background(subtleWash(accent)).
		Width(innerWidth).
		Padding(0, 1).
		Render(badge)

	body := make([]string, 0, len(bodyLines))
	for i, line := range bodyLines {
		body = append(body, lipgloss.NewStyle().
			Width(innerWidth).
			Padding(0, 1).
			Render(renderSelectableText(line, selections[i+1], t.Text, false)))
	}

	content := lipgloss.JoinVertical(lipgloss.Left, append([]string{header}, body...)...)
	style := lipgloss.NewStyle().
		Border(lipgloss.ThickBorder(), false, false, false, true).
		BorderForeground(accent)
	if width > 4 {
		style = style.Width(width - 2)
	}
	return style.Render(content)
}

func renderSelectableMessageBlock(label string, bodyLines []string, accent color.Color, width int, boldLabel bool, selections map[int]selectionSpan) string {
	t := theme.Current()
	if t == nil {
		return label + "\n" + strings.Join(bodyLines, "\n")
	}

	head := lipgloss.NewStyle().
		Foreground(accent).
		Bold(boldLabel).
		Render(renderSelectableText(label, selections[0], accent, false))
	bodyRows := make([]string, 0, len(bodyLines)+1)
	bodyRows = append(bodyRows, head)
	for i, line := range bodyLines {
		bodyRows = append(bodyRows, renderSelectableText(line, selections[i+1], t.Text, false))
	}

	style := lipgloss.NewStyle().
		Border(lipgloss.NormalBorder(), false, false, false, true).
		BorderForeground(accent).
		Padding(0, 1).
		Width(cappedWidth(width))
	return style.Render(lipgloss.JoinVertical(lipgloss.Left, bodyRows...))
}

func renderSelectableSystemBlock(bodyLines []string, width int, selections map[int]selectionSpan) string {
	t := theme.Current()
	if t == nil {
		return strings.Join(bodyLines, "\n")
	}

	lineWidth := cappedWidth(width)
	rows := []string{
		lipgloss.NewStyle().
			Foreground(t.TextMuted).
			Width(lineWidth).
			Align(lipgloss.Center).
			Render("─── system ───"),
	}
	for i, line := range bodyLines {
		rows = append(rows, lipgloss.NewStyle().
			Foreground(t.TextMuted).
			Width(lineWidth).
			Align(lipgloss.Center).
			Render(renderSelectableText(line, selections[i+1], t.TextMuted, false)))
	}
	if width <= 4 {
		return lipgloss.JoinVertical(lipgloss.Left, rows...)
	}
	return lipgloss.NewStyle().Width(width - 2).Render(lipgloss.JoinVertical(lipgloss.Left, rows...))
}

func renderSelectableText(text string, span selectionSpan, normal color.Color, preserveBackground bool) string {
	t := theme.Current()
	if t == nil {
		return text
	}

	if span.end <= span.start {
		return lipgloss.NewStyle().Foreground(normal).Render(text)
	}

	total := runeCount(text)
	start := clampInt(span.start, 0, total)
	end := clampInt(span.end, 0, total)
	if start >= end {
		return lipgloss.NewStyle().Foreground(normal).Render(text)
	}

	base := lipgloss.NewStyle().Foreground(normal)
	selected := lipgloss.NewStyle().
		Foreground(t.Background).
		Background(t.Accent).
		Bold(true)
	if preserveBackground {
		selected = selected.Background(t.Panel)
	}

	before := sliceRunes(text, 0, start)
	middle := sliceRunes(text, start, end)
	after := sliceRunes(text, end, total)
	return base.Render(before) + selected.Render(middle) + base.Render(after)
}

func (m *MessagesModel) renderContent() string {
	t := theme.Current()
	if t == nil {
		return ""
	}

	sections := make([]string, 0, len(m.tools)+3)
	if body := m.renderMessageBody(t); body != "" {
		sections = append(sections, body)
	}
	if m.progress != "" {
		sections = append(sections, renderRoleBlock("ASSISTANT", m.renderAssistant(m.progress), t.TranscriptStreaming, m.width))
	}
	if m.thinking != "" {
		sections = append(sections, renderRoleBlock("THINKING", m.thinking, t.TranscriptThinking, m.width))
	}
	for i, tool := range m.tools {
		expanded := m.expandedTools[strconv.Itoa(i)]
		sections = append(sections, renderToolCall(tool, m.width, expanded))
	}
	return strings.Join(sections, "\n\n")
}

func (m *MessagesModel) renderMessageBody(t *theme.Theme) string {
	signature := transcriptRenderSignature()
	if len(m.cache) > len(m.messages) {
		m.cache = m.cache[:len(m.messages)]
	}
	if len(m.messages) == 0 {
		m.messageBody = ""
		return ""
	}
	if len(m.cache) == len(m.messages) && m.messageBody != "" {
		return m.messageBody
	}
	if len(m.cache) == len(m.messages)-1 && m.messageBody != "" {
		block := m.renderCachedMessage(len(m.messages)-1, m.messages[len(m.messages)-1], t, signature)
		m.messageBody = m.messageBody + "\n\n" + block
		return m.messageBody
	}

	blocks := make([]string, 0, len(m.messages))
	for i, msg := range m.messages {
		blocks = append(blocks, m.renderCachedMessage(i, msg, t, signature))
	}
	m.cache = m.cache[:len(m.messages)]
	m.messageBody = strings.Join(blocks, "\n\n")
	return m.messageBody
}

func (m *MessagesModel) renderCachedMessage(index int, msg ChatMessage, t *theme.Theme, signature string) string {
	if index < len(m.cache) {
		entry := m.cache[index]
		if entry.role == msg.Role &&
			entry.content == msg.Content &&
			entry.duration == msg.Duration &&
			entry.width == m.width &&
			entry.signature == signature {
			return entry.rendered
		}
	}

	rendered := m.renderMessage(msg, t)
	entry := messageCache{
		role:      msg.Role,
		content:   msg.Content,
		duration:  msg.Duration,
		width:     m.width,
		signature: signature,
		rendered:  rendered,
	}
	if index < len(m.cache) {
		m.cache[index] = entry
	} else {
		m.cache = append(m.cache, entry)
	}
	return rendered
}

func (m *MessagesModel) renderMessage(msg ChatMessage, t *theme.Theme) string {
	switch msg.Role {
	case "user":
		return renderRoleBlock("USER", msg.Content, t.TranscriptUserAccent, m.width)
	case "assistant":
		return renderRoleBlock("ASSISTANT", m.renderAssistant(msg.Content), t.TranscriptAssistantAccent, m.width)
	case "system":
		return renderSystemMessage(msg.Content, m.width)
	case "error":
		return renderMessageBlock("ERROR", msg.Content, t.Error, m.width)
	case "thinking":
		return renderThinkingBlock(msg.Content, msg.Duration, t.TranscriptThinking, m.width)
	default:
		return msg.Content
	}
}

func (m *MessagesModel) renderAssistant(content string) string {
	renderer := m.markdownRenderer()
	if renderer == nil || strings.TrimSpace(content) == "" {
		return content
	}
	rendered, err := renderer.Render(content)
	if err != nil {
		return content
	}
	return strings.TrimSpace(rendered)
}

func (m *MessagesModel) markdownRenderer() *glamour.TermRenderer {
	width := max(20, m.width-2)
	signature := markdownSignature()
	if m.renderer != nil && m.rendererWidth == width && m.rendererStyle == signature {
		return m.renderer
	}
	renderer, err := glamour.NewTermRenderer(
		glamour.WithStyles(markdownStyleConfig()),
		glamour.WithWordWrap(width),
		glamour.WithPreservedNewLines(),
		glamour.WithChromaFormatter("terminal16m"),
	)
	if err != nil {
		return nil
	}
	m.renderer = renderer
	m.rendererWidth = width
	m.rendererStyle = signature
	return m.renderer
}

func renderMessageBlock(label, body string, accent color.Color, width int) string {
	t := theme.Current()
	if t == nil {
		return label + "\n" + body
	}
	head := lipgloss.NewStyle().
		Foreground(accent).
		Bold(true).
		Render(label)
	content := lipgloss.JoinVertical(lipgloss.Left, head, body)
	style := lipgloss.NewStyle().
		Border(lipgloss.NormalBorder(), false, false, false, true).
		BorderForeground(accent).
		Padding(0, 1)
	style = style.Width(cappedWidth(width))
	return style.Render(content)
}

func markdownSignature() string {
	current := theme.Current()
	if current == nil {
		return "default"
	}
	return strings.Join([]string{
		current.Name,
		colorHex(current.Text),
		colorHex(current.TextMuted),
		colorHex(current.MarkdownHeading),
		colorHex(current.MarkdownLink),
		colorHex(current.MarkdownCode),
		colorHex(current.SyntaxKeyword),
		colorHex(current.SyntaxString),
		colorHex(current.SyntaxComment),
	}, ":")
}

func transcriptRenderSignature() string {
	current := theme.Current()
	if current == nil {
		return "default"
	}
	return strings.Join([]string{
		markdownSignature(),
		colorHex(current.TranscriptUserAccent),
		colorHex(current.TranscriptAssistantAccent),
		colorHex(current.TranscriptThinking),
		colorHex(current.TranscriptStreaming),
		colorHex(current.Error),
	}, ":")
}

func markdownStyleConfig() ansi.StyleConfig {
	current := theme.Current()
	if current == nil {
		return ansi.StyleConfig{}
	}
	background := colorPtr(colorHex(current.Background))
	text := colorPtr(colorHex(current.Text))
	muted := colorPtr(colorHex(current.TextMuted))
	heading := colorPtr(colorHex(current.MarkdownHeading))
	link := colorPtr(colorHex(current.MarkdownLink))
	code := colorPtr(colorHex(current.MarkdownCode))
	keyword := colorPtr(colorHex(current.SyntaxKeyword))
	stringColor := colorPtr(colorHex(current.SyntaxString))
	comment := colorPtr(colorHex(current.SyntaxComment))

	return ansi.StyleConfig{
		Document: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{
				Color:           text,
				BackgroundColor: background,
			},
		},
		BlockQuote: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{
				Color:  muted,
				Italic: boolPtr(true),
				Prefix: "┃ ",
			},
			Indent:      uintPtr(1),
			IndentToken: stringPtr(" "),
		},
		List: ansi.StyleList{
			LevelIndent: 2,
			StyleBlock: ansi.StyleBlock{
				IndentToken: stringPtr(" "),
				StylePrimitive: ansi.StylePrimitive{
					Color: text,
				},
			},
		},
		Heading: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{
				Color: heading,
				Bold:  boolPtr(true),
			},
		},
		H1: headingBlock("# ", heading),
		H2: headingBlock("## ", heading),
		H3: headingBlock("### ", heading),
		H4: headingBlock("#### ", heading),
		H5: headingBlock("##### ", heading),
		H6: headingBlock("###### ", heading),
		Emph: ansi.StylePrimitive{
			Color:  heading,
			Italic: boolPtr(true),
		},
		Strong: ansi.StylePrimitive{
			Color: text,
			Bold:  boolPtr(true),
		},
		HorizontalRule: ansi.StylePrimitive{
			Color:  muted,
			Format: "\n─────────────────────────────────────────\n",
		},
		Item: ansi.StylePrimitive{
			BlockPrefix: "• ",
			Color:       link,
		},
		Enumeration: ansi.StylePrimitive{
			BlockPrefix: ". ",
			Color:       link,
		},
		Task: ansi.StyleTask{
			Ticked:   "[✓] ",
			Unticked: "[ ] ",
		},
		Link: ansi.StylePrimitive{
			Color:     link,
			Underline: boolPtr(true),
		},
		LinkText: ansi.StylePrimitive{
			Color: link,
			Bold:  boolPtr(true),
		},
		Code: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{
				Color:           code,
				BackgroundColor: background,
			},
		},
		CodeBlock: ansi.StyleCodeBlock{
			StyleBlock: ansi.StyleBlock{
				StylePrimitive: ansi.StylePrimitive{
					Color:           code,
					BackgroundColor: background,
					Prefix:          " ",
				},
			},
			Chroma: &ansi.Chroma{
				Background:       ansi.StylePrimitive{BackgroundColor: background},
				Text:             ansi.StylePrimitive{BackgroundColor: background, Color: text},
				Error:            ansi.StylePrimitive{BackgroundColor: background, Color: colorPtr(colorHex(current.TranscriptError))},
				Comment:          ansi.StylePrimitive{BackgroundColor: background, Color: comment},
				Keyword:          ansi.StylePrimitive{BackgroundColor: background, Color: keyword},
				KeywordReserved:  ansi.StylePrimitive{BackgroundColor: background, Color: keyword},
				KeywordNamespace: ansi.StylePrimitive{BackgroundColor: background, Color: keyword},
				KeywordType:      ansi.StylePrimitive{BackgroundColor: background, Color: keyword},
				Name:             ansi.StylePrimitive{BackgroundColor: background, Color: text},
				NameBuiltin:      ansi.StylePrimitive{BackgroundColor: background, Color: link},
				NameFunction:     ansi.StylePrimitive{BackgroundColor: background, Color: link},
				LiteralString:    ansi.StylePrimitive{BackgroundColor: background, Color: stringColor},
				LiteralNumber:    ansi.StylePrimitive{BackgroundColor: background, Color: stringColor},
				Operator:         ansi.StylePrimitive{BackgroundColor: background, Color: text},
				Punctuation:      ansi.StylePrimitive{BackgroundColor: background, Color: muted},
			},
		},
	}
}

func (m *MessagesModel) View() string {
	if m.dirty {
		m.sync(m.viewport.AtBottom() || m.viewport.TotalLineCount() == 0)
	}
	if m.width <= 0 || m.height <= 0 {
		return m.rendered
	}
	if strings.TrimSpace(m.rendered) == "" {
		return strings.Repeat("\n", max(0, m.height-1))
	}
	return m.viewport.View()
}

func headingBlock(prefix string, color *string) ansi.StyleBlock {
	return ansi.StyleBlock{
		StylePrimitive: ansi.StylePrimitive{
			Prefix: prefix,
			Color:  color,
			Bold:   boolPtr(true),
		},
	}
}

func colorPtr(hex string) *string {
	return &hex
}

func boolPtr(value bool) *bool {
	return &value
}

func stringPtr(value string) *string {
	return &value
}

func uintPtr(value uint) *uint {
	return &value
}
