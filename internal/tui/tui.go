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
	cfgpkg "github.com/Nomadcxx/smolbot/pkg/config"
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
type ChannelInboundMsg struct {
	Channel string
	ChatID  string
	Content string
}
type ChannelOutboundMsg struct {
	Channel string
	ChatID  string
	Content string
}
type SessionResetDoneMsg struct{ Key string }
type ModelsLoadedMsg struct {
	Models  []client.ModelInfo
	Current string
}
type ModelSetMsg struct{ ID string }
type ModelCurrentMsg struct {
	Current string
}
type SkillsLoadedMsg struct{ Skills []client.SkillInfo }
type MCPServersLoadedMsg struct{ Servers []client.MCPServerInfo }
type ProvidersLoadedMsg struct {
	Models  []client.ModelInfo
	Current string
	Status  client.StatusPayload
}
type CompactStartMsg struct{}
type CompactDoneMsg struct {
	Compacted  bool
	Reason     string
	Original   int
	Compressed int
	Reduction  float64
}
type CompactErrorMsg struct{ Err error }
type CompactionTickMsg struct{}

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
	Compact(session string) (*client.CompactResult, error)
	Skills() ([]client.SkillInfo, error)
	MCPServers() ([]client.MCPServerInfo, error)
}

type Model struct {
	width, height   int
	app             *app.App
	providerConfig  *cfgpkg.Config
	client          gatewayClient
	header          header.Model
	messages        chat.MessagesModel
	editor          chat.EditorModel
	status          status.Model
	footer          status.FooterModel
	dialog          Dialog
	eventCh         chan tea.Msg
	connected       bool
	reconnectWait   time.Duration
	streaming       bool
	currentRunID    string
	contextWarned   bool
	compactionFrame int
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
		app:            a,
		providerConfig: loadProviderConfig(),
		client:         c,
		header:         header.New(),
		messages:       chat.NewMessages(),
		editor:         chat.NewEditor(),
		status:         status.New(a),
		footer:         status.NewFooter(a),
		eventCh:        eventCh,
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
			return StatusLoadedMsg{Echo: echo}
		}
		return StatusLoadedMsg{Payload: payload, Echo: echo}
	}
}

func (m Model) compactCmd() tea.Cmd {
	return func() tea.Msg {
		result, err := m.client.Compact(m.app.Session)
		if err != nil {
			return CompactErrorMsg{Err: err}
		}
		return CompactDoneMsg{
			Compacted:  result.Compacted,
			Reason:     result.Reason,
			Original:   result.OriginalTokens,
			Compressed: result.CompressedTokens,
			Reduction:  result.ReductionPercent,
		}
	}
}

