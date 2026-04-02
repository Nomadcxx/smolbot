package chat

import (
	"encoding/json"
	"fmt"
	"image/color"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"

	lipgloss "charm.land/lipgloss/v2"
	"github.com/Nomadcxx/smolbot/internal/format"
	"github.com/Nomadcxx/smolbot/internal/theme"
	"github.com/pmezard/go-difflib/difflib"
)

const maxTextWidth = 120

func cappedWidth(available int) int {
	if available <= 0 {
		return maxTextWidth
	}
	w := available - 2
	if w > maxTextWidth {
		return maxTextWidth
	}
	return max(20, w)
}

type ToolCall struct {
	ID     string
	Name   string
	Input  string
	Status string
	Output string
}

const maxToolOutputLines = 10
const maxToolOutputBytes = 4096

func renderToolCall(tc ToolCall, width int, expanded bool) string {
	t := theme.Current()
	if t == nil {
		return tc.Name
	}

	title := toolDisplayTitle(tc)
	if raw := strings.TrimSpace(tc.Name); raw != "" && !strings.Contains(title, raw) {
		title = fmt.Sprintf("%s (%s)", title, raw)
	}
	content := toolBlockContent(tc, width, expanded, t)
	state := toolBlockState(tc.Status)
	return RenderToolBlock(ToolBlockOpts{
		Title:        title,
		Content:      content,
		State:        state,
		Width:        width,
		SpinnerFrame: 0,
	}, t)
}

func toolIcon(status string, t *theme.Theme) (string, color.Color) {
	switch status {
	case "running":
		return "●", t.ToolStateRunning
	case "done":
		return "✓", t.ToolStateDone
	case "error":
		return "✗", t.ToolStateError
	default:
		return "•", t.TextMuted
	}
}

func toolBlockState(status string) ToolBlockState {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "running":
		return ToolBlockRunning
	case "done":
		return ToolBlockDone
	case "error":
		return ToolBlockError
	default:
		return ToolBlockDone
	}
}

func toolDisplayTitle(tc ToolCall) string {
	switch normalizeToolName(tc.Name) {
	case "read_file":
		return formatFileToolTitle("Read", extractJSONField(tc.Input, "path"))
	case "write_file":
		return formatFileToolTitle("Write", extractJSONField(tc.Input, "path"))
	case "edit_file":
		return formatFileToolTitle("Edit", extractJSONField(tc.Input, "path"))
	case "list_dir":
		path := strings.TrimSpace(extractJSONField(tc.Input, "path"))
		if path == "" {
			return "LIST DIR"
		}
		return "List " + filepath.Base(path)
	case "exec":
		return "Shell"
	case "web_search":
		query := strings.TrimSpace(extractJSONField(tc.Input, "query"))
		if query == "" {
			return "Web Search"
		}
		return "Search " + truncatePreview(query, 48)
	case "web_fetch":
		target := strings.TrimSpace(extractJSONField(tc.Input, "url"))
		if target == "" {
			return "Web Fetch"
		}
		return "Fetch " + truncatePreview(target, 48)
	case "message":
		return "Message"
	case "cron":
		return "Cron"
	default:
		return strings.ToUpper(strings.ReplaceAll(strings.TrimSpace(tc.Name), "_", " "))
	}
}

func formatFileToolTitle(verb, path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return strings.ToUpper(verb + " FILE")
	}
	base := filepath.Base(path)
	if base == "." || base == string(filepath.Separator) || base == "" {
		return verb + " " + path
	}
	return verb + " " + base
}

func toolBlockContent(tc ToolCall, width int, expanded bool, t *theme.Theme) string {
	switch normalizeToolName(tc.Name) {
	case "edit_file":
		if content, ok := renderEditToolContent(tc, width, expanded, t); ok {
			return content
		}
	}

	sections := make([]string, 0, 2)

	if input := toolInputSummary(tc.Name, tc.Input); strings.TrimSpace(input) != "" {
		sections = append(sections, "INPUT\n"+input)
	}

	if output := toolOutputSummary(tc.Status, tc.Output, expanded); strings.TrimSpace(output) != "" {
		sections = append(sections, toolOutputLabel(tc.Name, tc.Status)+"\n"+output)
	} else if strings.EqualFold(strings.TrimSpace(tc.Status), "running") {
		sections = append(sections, "STATUS\nrunning...")
	}

	return strings.Join(sections, "\n\n")
}

