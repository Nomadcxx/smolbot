package chat

import (
	"strings"
	"testing"

	"github.com/Nomadcxx/smolbot/internal/theme"
	_ "github.com/Nomadcxx/smolbot/internal/theme/themes"
)

func TestMessagesSelectionSingleLine(t *testing.T) {
	useSelectionTheme(t)

	model := NewMessages()
	model.SetSize(40, 10)
	model.AppendUser("hello world")

	if !model.HandleMouseDown(2, 1) {
		t.Fatal("expected mouse down to start selection")
	}
	if !model.HandleMouseDrag(7, 1) {
		t.Fatal("expected mouse drag to update selection")
	}
	if !model.HandleMouseUp(7, 1) {
		t.Fatal("expected mouse up to finalize selection")
	}

	if !model.HasSelection() {
		t.Fatal("expected selection to exist")
	}
	if got := model.SelectedText(); got != "hello" {
		t.Fatalf("SelectedText = %q, want hello", got)
	}
}

func TestMessagesSelectionMultiLine(t *testing.T) {
	useSelectionTheme(t)

	model := NewMessages()
	model.SetSize(40, 10)
	model.AppendUser("alpha\nbeta\ngamma")

	if !model.HandleMouseDown(3, 1) {
		t.Fatal("expected mouse down to start selection")
	}
	if !model.HandleMouseDrag(4, 3) {
		t.Fatal("expected mouse drag to update selection")
	}
	if !model.HandleMouseUp(4, 3) {
		t.Fatal("expected mouse up to finalize selection")
	}

	if got := model.SelectedText(); got != "lpha\nbeta\nga" {
		t.Fatalf("SelectedText = %q, want multiline slice", got)
	}
}

func TestMessagesSelectionAccountsForViewportPadding(t *testing.T) {
	useSelectionTheme(t)

	model := NewMessages()
	model.SetSize(40, 10)
	model.AppendUser("hello world")

	if !model.HandleMouseDown(2, 1) {
		t.Fatal("expected mouse down to start selection")
	}
	if !model.HandleMouseDrag(7, 1) {
		t.Fatal("expected mouse drag to update selection")
	}
	if !model.HandleMouseUp(7, 1) {
		t.Fatal("expected mouse up to finalize selection")
	}

	if got := model.SelectedText(); got != "hello" {
		t.Fatalf("SelectedText = %q, want hello", got)
	}
}

func TestMessagesSelectionWrappedLine(t *testing.T) {
	testMessagesSelectionWrappedLine(t)
}

func TestHighlightContent(t *testing.T) {
	testMessagesSelectionWrappedLine(t)
}

func TestSelectionHighlight(t *testing.T) {
	useSelectionTheme(t)

	model := NewMessages()
	model.SetSize(40, 10)
	model.AppendAssistant("# Heading\n\nUse `code`")

	base := model.View()
	if strings.Contains(base, ansiBgHex(colorHex(theme.Current().Accent))) {
		t.Fatalf("expected no selection background before drag, got %q", base)
	}

	if !model.HandleMouseDown(2, 1) {
		t.Fatal("expected mouse down to start selection")
	}
	if !model.HandleMouseDrag(7, 1) {
		t.Fatal("expected mouse drag to update selection")
	}
	if !model.HandleMouseUp(7, 1) {
		t.Fatal("expected mouse up to finalize selection")
	}

	if !model.HasSelection() {
		t.Fatal("expected selection to exist")
	}

	view := model.View()
	selectionBg := ansiBgHex(colorHex(theme.Current().Accent))
	if !strings.Contains(view, selectionBg) {
		t.Fatalf("expected rendered view to include selection background %q, got %q", selectionBg, view)
	}
	headingFg := ansiFg(colorHex(theme.Current().MarkdownHeading))
	if !strings.Contains(view, headingFg) {
		t.Fatalf("expected rendered view to preserve markdown heading formatting %q, got %q", headingFg, view)
	}
	if view == base {
		t.Fatalf("expected rendered view to change when selection is active")
	}

	model.ClearSelection()
	cleared := model.View()
	if strings.Contains(cleared, selectionBg) {
		t.Fatalf("expected selection background to disappear after clear, got %q", cleared)
	}
}

func TestSelectionPreservesAssistantMarkdownRendering(t *testing.T) {
	useSemanticMarkdownTheme(t)

	model := NewMessages()
	model.SetSize(60, 12)
	model.AppendAssistant("# Heading\n\nParagraph text")

	headingColor := ansiFg(colorHex(theme.Current().MarkdownHeading))
	base := model.View()
	if !strings.Contains(base, headingColor) {
		t.Fatalf("expected base render to include heading style %q, got %q", headingColor, base)
	}

	line := firstLineContaining(model.plainLines, "Paragraph")
	if line < 0 {
		t.Fatalf("expected paragraph line in %#v", model.plainLines)
	}
	if !model.HandleMouseDown(2, line) {
		t.Fatal("expected mouse down to start selection")
	}
	if !model.HandleMouseDrag(11, line) {
		t.Fatal("expected mouse drag to update selection")
	}
	if !model.HandleMouseUp(11, line) {
		t.Fatal("expected mouse up to finalize selection")
	}

	view := model.View()
	if !strings.Contains(view, headingColor) {
		t.Fatalf("expected heading formatting to remain during selection, got %q", view)
	}
	if !strings.Contains(view, ansiBgHex(colorHex(theme.Current().Accent))) {
		t.Fatalf("expected selection background in rendered view, got %q", view)
	}
	if !strings.Contains(view, "Paragraph") || !strings.Contains(view, "ASSISTANT") {
		t.Fatalf("expected assistant block structure to remain during selection, got %q", view)
	}
}

