package chat

import (
	"regexp"
	"strconv"
	"strings"
	"testing"

	lipgloss "charm.land/lipgloss/v2"
	"github.com/Nomadcxx/smolbot/internal/theme"
	_ "github.com/Nomadcxx/smolbot/internal/theme/themes"
)

var ansiEscape = regexp.MustCompile(`\x1b\[[0-9;]*m`)

func stripANSI(s string) string {
	return ansiEscape.ReplaceAllString(s, "")
}

func TestMessagesModelRendersToolLifecycle(t *testing.T) {
	if !theme.Set("nord") {
		t.Fatal("expected nord theme to be registered")
	}

	model := NewMessages()
	model.SetSize(80, 20)
	model.AppendUser("hello")
	model.StartTool("read_file", "")
	model.FinishTool("read_file", "done", "loaded config")

	view := model.View()
	if !strings.Contains(view, "read_file") {
		t.Fatalf("expected tool name in view, got %q", view)
	}
	if !strings.Contains(view, "loaded config") {
		t.Fatalf("expected tool output in view, got %q", view)
	}
}

func TestMarkdownStyleConfigUsesSemanticThemeTokens(t *testing.T) {
	useSemanticMarkdownTheme(t)

	current := theme.Current()
	if current == nil {
		t.Fatal("expected a current theme")
	}

	cfg := markdownStyleConfig()

	assertStyleColor(t, "heading", cfg.H1.Color, colorHex(current.MarkdownHeading))
	assertStyleColor(t, "link", cfg.Link.Color, colorHex(current.MarkdownLink))
	assertStyleColor(t, "inline code", cfg.Code.Color, colorHex(current.MarkdownCode))
	assertStyleColor(t, "code background", cfg.Code.BackgroundColor, colorHex(current.Background))
	assertStyleColor(t, "keyword", cfg.CodeBlock.Chroma.Keyword.Color, colorHex(current.SyntaxKeyword))
	assertStyleColor(t, "string", cfg.CodeBlock.Chroma.LiteralString.Color, colorHex(current.SyntaxString))
	assertStyleColor(t, "comment", cfg.CodeBlock.Chroma.Comment.Color, colorHex(current.SyntaxComment))
}

func TestAssistantMarkdownRenderingUsesSemanticChrome(t *testing.T) {
	useSemanticMarkdownTheme(t)

	model := NewMessages()
	model.SetSize(80, 20)
	rendered := model.renderAssistant("# Heading\n\nUse `code` and [docs](https://example.com).\n\n```go\n// comment\nfmt.Println(\"hi\")\n```")

	current := theme.Current()
	if current == nil {
		t.Fatal("expected a current theme")
	}

	if !strings.Contains(rendered, "Heading") {
		t.Fatalf("expected heading text in render, got %q", rendered)
	}
	if !strings.Contains(rendered, "docs") {
		t.Fatalf("expected link text in render, got %q", rendered)
	}
	if !strings.Contains(rendered, "Println") || !strings.Contains(rendered, "\"hi\"") {
		t.Fatalf("expected code block content in render, got %q", rendered)
	}
	if !strings.Contains(rendered, ansiFg(colorHex(current.MarkdownHeading))) {
		t.Fatalf("expected heading to use semantic heading color, got %q", rendered)
	}
	if !strings.Contains(rendered, ansiFg(colorHex(current.MarkdownLink))) {
		t.Fatalf("expected link to use semantic link color, got %q", rendered)
	}
	if !strings.Contains(rendered, ansiFg(colorHex(current.SyntaxComment))) {
		t.Fatalf("expected code block comments to use semantic comment color, got %q", rendered)
	}
}

func TestMessageRowsHaveDistinctStructure(t *testing.T) {
	if !theme.Set("nord") {
		t.Fatal("expected nord theme to be registered")
	}

	model := NewMessages()
	model.SetSize(80, 20)
	model.AppendUser("hello from user")
	model.AppendAssistant("hello from assistant")
	model.AppendError("boom")

	view := model.View()
	if !strings.Contains(view, "USER") {
		t.Fatalf("expected user row label, got %q", view)
	}
	if !strings.Contains(view, "ASSISTANT") {
		t.Fatalf("expected assistant row label, got %q", view)
	}
	if !strings.Contains(view, "ERROR") {
		t.Fatalf("expected error row label, got %q", view)
	}
	if !strings.Contains(view, "│") {
		t.Fatalf("expected structured row chrome, got %q", view)
	}
}

func TestMessagesModelRendersReadableProgressAndThinkingRows(t *testing.T) {
	if !theme.Set("nord") {
		t.Fatal("expected nord theme to be registered")
	}

	model := NewMessages()
	model.SetSize(80, 20)
	model.AppendUser("working")
	model.SetProgress("Streaming response chunks")
	model.SetThinking("Reviewing files")

	view := model.View()
	if !strings.Contains(view, "STREAM") {
		t.Fatalf("expected progress row label, got %q", view)
	}
	if !strings.Contains(view, "Streaming response chunks") {
		t.Fatalf("expected progress content, got %q", view)
	}
	if !strings.Contains(view, "THINKING") {
		t.Fatalf("expected thinking row label, got %q", view)
	}
	if !strings.Contains(view, "Reviewing files") {
		t.Fatalf("expected thinking content, got %q", view)
	}
}

