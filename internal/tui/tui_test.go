package tui

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/Nomadcxx/nanobot-go/internal/tui/client"
	"github.com/Nomadcxx/nanobot-go/internal/tui/theme"
	_ "github.com/Nomadcxx/nanobot-go/internal/tui/theme/themes"
)

type fakeClient struct {
	sessions []client.SessionInfo
	models   []client.ModelInfo
	current  string
	status   string
	chatRun  string
	aborts   []abortCall
	modelErr error
	resetErr error
}

type abortCall struct {
	session string
	runID   string
}

func (f *fakeClient) Connect() (*client.HelloPayload, error) { return &client.HelloPayload{}, nil }
func (f *fakeClient) Close()                                 {}
func (f *fakeClient) SetOnEvent(func(client.Event))          {}
func (f *fakeClient) SetOnClose(func())                      {}
func (f *fakeClient) ChatSend(session, message string) (string, error) {
	if f.chatRun != "" {
		return f.chatRun, nil
	}
	return "run-1", nil
}
func (f *fakeClient) ChatAbort(session, runID string) error {
	f.aborts = append(f.aborts, abortCall{session: session, runID: runID})
	return nil
}
func (f *fakeClient) ChatHistory(session string, limit int) ([]client.HistoryMessage, error) {
	return nil, nil
}
func (f *fakeClient) SessionsList() ([]client.SessionInfo, error) { return f.sessions, nil }
func (f *fakeClient) SessionsReset(key string) error {
	return f.resetErr
}
func (f *fakeClient) ModelsList() ([]client.ModelInfo, string, error) {
	return f.models, f.current, nil
}
func (f *fakeClient) ModelsSet(id string) error {
	if f.modelErr != nil {
		return f.modelErr
	}
	f.current = id
	return nil
}
func (f *fakeClient) Status() (client.StatusPayload, error) {
	var payload client.StatusPayload
	if err := json.Unmarshal([]byte(f.status), &payload); err != nil {
		return client.StatusPayload{}, err
	}
	return payload, nil
}

func TestHandleSlashCommandSessionNewClearsMessages(t *testing.T) {
	model := New(Config{})
	model.messages.AppendUser("old message")

	updated, _ := model.handleSlashCommand("/session new")
	got := updated.(Model)

	if !strings.HasPrefix(got.app.Session, "tui:") {
		t.Fatalf("expected new tui session, got %q", got.app.Session)
	}
	if strings.Contains(got.messages.View(), "old message") {
		t.Fatalf("expected messages to be cleared, got %q", got.messages.View())
	}
}

func TestHandleSlashCommandThemeSwitchesTheme(t *testing.T) {
	model := New(Config{})

	updated, _ := model.handleSlashCommand("/theme dracula")
	got := updated.(Model)

	if current := theme.Current(); current == nil || current.Name != "dracula" {
		t.Fatalf("expected dracula theme, got %#v", current)
	}
	if got.app.Theme != "dracula" {
		t.Fatalf("expected app theme to update, got %q", got.app.Theme)
	}
}

func TestModelSelectionWaitsForGatewaySuccess(t *testing.T) {
	model := New(Config{})
	model.app.Model = "model-a"
	model.client = &fakeClient{modelErr: errors.New("nope")}

	updated, cmd := model.handleSlashCommand("/model model-b")
	got := updated.(Model)
	if got.app.Model != "model-a" {
		t.Fatalf("expected model to stay unchanged before confirmation, got %q", got.app.Model)
	}

	msg := cmd()
	updated, _ = got.Update(msg)
	got = updated.(Model)
	if got.app.Model != "model-a" {
		t.Fatalf("expected failed model change to be ignored, got %q", got.app.Model)
	}
}

func TestHandleSlashCommandSessionOpensDialog(t *testing.T) {
	model := New(Config{})
	model.client = &fakeClient{
		sessions: []client.SessionInfo{
			{Key: "tui:main"},
			{Key: "tui:alt"},
		},
	}

	_, cmd := model.handleSlashCommand("/session")
	if cmd == nil {
		t.Fatal("expected session command to return loader cmd")
	}

	msg := cmd()
	updated, _ := model.Update(msg)
	got := updated.(Model)
	if got.dialog == nil {
		t.Fatal("expected session dialog to open")
	}
}

func TestHandleSlashCommandModelOpensDialog(t *testing.T) {
	model := New(Config{})
	model.client = &fakeClient{
		models: []client.ModelInfo{
			{ID: "model-a", Name: "Model A"},
			{ID: "model-b", Name: "Model B"},
		},
		current: "model-a",
	}

	_, cmd := model.handleSlashCommand("/model")
	if cmd == nil {
		t.Fatal("expected model command to return loader cmd")
	}

	msg := cmd()
	updated, _ := model.Update(msg)
	got := updated.(Model)
	if got.dialog == nil {
		t.Fatal("expected model dialog to open")
	}
}

