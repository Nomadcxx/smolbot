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
	"github.com/Nomadcxx/smolbot/internal/components/common"
	dialogcmp "github.com/Nomadcxx/smolbot/internal/components/dialog"
	"github.com/Nomadcxx/smolbot/internal/components/header"
	sidebarcmp "github.com/Nomadcxx/smolbot/internal/components/sidebar"
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
type CronJobsLoadedMsg struct {
	Jobs []client.CronJob
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
type ClipboardCopiedMsg struct{ Text string }
type ClipboardErrorMsg struct{ Err error }
type flashClearMsg struct{ Seq int }
type flushProgressMsg struct{ Seq int }

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
	CronJobs() ([]client.CronJob, error)
	Compact(session string) (*client.CompactResult, error)
	Skills() ([]client.SkillInfo, error)
	MCPServers() ([]client.MCPServerInfo, error)
}

type Model struct {
	width, height        int
	app                  *app.App
	providerConfig       *cfgpkg.Config
	client               gatewayClient
	header               header.Model
	messages             chat.MessagesModel
	editor               chat.EditorModel
	status               status.Model
	footer               status.FooterModel
	dialog               Dialog
	eventCh              chan tea.Msg
	connected            bool
	reconnectWait        time.Duration
	streaming            bool
	currentRunID         string
	contextWarned        bool
	usageAlertKey        string
	flashSeq             int
	compactionFrame      int
	progressBuffer       string
	progressFlushPending bool
	progressFlushSeq     int
	sidebarVisible       bool
	compactMode          bool
	detailsOpen          bool
	mainWidth            int
	sidebarWidth         int
	overlayHeight        int
	headerWidth          int
	statusWidth          int
	footerWidth          int
	messagesWidth        int
	sidebar              sidebarcmp.Model
	clipboardWrite       func(string) error
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
		sidebarVisible: a.SidebarVisible,
		sidebar:        newSidebar(a, cfg.MCPServers),
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
			Theme:          m.app.Theme,
			LastSession:    m.app.Session,
			LastModel:      m.app.Model,
			SidebarVisible: boolPtr(m.sidebarVisible),
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
			Theme:          m.app.Theme,
			LastSession:    m.app.Session,
			LastModel:      m.app.Model,
			SidebarVisible: boolPtr(m.sidebarVisible),
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

func (m Model) loadCronJobsCmd() tea.Cmd {
	return func() tea.Msg {
		jobs, err := m.client.CronJobs()
		if err != nil {
			return nil
		}
		return CronJobsLoadedMsg{Jobs: jobs}
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

func (m Model) flashClearCmd(seq int) tea.Cmd {
	return tea.Tick(2*time.Second, func(time.Time) tea.Msg { return flashClearMsg{Seq: seq} })
}

func (m Model) copyTextCmd(content string) tea.Cmd {
	if strings.TrimSpace(content) == "" {
		return nil
	}
	return func() tea.Msg {
		return ClipboardCopiedMsg{Text: content}
	}
}

func (m Model) copyLastAssistantCmd() tea.Cmd {
	return m.copyTextCmd(m.messages.LastAssistantContent())
}

func (m Model) nativeClipboardCmd(text string) tea.Cmd {
	return func() tea.Msg {
		_ = common.WriteClipboard(text, m.clipboardWrite)
		return nil
	}
}

func (m Model) progressFlushCmd(seq int) tea.Cmd {
	return tea.Tick(16*time.Millisecond, func(time.Time) tea.Msg { return flushProgressMsg{Seq: seq} })
}

func (m *Model) resetProgressFlush() {
	m.progressBuffer = ""
	m.progressFlushPending = false
	m.progressFlushSeq++
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.compactMode = m.width >= 80 && m.width < 120
		if m.compactMode || m.width < 80 {
			m.detailsOpen = false
		}
		m.header.SetCompact(m.height <= 30)
		m.editor.SetCompact(m.height <= 30)
		m.recalcLayout()
		return m, nil
	case ConnectedMsg:
		m.connected = true
		m.reconnectWait = 0
		m.status.SetConnected(true)
		m.status.SetReconnecting(false)
		if cwd, err := os.Getwd(); err == nil {
			m.header.SetWorkDir(cwd)
			m.sidebar.SetCWD(cwd)
		}
		m.sidebar.SetSession(m.app.Session)
		m.sidebar.SetModel(m.app.Model)
		m.footer.SetSession(m.app.Session)
		m.footer.SetModel(m.app.Model)
		m.recalcLayout()
		return m, tea.Batch(
			m.loadHistoryCmd(),
			m.syncModelCmd(),
			m.syncStatusCmd(false),
			m.loadCronJobsCmd(),
		)
	case CtrlCMsg:
		return m.handleCtrlC()
	case ChatStartedMsg:
		m.currentRunID = msg.RunID
		return m, nil
	case ChatDoneMsg:
		m.resetProgressFlush()
		m.streaming = false
		m.currentRunID = ""
		m.status.SetStreaming(false)
		m.messages.AppendAssistant(msg.Content)
		return m, tea.Batch(m.syncStatusCmd(false), m.loadCronJobsCmd())
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
		m.progressBuffer += msg.Content
		if m.progressFlushPending {
			return m, nil
		}
		m.progressFlushPending = true
		m.progressFlushSeq++
		return m, m.progressFlushCmd(m.progressFlushSeq)
	case ThinkingDoneMsg:
		m.messages.AppendThinking(msg.Content)
		return m, nil
	case ChatErrorMsg:
		m.resetProgressFlush()
		m.streaming = false
		m.currentRunID = ""
		m.status.SetStreaming(false)
		m.messages.AppendError(msg.Message)
		return m, nil
	case flushProgressMsg:
		if msg.Seq != m.progressFlushSeq {
			return m, nil
		}
		m.progressFlushPending = false
		if m.progressBuffer == "" {
			return m, nil
		}
		m.messages.SetProgress(m.messages.GetProgress() + m.progressBuffer)
		m.progressBuffer = ""
		return m, nil
	case ClipboardCopiedMsg:
		m.messages.ClearSelection()
		m.flashSeq++
		m.status.SetFlash("Copied to clipboard")
		return m, tea.Sequence(
			tea.SetClipboard(msg.Text),
			m.nativeClipboardCmd(msg.Text),
			m.flashClearCmd(m.flashSeq),
		)
	case ClipboardErrorMsg:
		if msg.Err != nil {
			m.messages.AppendError("Clipboard copy failed: " + msg.Err.Error())
		}
		return m, nil
	case flashClearMsg:
		if msg.Seq == m.flashSeq {
			m.status.ClearFlash()
		}
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
			m.footer.SetModel(msg.Payload.Model)
		}
		m.footer.SetSession(firstNonEmpty(msg.Payload.Session, m.app.Session))
		m.footer.SetUsage(msg.Payload.Usage)
		m.header.SetModel(m.app.Model)
		m.sidebar.SetSession(firstNonEmpty(msg.Payload.Session, m.app.Session))
		m.sidebar.SetModel(m.app.Model)
		m.sidebar.SetUsage(msg.Payload.Usage)
		m.sidebar.SetPersistedUsage(msg.Payload.PersistedUsage)
		m.maybeWarnPersistedUsage(firstNonEmpty(msg.Payload.Session, m.app.Session), msg.Payload.UsageAlert, msg.Payload.PersistedUsage)
		m.sidebar.SetChannels(mapChannelEntries(msg.Payload.Channels))
		if cwd, err := os.Getwd(); err == nil {
			m.sidebar.SetCWD(cwd)
		}
		if msg.Payload.Usage.ContextWindow > 0 && msg.Payload.Usage.TotalTokens > 0 {
			pct := int((float64(msg.Payload.Usage.TotalTokens)/float64(msg.Payload.Usage.ContextWindow))*100 + 0.5)
			m.header.SetContextPercent(pct)
			m.maybeWarnContextUsage(pct)
		}
		if msg.Echo {
			m.messages.AppendAssistant(formatStatusSummary(msg.Payload))
		}
		m.recalcLayout()
		return m, nil
	case ModelCurrentMsg:
		if msg.Current != "" {
			m.app.Model = msg.Current
			m.footer.SetModel(msg.Current)
			m.header.SetModel(msg.Current)
			m.sidebar.SetModel(msg.Current)
		}
		return m, nil
	case CronJobsLoadedMsg:
		m.sidebar.SetCronJobs(msg.Jobs)
		m.recalcLayout()
		return m, nil
	case SessionsLoadedMsg:
		m.dialog = sessionDialog{dialogcmp.NewSessions(msg.Sessions, m.app.Session)}
		return m, nil
	case SessionResetDoneMsg:
		m.app.Session = msg.Key
		m.sidebar.SetSession(msg.Key)
		m.footer.SetSession(msg.Key)
		m.messages = chat.NewMessages()
		m.contextWarned = false
		m.clearPersistedUsageWarning()
		m.resetProgressFlush()
		m.recalcLayout()
		return m, tea.Batch(m.persistStateCmd(), m.syncStatusCmd(false))
	case ModelsLoadedMsg:
		current := msg.Current
		if current == "" {
			current = m.app.Model
		}
		m.dialog = modelsDialog{dialogcmp.NewModels(msg.Models, current)}
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
		m.footer.SetModel(msg.ID)
		m.sidebar.SetModel(msg.ID)
		m.recalcLayout()
		return m, tea.Batch(m.persistStateCmd(), m.syncStatusCmd(false))
	case dialogcmp.SessionChosenMsg:
		m.app.Session = msg.Key
		m.sidebar.SetSession(msg.Key)
		m.footer.SetSession(msg.Key)
		m.messages = chat.NewMessages()
		m.dialog = nil
		m.contextWarned = false
		m.clearPersistedUsageWarning()
		m.resetProgressFlush()
		m.recalcLayout()
		return m, tea.Batch(m.loadHistoryCmd(), m.persistStateCmd(), m.syncStatusCmd(false))
	case dialogcmp.SessionNewMsg:
		m.app.Session = "tui:" + time.Now().Format("20060102-150405")
		m.sidebar.SetSession(m.app.Session)
		m.footer.SetSession(m.app.Session)
		m.messages = chat.NewMessages()
		m.dialog = nil
		m.contextWarned = false
		m.clearPersistedUsageWarning()
		m.resetProgressFlush()
		m.recalcLayout()
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
		m.resetProgressFlush()
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
			m.sidebar.SetCompression(&p)
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
			usage := client.UsageInfo{
				PromptTokens:     p.PromptTokens,
				CompletionTokens: p.CompletionTokens,
				TotalTokens:      p.TotalTokens,
				ContextWindow:    p.ContextWindow,
			}
			m.footer.SetUsage(usage)
			m.sidebar.SetUsage(usage)
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
			mapped = ChatProgressMsg{Content: p.Content}
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
		m.recalcLayout()
		return m, tea.Batch(cmds...)
	case CompressionStatusMsg:
		m.footer.SetCompression(&msg.Info)
		m.sidebar.SetCompression(&msg.Info)
		m.recalcLayout()
		return m, nil
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
	case tea.MouseClickMsg:
		if m.dialog != nil {
			return m, nil
		}
		mouse := msg.Mouse()
		if m.shouldConsumeMouse(mouse) {
			return m, nil
		}
		if x, y, ok := m.transcriptMousePoint(mouse); ok && m.messages.HandleMouseDown(x, y) {
			return m, nil
		}
	case tea.MouseMotionMsg:
		if m.dialog != nil {
			return m, nil
		}
		mouse := msg.Mouse()
		if x, y, ok := m.transcriptMousePoint(mouse); ok && m.messages.HandleMouseDrag(x, y) {
			return m, nil
		}
	case tea.MouseReleaseMsg:
		if m.dialog != nil {
			return m, nil
		}
		mouse := msg.Mouse()
		x, y, ok := m.transcriptMousePoint(mouse)
		if !ok {
			x = mouse.X
			y = mouse.Y
		}
		if m.messages.HandleMouseUp(x, y) {
			return m, m.copyTextCmd(m.messages.SelectedText())
		}
		return m, nil
	case tea.MouseMsg:
		if m.dialog == nil && m.shouldConsumeMouse(msg.Mouse()) {
			return m, nil
		}
	case tea.KeyMsg:
		if m.dialog != nil {
			next, cmd := m.dialog.Update(msg)
			m.dialog = next
			return m, cmd
		}
		switch msg.String() {
		case "ctrl+c":
			return m.handleCtrlC()
		case "esc":
			if m.editor.Focused() {
				m.editor.Blur()
				return m, nil
			}
		case "i":
			if !m.editor.Focused() {
				return m, m.editor.Focus()
			}
		case "ctrl+d":
			if m.width < 80 {
				return m, nil
			}
			if m.compactMode {
				m.detailsOpen = !m.detailsOpen
				m.recalcLayout()
				return m, nil
			}
			m.sidebarVisible = !m.sidebarVisible
			m.sidebar.SetVisible(m.sidebarVisible)
			m.recalcLayout()
			return m, m.persistStateCmd()
		case "f1", "ctrl+m":
			m.dialog = newMenuDialog()
			return m, nil
		case "c", "y":
			if !m.editor.Focused() {
				if cmd := m.copyLastAssistantCmd(); cmd != nil {
					return m, cmd
				}
				return m, nil
			}
		case "pgup", "pgdown", "home", "end", "ctrl+l":
			m.messages.HandleKey(msg.String())
			return m, nil
		}
	}
	var editorCmd tea.Cmd
	if m.editor.Focused() {
		m.editor, editorCmd = m.editor.Update(msg)
		if editorCmd != nil {
			cmds = append(cmds, editorCmd)
		}
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
		m.resetProgressFlush()
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
			m.resetProgressFlush()
			m.app.Session = "tui:" + time.Now().Format("20060102-150405")
			m.sidebar.SetSession(m.app.Session)
			m.footer.SetSession(m.app.Session)
			m.messages = chat.NewMessages()
			m.contextWarned = false
			m.clearPersistedUsageWarning()
			m.recalcLayout()
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
		m.resetProgressFlush()
		m.messages = chat.NewMessages()
		m.contextWarned = false
		m.clearPersistedUsageWarning()
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
		m.resetProgressFlush()
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

	mainWidth := m.mainWidth
	if mainWidth <= 0 {
		mainWidth = m.width
	}
	main := lipgloss.JoinVertical(
		lipgloss.Left,
		m.header.View(),
		transcriptFrameView(m.messages.View(), mainWidth, m.messages.HasContentAbove()),
		m.status.View(),
		m.editor.View(),
	)

	content := main
	if m.compactMode && m.detailsOpen && m.width >= 80 {
		overlaySidebar := m.sidebar
		overlaySidebar.SetSize(mainWidth, m.height)
		overlay := lipgloss.NewStyle().
			Width(mainWidth).
			Background(t.Panel).
			Foreground(t.Text).
			Render(overlaySidebar.CompactView())
		content = lipgloss.JoinVertical(lipgloss.Left, overlay, main)
	} else if m.shouldShowSidebar() {
		mainWithFooter := lipgloss.JoinVertical(lipgloss.Left, main, m.footer.View())
		sidebar := lipgloss.NewStyle().
			Width(m.sidebarWidth).
			Height(lipgloss.Height(mainWithFooter)).
			Background(t.Panel).
			Foreground(t.Text).
			Render(m.sidebar.View())
		separator := m.renderSidebarSeparator(lipgloss.Height(mainWithFooter))
		content = lipgloss.JoinHorizontal(lipgloss.Top, mainWithFooter, separator, sidebar)
	} else {
		content = lipgloss.JoinVertical(lipgloss.Left, content, m.footer.View())
	}
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

func (m *Model) maybeWarnPersistedUsage(session string, alert *client.UsageAlert, summary *client.UsageSummary) {
	if alert == nil {
		alert = usageAlertFromSummary(summary)
	}
	key := usageAlertKey(session, alert)
	if key == "" {
		m.clearPersistedUsageWarning()
		return
	}
	if m.usageAlertKey == key {
		return
	}
	m.usageAlertKey = key
	if strings.TrimSpace(alert.Message) != "" {
		m.messages.AppendSystem(alert.Message)
	}
}

func (m *Model) clearPersistedUsageWarning() {
	m.usageAlertKey = ""
}

func usageAlertKey(session string, alert *client.UsageAlert) string {
	if alert == nil {
		return ""
	}
	if strings.TrimSpace(alert.WarningLevel) == "" && strings.TrimSpace(alert.Message) == "" {
		return ""
	}
	return strings.Join([]string{
		strings.TrimSpace(session),
		strings.TrimSpace(alert.ProviderID),
		strings.TrimSpace(alert.ModelName),
		strings.TrimSpace(alert.BudgetStatus),
		strings.TrimSpace(alert.WarningLevel),
		strings.TrimSpace(alert.Message),
	}, "\x00")
}

func usageAlertFromSummary(summary *client.UsageSummary) *client.UsageAlert {
	if summary == nil {
		return nil
	}
	if strings.TrimSpace(summary.WarningLevel) != "" {
		return &client.UsageAlert{
			ProviderID:   summary.ProviderID,
			ModelName:    summary.ModelName,
			BudgetStatus: summary.BudgetStatus,
			WarningLevel: summary.WarningLevel,
			Message:      persistedUsageAlertMessage(summary.ProviderID, summary.ModelName, summary.WarningLevel),
		}
	}
	if quotaAlert := quotaAlertFromSummary(summary); quotaAlert != nil {
		return quotaAlert
	}
	return nil
}

func persistedUsageAlertMessage(providerID, modelName, warningLevel string) string {
	label := strings.TrimSpace(modelName)
	providerID = strings.TrimSpace(providerID)
	if label == "" {
		label = providerID
	} else if providerID != "" && !strings.HasPrefix(label, providerID+"/") {
		label = providerID + "/" + label
	}
	if label == "" {
		label = "usage"
	}
	level := strings.TrimSpace(warningLevel)
	if level == "" {
		return "Usage warning for " + label + "."
	}
	return fmt.Sprintf("Usage warning for %s: %s budget threshold reached.", label, level)
}

func quotaAlertFromSummary(summary *client.UsageSummary) *client.UsageAlert {
	if summary == nil || summary.Quota == nil {
		return nil
	}
	state := strings.ToLower(strings.TrimSpace(summary.Quota.State))
	if state != "expired" && state != "stale" && state != "unavailable" {
		return nil
	}

	return &client.UsageAlert{
		ProviderID:   summary.ProviderID,
		ModelName:    summary.ModelName,
		BudgetStatus: "quota",
		WarningLevel: state,
		Message:      quotaAlertMessage(summary.ProviderID, summary.ModelName, state),
	}
}

func quotaAlertMessage(providerID, modelName, state string) string {
	label := strings.TrimSpace(modelName)
	providerID = strings.TrimSpace(providerID)
	if label == "" {
		label = providerID
	} else if providerID != "" && !strings.HasPrefix(label, providerID+"/") {
		label = providerID + "/" + label
	}
	if label == "" {
		label = "usage"
	}

	switch state {
	case "expired":
		return fmt.Sprintf("Quota status for %s expired. Reconnect your Ollama web session to refresh account usage.", label)
	case "stale":
		return fmt.Sprintf("Quota status for %s is stale. Showing the last cached account usage.", label)
	default:
		return fmt.Sprintf("Quota status for %s is unavailable. Observed usage remains available.", label)
	}
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

func (m *Model) recalcLayout() {
	if m.width <= 0 || m.height <= 0 {
		return
	}

	m.mainWidth = m.width
	m.sidebarWidth = 0
	m.overlayHeight = 0

	if m.compactMode {
		if m.detailsOpen && m.width >= 80 {
			overlaySidebar := m.sidebar
			overlaySidebar.SetSize(m.mainWidth, m.height-1)
			m.overlayHeight = lipgloss.Height(overlaySidebar.CompactView())
		}
	} else if m.sidebarVisible && m.width >= 120 {
		m.sidebarWidth = sidebarcmp.DefaultWidth
		m.mainWidth = max(1, m.width-m.sidebarWidth-1)
	}

	headerH := m.header.Height()
	editorH := m.editor.Height()
	statusH := 1
	footerH := 1
	chatH := m.height - headerH - editorH - statusH - footerH - m.overlayHeight
	if chatH < 3 {
		chatH = 3
	}

	m.header.SetWidth(m.mainWidth)
	m.headerWidth = m.mainWidth
	m.messages.SetSize(max(1, m.mainWidth-2), chatH)
	m.messagesWidth = max(1, m.mainWidth-2)
	m.editor.SetWidth(m.mainWidth)
	m.status.SetWidth(m.mainWidth)
	m.statusWidth = m.mainWidth
	m.footer.SetWidth(m.mainWidth)
	m.footerWidth = m.mainWidth
	m.sidebar.SetSize(sidebarcmp.DefaultWidth, m.height)
	if m.compactMode {
		m.sidebar.SetVisible(false)
	} else {
		m.sidebar.SetVisible(m.sidebarVisible && m.width >= 120)
	}
}

func (m Model) shouldShowSidebar() bool {
	return !m.compactMode && m.sidebarVisible && m.sidebarWidth > 0
}

func (m Model) shouldConsumeMouse(mouse tea.Mouse) bool {
	if m.compactMode {
		return m.detailsOpen && mouse.Y >= 0 && mouse.Y < m.overlayHeight
	}
	if !m.shouldShowSidebar() {
		return false
	}
	return mouse.X >= m.mainWidth
}

func (m Model) transcriptMousePoint(mouse tea.Mouse) (int, int, bool) {
	if m.width <= 0 || m.height <= 0 {
		return 0, 0, false
	}
	if mouse.X < 0 || mouse.X >= m.mainWidth {
		return 0, 0, false
	}
	headerH := m.header.Height()
	y := mouse.Y - headerH
	if y < 0 {
		return 0, 0, false
	}
	if m.messages.HasContentAbove() {
		if y == 0 {
			return 0, 0, false
		}
		y--
	}
	if y < 0 || y >= m.messages.Height() {
		return 0, 0, false
	}
	return mouse.X, y, true
}

func (m Model) renderSidebarSeparator(height int) string {
	if height <= 0 {
		return ""
	}
	t := theme.Current()
	if t == nil {
		return ""
	}
	return lipgloss.NewStyle().
		Width(1).
		Height(height).
		Foreground(t.Border).
		Render(strings.Repeat("│\n", max(0, height-1)) + "│")
}

func boolPtr(v bool) *bool {
	return &v
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func newSidebar(a *app.App, mcps []client.MCPServerInfo) sidebarcmp.Model {
	sidebar := sidebarcmp.New()
	sidebar.SetVisible(a.SidebarVisible)
	sidebar.SetSession(a.Session)
	sidebar.SetModel(a.Model)
	sidebar.SetMCPs(mapMCPEntries(mcps))
	return sidebar
}

func mapChannelEntries(channels []client.ChannelStatus) []sidebarcmp.ChannelEntry {
	items := make([]sidebarcmp.ChannelEntry, 0, len(channels))
	for _, channel := range channels {
		items = append(items, sidebarcmp.ChannelEntry{
			Name:  channel.Name,
			State: channel.Status,
		})
	}
	return items
}

func mapMCPEntries(servers []client.MCPServerInfo) []sidebarcmp.MCPEntry {
	items := make([]sidebarcmp.MCPEntry, 0, len(servers))
	for _, server := range servers {
		items = append(items, sidebarcmp.MCPEntry{
			Name:   server.Name,
			Status: server.Status,
			Tools:  server.Tools,
		})
	}
	return items
}
