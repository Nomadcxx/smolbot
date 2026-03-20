package chat

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
)

func TestNewEditorStartsFocused(t *testing.T) {
	model := NewEditor()

	if !model.textarea.Focused() {
		t.Fatal("expected editor textarea to start focused")
	}
}

func TestEditorShiftEnterInsertsNewline(t *testing.T) {
	model := NewEditor()
	model.textarea.SetValue("first")

	updated, _ := model.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter, Mod: tea.ModShift}))

	if !strings.Contains(updated.textarea.Value(), "\n") {
		t.Fatalf("expected newline in editor value, got %q", updated.textarea.Value())
	}
}

func TestEditorNavigatesPromptHistory(t *testing.T) {
	model := NewEditor()

	model.textarea.SetValue("first")
	model, _ = model.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}))
	_ = model.Submitted()

	model.textarea.SetValue("second")
	model, _ = model.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}))
	_ = model.Submitted()

	model, _ = model.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyUp}))
	if got := model.textarea.Value(); got != "second" {
		t.Fatalf("expected newest history entry, got %q", got)
	}

	model.textarea.Reset()
	model, _ = model.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyUp}))
	if got := model.textarea.Value(); got != "first" {
		t.Fatalf("expected older history entry, got %q", got)
	}

	model.textarea.Reset()
	model, _ = model.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyDown}))
	if got := model.textarea.Value(); got != "second" {
		t.Fatalf("expected newer history entry, got %q", got)
	}
}
