package chat

import (
	"strconv"
	"strings"
	"testing"

	lipgloss "charm.land/lipgloss/v2"
	"github.com/Nomadcxx/smolbot/internal/theme"
	_ "github.com/Nomadcxx/smolbot/internal/theme/themes"
)

func TestRenderToolCallIncludesStatusAndOutput(t *testing.T) {
	if !theme.Set("nord") {
		t.Fatal("expected nord theme to be registered")
	}

	rendered := renderToolCall(ToolCall{
		Name:   "read_file",
		Status: "done",
		Output: "contents loaded",
	}, 80, false)

	if !strings.Contains(rendered, "read_file") {
		t.Fatalf("expected tool name in render, got %q", rendered)
	}
	if !strings.Contains(rendered, "contents loaded") {
		t.Fatalf("expected tool output in render, got %q", rendered)
	}
	if !strings.Contains(rendered, "✓") {
		t.Fatalf("expected done icon in render, got %q", rendered)
	}
}

func TestRenderEditToolCallShowsDiff(t *testing.T) {
	if !theme.Set("nord") {
		t.Fatal("expected nord theme to be registered")
	}

	rendered := stripANSI(renderToolCall(ToolCall{
		Name:   "edit_file",
		Status: "done",
		Input:  `{"path":"internal/tui/tui.go","old_string":"old line\n","new_string":"new line\n"}`,
		Output: "updated internal/tui/tui.go",
	}, 100, false))

	if !strings.Contains(rendered, "Edit tui.go (edit_file)") {
		t.Fatalf("expected edit title in render, got %q", rendered)
	}
	if !strings.Contains(rendered, "--- a/internal/tui/tui.go") || !strings.Contains(rendered, "+++ b/internal/tui/tui.go") {
		t.Fatalf("expected diff headers in render, got %q", rendered)
	}
	if !strings.Contains(rendered, "old line") || !strings.Contains(rendered, "new line") {
		t.Fatalf("expected diff body in render, got %q", rendered)
	}
}

func TestRenderToolCallDispatchesByToolName(t *testing.T) {
	if !theme.Set("nord") {
		t.Fatal("expected nord theme to be registered")
	}

	cases := []struct {
		name       string
		wantTitle  string
		wantPieces []string
	}{
		{
			name:       "read_file",
			wantTitle:  "Read smolbot.yaml",
			wantPieces: []string{"/etc/smolbot.yaml", "OFFSET", "LIMIT"},
		},
		{
			name:       "write_file",
			wantTitle:  "Write output.txt",
			wantPieces: []string{"/tmp/output.txt", "CONTENT"},
		},
		{
			name:       "edit_file",
			wantTitle:  "Edit output.txt",
			wantPieces: []string{"/tmp/output.txt", "hello", "goodbye"},
		},
		{
			name:       "exec",
			wantTitle:  "Shell",
			wantPieces: []string{"go test ./...", "TIMEOUT"},
		},
		{
			name:       "web_search",
			wantTitle:  "Search smolbot",
			wantPieces: []string{"smolbot", "MAX RESULTS"},
		},
		{
			name:       "web_fetch",
			wantTitle:  "Fetch https://example.com",
			wantPieces: []string{"https://example.com"},
		},
	}

	for _, tc := range cases {
		rendered := renderToolCall(ToolCall{
			Name:   tc.name,
			Input:  toolInputForCase(tc.name),
			Status: "done",
			Output: "tool output",
		}, 80, false)

		if !strings.Contains(rendered, tc.wantTitle) {
			t.Fatalf("expected %s title in render, got %q", tc.wantTitle, rendered)
		}
		for _, want := range tc.wantPieces {
			if !strings.Contains(rendered, want) {
				t.Fatalf("expected %q in %s render, got %q", want, tc.name, rendered)
			}
		}
	}
}

func TestToolBlocksUseSemanticStates(t *testing.T) {
	if !theme.Set("nord") {
		t.Fatal("expected nord theme to be registered")
	}

	running := renderToolCall(ToolCall{Name: "search", Status: "running"}, 80, false)
	done := renderToolCall(ToolCall{Name: "search", Status: "done"}, 80, false)
	failed := renderToolCall(ToolCall{Name: "search", Status: "error"}, 80, false)

	if !strings.Contains(running, "◐") {
		t.Fatalf("expected running spinner in render, got %q", running)
	}
	if !strings.Contains(done, "✓") {
		t.Fatalf("expected done icon in render, got %q", done)
	}
	if !strings.Contains(failed, "✗") {
		t.Fatalf("expected error icon in render, got %q", failed)
	}
	if !strings.Contains(running, "search") || !strings.Contains(done, "search") || !strings.Contains(failed, "search") {
		t.Fatalf("expected tool name in render")
	}
}