func renderEditToolContent(tc ToolCall, width int, expanded bool, t *theme.Theme) (string, bool) {
	fields, ok := parseJSONObject(strings.TrimSpace(tc.Input))
	if !ok {
		return "", false
	}

	oldText := coerceString(fields["old_string"])
	newText := coerceString(fields["new_string"])
	if strings.EqualFold(strings.TrimSpace(tc.Status), "done") && (oldText != "" || newText != "") {
		diff := generateEditDiff(extractJSONField(tc.Input, "path"), oldText, newText)
		content := RenderDiff(diff, max(24, cappedWidth(width)-4), t)
		if meta := summarizeToolFields(fields, []string{"path", "replace_all"}); strings.TrimSpace(meta) != "" {
			content = "INPUT\n" + meta + "\n\nDIFF\n" + content
		}
		return content, true
	}

	if strings.EqualFold(strings.TrimSpace(tc.Status), "running") {
		meta := summarizeToolFields(fields, []string{"path", "replace_all"})
		if strings.TrimSpace(meta) == "" {
			meta = "Preparing edit..."
		}
		return "INPUT\n" + meta + "\n\nSTATUS\nPreparing diff...", true
	}

	return "", false
}

func toolInputSummary(name, raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}

	fields, ok := parseJSONObject(raw)
	if !ok {
		return raw
	}

	switch normalizeToolName(name) {
	case "read_file":
		return summarizeToolFields(fields, []string{"path", "offset", "limit", "extraAllowedDirs"})
	case "write_file":
		return summarizeToolFields(fields, []string{"path", "content"})
	case "edit_file":
		return summarizeToolFields(fields, []string{"path", "replace_all"})
	case "list_dir":
		return summarizeToolFields(fields, []string{"path", "recursive", "max_depth"})
	case "exec":
		return summarizeToolFields(fields, []string{"command", "timeout"})
	case "web_search":
		return summarizeToolFields(fields, []string{"query", "maxResults"})
	case "web_fetch":
		return summarizeToolFields(fields, []string{"url"})
	case "message":
		return summarizeToolFields(fields, []string{"channel", "chat_id", "content"})
	case "cron":
		return summarizeToolFields(fields, []string{"action", "id", "name", "schedule", "timezone", "reminder", "channel", "chat_id", "isEnabled"})
	default:
		return prettyJSON(raw)
	}
}

func toolOutputSummary(status, output string, expanded bool) string {
	output = strings.TrimRight(strings.TrimSpace(output), "\n")
	if output == "" {
		if strings.EqualFold(strings.TrimSpace(status), "running") {
			return "running..."
		}
		return ""
	}

	// Apply error-specific truncation limits for error status.
	if strings.EqualFold(strings.TrimSpace(status), "error") {
		output = applyErrorTruncation(output)
	}

	if expanded {
		return output
	}

	if len(output) > maxToolOutputBytes {
		truncated := output[:maxToolOutputBytes]
		lastNewline := strings.LastIndex(truncated, "\n")
		if lastNewline > maxToolOutputBytes/2 {
			truncated = truncated[:lastNewline]
		}
		return truncated + fmt.Sprintf("\n… (%d bytes hidden)", len(output)-len(truncated))
	}

	lines := strings.Split(output, "\n")
	if len(lines) <= maxToolOutputLines {
		return output
	}

	hidden := len(lines) - maxToolOutputLines
	return strings.Join(lines[:maxToolOutputLines], "\n") + fmt.Sprintf("\n… (%d lines hidden)", hidden)
}

