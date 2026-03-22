package tui

import (
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/Nomadcxx/smolbot/internal/app"
	"github.com/Nomadcxx/smolbot/internal/client"
	"github.com/Nomadcxx/smolbot/internal/theme"
	_ "github.com/Nomadcxx/smolbot/internal/theme/themes"
)

var ansiPattern = regexp.MustCompile(`\x1b\[[0-9;]*m`)

type fakeClient struct {
	sessions []client.SessionInfo
	models   []client.ModelInfo
	current  string
	status   client.StatusPayload
	statuses map[string]client.StatusPayload
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
func (f *fakeClient) ModelsSet(id string) (string, error) {
	if f.modelErr != nil {
		return "", f.modelErr
	}
	if f.current != "" {
		return f.current, nil
	}
	f.current = id
	return f.current, nil
}
func (f *fakeClient) Status(session string) (client.StatusPayload, error) {
	if f.statuses != nil {
		if payload, ok := f.statuses[session]; ok {
			return payload, nil
		}
	}
	if f.status.Session == "" {
		f.status.Session = session
	}
	return f.status, nil
}

func plain(text string) string {
	return ansiPattern.ReplaceAllString(text, "")
}

func TestHandleSlashCommandSessionNewClearsMessages(t *testing.T) {
	model := New(app.Config{})
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
	model := New(app.Config{})

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
	model := New(app.Config{})
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
	model := New(app.Config{})
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
	model := New(app.Config{})
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

func TestHandleSlashCommandStatusReturnsChatDoneMsg(t *testing.T) {
	model := New(app.Config{})
	model.client = &fakeClient{status: client.StatusPayload{Model: "test"}}

	_, cmd := model.handleSlashCommand("/status")
	if cmd == nil {
		t.Fatal("expected status command to return cmd")
	}

	msg := cmd()
	done, ok := msg.(StatusLoadedMsg)
	if !ok {
		t.Fatalf("expected StatusLoadedMsg, got %T", msg)
	}
	if done.Payload.Model != "test" || !done.Echo {
		t.Fatalf("unexpected status payload: %#v", done)
	}
}

func TestSessionResetWaitsForGatewaySuccess(t *testing.T) {
	model := New(app.Config{})
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

func TestStatusResultIsRenderedIntoChat(t *testing.T) {
	model := New(app.Config{})
	model.client = &fakeClient{status: client.StatusPayload{Model: "test"}}

	_, cmd := model.handleSlashCommand("/status")
	msg := cmd()
	updated, _ := model.Update(msg)
	got := updated.(Model)

	view := plain(got.messages.View())
	if !strings.Contains(view, "status: model") || !strings.Contains(view, "test") {
		t.Fatalf("expected status payload in chat, got %q", got.messages.View())
	}
}

func TestStatusLoadedUpdatesFooterUsage(t *testing.T) {
	model := New(app.Config{})
	model.width = 80
	model.footer.SetWidth(80)

	updated, _ := model.Update(StatusLoadedMsg{
		Payload: client.StatusPayload{
			Model:   "ollama/qwen3:8b",
			Session: "tui:main",
			Usage: client.UsageInfo{
				TotalTokens:   68000,
				ContextWindow: 200000,
			},
		},
	})
	got := updated.(Model)

	view := plain(got.footer.View())
	if !strings.Contains(view, "model ollama/qwen3:8b") {
		t.Fatalf("expected footer model update, got %q", view)
	}
	if !strings.Contains(view, "34% (68K/200K)") {
		t.Fatalf("expected footer usage update, got %q", view)
	}
}

func TestSyncStatusUsesCurrentSession(t *testing.T) {
	model := New(app.Config{})
	model.app.Session = "tui:alt"
	model.client = &fakeClient{
		statuses: map[string]client.StatusPayload{
			"tui:alt": {
				Model:   "ollama/qwen3:8b",
				Session: "tui:alt",
				Usage: client.UsageInfo{
					TotalTokens:   42000,
					ContextWindow: 200000,
				},
			},
		},
	}
	model.width = 80
	model.footer.SetWidth(80)

	msg := model.syncStatusCmd(false)()
	updated, _ := model.Update(msg)
	got := updated.(Model)

	view := plain(got.footer.View())
	if !strings.Contains(view, "session tui:alt") {
		t.Fatalf("expected footer to keep current session label, got %q", view)
	}
	if !strings.Contains(view, "21% (42K/200K)") {
		t.Fatalf("expected footer to use current session usage, got %q", view)
	}
}

func TestModelSetUsesNormalizedGatewayCurrent(t *testing.T) {
	model := New(app.Config{})
	model.client = &fakeClient{current: "ollama/qwen3:8b"}

	updated, cmd := model.handleSlashCommand("/model ollama_chat/qwen3:8b")
	got := updated.(Model)
	msg := cmd()
	updated, _ = got.Update(msg)
	got = updated.(Model)

	if got.app.Model != "ollama/qwen3:8b" {
		t.Fatalf("expected normalized gateway model id, got %q", got.app.Model)
	}
}

func TestEventMsgUpdatesToolLifecycle(t *testing.T) {
	model := New(app.Config{})
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
	model := New(app.Config{})

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
	model := New(app.Config{})

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
	model := New(app.Config{})
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

func TestSlashDoesNotOpenMenu(t *testing.T) {
	model := New(app.Config{})
	model.width = 80
	model.height = 24

	updated, _ := model.Update(tea.KeyPressMsg(tea.Key{Code: '/', Text: "/"}))
	got := updated.(Model)

	if got.editor.Value() != "/" {
		t.Fatalf("expected slash to be inserted into editor, got %q", got.editor.Value())
	}
	if got.dialog != nil {
		t.Fatalf("expected slash to stay in editor without opening menu, got %T", got.dialog)
	}
}

func TestF1OpensCenteredMenu(t *testing.T) {
	model := New(app.Config{})
	updated, _ := model.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	got := updated.(Model)

	updated, _ = got.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyF1}))
	got = updated.(Model)

	if got.dialog == nil {
		t.Fatal("expected f1 to open menu overlay")
	}
	if !strings.Contains(plain(got.View().Content), "//// MENU ////") {
		t.Fatalf("expected centered menu overlay, got %q", plain(got.View().Content))
	}
}

func TestF1MenuRendersCenteredAwayFromTopLeft(t *testing.T) {
	model := New(app.Config{})

	updated, _ := model.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	got := updated.(Model)
	updated, _ = got.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyF1}))
	got = updated.(Model)

	lines := strings.Split(plain(got.View().Content), "\n")
	borderRow := -1
	borderCol := -1
	for i, line := range lines {
		if strings.Contains(line, "╭") && strings.Contains(line, "╮") {
			borderRow = i
			borderCol = strings.Index(line, "╭")
			break
		}
	}
	if borderRow < 2 {
		t.Fatalf("expected menu popup to be vertically centered, got row %d in view %q", borderRow, plain(got.View().Content))
	}
	if borderCol < 8 {
		t.Fatalf("expected menu popup to be horizontally centered, got col %d in view %q", borderCol, plain(got.View().Content))
	}
}