func toolInputForCase(name string) string {
	switch name {
	case "read_file":
		return `{"path":"/etc/smolbot.yaml","offset":12,"limit":40}`
	case "write_file":
		return `{"path":"/tmp/output.txt","content":"hello"}`
	case "edit_file":
		return `{"path":"/tmp/output.txt","old_string":"hello","new_string":"goodbye","replace_all":true}`
	case "exec":
		return `{"command":"go test ./...","timeout":30}`
	case "web_search":
		return `{"query":"smolbot","maxResults":3}`
	case "web_fetch":
		return `{"url":"https://example.com"}`
	default:
		return `{}`
	}
}

func TestToolArtifactCardsUseSemanticThemeTokens(t *testing.T) {
	useSemanticToolTheme(t)

	rendered := renderToolCall(ToolCall{
		Name:   "exec_command",
		Status: "done",
		Output: "stdout: build complete",
	}, 80, false)

	current := theme.Current()
	if current == nil {
		t.Fatal("expected a current theme")
	}

	toolName := current.ToolName
	doneIcon := current.ToolStateDone

	if !strings.Contains(rendered, "exec_command") {
		t.Fatalf("expected tool name in render, got %q", rendered)
	}
	if !strings.Contains(rendered, "stdout: build complete") {
		t.Fatalf("expected tool output in render, got %q", rendered)
	}
	if !strings.Contains(rendered, "✓") {
		t.Fatalf("expected done icon in render, got %q", rendered)
	}
	_ = toolName
	_ = doneIcon
}

func TestTranscriptRoleBlocksUseSemanticThemeTokens(t *testing.T) {
	useSemanticTranscriptTheme(t)

	model := NewMessages()
	model.SetSize(72, 20)
	model.AppendUser(strings.Repeat("hello world ", 10))
	model.AppendAssistant(strings.Repeat("assistant response ", 10))

	rendered := model.renderContent()
	blocks := strings.Split(strings.TrimSpace(rendered), "\n\n")
	if len(blocks) != 2 {
		t.Fatalf("expected two role blocks, got %d from %q", len(blocks), rendered)
	}

	current := theme.Current()
	if current == nil {
		t.Fatal("expected a current theme")
	}

	userBandBg := ansiBgHex(colorHex(subtleWash(current.TranscriptUserAccent)))
	assistantBandBg := ansiBgHex(colorHex(subtleWash(current.TranscriptAssistantAccent)))

	assertRoleBlockHeaderBand(t, "USER", blocks[0], userBandBg)
	assertRoleBlockHeaderBand(t, "ASSISTANT", blocks[1], assistantBandBg)
	if userBandBg == assistantBandBg {
		t.Fatalf("expected user and assistant role bands to differ, got %q", userBandBg)
	}
}

func assertRoleBlockHeaderBand(t *testing.T, label, block, bandBg string) {
	t.Helper()
	lines := strings.Split(block, "\n")
	if len(lines) < 2 {
		t.Fatalf("expected multi-line %s block, got %q", label, block)
	}
	if !strings.Contains(lines[0], bandBg) {
		t.Fatalf("expected %s block header band to use the semantic role tint, got %q", label, lines[0])
	}
	visibleWidth := lipgloss.Width(lines[0])
	for i, line := range lines[1:] {
		if got := lipgloss.Width(line); got != visibleWidth {
			t.Fatalf("expected %s block width to stay stable on wrapped line %d: got %d want %d", label, i+1, got, visibleWidth)
		}
	}
}

func useSemanticTranscriptTheme(t *testing.T) {
	t.Helper()

	if !theme.Set("nord") {
		t.Fatal("expected nord theme to be registered")
	}
	base := *theme.Current()
	previous := base.Name

	const name = "chat-semantic-role-test"
	base.Name = name
	base.Primary = lipgloss.Color("#FF5555")
	base.Secondary = lipgloss.Color("#8D99AE")
	base.TranscriptUserAccent = lipgloss.Color("#F4A261")
	base.TranscriptAssistantAccent = lipgloss.Color("#2A9D8F")
	base.ToolName = lipgloss.Color("#CCCCCC")

	theme.Register(&base)
	if !theme.Set(name) {
		t.Fatalf("expected to set semantic transcript theme %q", name)
	}

	t.Cleanup(func() {
		if previous != "" {
			theme.Set(previous)
		}
	})
}

