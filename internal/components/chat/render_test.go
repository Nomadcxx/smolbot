package chat

import (
	"strings"
	"testing"
)

func TestThinkingBlockRender(t *testing.T) {
	m := NewMessages()
	m.SetSize(80, 20)
	
	// Simulate thinking flow
	m.SetThinking("Thinking part 1... ")
	m.SetThinking("Thinking part 1... part 2... ")
	m.AppendThinking("Final thinking content")
	
	view := m.View()
	
	t.Logf("View output:\n%s", view)
	
	if !strings.Contains(view, "THINKING") {
		t.Errorf("No THINKING label in view")
	}
	
	if !strings.Contains(view, "Final thinking content") {
		t.Errorf("No thinking content in view")
	}
}

func TestToolTruncation(t *testing.T) {
	m := NewMessages()
	m.SetSize(80, 20)

	// Create tool with >10 lines
	output := "line1\nline2\nline3\nline4\nline5\nline6\nline7\nline8\nline9\nline10\nline11\nline12"
	m.StartTool("test-id", "test_tool", "{}")
	m.FinishTool("test-id", "test_tool", "done", output)

	// Verbose mode renders full block with truncation
	m.ToggleVerbose()
	view := m.View()
	t.Logf("Tool view:\n%s", view)

	if !strings.Contains(view, "2 lines hidden") {
		t.Errorf("No truncation hint in view")
	}
}