func TestF1MenuNavigatesToThemesSubmenu(t *testing.T) {
	model := New(app.Config{})
	model.width = 80
	model.height = 24

	updated, _ := model.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyF1}))
	got := updated.(Model)
	updated, _ = got.Update(tea.KeyPressMsg(tea.Key{Code: 'j', Text: "j"}))
	got = updated.(Model)

	updated, cmd := got.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}))
	got = updated.(Model)
	if cmd != nil {
		updated, _ = got.Update(cmd())
		got = updated.(Model)
	}

	if !strings.Contains(plain(got.View().Content), "Themes") {
		t.Fatalf("expected themes submenu after selection, got %q", plain(got.View().Content))
	}
}

func TestF1MenuDoesNotExtendLayoutHeight(t *testing.T) {
	model := New(app.Config{})
	model.width = 80
	model.height = 12

	updated, _ := model.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyF1}))
	got := updated.(Model)

	lines := strings.Count(got.View().Content, "\n") + 1
	if lines > model.height {
		t.Fatalf("expected menu overlay to render within height %d, got %d lines", model.height, lines)
	}
}

func TestMenuOverlayKeepsTranscriptFrameVisible(t *testing.T) {
	model := New(app.Config{})

	updated, _ := model.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	got := updated.(Model)
	got.messages.AppendUser("hello")
	got.messages.AppendAssistant("world")

	updated, _ = got.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyF1}))
	got = updated.(Model)

	view := plain(got.View().Content)
	if !strings.Contains(view, "//// MENU ////") {
		t.Fatalf("expected menu overlay in view, got %q", view)
	}
	if strings.Count(view, "╭") < 3 || strings.Count(view, "╰") < 2 {
		t.Fatalf("expected transcript frame to remain visible under menu overlay, got %q", view)
	}
	if !strings.Contains(view, "USER") || !strings.Contains(view, "ASSISTANT") {
		t.Fatalf("expected transcript content to remain visible around overlay, got %q", view)
	}
}