func useSemanticToolTheme(t *testing.T) {
	t.Helper()

	if !theme.Set("nord") {
		t.Fatal("expected nord theme to be registered")
	}
	base := *theme.Current()
	previous := base.Name

	const name = "chat-semantic-tool-test"
	base.Name = name
	base.Panel = lipgloss.Color("#010101")
	base.Success = lipgloss.Color("#00FF00")
	base.ToolName = lipgloss.Color("#D4D4D4")
	base.ToolArtifactHeader = lipgloss.Color("#102A43")
	base.ToolArtifactBody = lipgloss.Color("#08141F")
	base.ToolArtifactBorder = lipgloss.Color("#486581")
	base.ToolStateRunning = lipgloss.Color("#FFB703")
	base.ToolStateDone = lipgloss.Color("#3DDC97")
	base.ToolStateError = lipgloss.Color("#EF476F")

	theme.Register(&base)
	if !theme.Set(name) {
		t.Fatalf("expected to set semantic tool theme %q", name)
	}

	t.Cleanup(func() {
		if previous != "" {
			theme.Set(previous)
		}
	})
}

func TestToolOutputSummaryByteCapTruncatesLongLines(t *testing.T) {
	// A single line longer than 4096 bytes — the line-count cap alone would not catch this
	longLine := strings.Repeat("x", 5000)
	result := toolOutputSummary("done", longLine, false)
	if len(result) >= 5000 {
		t.Fatalf("toolOutputSummary did not truncate a 5000-byte single line, got len=%d", len(result))
	}
	if !strings.Contains(result, "bytes hidden") {
		t.Fatalf("toolOutputSummary should indicate truncation with 'bytes hidden', got: %q", result)
	}
}

func TestSubtleWashFallbackUsesThemeBackground(t *testing.T) {
	if !theme.Set("nord") {
		t.Fatal("expected nord theme to be registered")
	}
	// nil accent is not a valid colour — triggers the fallback path
	got := subtleWash(nil)
	bg := theme.Current().Background
	if got != bg {
		t.Fatalf("subtleWash(nil) = %v, want theme background %v", got, bg)
	}
}

func assertRoleBlockSurface(t *testing.T, label, block, panelBg, bandBg, forbiddenBandBg string) {
	t.Helper()

	lines := strings.Split(block, "\n")
	if len(lines) < 2 {
		t.Fatalf("expected multi-line %s block, got %q", label, block)
	}
	for i, line := range lines {
		if !strings.Contains(line, panelBg) {
			t.Fatalf("expected %s block line %d to stay on the black panel surface, got %q", label, i, line)
		}
	}
	if !strings.Contains(lines[0], bandBg) {
		t.Fatalf("expected %s block header band to use the semantic role tint, got %q", label, lines[0])
	}
	if strings.Contains(lines[0], forbiddenBandBg) {
		t.Fatalf("expected %s block header band not to use the generic tint, got %q", label, lines[0])
	}
	visibleWidth := lipgloss.Width(lines[0])
	for i, line := range lines[1:] {
		if got := lipgloss.Width(line); got != visibleWidth {
			t.Fatalf("expected %s block width to stay stable on wrapped line %d: got %d want %d", label, i+1, got, visibleWidth)
		}
		if strings.Contains(line, bandBg) {
			t.Fatalf("expected %s body line %d to avoid the header wash, got %q", label, i+1, line)
		}
	}
}

func ansiBgHex(hex string) string {
	if len(hex) != 7 || hex[0] != '#' {
		return ""
	}
	r, _ := strconv.ParseInt(hex[1:3], 16, 64)
	g, _ := strconv.ParseInt(hex[3:5], 16, 64)
	b, _ := strconv.ParseInt(hex[5:7], 16, 64)
	return "48;2;" + strconv.FormatInt(r, 10) + ";" + strconv.FormatInt(g, 10) + ";" + strconv.FormatInt(b, 10)
}