func TestHandleSlashCommandStatusReturnsStructuredMetadataMsg(t *testing.T) {
	model := New(Config{})
	model.client = &fakeClient{status: `{"model":"test","provider":"openai"}`}

	_, cmd := model.handleSlashCommand("/status")
	if cmd == nil {
		t.Fatal("expected status command to return cmd")
	}

	msg := cmd()
	loaded, ok := msg.(StatusLoadedMsg)
	if !ok {
		t.Fatalf("expected StatusLoadedMsg, got %T", msg)
	}
	if loaded.Status.Model != "test" || loaded.Status.Provider != "openai" {
		t.Fatalf("unexpected status payload: %#v", loaded.Status)
	}
}

func TestSessionResetWaitsForGatewaySuccess(t *testing.T) {
	model := New(Config{})
	model.client = &fakeClient{resetErr: errors.New("boom")}
	model.messages.AppendUser("keep me")

	updated, cmd := model.handleSlashCommand("/session reset")
	got := updated.(Model)
	if !strings.Contains(got.messages.View(), "keep me") {
		t.Fatalf("expected transcript to remain until reset succeeds, got %q", got.messages.View())
	}

	msg := cmd()
	updated, _ = got.Update(msg)
	got = updated.(Model)
	if !strings.Contains(got.messages.View(), "keep me") {
		t.Fatalf("expected failed reset to keep transcript, got %q", got.messages.View())
	}
}

func TestStatusResultUpdatesMetadataWithoutTranscriptNoise(t *testing.T) {
	model := New(Config{})
	model.client = &fakeClient{status: `{"model":"test","provider":"openai","connectedClients":2}`}

	_, cmd := model.handleSlashCommand("/status")
	msg := cmd()
	updated, _ := model.Update(msg)
	got := updated.(Model)

	if strings.Contains(got.messages.View(), `"model":"test"`) {
		t.Fatalf("expected status payload to stay out of chat, got %q", got.messages.View())
	}
	if !strings.Contains(got.footer.View(), "test") || !strings.Contains(got.footer.View(), "openai") {
		t.Fatalf("expected footer metadata to render, got %q", got.footer.View())
	}
}

func TestEventMsgUpdatesToolLifecycle(t *testing.T) {
	model := New(Config{})
	model.messages.SetSize(80, 20)

	startPayload, _ := json.Marshal(client.ToolStartPayload{Name: "read_file"})
	updated, _ := model.Update(EventMsg{
		Event: client.Event{Type: client.FrameEvent, Event: "chat.tool.start", Payload: startPayload, Seq: 1},
	})
	got := updated.(Model)

	donePayload, _ := json.Marshal(client.ToolDonePayload{Name: "read_file", Output: "loaded config"})
	updated, _ = got.Update(EventMsg{
		Event: client.Event{Type: client.FrameEvent, Event: "chat.tool.done", Payload: donePayload, Seq: 2},
	})
	got = updated.(Model)

	view := got.messages.View()
	if !strings.Contains(view, "read_file") {
		t.Fatalf("expected tool name in view, got %q", view)
	}
	if !strings.Contains(view, "loaded config") {
		t.Fatalf("expected tool output in view, got %q", view)
	}
}

func TestWaitForEventIsResubscribedAfterDisconnect(t *testing.T) {
	model := New(Config{})

	updated, cmd := model.Update(DisconnectedMsg{})
	if cmd == nil {
		t.Fatal("expected reconnect/listener command after disconnect")
	}
	got := updated.(Model)
	if got.connected {
		t.Fatal("expected model to be marked disconnected")
	}
}

func TestCtrlCQuitsImmediatelyWhenIdle(t *testing.T) {
	model := New(Config{})

	updated, cmd := model.Update(tea.KeyPressMsg(tea.Key{Code: 'c', Mod: tea.ModCtrl}))
	_ = updated.(Model)
	if cmd == nil {
		t.Fatal("expected ctrl+c to quit")
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Fatalf("expected quit command, got %T", cmd())
	}
}

func TestCtrlCAbortsStreamingRun(t *testing.T) {
	fake := &fakeClient{}
	model := New(Config{})
	model.client = fake
	model.streaming = true
	model.currentRunID = "run-123"

	updated, cmd := model.Update(tea.KeyPressMsg(tea.Key{Code: 'c', Mod: tea.ModCtrl}))
	got := updated.(Model)

	if cmd != nil {
		t.Fatalf("expected abort path to avoid quit cmd, got %T", cmd)
	}
	if got.streaming {
		t.Fatal("expected streaming to stop after ctrl+c")
	}
	if len(fake.aborts) != 1 {
		t.Fatalf("expected one abort call, got %d", len(fake.aborts))
	}
	if fake.aborts[0].runID != "run-123" {
		t.Fatalf("expected abort run id to match, got %#v", fake.aborts[0])
	}
}

func TestInterruptMsgIsMappedToCtrlCMessage(t *testing.T) {
	msg := FilterProgramMsg(nil, tea.InterruptMsg{})
	if _, ok := msg.(CtrlCMsg); !ok {
		t.Fatalf("expected InterruptMsg to map to CtrlCMsg, got %T", msg)
	}
}