func TestTranscriptFrameAddsSpacerBelowHeader(t *testing.T) {
	model := New(app.Config{})
	updated, _ := model.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	got := updated.(Model)
	got.messages.AppendUser("hello")

	lines := strings.Split(plain(got.View().Content), "\n")
	headerRow := -1
	frameRow := -1
	for i, line := range lines {
		if headerRow == -1 && strings.Contains(line, "▄▄▄▄▄▄") {
			headerRow = i
		}
		if frameRow == -1 && strings.Contains(line, "╭") && strings.Contains(line, "╮") {
			frameRow = i
		}
	}
	if headerRow == -1 || frameRow == -1 {
		t.Fatalf("expected both header art and transcript frame in view %q", plain(got.View().Content))
	}
	if frameRow-headerRow < 2 {
		t.Fatalf("expected spacer row between header and transcript frame, header row=%d frame row=%d view=%q", headerRow, frameRow, plain(got.View().Content))
	}
}

func TestTranscriptAreaHasOwnBorder(t *testing.T) {
	model := New(app.Config{})
	updated, _ := model.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	got := updated.(Model)
	got.messages.AppendUser("hello")
	got.messages.AppendAssistant("world")

	view := plain(got.View().Content)
	if strings.Count(view, "╭") < 2 {
		t.Fatalf("expected separate transcript and editor borders, got %q", view)
	}
	if !strings.Contains(view, "● ") {
		t.Fatalf("expected status row outside transcript frame, got %q", view)
	}
}

func TestMenuCloseRestoresEditorFlow(t *testing.T) {
	model := New(app.Config{})
	model.width = 80
	model.height = 24

	updated, _ := model.Update(tea.KeyPressMsg(tea.Key{Code: 'm', Mod: tea.ModCtrl}))
	got := updated.(Model)
	updated, cmd := got.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEsc}))
	got = updated.(Model)
	if cmd == nil {
		t.Fatal("expected escape to close menu")
	}
	updated, _ = got.Update(cmd())
	got = updated.(Model)
	if got.dialog != nil {
		t.Fatal("expected menu to close after escape")
	}

	updated, _ = got.Update(tea.KeyPressMsg(tea.Key{Code: 'x', Text: "x"}))
	got = updated.(Model)
	if got.editor.Value() != "x" {
		t.Fatalf("expected editor flow after closing menu, got %q", got.editor.Value())
	}
}

