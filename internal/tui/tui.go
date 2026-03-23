package tui

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	lipgloss "charm.land/lipgloss/v2"
	"github.com/Nomadcxx/smolbot/internal/app"
	"github.com/Nomadcxx/smolbot/internal/client"
	"github.com/Nomadcxx/smolbot/internal/components/chat"
	dialogcmp "github.com/Nomadcxx/smolbot/internal/components/dialog"
	"github.com/Nomadcxx/smolbot/internal/components/header"
	"github.com/Nomadcxx/smolbot/internal/components/status"
	"github.com/Nomadcxx/smolbot/internal/theme"
	_ "github.com/Nomadcxx/smolbot/internal/theme/themes"
)

type ConnectedMsg struct{ Hello *client.HelloPayload }
type CtrlCMsg struct{}
type DisconnectedMsg struct{}
type EventMsg struct{ Event client.Event }
type ChatStartedMsg struct{ RunID string }
type ChatDoneMsg struct{ Content string }
type ChatErrorMsg struct{ Message string }
type ChatProgressMsg struct{ Content string }
type ThinkingDoneMsg struct{ Content string }
type HistoryLoadedMsg struct{ Messages []client.HistoryMessage }
type StatusLoadedMsg struct {
	Payload client.StatusPayload
	Echo    bool
}
type CompressionStatusMsg struct {
	Info client.CompressionInfo
}
type SessionsLoadedMsg struct{ Sessions []client.SessionInfo }
type SessionResetDoneMsg struct{ Key string }
type ModelsLoadedMsg struct {
	Models  []client.ModelInfo
	Current string
}
type ModelSetMsg struct{ ID string }
type ModelCurrentMsg struct {
	Current string
}

type Dialog interface {
	Update(tea.Msg) (Dialog, tea.Cmd)
	View() string
}

type gatewayClient interface {
	Connect() (*client.HelloPayload, error)
	Close()
	SetOnEvent(func(client.Event))
	SetOnClose(func())
	ChatSend(session, message string) (string, error)
	ChatAbort(session, runID string) error
	ChatHistory(session string, limit int) ([]client.HistoryMessage, error)
	SessionsList() ([]client.SessionInfo, error)
	SessionsReset(key string) error
	ModelsList() ([]client.ModelInfo, string, error)
	ModelsSet(id string) (string, error)
	Status(session string) (client.StatusPayload, error)
}

type Model struct {
	width, height int
	app           *app.App
	client        gatewayClient
	header        header.Model
	messages      chat.MessagesModel
	editor        chat.EditorModel
	status        status.Model
	footer        status.FooterModel
	dialog        Dialog
	eventCh       chan tea.Msg
	connected     bool
	reconnectWait time.Duration
	streaming     bool
	currentRunID  string
}

func New(cfg app.Config) Model {
	a := app.New(cfg)
	_ = theme.Set(a.Theme)
	if theme.Current() == nil {
		_ = theme.Set("nord")
	}

	eventCh := make(chan tea.Msg, 64)
	c := client.New(a.WSURL())

	return Model{
		app:      a,
		client:   c,
		header:   header.New(),
		messages: chat.NewMessages(),
		editor:   chat.NewEditor(),
		status:   status.New(a),
		footer:   status.NewFooter(a),
		eventCh:  eventCh,
	}
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(m.editor.Init(), m.connectCmd(), m.waitForEventCmd())
}

func FilterProgramMsg(_ tea.Model, msg tea.Msg) tea.Msg {
	if _, ok := msg.(tea.InterruptMsg); ok {
		return CtrlCMsg{}
	}
	return msg
}

func (m Model) connectCmd() tea.Cmd {
	return func() tea.Msg {
		m.client.SetOnEvent(func(evt client.Event) {
			m.eventCh <- EventMsg{Event: evt}
		})
		m.client.SetOnClose(func() {
			m.eventCh <- DisconnectedMsg{}
		})
		hello, err := m.client.Connect()
		if err != nil {
			return DisconnectedMsg{}
		}
		return ConnectedMsg{Hello: hello}
	}
}