func TestToolBlocksUseSemanticChromeByStatus(t *testing.T) {
	if !theme.Set("nord") {
		t.Fatal("expected nord theme to be registered")
	}

	running := renderToolCall(ToolCall{Name: "search", Status: "running"}, 80)
	done := renderToolCall(ToolCall{Name: "search", Status: "done"}, 80)
	failed := renderToolCall(ToolCall{Name: "search", Status: "error"}, 80)

	current := theme.Current()
	if current == nil {
		t.Fatal("expected a current theme")
	}

	if !strings.Contains(running, ansiBg(colorHex(current.ToolStateRunning))) {
		t.Fatalf("expected running tool block to use semantic running chrome, got %q", running)
	}
	if !strings.Contains(done, ansiBg(colorHex(current.ToolStateDone))) {
		t.Fatalf("expected done tool block to use semantic done chrome, got %q", done)
	}
	if !strings.Contains(failed, ansiBg(colorHex(current.ToolStateError))) {
		t.Fatalf("expected error tool block to use semantic error chrome, got %q", failed)
	}
}

func TestRoleBlocksLimitAccentWashToHeaderBand(t *testing.T) {
	if !theme.Set("nord") {
		t.Fatal("expected nord theme to be registered")
	}

	block := renderRoleBlock("USER", "hello from user wraps maybe", theme.Current().Primary, 40)
	lines := strings.Split(block, "\n")
	if len(lines) < 2 {
		t.Fatalf("expected multi-line role block, got %q", block)
	}
	headerBg := ansiBg(colorHex(subtleWash(theme.Current().Primary)))
	panelBg := ansiBg(colorHex(theme.Current().Panel))

	if !strings.Contains(lines[0], headerBg) {
		t.Fatalf("expected header band to use subtle role tint, got %q", lines[0])
	}
	if strings.Contains(lines[1], headerBg) {
		t.Fatalf("expected body row to avoid role tint background, got %q", lines[1])
	}
	if !strings.Contains(lines[1], panelBg) {
		t.Fatalf("expected body row to stay on black panel background, got %q", lines[1])
	}
}

func useSemanticMarkdownTheme(t *testing.T) {
	t.Helper()

	if !theme.Set("nord") {
		t.Fatal("expected nord theme to be registered")
	}
	base := *theme.Current()
	previous := base.Name

	const name = "chat-semantic-markdown-test"
	base.Name = name
	base.Background = lipgloss.Color("#000000")
	base.Text = lipgloss.Color("#F8FAFC")
	base.TextMuted = lipgloss.Color("#94A3B8")
	base.MarkdownHeading = lipgloss.Color("#FF7A90")
	base.MarkdownLink = lipgloss.Color("#62D0FF")
	base.MarkdownCode = lipgloss.Color("#C792EA")
	base.SyntaxKeyword = lipgloss.Color("#FFB86C")
	base.SyntaxString = lipgloss.Color("#A3E635")
	base.SyntaxComment = lipgloss.Color("#7DD3FC")

	theme.Register(&base)
	if !theme.Set(name) {
		t.Fatalf("expected to set semantic markdown theme %q", name)
	}

	t.Cleanup(func() {
		if previous != "" {
			theme.Set(previous)
		}
	})
}

func assertStyleColor(t *testing.T, label string, got *string, want string) {
	t.Helper()
	if got == nil {
		t.Fatalf("expected %s color to be set", label)
	}
	if *got != want {
		t.Fatalf("expected %s color %q, got %q", label, want, *got)
	}
}

func TestThinkingContentPersistsAfterAssistantResponse(t *testing.T) {
	if !theme.Set("nord") { t.Fatal("expected nord theme") }
	model := NewMessages()
	model.SetSize(80, 40)
	model.AppendUser("what is 2+2?")
	model.AppendThinking("Let me reason through this: 2+2 equals 4.")
	model.AppendAssistant("The answer is 4.")
	view := model.View()
	if !strings.Contains(view, "THINKING") {
		t.Fatalf("expected THINKING label in view after assistant responded, got %q", view)
	}
	if !strings.Contains(view, "Let me reason through this") {
		t.Fatalf("expected thinking content to persist in view, got %q", view)
	}
	if !strings.Contains(stripANSI(view), "The answer is 4.") {
		t.Fatalf("expected assistant response also visible, got %q", view)
	}
}

func ansiFg(hex string) string {
	if len(hex) != 7 || hex[0] != '#' {
		return ""
	}
	return "38;2;" + parseHexPair(hex[1:3]) + ";" + parseHexPair(hex[3:5]) + ";" + parseHexPair(hex[5:7])
}

func ansiBg(hex string) string {
	if len(hex) != 7 || hex[0] != '#' {
		return ""
	}
	return "48;2;" + parseHexPair(hex[1:3]) + ";" + parseHexPair(hex[3:5]) + ";" + parseHexPair(hex[5:7])
}

func parseHexPair(pair string) string {
	value, err := strconv.ParseInt(pair, 16, 64)
	if err != nil {
		return pair
	}
	return strconv.FormatInt(value, 10)
}