func TestSelectionPreservesToolRendering(t *testing.T) {
	useSelectionTheme(t)

	model := NewMessages()
	model.SetSize(80, 14)
	model.StartTool("tc1", "read_file", `{"path": "/etc/smolbot.yaml", "offset": 0, "limit": 20}`)
	model.FinishTool("tc1", "read_file", "done", "loaded config")

	// Collapsed mode: summary line contains "Read 1 file"
	line := firstLineContaining(model.plainLines, "Read 1 file")
	if line < 0 {
		t.Fatalf("expected collapsed summary line in %#v", model.plainLines)
	}
	if !model.HandleMouseDown(2, line) {
		t.Fatal("expected mouse down to start selection")
	}
	if !model.HandleMouseDrag(8, line) {
		t.Fatal("expected mouse drag to update selection")
	}
	if !model.HandleMouseUp(8, line) {
		t.Fatal("expected mouse up to finalize selection")
	}

	view := model.View()
	if !strings.Contains(view, "✓") {
		t.Fatalf("expected tool state chrome to remain during selection, got %q", view)
	}
	// plainLines holds stripped text; the ANSI-rendered view has escape codes between words.
	if firstLineContaining(model.plainLines, "Read 1 file") < 0 {
		t.Fatalf("expected collapsed summary to remain during selection, plainLines=%#v", model.plainLines)
	}
	if !strings.Contains(view, ansiBgHex(colorHex(theme.Current().Accent))) {
		t.Fatalf("expected selection background in rendered view, got %q", view)
	}
}

func testMessagesSelectionWrappedLine(t *testing.T) {
	useSelectionTheme(t)

	model := NewMessages()
	model.SetSize(18, 10)
	model.AppendUser("alpha beta gamma delta")

	if len(model.plainLines) < 3 {
		t.Fatalf("expected wrapped plain lines, got %#v", model.plainLines)
	}

	if !model.HandleMouseDown(2, 1) {
		t.Fatal("expected mouse down to start selection")
	}
	if !model.HandleMouseDrag(runeCount(model.plainLines[2])+2, 2) {
		t.Fatal("expected mouse drag to update selection")
	}
	if !model.HandleMouseUp(runeCount(model.plainLines[2])+2, 2) {
		t.Fatal("expected mouse up to finalize selection")
	}

	want := strings.Join(model.plainLines[1:3], "\n")
	if got := model.SelectedText(); got != want {
		t.Fatalf("SelectedText = %q, want %q", got, want)
	}
}

func TestMessagesSelectionRespectsViewportOffset(t *testing.T) {
	useSelectionTheme(t)

	model := NewMessages()
	model.SetSize(24, 6)
	for i := 0; i < 12; i++ {
		model.AppendAssistant("message " + strings.Repeat("x", i%3+1))
	}

	offset := model.ViewportOffset()
	if offset == 0 {
		t.Fatal("expected viewport offset to be non-zero after filling the transcript")
	}

	line := firstSelectableContentLine(model.plainLines, offset)
	if line < 0 {
		t.Fatalf("expected a selectable line after offset, got %#v", model.plainLines)
	}
	y := line - offset
	want := model.plainLines[line]

	if !model.HandleMouseDown(2, y) {
		t.Fatal("expected mouse down to start selection")
	}
	if !model.HandleMouseDrag(runeCount(want)+2, y) {
		t.Fatal("expected mouse drag to update selection")
	}
	if !model.HandleMouseUp(runeCount(want)+2, y) {
		t.Fatal("expected mouse up to finalize selection")
	}

	if got := model.SelectedText(); got != want {
		t.Fatalf("SelectedText = %q, want %q", got, want)
	}
}

func TestMessagesSelectionClear(t *testing.T) {
	testClearSelection(t)
}

func TestClearSelection(t *testing.T) {
	testClearSelection(t)
}

func testClearSelection(t *testing.T) {
	useSelectionTheme(t)

	model := NewMessages()
	model.SetSize(40, 10)
	model.AppendUser("clear me")
	model.HandleMouseDown(2, 1)
	model.HandleMouseDrag(7, 1)
	model.HandleMouseUp(7, 1)

	if !model.HasSelection() {
		t.Fatal("expected selection to exist before clear")
	}

	model.ClearSelection()

	if model.HasSelection() {
		t.Fatal("expected selection to be cleared")
	}
	if got := model.SelectedText(); got != "" {
		t.Fatalf("SelectedText = %q, want empty", got)
	}
}

func useSelectionTheme(t *testing.T) {
	t.Helper()
	if !theme.Set("nord") {
		t.Fatal("expected nord theme to be registered")
	}
}

func firstSelectableContentLine(lines []string, start int) int {
	for i := start; i < len(lines); i++ {
		text := strings.TrimSpace(lines[i])
		if text == "" {
			continue
		}
		switch text {
		case "USER", "ASSISTANT", "THINKING", "SYSTEM", "ERROR":
			continue
		default:
			return i
		}
	}
	return -1
}

func firstLineContaining(lines []string, needle string) int {
	for i, line := range lines {
		if strings.Contains(line, needle) {
			return i
		}
	}
	return -1
}