func toolOutputLabel(name, status string) string {
	if strings.EqualFold(strings.TrimSpace(status), "error") {
		return "ERROR"
	}

	switch normalizeToolName(name) {
	case "read_file", "web_fetch":
		return "CONTENT"
	case "write_file", "edit_file":
		return "RESULT"
	case "exec":
		return "OUTPUT"
	case "web_search":
		return "RESULTS"
	case "message":
		return "DELIVERY"
	default:
		return "OUTPUT"
	}
}

func summarizeToolFields(fields map[string]any, orderedKeys []string) string {
	if len(fields) == 0 {
		return ""
	}

	lines := make([]string, 0, len(fields))
	seen := make(map[string]struct{}, len(fields))
	appendField := func(key string) {
		value, ok := fields[key]
		if !ok {
			return
		}
		lines = append(lines, formatToolField(key, value))
		seen[key] = struct{}{}
	}

	for _, key := range orderedKeys {
		appendField(key)
	}

	extras := make([]string, 0, len(fields)-len(seen))
	for key := range fields {
		if _, ok := seen[key]; !ok {
			extras = append(extras, key)
		}
	}
	sort.Strings(extras)
	for _, key := range extras {
		appendField(key)
	}

	return strings.Join(lines, "\n")
}

func formatToolField(key string, value any) string {
	label := humanizeToolKey(key)
	return label + ": " + formatToolValue(value)
}

func formatToolValue(value any) string {
	switch v := value.(type) {
	case string:
		return truncatePreview(v, 160)
	case bool:
		if v {
			return "true"
		}
		return "false"
	case float64:
		if v == float64(int64(v)) {
			return strconv.FormatInt(int64(v), 10)
		}
		return strconv.FormatFloat(v, 'f', -1, 64)
	case []any:
		parts := make([]string, 0, len(v))
		for _, item := range v {
			parts = append(parts, formatToolValue(item))
		}
		return "[" + strings.Join(parts, ", ") + "]"
	case map[string]any:
		pretty, err := json.Marshal(v)
		if err != nil {
			return fmt.Sprint(v)
		}
		return string(pretty)
	default:
		pretty, err := json.Marshal(v)
		if err != nil {
			return fmt.Sprint(v)
		}
		return string(pretty)
	}
}

func truncatePreview(value string, limit int) string {
	if limit <= 0 {
		return value
	}
	runes := []rune(value)
	if len(runes) <= limit {
		return value
	}
	return string(runes[:limit]) + "…"
}

func parseJSONObject(raw string) (map[string]any, bool) {
	var fields map[string]any
	if err := json.Unmarshal([]byte(raw), &fields); err != nil {
		return nil, false
	}
	return fields, true
}

func prettyJSON(raw string) string {
	var value any
	if err := json.Unmarshal([]byte(raw), &value); err != nil {
		return raw
	}
	pretty, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return raw
	}
	return string(pretty)
}

func extractJSONField(raw, key string) string {
	fields, ok := parseJSONObject(strings.TrimSpace(raw))
	if !ok {
		return ""
	}
	return coerceString(fields[key])
}

func coerceString(value any) string {
	switch v := value.(type) {
	case string:
		return v
	default:
		return ""
	}
}

func generateEditDiff(path, oldText, newText string) string {
	path = strings.TrimSpace(path)
	label := path
	if label == "" {
		label = "file"
	}
	diff, err := difflib.GetUnifiedDiffString(difflib.UnifiedDiff{
		A:        difflib.SplitLines(ensureTrailingNewline(oldText)),
		B:        difflib.SplitLines(ensureTrailingNewline(newText)),
		FromFile: "a/" + label,
		ToFile:   "b/" + label,
		Context:  3,
	})
	if err != nil || strings.TrimSpace(diff) == "" {
		return strings.Join([]string{
			"--- a/" + label,
			"+++ b/" + label,
			"@@",
			"-" + oldText,
			"+" + newText,
		}, "\n")
	}
	return strings.TrimRight(diff, "\n")
}