func TestOverlayCloseRestoresEditorFlow(t *testing.T) {
	model := New(app.Config{})
	model.width = 80
	model.height = 24

	updated, _ := model.handleSlashCommand("/theme")
	got := updated.(Model)
	if got.dialog == nil {
		t.Fatal("expected theme chooser dialog")
	}

	updated, cmd := got.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEsc}))
	got = updated.(Model)
	if cmd == nil {
		t.Fatal("expected escape to close the overlay")
	}
	updated, _ = got.Update(cmd())
	got = updated.(Model)
	if got.dialog != nil {
		t.Fatal("expected overlay to close after escape")
	}

	updated, _ = got.Update(tea.KeyPressMsg(tea.Key{Code: 'x', Text: "x"}))
	got = updated.(Model)
	if got.editor.Value() != "x" {
		t.Fatalf("expected editor flow to resume after overlay close, got %q", got.editor.Value())
	}
}

func TestThemeCommandWithoutArgsOpensThemeChooser(t *testing.T) {
	model := New(app.Config{})

	updated, _ := model.handleSlashCommand("/theme")
	got := updated.(Model)

	if got.dialog == nil {
		t.Fatal("expected theme chooser dialog")
	}
}

func TestThemeChooserDoesNotExtendLayoutHeight(t *testing.T) {
	model := New(app.Config{})
	model.width = 80
	model.height = 12

	updated, _ := model.handleSlashCommand("/theme")
	got := updated.(Model)
	if got.dialog == nil {
		t.Fatal("expected theme chooser dialog")
	}

	lines := strings.Count(got.View().Content, "\n") + 1
	if lines > model.height {
		t.Fatalf("expected theme chooser to render within height %d, got %d lines", model.height, lines)
	}
	view := plain(got.View().Content)
	if !strings.Contains(view, "Themes") {
		t.Fatalf("expected theme chooser title to remain visible, got %q", view)
	}
	if !strings.Contains(view, "Theme: ") {
		t.Fatalf("expected at least one theme choice to remain visible, got %q", view)
	}
}

func TestSessionDialogShowsCurrentMarker(t *testing.T) {
	model := New(app.Config{})
	model.width = 80
	model.height = 24
	model.client = &fakeClient{
		sessions: []client.SessionInfo{
			{Key: "tui:main", Preview: "Current conversation"},
			{Key: "tui:alt", Preview: "Alternative thread"},
		},
	}
	model.app.Session = "tui:main"

	_, cmd := model.handleSlashCommand("/session")
	updated, _ := model.Update(cmd())
	got := updated.(Model)
	view := plain(got.View().Content)
	if !strings.Contains(view, "tui:main") || !strings.Contains(view, "current") {
		t.Fatalf("expected current session marker in dialog, got %q", view)
	}
	if !strings.Contains(view, "Current conversation") {
		t.Fatalf("expected session preview in dialog, got %q", view)
	}
}

func TestSessionDialogShowsOverflowCues(t *testing.T) {
	model := New(app.Config{})
	model.width = 80
	model.height = 24
	model.client = &fakeClient{
		sessions: []client.SessionInfo{
			{Key: "tui:00", Preview: "preview 00"},
			{Key: "tui:01", Preview: "preview 01"},
			{Key: "tui:02", Preview: "preview 02"},
			{Key: "tui:03", Preview: "preview 03"},
			{Key: "tui:04", Preview: "preview 04"},
			{Key: "tui:05", Preview: "preview 05"},
			{Key: "tui:06", Preview: "preview 06"},
			{Key: "tui:07", Preview: "preview 07"},
			{Key: "tui:08", Preview: "preview 08"},
			{Key: "tui:09", Preview: "preview 09"},
		},
	}

	_, cmd := model.handleSlashCommand("/session")
	updated, _ := model.Update(cmd())
	got := updated.(Model)
	if !strings.Contains(plain(got.View().Content), "▼ more below") {
		t.Fatalf("expected session overflow cue below initial window, got %q", plain(got.View().Content))
	}
	for i := 0; i < 5; i++ {
		updated, _ = got.Update(tea.KeyPressMsg(tea.Key{Code: 'j', Text: "j"}))
		got = updated.(Model)
	}
	view := plain(got.View().Content)
	if !strings.Contains(view, "▲ more above") || !strings.Contains(view, "▼ more below") {
		t.Fatalf("expected session overflow cues in dialog, got %q", view)
	}
}