func TestSlashKeyGoesToEditor(t *testing.T) {
	model := New(Config{})
	model.width = 80
	model.height = 24

	updated, _ := model.Update(tea.KeyPressMsg(tea.Key{Code: '/', Text: "/"}))
	got := updated.(Model)

	if got.dialog == nil {
		t.Fatal("expected slash palette to open through shared dialog ownership")
	}
	if got.editor.Value() != "/" {
		t.Fatalf("expected slash to be inserted into editor, got %q", got.editor.Value())
	}
	if !strings.Contains(got.View().Content, "Commands") {
		t.Fatalf("expected slash dropdown to be visible, got %q", got.View().Content)
	}
}

func TestSlashDropdownFiltersFromEditorValue(t *testing.T) {
	model := New(Config{})
	model.width = 80
	model.height = 24

	updated, _ := model.Update(tea.KeyPressMsg(tea.Key{Code: '/', Text: "/"}))
	got := updated.(Model)
	updated, _ = got.Update(tea.KeyPressMsg(tea.Key{Code: 's', Text: "s"}))
	got = updated.(Model)
	updated, _ = got.Update(tea.KeyPressMsg(tea.Key{Code: 't', Text: "t"}))
	got = updated.(Model)

	view := got.View().Content
	if !strings.Contains(view, "/status") {
		t.Fatalf("expected filtered slash dropdown to include /status, got %q", view)
	}
	if strings.Contains(view, "/model") {
		t.Fatalf("expected filtered slash dropdown to exclude unrelated commands, got %q", view)
	}
}

func TestSlashDropdownDoesNotExtendLayoutHeight(t *testing.T) {
	model := New(Config{})
	model.width = 80
	model.height = 12

	updated, _ := model.Update(tea.KeyPressMsg(tea.Key{Code: '/', Text: "/"}))
	got := updated.(Model)

	lines := strings.Count(got.View().Content, "\n") + 1
	if lines > model.height {
		t.Fatalf("expected slash dropdown to render as overlay within height %d, got %d lines", model.height, lines)
	}
}

func TestSlashDropdownEnterExecutesCurrentCommand(t *testing.T) {
	model := New(Config{})
	model.width = 80
	model.height = 24
	model.client = &fakeClient{status: `{"model":"test"}`}

	updated, _ := model.Update(tea.KeyPressMsg(tea.Key{Code: '/', Text: "/"}))
	got := updated.(Model)
	updated, _ = got.Update(tea.KeyPressMsg(tea.Key{Code: 's', Text: "s"}))
	got = updated.(Model)
	updated, _ = got.Update(tea.KeyPressMsg(tea.Key{Code: 't', Text: "t"}))
	got = updated.(Model)

	updated, cmd := got.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}))
	got = updated.(Model)
	if cmd == nil {
		t.Fatal("expected enter on slash dropdown to execute the current command")
	}

	msg := cmd()
	updated, _ = got.Update(msg)
	got = updated.(Model)

	if got.editor.Value() != "" {
		t.Fatalf("expected editor to clear after command selection, got %q", got.editor.Value())
	}
	if strings.Contains(got.messages.View(), `"model":"test"`) {
		t.Fatalf("expected selected /status command to avoid transcript noise, got %q", got.messages.View())
	}
	if !strings.Contains(got.footer.View(), "test") {
		t.Fatalf("expected selected /status command to update metadata footer, got %q", got.footer.View())
	}
	if got.dialog != nil {
		t.Fatal("expected slash palette to close after command selection")
	}
}

func TestThemeCommandWithoutArgsOpensThemeChooser(t *testing.T) {
	model := New(Config{})

	updated, _ := model.handleSlashCommand("/theme")
	got := updated.(Model)

	if got.dialog == nil {
		t.Fatal("expected theme chooser dialog")
	}
}

func TestViewDoesNotForceTerminalBackground(t *testing.T) {
	model := New(Config{})
	model.width = 80
	model.height = 24
	view := model.View()
	if view.BackgroundColor != nil {
		t.Fatalf("expected root view background to be transparent/default, got %#v", view.BackgroundColor)
	}
}

func TestHelpCommandAddsAssistantMessage(t *testing.T) {
	model := New(Config{})

	updated, _ := model.handleSlashCommand("/help")
	got := updated.(Model)
	if !strings.Contains(got.messages.View(), "Commands:") {
		t.Fatalf("expected help output in messages, got %q", got.messages.View())
	}
}

func TestQuitCommandReturnsQuit(t *testing.T) {
	model := New(Config{})

	_, cmd := model.handleSlashCommand("/quit")
	if cmd == nil {
		t.Fatal("expected quit command")
	}
	msg := cmd()
	if _, ok := msg.(tea.QuitMsg); !ok {
		t.Fatalf("expected quit message, got %T", msg)
	}
}