func ensureTrailingNewline(value string) string {
	value = strings.ReplaceAll(value, "\r\n", "\n")
	if strings.HasSuffix(value, "\n") {
		return value
	}
	return value + "\n"
}

func normalizeToolName(name string) string {
	return strings.ToLower(strings.TrimSpace(name))
}

func humanizeToolKey(key string) string {
	key = strings.TrimSpace(key)
	if key == "" {
		return ""
	}

	parts := strings.Fields(strings.ReplaceAll(key, "_", " "))
	for i, part := range parts {
		parts[i] = camelToWords(part)
	}
	return strings.ToUpper(strings.Join(parts, " "))
}

func camelToWords(value string) string {
	var out []rune
	var prev rune
	for i, r := range value {
		if i > 0 && unicode.IsUpper(r) && (unicode.IsLower(prev) || unicode.IsDigit(prev)) {
			out = append(out, ' ')
		}
		out = append(out, r)
		prev = r
	}
	return string(out)
}

func renderRoleBlock(label, body string, accent color.Color, width int) string {
	t := theme.Current()
	if t == nil {
		return label + "\n" + body
	}
	if accent == nil {
		accent = t.Primary
	}
	innerWidth := cappedWidth(width)
	badge := lipgloss.NewStyle().
		Background(accent).
		Foreground(t.Background).
		Bold(true).
		Padding(0, 1).
		Render(label)

	header := lipgloss.NewStyle().
		Background(subtleWash(accent)).
		Width(innerWidth).
		Padding(0, 1).
		Render(badge)
	contentBody := lipgloss.NewStyle().
		Foreground(t.Text).
		Width(innerWidth).
		Padding(0, 1).
		Render(body)
	content := lipgloss.JoinVertical(lipgloss.Left, header, contentBody)
	style := lipgloss.NewStyle().
		Border(lipgloss.ThickBorder(), false, false, false, true).
		BorderForeground(accent).
		Padding(0, 0)
	if width > 4 {
		style = style.Width(width - 2)
	}
	return style.Render(content)
}

func subtleWash(accent color.Color) color.Color {
	if accent == nil {
		if t := theme.Current(); t != nil {
			return t.Background
		}
		return lipgloss.Color("#111111")
	}
	hex := colorHex(accent)
	if len(hex) != 7 || hex[0] != '#' {
		if t := theme.Current(); t != nil {
			return t.Background
		}
		return lipgloss.Color("#111111")
	}
	r, _ := strconv.ParseInt(hex[1:3], 16, 64)
	g, _ := strconv.ParseInt(hex[3:5], 16, 64)
	b, _ := strconv.ParseInt(hex[5:7], 16, 64)
	return lipgloss.Color(fmt.Sprintf("#%02X%02X%02X", int(r)/5, int(g)/5, int(b)/5))
}

func colorHex(value color.Color) string {
	r, g, b, _ := value.RGBA()
	return fmt.Sprintf("#%02X%02X%02X", uint8(r>>8), uint8(g>>8), uint8(b>>8))
}

func renderThinkingBlock(body string, dur time.Duration, accent color.Color, width int) string {
	t := theme.Current()
	if t == nil {
		return "THINKING\n" + body
	}
	if accent == nil {
		accent = t.Primary
	}
	innerWidth := cappedWidth(width)
	badge := lipgloss.NewStyle().
		Background(accent).
		Foreground(t.Background).
		Bold(true).
		Padding(0, 1).
		Render("THINKING")

	header := lipgloss.NewStyle().
		Background(subtleWash(accent)).
		Width(innerWidth).
		Padding(0, 1).
		Render(badge)

	bodyLines := strings.Split(body, "\n")
	truncHint := ""
	if len(bodyLines) > maxToolOutputLines {
		hidden := len(bodyLines) - maxToolOutputLines
		body = strings.Join(bodyLines[:maxToolOutputLines], "\n")
		truncHint = fmt.Sprintf("… (%d lines hidden)", hidden)
	}

	contentBody := lipgloss.NewStyle().
		Foreground(t.Text).
		Width(innerWidth).
		Padding(0, 1).
		Render(body)

	var rows []string
	rows = append(rows, header, contentBody)

	if truncHint != "" {
		rows = append(rows, lipgloss.NewStyle().
			Foreground(t.TextMuted).
			Italic(true).
			Width(innerWidth).
			Padding(0, 1).
			Render(truncHint))
	}

	if dur > 0 {
		footer := lipgloss.NewStyle().
			Background(subtleWash(accent)).
			Foreground(t.TextMuted).
			Width(innerWidth).
			Padding(0, 1).
			Render("Thought for " + formatDuration(dur))
		rows = append(rows, footer)
	}

	content := lipgloss.JoinVertical(lipgloss.Left, rows...)
	style := lipgloss.NewStyle().
		Border(lipgloss.ThickBorder(), false, false, false, true).
		BorderForeground(accent).
		Padding(0, 0)
	if width > 4 {
		style = style.Width(width - 2)
	}
	return style.Render(content)
}