func TestModelDialogShowsCurrentModel(t *testing.T) {
	model := New(app.Config{})
	model.width = 80
	model.height = 24
	model.client = &fakeClient{
		models: []client.ModelInfo{
			{ID: "gpt-5", Name: "GPT-5", Provider: "openai"},
			{ID: "claude-opus", Name: "Claude Opus", Provider: "anthropic"},
		},
		current: "gpt-5",
	}

	_, cmd := model.handleSlashCommand("/model")
	updated, _ := model.Update(cmd())
	got := updated.(Model)
	view := plain(got.View().Content)
	if !strings.Contains(view, "GPT-5") || !strings.Contains(view, "gpt-5") {
		t.Fatalf("expected model dialog to show model label and id, got %q", view)
	}
	if !strings.Contains(view, "openai") || !strings.Contains(view, "current") {
		t.Fatalf("expected model dialog to show provider and current marker, got %q", view)
	}
}

func TestSelectorsShareOverlayBehavior(t *testing.T) {
	model := New(app.Config{})
	model.width = 80
	model.height = 20
	model.client = &fakeClient{
		sessions: []client.SessionInfo{{Key: "tui:main"}, {Key: "tui:alt"}},
		models:   []client.ModelInfo{{ID: "fast", Name: "Fast"}, {ID: "smart", Name: "Smart"}},
		current:  "fast",
	}

	checkOverlay := func(input, title string) {
		t.Helper()

		updated, cmd := model.handleSlashCommand(input)
		got := updated.(Model)
		if cmd != nil {
			updated, _ = got.Update(cmd())
			got = updated.(Model)
		}
		if got.dialog == nil {
			t.Fatalf("expected overlay for %s", input)
		}

		view := plain(got.View().Content)
		if !strings.Contains(view, title) {
			t.Fatalf("expected %s title in overlay, got %q", title, view)
		}
		if !strings.Contains(view, "Esc close") {
			t.Fatalf("expected close help treatment in overlay, got %q", view)
		}
		lines := strings.Count(view, "\n") + 1
		if lines > got.height {
			t.Fatalf("expected overlay to remain within viewport height %d, got %d", got.height, lines)
		}

		updated, closeCmd := got.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEsc}))
		got = updated.(Model)
		if closeCmd == nil {
			t.Fatalf("expected esc to close overlay for %s", input)
		}
		updated, _ = got.Update(closeCmd())
		got = updated.(Model)
		if got.dialog != nil {
			t.Fatalf("expected overlay to close for %s", input)
		}
	}

	checkOverlay("/theme", "Themes")
	checkOverlay("/session", "Sessions")
	checkOverlay("/model", "Models")
}

func TestCompactLayoutOnShortTerminals(t *testing.T) {
	model := New(app.Config{})

	updated, _ := model.Update(tea.WindowSizeMsg{Width: 80, Height: 8})
	got := updated.(Model)
	view := plain(got.View().Content)

	if strings.Contains(view, "▄▄▄▄▄▄") {
		t.Fatalf("expected compact header treatment on short terminal, got %q", view)
	}
	if !strings.Contains(strings.ToLower(view), "nanobot") {
		t.Fatalf("expected compact layout to keep app identity visible, got %q", view)
	}
	if lines := strings.Count(view, "\n") + 1; lines > 8 {
		t.Fatalf("expected compact layout to stay within height 8, got %d lines", lines)
	}
}

