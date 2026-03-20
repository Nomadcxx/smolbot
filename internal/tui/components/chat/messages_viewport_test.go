package chat

import "testing"

func TestMessagesModelScrollKeysDriveViewport(t *testing.T) {
	model := NewMessages()
	model.SetSize(40, 5)
	for i := 0; i < 16; i++ {
		model.AppendAssistant("message")
	}

	_ = model.View()
	model.ScrollToBottom()
	if !model.viewport.AtBottom() {
		t.Fatal("expected viewport to start at bottom")
	}

	initialOffset := model.viewport.YOffset()
	model.HandleKey("pgup")
	if model.viewport.YOffset() >= initialOffset {
		t.Fatalf("expected pgup to reduce offset, got %d -> %d", initialOffset, model.viewport.YOffset())
	}

	model.HandleKey("home")
	if !model.viewport.AtTop() {
		t.Fatal("expected home to move viewport to top")
	}

	model.HandleKey("end")
	if !model.viewport.AtBottom() {
		t.Fatal("expected end to move viewport to bottom")
	}
}