func (m Model) waitForEventCmd() tea.Cmd {
	return func() tea.Msg {
		return <-m.eventCh
	}
}

func (m Model) loadHistoryCmd() tea.Cmd {
	return func() tea.Msg {
		msgs, err := m.client.ChatHistory(m.app.Session, 100)
		if err != nil {
			return nil
		}
		return HistoryLoadedMsg{Messages: msgs}
	}
}

func (m Model) sendChatCmd(message string) tea.Cmd {
	return func() tea.Msg {
		runID, err := m.client.ChatSend(m.app.Session, message)
		if err != nil {
			return ChatErrorMsg{Message: err.Error()}
		}
		return ChatStartedMsg{RunID: runID}
	}
}

func (m Model) reconnectCmd(delay time.Duration) tea.Cmd {
	return func() tea.Msg {
		if delay > 0 {
			time.Sleep(delay)
		}
		return m.connectCmd()()
	}
}

func (m Model) saveStateCmd() tea.Cmd {
	return func() tea.Msg {
		_ = app.SaveState(app.State{
			Theme:       m.app.Theme,
			LastSession: m.app.Session,
			LastModel:   m.app.Model,
		})
		if m.client != nil {
			m.client.Close()
		}
		return tea.QuitMsg{}
	}
}

func (m Model) persistStateCmd() tea.Cmd {
	return func() tea.Msg {
		_ = app.SaveState(app.State{
			Theme:       m.app.Theme,
			LastSession: m.app.Session,
			LastModel:   m.app.Model,
		})
		return nil
	}
}

func (m Model) syncModelCmd() tea.Cmd {
	return func() tea.Msg {
		_, current, err := m.client.ModelsList()
		if err != nil {
			return nil
		}
		return ModelCurrentMsg{Current: current}
	}
}

