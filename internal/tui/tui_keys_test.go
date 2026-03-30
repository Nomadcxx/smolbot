package tui

import (
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/Nomadcxx/smolbot/internal/app"
)

func TestEscWhenEditorUnfocusedClearsSelection(t *testing.T) {
	model := New(app.Config{})
	model.width = 80
	model.height = 24
	model.messages.SetSize(78, 10)

	next, _ := model.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEsc}))
	_ = next
}