func renderSystemMessage(body string, width int) string {
	t := theme.Current()
	if t == nil {
		return body
	}
	lineWidth := cappedWidth(width)
	divider := "─── system ───"
	style := lipgloss.NewStyle().
		Foreground(t.TextMuted).
		Width(lineWidth).
		Align(lipgloss.Center)
	content := lipgloss.JoinVertical(
		lipgloss.Left,
		style.Render(divider),
		style.Render(body),
	)
	if width <= 4 {
		return content
	}
	return lipgloss.NewStyle().Width(width - 2).Render(content)
}

func formatDuration(d time.Duration) string {
	if d < time.Second {
		return fmt.Sprintf("%dms", d.Milliseconds())
	}
	return fmt.Sprintf("%.1fs", d.Seconds())
}

// --- Phase 7: Compact Standalone Tool Renderers ---

// renderStandaloneToolCompact renders a single standalone tool with a brief summary.
// Verbose mode and unknown tools fall back to the full renderToolCall.
func renderStandaloneToolCompact(tc ToolCall, width int, t *theme.Theme) string {
if t == nil {
return tc.Name
}
switch normalizeToolName(tc.Name) {
case "write_file", "writefile", "file_write":
return renderWriteCompact(tc, t)
case "edit_file", "editfile", "file_edit":
return renderEditCompact(tc, t)
case "exec", "bash", "shell", "run":
return renderExecCompact(tc, t)
case "web_fetch", "fetch", "curl":
return renderFetchCompact(tc, t)
case "message":
return renderMessageCompact(tc, t)
case "spawn", "spawn_agent":
return renderSpawnCompact(tc, t)
default:
return renderGenericCompact(tc, t)
}
}

// toolIndicatorCompact returns the status glyph and its foreground color.
func toolIndicatorCompact(status string, t *theme.Theme) (string, color.Color) {
switch strings.ToLower(status) {
case "running":
return "●", t.Warning
case "error":
return "✗", t.Error
default:
return "✓", t.Success
}
}

func compactPrefix(status string, t *theme.Theme) string {
glyph, col := toolIndicatorCompact(status, t)
return lipgloss.NewStyle().Foreground(col).Render(glyph)
}

func renderWriteCompact(tc ToolCall, t *theme.Theme) string {
path := extractJSONField(tc.Input, "path")
baseName := filepath.Base(path)
if baseName == "" || baseName == "." {
baseName = path
}

content := extractJSONField(tc.Input, "content")
lineCount := 0
if content != "" {
lineCount = strings.Count(content, "\n") + 1
}

var verb string
switch strings.ToLower(tc.Status) {
case "running":
verb = "Writing"
case "error":
verb = "Failed to write"
default:
verb = "Wrote"
}

meta := ""
if lineCount > 0 {
meta = " (" + format.FormatTokens(lineCount) + " lines)"
}
return compactPrefix(tc.Status, t) + " " + fmt.Sprintf("%s %s%s", verb, baseName, meta)
}