func (m Model) syncStatusCmd(echo bool) tea.Cmd {
	return func() tea.Msg {
		payload, err := m.client.Status(m.app.Session)
		if err != nil {
			return ChatErrorMsg{Message: err.Error()}
		}
		return StatusLoadedMsg{Payload: payload, Echo: echo}
	}
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		compact := m.height <= 30
		m.header.SetCompact(compact)
		m.editor.SetCompact(compact)
		headerH := m.header.Height()
		editorH := m.editor.Height()
		statusH := 1
		footerH := 1
		spacerH := 1
		frameH := m.height - headerH - spacerH - editorH - statusH - footerH
		if frameH < 5 {
			frameH = 5
		}
		chatH := frameH - 2
		if chatH < 3 {
			chatH = 3
		}
		m.header.SetWidth(m.width)
		m.messages.SetSize(max(1, m.width-2), chatH)
		m.editor.SetWidth(m.width)
		m.status.SetWidth(m.width)
		m.footer.SetWidth(m.width)
		return m, nil
	case ConnectedMsg:
		m.connected = true
		m.reconnectWait = 0
		m.status.SetConnected(true)
		m.status.SetReconnecting(false)
		if cwd, err := os.Getwd(); err == nil {
			m.header.SetWorkDir(cwd)
		}
		return m, tea.Batch(m.loadHistoryCmd(), m.syncModelCmd(), m.syncStatusCmd(false))
	case CtrlCMsg:
		return m.handleCtrlC()
	case ChatStartedMsg:
		m.currentRunID = msg.RunID
		return m, nil
	case ChatDoneMsg:
		m.streaming = false
		m.currentRunID = ""
		m.status.SetStreaming(false)
		m.messages.AppendAssistant(msg.Content)
		return m, m.syncStatusCmd(false)
	case ChatProgressMsg:
		m.messages.SetProgress(msg.Content)
		return m, nil
	case ThinkingDoneMsg:
		m.messages.AppendThinking(msg.Content)
		return m, nil
	case ChatErrorMsg:
		m.streaming = false
		m.currentRunID = ""
		m.status.SetStreaming(false)
		m.messages.AppendError(msg.Message)
		return m, nil
	case HistoryLoadedMsg:
		history := make([]chat.ChatMessage, 0, len(msg.Messages))
		for _, hm := range msg.Messages {
			if hm.Role == "user" {
				history = append(history, chat.ChatMessage{Role: "user", Content: hm.Content})
			} else if hm.Role == "assistant" {
				history = append(history, chat.ChatMessage{Role: "assistant", Content: hm.Content})
			}
		}
		m.messages.ReplaceHistory(history)
		return m, nil
	case StatusLoadedMsg:
		if msg.Payload.Model != "" {
			m.app.Model = msg.Payload.Model
		}
		m.footer.SetUsage(msg.Payload.Usage)
		m.header.SetModel(m.app.Model)
		if msg.Payload.Usage.ContextWindow > 0 && msg.Payload.Usage.TotalTokens > 0 {
			pct := int((float64(msg.Payload.Usage.TotalTokens) / float64(msg.Payload.Usage.ContextWindow)) * 100 + 0.5)
			m.header.SetContextPercent(pct)
		}
		if msg.Echo {
			m.messages.AppendAssistant(formatStatusSummary(msg.Payload))
		}
		return m, nil
	case ModelCurrentMsg:
		if msg.Current != "" {
			m.app.Model = msg.Current
		}
		return m, nil
	case SessionsLoadedMsg:
		m.dialog = sessionDialog{dialogcmp.NewSessions(msg.Sessions, m.app.Session)}
		return m, nil
	case SessionResetDoneMsg:
		m.app.Session = msg.Key
		m.messages = chat.NewMessages()
		return m, tea.Batch(m.persistStateCmd(), m.syncStatusCmd(false))
	case ModelsLoadedMsg:
		m.dialog = modelsDialog{dialogcmp.NewModels(msg.Models, msg.Current)}
		return m, nil
	case ModelSetMsg:
		m.app.Model = msg.ID
		return m, tea.Batch(m.persistStateCmd(), m.syncStatusCmd(false))
	case dialogcmp.SessionChosenMsg:
		m.app.Session = msg.Key
		m.messages = chat.NewMessages()
		m.dialog = nil
		return m, tea.Batch(m.loadHistoryCmd(), m.persistStateCmd(), m.syncStatusCmd(false))
	case dialogcmp.SessionNewMsg:
		m.app.Session = "tui:" + time.Now().Format("20060102-150405")
		m.messages = chat.NewMessages()
		m.dialog = nil
		return m, tea.Batch(m.persistStateCmd(), m.syncStatusCmd(false))
	case dialogcmp.SessionResetMsg:
		m.dialog = nil
		return m, func() tea.Msg {
			if err := m.client.SessionsReset(msg.Key); err != nil {
				return ChatErrorMsg{Message: err.Error()}
			}
			return SessionResetDoneMsg{Key: msg.Key}
		}
	case dialogcmp.ModelChosenMsg:
		m.dialog = nil
		return m, func() tea.Msg {
			current, err := m.client.ModelsSet(msg.ID)
			if err != nil {
				return ChatErrorMsg{Message: err.Error()}
			}
			return ModelSetMsg{ID: current}
		}
	case dialogcmp.CommandChosenMsg:
		m.dialog = nil
		m.editor.SetValue("")
		if isMenuCloseCommand(msg.Command) {
			return m, nil
		}
		return m.handleSlashCommand(msg.Command)
	case dialogcmp.CloseDialogMsg:
		m.dialog = nil
		return m, nil
	case DisconnectedMsg:
		m.connected = false
		m.streaming = false
		m.currentRunID = ""
		m.status.SetConnected(false)
		m.status.SetStreaming(false)
		m.status.SetReconnecting(true)
		m.footer.SetUsage(client.UsageInfo{})
		if m.reconnectWait == 0 {
			m.reconnectWait = time.Second
		} else if m.reconnectWait < 8*time.Second {
			m.reconnectWait *= 2
		}
		return m, tea.Batch(m.reconnectCmd(m.reconnectWait), m.waitForEventCmd())
	case EventMsg:
		cmds = append(cmds, m.waitForEventCmd())
		var mapped tea.Msg
		switch msg.Event.Event {
		case "chat.done":
			var p client.ChatDonePayload
			_ = json.Unmarshal(msg.Event.Payload, &p)
			mapped = ChatDoneMsg{Content: p.Content}
		case "chat.progress":
			var p client.ProgressPayload
			_ = json.Unmarshal(msg.Event.Payload, &p)
			mapped = ChatProgressMsg{Content: p.Content}
		case "chat.error":
			var p client.ChatErrorPayload
			_ = json.Unmarshal(msg.Event.Payload, &p)
			mapped = ChatErrorMsg{Message: p.Message}
		case "chat.thinking":
			var p client.ThinkingPayload
			_ = json.Unmarshal(msg.Event.Payload, &p)
			m.messages.SetThinking(m.messages.GetThinking() + p.Content)
		case "chat.thinking.done":
			var p client.ThinkingDonePayload
			_ = json.Unmarshal(msg.Event.Payload, &p)
			mapped = ThinkingDoneMsg{Content: p.Content}
		case "chat.tool.start":
			var p client.ToolStartPayload
			_ = json.Unmarshal(msg.Event.Payload, &p)
			m.messages.StartTool(p.ID, p.Name, p.Input)
		case "chat.tool.done":
			var p client.ToolDonePayload
			_ = json.Unmarshal(msg.Event.Payload, &p)
			status := "done"
			if p.Error != "" {
				status = "error"
			}
			m.messages.FinishTool(p.ID, p.Name, status, p.Output)
		case "context.compressed":
			var p client.CompressionInfo
			_ = json.Unmarshal(msg.Event.Payload, &p)
			m.footer.SetCompression(&p)
		case "chat.usage":
			var p client.UsagePayload
			_ = json.Unmarshal(msg.Event.Payload, &p)
			m.footer.SetUsage(client.UsageInfo{
				PromptTokens:     p.PromptTokens,
				CompletionTokens: p.CompletionTokens,
				TotalTokens:      p.TotalTokens,
				ContextWindow:    p.ContextWindow,
			})
			if p.ContextWindow > 0 && p.TotalTokens > 0 {
				pct := int((float64(p.TotalTokens) / float64(p.ContextWindow)) * 100 + 0.5)
				m.header.SetContextPercent(pct)
			}
		}
		if mapped != nil {
			nextModel, cmd := m.Update(mapped)
			next := nextModel.(Model)
			if cmd != nil {
				cmds = append(cmds, cmd)
			}
			return next, tea.Batch(cmds...)
		}
		return m, tea.Batch(cmds...)
	case tea.MouseWheelMsg:
		if m.dialog == nil {
			switch msg.Button {
			case tea.MouseWheelUp:
				m.messages.HandleKey("pgup")
			case tea.MouseWheelDown:
				m.messages.HandleKey("pgdown")
			}
		}
		return m, nil
	case tea.KeyMsg:
		if m.dialog != nil {
			next, cmd := m.dialog.Update(msg)
			m.dialog = next
			return m, cmd
		}
		switch msg.String() {
		case "ctrl+c":
			return m.handleCtrlC()
		case "f1", "ctrl+m":
			m.dialog = newMenuDialog()
			return m, nil
		case "pgup", "pgdown", "home", "end", "ctrl+l":
			m.messages.HandleKey(msg.String())
			return m, nil
		}
	}
	var editorCmd tea.Cmd
	m.editor, editorCmd = m.editor.Update(msg)
	if editorCmd != nil {
		cmds = append(cmds, editorCmd)
	}
	if submitted := m.editor.Submitted(); submitted != "" {
		if strings.HasPrefix(submitted, "/") {
			m.dialog = nil
			return m.handleSlashCommand(submitted)
		}
		m.messages.AppendUser(submitted)
		m.streaming = true
		m.status.SetStreaming(true)
		cmds = append(cmds, m.sendChatCmd(submitted))
	}
	return m, tea.Batch(cmds...)
}

