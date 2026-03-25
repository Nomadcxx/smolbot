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
	model.StartTool("tc1", "read_file", "")
	model.FinishTool("tc1", "read_file", "done", "loaded config")

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
	if !strings.Contains(view, "ASSISTANT") {
		t.Fatalf("expected progress rendered as ASSISTANT block, got %q", view)
	}
	if !strings.Contains(stripANSI(view), "Streaming response chunks") {
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

	running := renderToolCall(ToolCall{Name: "search", Status: "running"}, 80, false)
	done := renderToolCall(ToolCall{Name: "search", Status: "done"}, 80, false)
	failed := renderToolCall(ToolCall{Name: "search", Status: "error"}, 80, false)

	if !strings.Contains(running, "●") {
		t.Fatalf("expected running icon, got %q", running)
	}
	if !strings.Contains(done, "✓") {
		t.Fatalf("expected done icon, got %q", done)
	}
	if !strings.Contains(failed, "✗") {
		t.Fatalf("expected error icon, got %q", failed)
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

	if !strings.Contains(lines[0], headerBg) {
		t.Fatalf("expected header band to use subtle role tint, got %q", lines[0])
	}
	if strings.Contains(lines[1], headerBg) {
		t.Fatalf("expected body row to avoid role tint background, got %q", lines[1])
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

func TestHasContentAboveReflectsScrollPosition(t *testing.T) {
	if !theme.Set("nord") { t.Fatal("expected nord theme") }
	model := NewMessages()
	model.SetSize(80, 10)
	for i := 0; i < 30; i++ {
		model.AppendAssistant("message " + strconv.Itoa(i))
	}
	model.HandleKey("end")
	if !model.HasContentAbove() {
		t.Fatal("expected HasContentAbove() = true after scrolling to bottom (content scrolled above)")
	}
	model.HandleKey("home")
	if model.HasContentAbove() {
		t.Fatal("expected HasContentAbove() = false at top (nothing scrolled above)")
	}
}

func TestToolInputIsDisplayedInToolBlock(t *testing.T) {
	if !theme.Set("nord") { t.Fatal("expected nord theme") }
	model := NewMessages()
	model.SetSize(80, 30)
	model.AppendUser("read the config")
	model.StartTool("tc2", "read_file", `{"path": "/etc/smolbot.yaml"}`)
	model.FinishTool("tc2", "read_file", "done", "config loaded")
	view := model.View()
	if !strings.Contains(view, `"path"`) {
		t.Fatalf("expected tool input to appear in view, got %q", view)
	}
}

func TestProgressAndThinkingBlocksUseSemanticTranscriptColors(t *testing.T) {
	const themeName = "transcript-color-test"
	if !theme.Set("nord") { t.Fatal("expected nord theme") }
	base := *theme.Current()
	base.Name = themeName
	base.TranscriptStreaming = lipgloss.Color("#FF0099")
	base.TranscriptThinking = lipgloss.Color("#00FF88")
	theme.Register(&base)
	if !theme.Set(themeName) { t.Fatalf("could not set test theme %q", themeName) }
	t.Cleanup(func() { theme.Set("nord") })

	model := NewMessages()
	model.SetSize(80, 20)
	model.AppendUser("go")
	model.SetProgress("streaming text...")
	model.SetThinking("reasoning...")

	view := model.View()
	if !strings.Contains(view, ansiFg("#FF0099")) {
		t.Fatalf("streaming ASSISTANT block should use TranscriptStreaming color #FF0099, got %q", view)
	}
	if !strings.Contains(view, ansiFg("#00FF88")) {
		t.Fatalf("THINKING block should use TranscriptThinking color #00FF88, got %q", view)
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