func renderEditCompact(tc ToolCall, t *theme.Theme) string {
path := extractJSONField(tc.Input, "path")
baseName := filepath.Base(path)
if baseName == "" || baseName == "." {
baseName = path
}

var verb string
switch strings.ToLower(tc.Status) {
case "running":
verb = "Editing"
case "error":
verb = "Failed to edit"
default:
verb = "Edited"
}

oldText := extractJSONField(tc.Input, "old_string")
newText := extractJSONField(tc.Input, "new_string")
delta := strings.Count(newText, "\n") - strings.Count(oldText, "\n")

meta := ""
if delta > 0 {
meta = fmt.Sprintf(" (+%d lines)", delta)
} else if delta < 0 {
meta = fmt.Sprintf(" (%d lines)", delta)
}
return compactPrefix(tc.Status, t) + " " + fmt.Sprintf("%s %s%s", verb, baseName, meta)
}

func renderExecCompact(tc ToolCall, t *theme.Theme) string {
command := extractJSONField(tc.Input, "command")
const maxCmdLen = 50
if len(command) > maxCmdLen {
command = command[:maxCmdLen-3] + "..."
}

var verb string
switch strings.ToLower(tc.Status) {
case "running":
verb = "Running"
case "error":
verb = "Failed"
default:
verb = "Ran"
}

meta := ""
if tc.Output != "" && strings.ToLower(tc.Status) != "running" {
outputLines := strings.Count(tc.Output, "\n")
if outputLines > 0 {
meta = fmt.Sprintf(" (%d lines)", outputLines)
}
}

cmdStyled := lipgloss.NewStyle().Foreground(t.TextMuted).Render("`" + command + "`")
return compactPrefix(tc.Status, t) + " " + fmt.Sprintf("%s %s%s", verb, cmdStyled, meta)
}

func renderFetchCompact(tc ToolCall, t *theme.Theme) string {
url := extractJSONField(tc.Input, "url")
const maxURLLen = 60
if len(url) > maxURLLen {
url = url[:maxURLLen-3] + "..."
}

var verb string
switch strings.ToLower(tc.Status) {
case "running":
verb = "Fetching"
case "error":
verb = "Failed to fetch"
default:
verb = "Fetched"
}

meta := ""
if len(tc.Output) > 0 && strings.ToLower(tc.Status) != "running" {
meta = " (" + format.FormatFileSize(int64(len(tc.Output))) + ")"
}
return compactPrefix(tc.Status, t) + " " + fmt.Sprintf("%s %s%s", verb, url, meta)
}

func renderMessageCompact(tc ToolCall, t *theme.Theme) string {
channel := extractJSONField(tc.Input, "channel")
chatID := extractJSONField(tc.Input, "chat_id")
if len(chatID) > 20 {
chatID = chatID[:17] + "..."
}

var verb string
switch strings.ToLower(tc.Status) {
case "running":
verb = "Sending to"
case "error":
verb = "Failed to send to"
default:
verb = "Sent to"
}

target := channel
if chatID != "" {
target = channel + ":" + chatID
}
return compactPrefix(tc.Status, t) + " " + fmt.Sprintf("%s %s", verb, target)
}

func renderGenericCompact(tc ToolCall, t *theme.Theme) string {
var text string
switch strings.ToLower(tc.Status) {
case "running":
text = "Running " + tc.Name
case "error":
text = tc.Name + " failed"
default:
text = tc.Name + " completed"
}
return compactPrefix(tc.Status, t) + " " + text
}

func renderSpawnCompact(tc ToolCall, t *theme.Theme) string {
agentType := extractJSONField(tc.Input, "type")
if agentType == "" {
agentType = extractJSONField(tc.Input, "agent_type")
}
if agentType == "" {
agentType = "agent"
}
var verb string
switch strings.ToLower(tc.Status) {
case "running":
verb = "Spawning"
case "error":
verb = "Failed to spawn"
default:
verb = "Spawned"
}
return compactPrefix(tc.Status, t) + " " + fmt.Sprintf("%s %s", verb, agentType)
}