func (m Model) handleCtrlC() (tea.Model, tea.Cmd) {
	if m.streaming {
		_ = m.client.ChatAbort(m.app.Session, m.currentRunID)
		m.streaming = false
		m.currentRunID = ""
		m.status.SetStreaming(false)
		return m, nil
	}
	return m, m.saveStateCmd()
}

func (m Model) handleSlashCommand(input string) (tea.Model, tea.Cmd) {
	parts := strings.Fields(input)
	if len(parts) == 0 {
		return m, nil
	}
	cmd := parts[0]
	args := ""
	if len(parts) > 1 {
		args = strings.Join(parts[1:], " ")
	}
	switch cmd {
	case "/session":
		if args == "new" {
			m.app.Session = "tui:" + time.Now().Format("20060102-150405")
			m.messages = chat.NewMessages()
			return m, m.persistStateCmd()
		}
		if args == "reset" {
			session := m.app.Session
			return m, func() tea.Msg {
				if err := m.client.SessionsReset(session); err != nil {
					return ChatErrorMsg{Message: err.Error()}
				}
				return SessionResetDoneMsg{Key: session}
			}
		}
		return m, func() tea.Msg {
			sessions, err := m.client.SessionsList()
			if err != nil {
				return ChatErrorMsg{Message: err.Error()}
			}
			return SessionsLoadedMsg{Sessions: sessions}
		}
	case "/model":
		if args != "" {
			return m, func() tea.Msg {
				current, err := m.client.ModelsSet(args)
				if err != nil {
					return ChatErrorMsg{Message: err.Error()}
				}
				return ModelSetMsg{ID: current}
			}
		}
		return m, func() tea.Msg {
			models, current, err := m.client.ModelsList()
			if err != nil {
				return ChatErrorMsg{Message: err.Error()}
			}
			return ModelsLoadedMsg{Models: models, Current: current}
		}
	case "/clear":
		m.messages = chat.NewMessages()
	case "/help":
		m.messages.AppendAssistant("Commands: /session, /session new, /session reset, /model, /model <name>, /theme <name>, /clear, /status, /help, /quit")
	case "/quit":
		return m, m.saveStateCmd()
	case "/theme":
		if args == "" {
			m.dialog = newThemesMenuDialog()
			return m, nil
		}
		if theme.Set(args) {
			m.app.Theme = args
			m.header = header.New()
			m.messages.InvalidateTheme()
			return m, m.persistStateCmd()
		}
		m.messages.AppendError("Unknown theme: " + args + ". Available: " + strings.Join(theme.List(), ", "))
	case "/status":
		return m, m.syncStatusCmd(true)
	default:
		m.messages.AppendError("Unknown command: " + cmd)
	}
	return m, nil
}