func TestHeaderArtIsCenteredAcrossViewport(t *testing.T) {
	model := New(app.Config{})

	updated, _ := model.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	got := updated.(Model)

	lines := strings.Split(plain(got.View().Content), "\n")
	artLine := ""
	for _, line := range lines {
		if strings.Contains(line, "▄▄▄▄▄▄") {
			artLine = line
			break
		}
	}
	if artLine == "" {
		t.Fatalf("expected ascii header line in view %q", plain(got.View().Content))
	}
	if col := strings.Index(artLine, "▄▄▄▄▄▄"); col < 6 {
		t.Fatalf("expected centered header art, got col %d in %q", col, artLine)
	}
}

func TestTranscriptKeepsMinimumUsableHeight(t *testing.T) {
	model := New(app.Config{})

	updated, _ := model.Update(tea.WindowSizeMsg{Width: 80, Height: 8})
	got := updated.(Model)
	chatLines := strings.Count(plain(got.messages.View()), "\n") + 1

	if chatLines < 3 {
		t.Fatalf("expected transcript area to keep at least 3 lines in compact mode, got %d", chatLines)
	}
}

func TestFrameHierarchyLayout(t *testing.T) {
	model := New(app.Config{})
	model.app.Model = "gpt-5"
	model.app.Session = "tui:main"

	updated, _ := model.Update(tea.WindowSizeMsg{Width: 80, Height: 16})
	got := updated.(Model)
	got.status.SetConnected(true)

	view := plain(got.View().Content)
	if !strings.Contains(view, "connected") {
		t.Fatalf("expected activity row in layout, got %q", view)
	}
	if !strings.Contains(view, "model gpt-5") || !strings.Contains(view, "session tui:main") {
		t.Fatalf("expected footer metadata in layout, got %q", view)
	}
	lines := strings.Split(view, "\n")
	editorRow := -1
	footerRow := -1
	for i, line := range lines {
		if editorRow == -1 && strings.Contains(line, "Send a message") {
			editorRow = i
		}
		if footerRow == -1 && strings.Contains(line, "model gpt-5") {
			footerRow = i
		}
	}
	if editorRow == -1 || footerRow == -1 || footerRow <= editorRow {
		t.Fatalf("expected footer metadata below the editor, editor=%d footer=%d view=%q", editorRow, footerRow, view)
	}
	if lines := strings.Count(view, "\n") + 1; lines > 16 {
		t.Fatalf("expected frame hierarchy to fit viewport height, got %d lines", lines)
	}
}

func TestEditorUsesThemeSurface(t *testing.T) {
	model := New(app.Config{})
	model.width = 80
	model.height = 16
	view := model.editor.View()

	if !strings.Contains(view, "48;2;0;0;0") {
		t.Fatalf("expected editor to consume black theme surface, got %q", view)
	}
}

func TestDialogsUsePanelSurface(t *testing.T) {
	model := New(app.Config{})
	model.width = 80
	model.height = 24

	updated, _ := model.handleSlashCommand("/theme")
	got := updated.(Model)
	view := got.View().Content

	if !strings.Contains(view, "48;2;0;0;0") {
		t.Fatalf("expected dialogs to render on black panel surface, got %q", view)
	}
}

func TestVisualSurfaceIntegration(t *testing.T) {
	model := New(app.Config{})
	model.width = 80
	model.height = 24
	model.messages.SetSize(80, 8)
	model.messages.AppendUser("hello from user")
	model.messages.AppendAssistant("hello from assistant")

	updated, _ := model.handleSlashCommand("/theme")
	got := updated.(Model)
	view := got.View()
	plainView := plain(view.Content)
	transcriptView := plain(got.messages.View())

	want := fmt.Sprintf("%#v", theme.Current().Background)
	if got := fmt.Sprintf("%#v", view.BackgroundColor); got != want {
		t.Fatalf("expected black root background, got %s want %s", got, want)
	}
	if strings.Count(view.Content, "48;2;0;0;0") < 2 {
		t.Fatalf("expected black surfaces to appear across editor/dialog stack, got %q", view.Content)
	}
	if !strings.Contains(transcriptView, "USER") || !strings.Contains(transcriptView, "ASSISTANT") {
		t.Fatalf("expected transcript semantics to remain intact under overlay, got %q", transcriptView)
	}
	if !strings.Contains(plainView, "Themes") {
		t.Fatalf("expected overlay to coexist with transcript content, got %q", plainView)
	}
}


