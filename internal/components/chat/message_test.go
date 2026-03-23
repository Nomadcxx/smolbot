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

func TestToolBlocksUseSemanticStates(t *testing.T) {
	if !theme.Set("nord") {
		t.Fatal("expected nord theme to be registered")
	}

	running := renderToolCall(ToolCall{Name: "search", Status: "running"}, 80, false)
	done := renderToolCall(ToolCall{Name: "search", Status: "done"}, 80, false)
	failed := renderToolCall(ToolCall{Name: "search", Status: "error"}, 80, false)

	if !strings.Contains(running, "RUNNING") {
		t.Fatalf("expected running state label, got %q", running)
	}
	if !strings.Contains(done, "DONE") {
		t.Fatalf("expected done state label, got %q", done)
	}
	if !strings.Contains(failed, "ERROR") {
		t.Fatalf("expected error state label, got %q", failed)
	}
	if !strings.Contains(running, "\x1b[48;") || !strings.Contains(done, "\x1b[48;") || !strings.Contains(failed, "\x1b[48;") {
		t.Fatalf("expected semantic state pills to use background styling")
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

	headerBg := ansiBgHex(colorHex(current.ToolArtifactHeader))
	bodyBg := ansiBgHex(colorHex(current.ToolArtifactBody))
	statusBg := ansiBgHex(colorHex(current.ToolStateDone))
	panelBg := ansiBgHex(colorHex(current.Panel))
	successBg := ansiBgHex(colorHex(current.Success))

	if !strings.Contains(rendered, "exec_command") {
		t.Fatalf("expected tool name in render, got %q", rendered)
	}
	if !strings.Contains(rendered, "stdout: build complete") {
		t.Fatalf("expected tool output in render, got %q", rendered)
	}
	if !strings.Contains(rendered, headerBg) {
		t.Fatalf("expected tool header to use semantic artifact header background, got %q", rendered)
	}
	if !strings.Contains(rendered, bodyBg) {
		t.Fatalf("expected tool body to use semantic artifact body background, got %q", rendered)
	}
	if !strings.Contains(rendered, statusBg) {
		t.Fatalf("expected tool status chip to use semantic done state background, got %q", rendered)
	}
	if strings.Contains(rendered, panelBg) {
		t.Fatalf("expected tool artifact to avoid generic panel surface, got %q", rendered)
	}
	if strings.Contains(rendered, successBg) {
		t.Fatalf("expected tool status chip to avoid generic success background, got %q", rendered)
	}
}

func TestTranscriptRoleBlocksUseSemanticThemeTokens(t *testing.T) {
	useSemanticTranscriptTheme(t)

	model := NewMessages()
	model.SetSize(72, 20)
	model.AppendUser(strings.Repeat("user cards should stay on the same black surface ", 2))
	model.AppendAssistant(strings.Repeat("assistant cards should be calmer but still wrap cleanly ", 2))

	rendered := model.renderContent()
	blocks := strings.Split(strings.TrimSpace(rendered), "\n\n")
	if len(blocks) != 2 {
		t.Fatalf("expected two role blocks, got %d from %q", len(blocks), rendered)
	}

	current := theme.Current()
	if current == nil {
		t.Fatal("expected a current theme")
	}

	panelBg := ansiBgHex(colorHex(current.Panel))
	userBandBg := ansiBgHex(colorHex(subtleWash(current.TranscriptUserAccent)))
	assistantBandBg := ansiBgHex(colorHex(subtleWash(current.TranscriptAssistantAccent)))
	primaryBandBg := ansiBgHex(colorHex(subtleWash(current.Primary)))
	secondaryBandBg := ansiBgHex(colorHex(subtleWash(current.Secondary)))

	assertRoleBlockSurface(t, "USER", blocks[0], panelBg, userBandBg, primaryBandBg)
	assertRoleBlockSurface(t, "ASSISTANT", blocks[1], panelBg, assistantBandBg, secondaryBandBg)
	if userBandBg == assistantBandBg {
		t.Fatalf("expected user and assistant role bands to differ, got %q", userBandBg)
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