func (m Model) View() tea.View {
	t := theme.Current()
	if t == nil {
		return tea.NewView("Loading...")
	}

	content := lipgloss.JoinVertical(
		lipgloss.Left,
		m.header.View(),
	)
	if !m.header.IsCompact() {
		content = lipgloss.JoinVertical(
			lipgloss.Left,
			content,
			transcriptSpacer(m.width),
		)
	}
	content = lipgloss.JoinVertical(
		lipgloss.Left,
		content,
		transcriptFrameView(m.messages.View(), m.width, m.messages.HasContentAbove()),
		m.status.View(),
		m.editor.View(),
		m.footer.View(),
	)
	if m.width > 0 && m.height > 0 {
		canvas := lipgloss.NewCanvas(m.width, m.height)
		layers := []*lipgloss.Layer{
			lipgloss.NewLayer(content).X(0).Y(0),
		}
		if m.dialog != nil {
			dialogView := m.dialog.View()
			layers = append(layers, lipgloss.NewLayer(dialogView).
				X(max(0, (m.width-lipgloss.Width(dialogView))/2)).
				Y(max(0, (m.height-lipgloss.Height(dialogView))/2)))
		}
		canvas.Compose(lipgloss.NewCompositor(layers...))
		content = canvas.Render()
	}
	view := tea.NewView(content)
	view.AltScreen = true
	view.MouseMode = tea.MouseModeCellMotion
	view.BackgroundColor = t.Background
	return view
}