func TestHelpCommandAddsAssistantMessage(t *testing.T) {
	model := New(app.Config{})

	updated, _ := model.handleSlashCommand("/help")
	got := updated.(Model)
	if !strings.Contains(got.messages.View(), "Commands:") {
		t.Fatalf("expected help output in messages, got %q", got.messages.View())
	}
}

func TestQuitCommandReturnsQuit(t *testing.T) {
	model := New(app.Config{})

	_, cmd := model.handleSlashCommand("/quit")
	if cmd == nil {
		t.Fatal("expected quit command")
	}
	msg := cmd()
	if _, ok := msg.(tea.QuitMsg); !ok {
		t.Fatalf("expected quit message, got %T", msg)
	}
}

func TestEscKeyDoesNotScrollTranscript(t *testing.T) {
	if !theme.Set("nord") {
		t.Fatal("expected nord theme")
	}
	model := New(app.Config{})
	model.width = 80
	model.height = 24
	model.messages.SetSize(78, 10)
	for i := 0; i < 20; i++ {
		model.messages.AppendAssistant("line " + strconv.Itoa(i))
	}
	model.messages.HandleKey("pgdown")
	offsetBefore := model.messages.ViewportOffset()
	if offsetBefore == 0 {
		t.Fatal("precondition: expected non-zero offset after pgdown")
	}

	next, _ := model.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEsc}))
	got := next.(Model)
	offsetAfter := got.messages.ViewportOffset()

	if offsetAfter != offsetBefore {
		t.Fatalf("esc should not change scroll position: before=%d after=%d", offsetBefore, offsetAfter)
	}
}

func TestMouseWheelScrollsTranscript(t *testing.T) {
	if !theme.Set("nord") { t.Fatal("expected nord theme") }
	model := New(app.Config{})
	model.width = 80
	model.height = 24
	model.messages.SetSize(78, 10)
	for i := 0; i < 30; i++ {
		model.messages.AppendAssistant("message line " + strconv.Itoa(i))
	}
	model.messages.HandleKey("end")
	bottomOffset := model.messages.ViewportOffset()

	next, _ := model.Update(tea.MouseWheelMsg{Button: tea.MouseWheelUp})
	got := next.(Model)
	if got.messages.ViewportOffset() >= bottomOffset {
		t.Fatalf("mouse wheel up should reduce scroll offset: before=%d after=%d", bottomOffset, got.messages.ViewportOffset())
	}
}

func TestThemeCommandShowsErrorOnUnknownTheme(t *testing.T) {
	model := New(app.Config{})
	model.width = 80
	model.height = 24
	next, _ := model.handleSlashCommand("/theme totally-not-a-real-theme")
	got := next.(Model)
	view := got.messages.View()
	if !strings.Contains(view, "Unknown theme") {
		t.Fatalf("expected error message for unknown theme, got %q", view)
	}
}

func TestThemeCommandInvalidatesMessageRender(t *testing.T) {
	if !theme.Set("nord") { t.Fatal("expected nord theme") }
	model := New(app.Config{})
	model.width = 80
	model.height = 24
	model.messages.SetSize(78, 10)
	model.messages.AppendAssistant("hello")
	next, _ := model.handleSlashCommand("/theme dracula")
	got := next.(Model)
	if !got.messages.IsDirty() {
		t.Fatal("expected messages to be dirty after theme change")
	}
}