func (m Model) compactTickCmd() tea.Cmd {
	return tea.Tick(80*time.Millisecond, func(time.Time) tea.Msg { return CompactionTickMsg{} })
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
	case CompactStartMsg:
		m.footer.SetCompacting(true)
		m.compactionFrame = 0
		m.footer.SetCompactionFrame(0)
		return m, m.compactTickCmd()
	case CompactDoneMsg:
		m.footer.SetCompacting(false)
		if !msg.Compacted {
			m.messages.AppendSystem(compactionReasonMessage(msg.Reason))
			return m, nil
		}
		m.footer.SetCompression(&client.CompressionInfo{
			Enabled:          true,
			OriginalTokens:   msg.Original,
			CompressedTokens: msg.Compressed,
			ReductionPercent: msg.Reduction,
		})
		m.messages.AppendSystem(fmt.Sprintf(
			"Context compacted: %s → %s (%.0f%% reduction)",
			formatTokens(msg.Original), formatTokens(msg.Compressed), msg.Reduction,
		))
		m.contextWarned = false
		return m, m.syncStatusCmd(false)
	case CompactErrorMsg:
		m.footer.SetCompacting(false)
		if msg.Err != nil {
			m.messages.AppendError("Context compaction failed: " + msg.Err.Error())
		}
		return m, nil
	case CompactionTickMsg:
		if !m.footer.IsCompacting() {
			return m, nil
		}
		m.compactionFrame++
		m.footer.SetCompactionFrame(m.compactionFrame)
		return m, m.compactTickCmd()
	case ChatProgressMsg:
		m.messages.SetProgress(m.messages.GetProgress() + msg.Content)
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
	case ChannelInboundMsg:
		label := "[" + msg.Channel + "] " + msg.Content
		m.messages.AppendUser(label)
		return m, nil
	case ChannelOutboundMsg:
		m.messages.AppendAssistant(msg.Content)
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
			pct := int((float64(msg.Payload.Usage.TotalTokens)/float64(msg.Payload.Usage.ContextWindow))*100 + 0.5)
			m.header.SetContextPercent(pct)
			m.maybeWarnContextUsage(pct)
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
		m.contextWarned = false
		return m, tea.Batch(m.persistStateCmd(), m.syncStatusCmd(false))
	case ModelsLoadedMsg:
		m.dialog = modelsDialog{dialogcmp.NewModels(msg.Models, msg.Current)}
		return m, nil
	case SkillsLoadedMsg:
		m.dialog = skillsDialog{dialogcmp.NewSkills(msg.Skills)}
		return m, nil
	case MCPServersLoadedMsg:
		m.dialog = mcpServersDialog{dialogcmp.NewMCPServers(msg.Servers)}
		return m, nil
	case ProvidersLoadedMsg:
		m.dialog = providersDialog{dialogcmp.NewProviders(buildProviderLines(msg.Models, msg.Current, msg.Status, m.providerConfig))}
		return m, nil
	case ModelSetMsg:
		m.app.Model = msg.ID
		return m, tea.Batch(m.persistStateCmd(), m.syncStatusCmd(false))
	case dialogcmp.SessionChosenMsg:
		m.app.Session = msg.Key
		m.messages = chat.NewMessages()
		m.dialog = nil
		m.contextWarned = false
		return m, tea.Batch(m.loadHistoryCmd(), m.persistStateCmd(), m.syncStatusCmd(false))
	case dialogcmp.SessionNewMsg:
		m.app.Session = "tui:" + time.Now().Format("20060102-150405")
		m.messages = chat.NewMessages()
		m.dialog = nil
		m.contextWarned = false
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
			m.footer.SetCompacting(false)
			m.messages.AppendSystem(fmt.Sprintf(
				"Context compacted: %s → %s (%.0f%% reduction)",
				formatTokens(p.OriginalTokens), formatTokens(p.CompressedTokens), p.ReductionPercent,
			))
			m.contextWarned = false
		case "compact.start":
			mapped = CompactStartMsg{}
		case "compact.done":
			m.footer.SetCompacting(false)
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
				pct := int((float64(p.TotalTokens)/float64(p.ContextWindow))*100 + 0.5)
				m.header.SetContextPercent(pct)
				m.maybeWarnContextUsage(pct)
			}
		case "channel.inbound":
			var p client.ChannelMessagePayload
			_ = json.Unmarshal(msg.Event.Payload, &p)
			mapped = ChannelInboundMsg{Channel: p.Channel, ChatID: p.ChatID, Content: p.Content}
		case "channel.outbound":
			var p client.ChannelMessagePayload
			_ = json.Unmarshal(msg.Event.Payload, &p)
			mapped = ChannelOutboundMsg{Channel: p.Channel, ChatID: p.ChatID, Content: p.Content}
		case "channel.progress":
			var p client.ChannelMessagePayload
			_ = json.Unmarshal(msg.Event.Payload, &p)
			m.messages.SetProgress(m.messages.GetProgress() + p.Content)
		case "channel.error":
			var p client.ChannelErrorPayload
			_ = json.Unmarshal(msg.Event.Payload, &p)
			m.messages.AppendError("[" + p.Channel + "] " + p.Error)
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
			m.contextWarned = false
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
		m.contextWarned = false
	case "/help":
		m.messages.AppendAssistant("Commands: /compact, /session, /session new, /session reset, /model, /model <name>, /theme <name>, /skills, /mcps, /providers, /keybindings, /clear, /status, /help, /quit")
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
	case "/compact", "/compress":
		m.footer.SetCompacting(true)
		m.compactionFrame = 0
		m.footer.SetCompactionFrame(0)
		m.contextWarned = false
		return m, m.compactCmd()
	case "/skills":
		return m, func() tea.Msg {
			skills, err := m.client.Skills()
			if err != nil {
				return ChatErrorMsg{Message: err.Error()}
			}
			return SkillsLoadedMsg{Skills: skills}
		}
	case "/mcps":
		return m, func() tea.Msg {
			servers, err := m.client.MCPServers()
			if err != nil {
				return ChatErrorMsg{Message: err.Error()}
			}
			return MCPServersLoadedMsg{Servers: servers}
		}
	case "/providers":
		return m, func() tea.Msg {
			models, current, err := m.client.ModelsList()
			if err != nil {
				return ChatErrorMsg{Message: err.Error()}
			}
			status, err := m.client.Status(m.app.Session)
			if err != nil {
				return ChatErrorMsg{Message: err.Error()}
			}
			return ProvidersLoadedMsg{Models: models, Current: current, Status: status}
		}
	case "/keybindings":
		m.dialog = keybindingsDialog{dialogcmp.NewKeybindings()}
		return m, nil
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
	content = lipgloss.JoinVertical(
		lipgloss.Left,
		content,
		transcriptFrameView(m.messages.View(), m.width, m.messages.HasContentAbove()),
		m.status.View(),
		m.editor.View(),
		m.footer.View(),
	)
	if m.width > 0 && m.height > 0 && m.dialog != nil {
		canvas := lipgloss.NewCanvas(m.width, m.height)
		layers := []*lipgloss.Layer{
			lipgloss.NewLayer(content).X(0).Y(0),
		}
		dialogView := m.dialog.View()
		layers = append(layers, lipgloss.NewLayer(dialogView).
			X(max(0, (m.width-lipgloss.Width(dialogView))/2)).
			Y(max(0, (m.height-lipgloss.Height(dialogView))/2)))
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

type skillsDialog struct{ dialogcmp.SkillsModel }

func (d skillsDialog) Update(msg tea.Msg) (Dialog, tea.Cmd) {
	next, cmd := d.SkillsModel.Update(msg)
	return skillsDialog{next}, cmd
}

type mcpServersDialog struct{ dialogcmp.MCPServersModel }

func (d mcpServersDialog) Update(msg tea.Msg) (Dialog, tea.Cmd) {
	next, cmd := d.MCPServersModel.Update(msg)
	return mcpServersDialog{next}, cmd
}

type providersDialog struct{ dialogcmp.ProvidersModel }

func (d providersDialog) Update(msg tea.Msg) (Dialog, tea.Cmd) {
	next, cmd := d.ProvidersModel.Update(msg)
	return providersDialog{next}, cmd
}

type keybindingsDialog struct{ dialogcmp.KeybindingsModel }

func (d keybindingsDialog) Update(msg tea.Msg) (Dialog, tea.Cmd) {
	next, cmd := d.KeybindingsModel.Update(msg)
	return keybindingsDialog{next}, cmd
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

func formatTokens(value int) string {
	return formatUsageTokens(value)
}

func compactionReasonMessage(reason string) string {
	switch strings.TrimSpace(reason) {
	case "compression disabled":
		return "Context compaction is disabled."
	case "not enough history":
		return "Nothing to compact yet."
	case "no reduction achieved":
		return "Context compaction did not reduce token usage."
	default:
		if strings.TrimSpace(reason) != "" {
			return "Context compaction skipped: " + reason
		}
		return "Context compaction skipped."
	}
}

func (m *Model) maybeWarnContextUsage(pct int) {
	if pct < 90 || m.contextWarned {
		return
	}
	m.contextWarned = true
	m.messages.AppendSystem(fmt.Sprintf("Context is %d%% full. Use /compact to free space.", pct))
}

func buildProviderLines(models []client.ModelInfo, current string, status client.StatusPayload, cfg *cfgpkg.Config) []string {
	currentModel := firstNonEmptyString(current, status.Model, "unknown")
	currentProvider := providerNameForModel(models, currentModel)
	if currentProvider == "" {
		currentProvider = "unknown"
	}

	lines := []string{
		"Current model: " + currentModel,
		"Current provider: " + currentProvider,
	}
	if cfg != nil {
		if providerCfg, ok := cfg.Providers[currentProvider]; ok && strings.TrimSpace(providerCfg.APIBase) != "" {
			lines = append(lines, "API base URL: "+providerCfg.APIBase)
		}
		if len(cfg.Providers) > 0 {
			names := make([]string, 0, len(cfg.Providers))
			for name := range cfg.Providers {
				names = append(names, name)
			}
			lines = append(lines, "Available providers: "+strings.Join(sortStrings(names), ", "))
		}
	}
	if status.Usage.ContextWindow > 0 {
		lines = append(lines, "Context window: "+formatTokens(status.Usage.ContextWindow))
	}
	return lines
}

func firstNonEmptyString(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func providerNameForModel(models []client.ModelInfo, current string) string {
	for _, model := range models {
		if model.ID == current {
			return model.Provider
		}
	}
	return ""
}

func sortStrings(values []string) []string {
	if len(values) < 2 {
		return values
	}
	for i := 1; i < len(values); i++ {
		for j := i; j > 0 && values[j] < values[j-1]; j-- {
			values[j], values[j-1] = values[j-1], values[j]
		}
	}
	return values
}

func loadProviderConfig() *cfgpkg.Config {
	paths := cfgpkg.DefaultPaths()
	cfg, err := cfgpkg.Load(paths.ConfigFile())
	if err == nil {
		return cfg
	}
	fallback := cfgpkg.DefaultConfig()
	return &fallback
}

func transcriptFrameView(content string, width int, hasContentAbove bool) string {
	if !hasContentAbove {
		return content
	}
	t := theme.Current()
	if t == nil {
		return content
	}
	hint := lipgloss.NewStyle().
		Foreground(t.TextMuted).
		Width(max(0, width)).
		Align(lipgloss.Right).
		Render("scroll ↑↓")
	return hint + "\n" + content
}