type sessionDialog struct{ dialogcmp.SessionsModel }

func (d sessionDialog) Update(msg tea.Msg) (Dialog, tea.Cmd) {
	next, cmd := d.SessionsModel.Update(msg)
	return sessionDialog{next}, cmd
}

type modelsDialog struct{ dialogcmp.ModelsModel }

func (d modelsDialog) Update(msg tea.Msg) (Dialog, tea.Cmd) {
	next, cmd := d.ModelsModel.Update(msg)
	return modelsDialog{next}, cmd
}

type commandsDialog struct {
	dialogcmp.CommandsModel
}

func (d commandsDialog) Update(msg tea.Msg) (Dialog, tea.Cmd) {
	next, cmd := d.CommandsModel.Update(msg)
	return commandsDialog{CommandsModel: next}, cmd
}

func formatStatusSummary(payload client.StatusPayload) string {
	parts := make([]string, 0, 4)
	if payload.Model != "" {
		parts = append(parts, "model "+payload.Model)
	}
	if payload.Session != "" {
		parts = append(parts, "session "+payload.Session)
	}
	if usage := formatUsageSummary(payload.Usage); usage != "" {
		parts = append(parts, "usage "+usage)
	}
	if len(payload.Channels) > 0 {
		channels := make([]string, 0, len(payload.Channels))
		for _, channel := range payload.Channels {
			channels = append(channels, channel.Name+"="+channel.Status)
		}
		parts = append(parts, "channels "+strings.Join(channels, ", "))
	}
	if len(parts) == 0 {
		return "status ok"
	}
	return "status: " + strings.Join(parts, " | ")
}

func formatUsageSummary(usage client.UsageInfo) string {
	if usage.ContextWindow <= 0 || usage.TotalTokens <= 0 {
		return ""
	}
	percentage := int((float64(usage.TotalTokens)/float64(usage.ContextWindow))*100 + 0.5)
	return fmt.Sprintf("%d%% (%s/%s)", percentage, formatUsageTokens(usage.TotalTokens), formatUsageTokens(usage.ContextWindow))
}

func formatUsageTokens(value int) string {
	switch {
	case value >= 1_000_000:
		text := fmt.Sprintf("%.1fM", float64(value)/1_000_000)
		return strings.Replace(text, ".0M", "M", 1)
	case value >= 1_000:
		text := fmt.Sprintf("%.1fK", float64(value)/1_000)
		return strings.Replace(text, ".0K", "K", 1)
	default:
		return fmt.Sprintf("%d", value)
	}
}

func transcriptSpacer(width int) string {
	if width <= 0 {
		return ""
	}
	return strings.Repeat(" ", width)
}

func transcriptFrameView(content string, width int, hasContentAbove bool) string {
	t := theme.Current()
	if t == nil {
		return content
	}
	style := lipgloss.NewStyle().
		Background(t.Panel).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(t.Border)
	if width > 2 {
		style = style.Width(width - 2)
	}
	frame := style.Render(content)
	if !hasContentAbove {
		return frame
	}
	hint := lipgloss.NewStyle().
		Foreground(t.TextMuted).
		Width(max(0, width-2)).
		Align(lipgloss.Right).
		Render("↑ PgUp/PgDn")
	return lipgloss.JoinVertical(lipgloss.Left, hint, frame)
}
