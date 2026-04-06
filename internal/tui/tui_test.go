package tui

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/Nomadcxx/smolbot/internal/app"
	"github.com/Nomadcxx/smolbot/internal/client"
	"github.com/Nomadcxx/smolbot/internal/theme"
	_ "github.com/Nomadcxx/smolbot/internal/theme/themes"
	cfgpkg "github.com/Nomadcxx/smolbot/pkg/config"
	"github.com/gorilla/websocket"
)

var ansiPattern = regexp.MustCompile(`\x1b\[[0-9;]*m`)

type sendCall struct {
	session string
	message string
}

type fakeClient struct {
	sessions       []client.SessionInfo
	models         []client.ModelInfo
	skills         []client.SkillInfo
	mcps           []client.MCPServerInfo
	jobs           []client.CronJob
	current        string
	status         client.StatusPayload
	statuses       map[string]client.StatusPayload
	compact        client.CompactResult
	chatRun        string
	sends          []sendCall
	aborts         []abortCall
	modelErr       error
	resetErr       error
	compactErr     error
	modelSets      []string
	modelSetResult string
	providerCfgResult client.ProviderConfigurePayload
	providerCfgErr    error
	providerRemResult client.ProviderRemovePayload
	providerRemErr    error
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
	f.sends = append(f.sends, sendCall{session: session, message: message})
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
	f.modelSets = append(f.modelSets, id)
	if f.modelErr != nil {
		return "", f.modelErr
	}
	if f.modelSetResult != "" {
		f.current = f.modelSetResult
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
func (f *fakeClient) Compact(session string) (*client.CompactResult, error) {
	if f.compactErr != nil {
		return nil, f.compactErr
	}
	result := f.compact
	return &result, nil
}
func (f *fakeClient) Skills() ([]client.SkillInfo, error)         { return f.skills, nil }
func (f *fakeClient) MCPServers() ([]client.MCPServerInfo, error) { return f.mcps, nil }
func (f *fakeClient) CronJobs() ([]client.CronJob, error)         { return f.jobs, nil }
func (f *fakeClient) ProviderConfigure(providerID, apiKey, apiBase string) (client.ProviderConfigurePayload, error) {
	if f.providerCfgErr != nil {
		return client.ProviderConfigurePayload{}, f.providerCfgErr
	}
	return f.providerCfgResult, nil
}
func (f *fakeClient) ProviderRemove(providerID string) (client.ProviderRemovePayload, error) {
	if f.providerRemErr != nil {
		return client.ProviderRemovePayload{}, f.providerRemErr
	}
	return f.providerRemResult, nil
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

func TestHandleSlashCommandProvidersShowsCurrentProviderConfig(t *testing.T) {
	model := New(app.Config{})
	model.client = &fakeClient{
		models: []client.ModelInfo{
			{ID: "claude-sonnet", Name: "Claude Sonnet", Provider: "anthropic"},
			{ID: "gpt-5", Name: "GPT-5", Provider: "openai"},
		},
		current: "gpt-5",
		status: client.StatusPayload{
			Model: "gpt-5",
			Usage: client.UsageInfo{
				ContextWindow: 200000,
			},
		},
	}
	cfg := cfgpkg.DefaultConfig()
	cfg.Providers = map[string]cfgpkg.ProviderConfig{
		"anthropic": {APIBase: "https://api.anthropic.example/v1"},
		"openai":    {APIBase: "https://api.openai.example/v1"},
	}
	model.providerConfig = &cfg

	_, cmd := model.handleSlashCommand("/providers")
	if cmd == nil {
		t.Fatal("expected providers command to return loader cmd")
	}

	msg := cmd()
	updated, _ := model.Update(msg)
	got := updated.(Model)
	if got.dialog == nil {
		t.Fatal("expected providers dialog to open")
	}

	view := plain(got.dialog.View())
	if !strings.Contains(view, "Active") {
		t.Fatalf("expected Active section header, got %q", view)
	}
	if !strings.Contains(view, "openai (active)") {
		t.Fatalf("expected active provider marker, got %q", view)
	}
	if !strings.Contains(view, "openai") {
		t.Fatalf("expected active provider name, got %q", view)
	}
	if !strings.Contains(view, "Model:") || !strings.Contains(view, "gpt-5") {
		t.Fatalf("expected current model field, got %q", view)
	}
	if !strings.Contains(view, "API Base:") || !strings.Contains(view, "https://api.openai.example/v1") {
		t.Fatalf("expected API base field for active provider, got %q", view)
	}
	if !strings.Contains(view, "Configured") {
		t.Fatalf("expected Configured section header, got %q", view)
	}
	if !strings.Contains(view, "anthropic") {
		t.Fatalf("expected configured provider name, got %q", view)
	}
	if !strings.Contains(view, "anthropic") {
		t.Fatalf("expected anthropic in Configured section, got %q", view)
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
				ContextWindow: 131072,
			},
		},
	})
	got := updated.(Model)

	if headerView := plain(got.header.View()); !strings.Contains(headerView, "52%") {
		t.Fatalf("expected header context percent 52, got %q", headerView)
	}
	view := plain(got.footer.View())
	if !strings.Contains(view, "model ollama/qwen3:8b") {
		t.Fatalf("expected footer model update, got %q", view)
	}
	if !strings.Contains(view, "52% (68K/131.1K)") {
		t.Fatalf("expected footer usage update, got %q", view)
	}
}

func TestStatusLoadedHydratesSidebarPersistedUsage(t *testing.T) {
	model := New(app.Config{})
	model.width = 120
	model.height = 32
	model.sidebarVisible = true
	model.recalcLayout()

	updated, _ := model.Update(StatusLoadedMsg{
		Payload: client.StatusPayload{
			Model:   "ollama/llama3.2",
			Session: "tui:main",
			Usage: client.UsageInfo{
				TotalTokens:   68000,
				ContextWindow: 131072,
			},
			PersistedUsage: &client.UsageSummary{
				ProviderID:    "ollama",
				ModelName:     "llama3.2",
				SessionTokens: 50,
				TodayTokens:   50,
				WeeklyTokens:  90,
				BudgetStatus:  "warning",
				WarningLevel:  "medium",
			},
		},
	})
	got := updated.(Model)

	sidebarView := plain(got.sidebar.View())
	if !strings.Contains(sidebarView, "USAGE") || !strings.Contains(sidebarView, "ollama / llama3.2") {
		t.Fatalf("expected persisted usage header and provider, got %q", sidebarView)
	}
	if !strings.Contains(sidebarView, "session 50") || !strings.Contains(sidebarView, "week 90") {
		t.Fatalf("expected persisted usage totals, got %q", sidebarView)
	}
	if !strings.Contains(sidebarView, "68K / 131.1K") {
		t.Fatalf("expected context usage to remain visible, got %q", sidebarView)
	}
}

func TestStatusLoadedHydratesQualifiedPersistedUsageModel(t *testing.T) {
	model := New(app.Config{})
	model.width = 120
	model.height = 32
	model.sidebarVisible = true
	model.recalcLayout()

	updated, _ := model.Update(StatusLoadedMsg{
		Payload: client.StatusPayload{
			Model:   "ollama/llama3.2",
			Session: "tui:main",
			PersistedUsage: &client.UsageSummary{
				ProviderID:    "ollama",
				ModelName:     "ollama/llama3.2",
				SessionTokens: 50,
				TodayTokens:   50,
				WeeklyTokens:  90,
			},
		},
	})
	got := updated.(Model)

	sidebarView := plain(got.sidebar.View())
	if !strings.Contains(sidebarView, "ollama/llama3.2") {
		t.Fatalf("expected qualified persisted usage label, got %q", sidebarView)
	}
	if strings.Contains(sidebarView, "ollama / ollama/llama3.2") {
		t.Fatalf("expected sidebar to avoid duplicate provider label, got %q", sidebarView)
	}
}

func TestChatUsageEventDoesNotOverwritePersistedUsageSummary(t *testing.T) {
	model := New(app.Config{})
	model.width = 120
	model.height = 32
	model.sidebarVisible = true
	model.recalcLayout()

	updated, _ := model.Update(StatusLoadedMsg{
		Payload: client.StatusPayload{
			Model:   "ollama/llama3.2",
			Session: "tui:main",
			PersistedUsage: &client.UsageSummary{
				ProviderID:    "ollama",
				ModelName:     "llama3.2",
				SessionTokens: 50,
				TodayTokens:   50,
				WeeklyTokens:  90,
			},
		},
	})
	got := updated.(Model)

	payload, _ := json.Marshal(client.UsagePayload{
		PromptTokens:     12,
		CompletionTokens: 8,
		TotalTokens:      20,
		ContextWindow:    8192,
	})
	updated, _ = got.Update(EventMsg{
		Event: client.Event{Type: client.FrameEvent, Event: "chat.usage", Payload: payload, Seq: 1},
	})
	got = updated.(Model)

	sidebarView := plain(got.sidebar.View())
	if !strings.Contains(sidebarView, "session 50") || !strings.Contains(sidebarView, "week 90") {
		t.Fatalf("expected persisted usage summary to remain, got %q", sidebarView)
	}
	if !strings.Contains(sidebarView, "20 / 8.2K") {
		t.Fatalf("expected live context usage to update independently, got %q", sidebarView)
	}
}

func TestStatusLoadedClearsSidebarPersistedUsageWhenSummaryMissing(t *testing.T) {
	model := New(app.Config{})
	model.width = 120
	model.height = 32
	model.sidebarVisible = true
	model.recalcLayout()

	updated, _ := model.Update(StatusLoadedMsg{
		Payload: client.StatusPayload{
			Model:   "ollama/llama3.2",
			Session: "tui:main",
			PersistedUsage: &client.UsageSummary{
				ProviderID:    "ollama",
				ModelName:     "llama3.2",
				SessionTokens: 50,
				TodayTokens:   50,
				WeeklyTokens:  90,
			},
		},
	})
	got := updated.(Model)

	updated, _ = got.Update(StatusLoadedMsg{
		Payload: client.StatusPayload{
			Model:          "ollama/llama3.2",
			Session:        "tui:main",
			PersistedUsage: nil,
		},
	})
	got = updated.(Model)

	sidebarView := plain(got.sidebar.View())
	if !strings.Contains(sidebarView, "USAGE") {
		t.Fatalf("expected usage section to remain visible, got %q", sidebarView)
	}
	if strings.Contains(sidebarView, "session 50") || strings.Contains(sidebarView, "week 90") {
		t.Fatalf("expected prior persisted usage values to clear, got %q", sidebarView)
	}
}

func TestPersistedUsageWarningIsDedupedUntilStateChanges(t *testing.T) {
	model := New(app.Config{})
	model.width = 120
	model.height = 32
	model.sidebarVisible = true
	model.messages.SetSize(120, 20)
	model.recalcLayout()

	warning := client.UsageAlert{
		ProviderID:   "ollama",
		ModelName:    "ollama/llama3.2",
		BudgetStatus: "warning",
		WarningLevel: "medium",
		Message:      "Usage warning for ollama/llama3.2: medium budget threshold reached.",
	}

	updated, _ := model.Update(StatusLoadedMsg{
		Payload: client.StatusPayload{
			Model:   "ollama/llama3.2",
			Session: "tui:main",
			PersistedUsage: &client.UsageSummary{
				ProviderID:    "ollama",
				ModelName:     "ollama/llama3.2",
				SessionTokens: 50,
				TodayTokens:   50,
				WeeklyTokens:  90,
				BudgetStatus:  "warning",
				WarningLevel:  "medium",
			},
			UsageAlert: &warning,
		},
	})
	got := updated.(Model)

	view := plain(got.messages.View())
	if strings.Count(view, warning.Message) != 1 {
		t.Fatalf("expected first persisted usage warning message, got %q", view)
	}

	updated, _ = got.Update(StatusLoadedMsg{
		Payload: client.StatusPayload{
			Model:   "ollama/llama3.2",
			Session: "tui:main",
			PersistedUsage: &client.UsageSummary{
				ProviderID:    "ollama",
				ModelName:     "ollama/llama3.2",
				SessionTokens: 50,
				TodayTokens:   50,
				WeeklyTokens:  90,
				BudgetStatus:  "warning",
				WarningLevel:  "medium",
			},
			UsageAlert: &warning,
		},
	})
	got = updated.(Model)
	view = plain(got.messages.View())
	if strings.Count(view, warning.Message) != 1 {
		t.Fatalf("expected unchanged persisted usage warning to be deduped, got %q", view)
	}

	escalated := client.UsageAlert{
		ProviderID:   "ollama",
		ModelName:    "ollama/llama3.2",
		BudgetStatus: "warning",
		WarningLevel: "critical",
		Message:      "Usage warning for ollama/llama3.2: critical budget threshold reached.",
	}
	updated, _ = got.Update(StatusLoadedMsg{
		Payload: client.StatusPayload{
			Model:   "ollama/llama3.2",
			Session: "tui:main",
			PersistedUsage: &client.UsageSummary{
				ProviderID:    "ollama",
				ModelName:     "ollama/llama3.2",
				SessionTokens: 95,
				TodayTokens:   95,
				WeeklyTokens:  95,
				BudgetStatus:  "warning",
				WarningLevel:  "critical",
			},
			UsageAlert: &escalated,
		},
	})
	got = updated.(Model)
	view = plain(got.messages.View())
	if strings.Count(view, warning.Message) != 1 || strings.Count(view, escalated.Message) != 1 {
		t.Fatalf("expected escalated persisted usage warning to append once, got %q", view)
	}
}

func TestClearCommandResetsPersistedUsageWarningLatch(t *testing.T) {
	model := New(app.Config{})
	model.width = 120
	model.height = 32
	model.sidebarVisible = true
	model.messages.SetSize(120, 20)
	model.recalcLayout()

	warning := client.UsageAlert{
		ProviderID:   "ollama",
		ModelName:    "ollama/llama3.2",
		BudgetStatus: "warning",
		WarningLevel: "medium",
		Message:      "Usage warning for ollama/llama3.2: medium budget threshold reached.",
	}
	status := StatusLoadedMsg{
		Payload: client.StatusPayload{
			Model:   "ollama/llama3.2",
			Session: "tui:main",
			PersistedUsage: &client.UsageSummary{
				ProviderID:    "ollama",
				ModelName:     "ollama/llama3.2",
				SessionTokens: 50,
				TodayTokens:   50,
				WeeklyTokens:  90,
				BudgetStatus:  "warning",
				WarningLevel:  "medium",
			},
			UsageAlert: &warning,
		},
	}

	updated, _ := model.Update(status)
	got := updated.(Model)
	if view := plain(got.messages.View()); strings.Count(view, warning.Message) != 1 {
		t.Fatalf("expected initial persisted usage warning, got %q", view)
	}

	updated, _ = got.handleSlashCommand("/clear")
	got = updated.(Model)
	if view := plain(got.messages.View()); strings.Contains(view, warning.Message) {
		t.Fatalf("expected clear to remove prior warning message, got %q", view)
	}

	updated, _ = got.Update(status)
	got = updated.(Model)
	if view := plain(got.messages.View()); strings.Count(view, warning.Message) != 1 {
		t.Fatalf("expected cleared transcript to allow warning to reappear, got %q", view)
	}
}

func TestPersistedUsageWarningFallsBackToSummaryWhenAlertPayloadMissing(t *testing.T) {
	model := New(app.Config{})
	model.width = 120
	model.height = 32
	model.sidebarVisible = true
	model.messages.SetSize(120, 20)
	model.recalcLayout()

	updated, _ := model.Update(StatusLoadedMsg{
		Payload: client.StatusPayload{
			Model:   "ollama/llama3.2",
			Session: "tui:main",
			PersistedUsage: &client.UsageSummary{
				ProviderID:    "ollama",
				ModelName:     "ollama/llama3.2",
				SessionTokens: 50,
				TodayTokens:   50,
				WeeklyTokens:  90,
				BudgetStatus:  "warning",
				WarningLevel:  "medium",
			},
		},
	})
	got := updated.(Model)

	view := plain(got.messages.View())
	if !strings.Contains(view, "Usage warning for ollama/llama3.2: medium budget threshold reached.") {
		t.Fatalf("expected persisted usage summary to fall back to a warning notification, got %q", view)
	}
}

func TestQuotaStateWarningIsDedupedUntilStateChanges(t *testing.T) {
	model := New(app.Config{})
	model.width = 120
	model.height = 32
	model.sidebarVisible = true
	model.messages.SetSize(120, 20)
	model.recalcLayout()

	expiredStatus := StatusLoadedMsg{
		Payload: client.StatusPayload{
			Model:   "ollama/llama3.2",
			Session: "tui:main",
			PersistedUsage: &client.UsageSummary{
				ProviderID:    "ollama",
				ModelName:     "ollama/llama3.2",
				SessionTokens: 50,
				TodayTokens:   50,
				WeeklyTokens:  90,
				Quota: &client.QuotaSummary{
					State: "expired",
				},
			},
		},
	}

	updated, _ := model.Update(expiredStatus)
	got := updated.(Model)
	expiredMessage := "Quota status for ollama/llama3.2 expired."
	view := plain(got.messages.View())
	if strings.Count(view, expiredMessage) != 1 {
		t.Fatalf("expected initial quota-expired warning, got %q", view)
	}

	updated, _ = got.Update(expiredStatus)
	got = updated.(Model)
	view = plain(got.messages.View())
	if strings.Count(view, expiredMessage) != 1 {
		t.Fatalf("expected unchanged quota-expired warning to be deduped, got %q", view)
	}

	staleStatus := StatusLoadedMsg{
		Payload: client.StatusPayload{
			Model:   "ollama/llama3.2",
			Session: "tui:main",
			PersistedUsage: &client.UsageSummary{
				ProviderID:    "ollama",
				ModelName:     "ollama/llama3.2",
				SessionTokens: 50,
				TodayTokens:   50,
				WeeklyTokens:  90,
				Quota: &client.QuotaSummary{
					State: "stale",
				},
			},
		},
	}
	updated, _ = got.Update(staleStatus)
	got = updated.(Model)
	staleMessage := "Quota status for ollama/llama3.2 is stale."
	view = plain(got.messages.View())
	if strings.Count(view, expiredMessage) != 1 || strings.Count(view, staleMessage) != 1 {
		t.Fatalf("expected stale quota warning to append once on state change, got %q", view)
	}
}

func TestClearResetsPersistedUsageWarningDeduper(t *testing.T) {
	model := New(app.Config{})
	model.width = 120
	model.height = 32
	model.sidebarVisible = true
	model.messages.SetSize(120, 20)
	model.recalcLayout()

	warning := client.UsageAlert{
		ProviderID:   "ollama",
		ModelName:    "ollama/llama3.2",
		BudgetStatus: "warning",
		WarningLevel: "medium",
		Message:      "Usage warning for ollama/llama3.2: medium budget threshold reached.",
	}

	updated, _ := model.Update(StatusLoadedMsg{
		Payload: client.StatusPayload{
			Model:      "ollama/llama3.2",
			Session:    "tui:main",
			UsageAlert: &warning,
		},
	})
	got := updated.(Model)

	cleared, _ := got.handleSlashCommand("/clear")
	got = cleared.(Model)

	updated, _ = got.Update(StatusLoadedMsg{
		Payload: client.StatusPayload{
			Model:      "ollama/llama3.2",
			Session:    "tui:main",
			UsageAlert: &warning,
		},
	})
	got = updated.(Model)

	view := plain(got.messages.View())
	if strings.Count(view, warning.Message) != 1 {
		t.Fatalf("expected persisted usage warning to reappear after /clear, got %q", view)
	}
}

func TestCopyShortcutUsesClipboardAndFlashesStatus(t *testing.T) {
	model := New(app.Config{})
	model.status.SetWidth(80)
	model.messages.SetSize(80, 20)
	model.messages.AppendAssistant("copy me")
	model.editor.Blur()

	updated, cmd := model.Update(tea.KeyPressMsg(tea.Key{Code: 'y'}))
	got := updated.(Model)
	if cmd == nil {
		t.Fatal("expected copy shortcut to return command")
	}

	msg := cmd()
	copied, ok := msg.(ClipboardCopiedMsg)
	if !ok {
		t.Fatalf("expected clipboard copied msg, got %#v", msg)
	}
	updated, nextCmd := got.Update(msg)
	got = updated.(Model)
	if nextCmd == nil {
		t.Fatal("expected flash clear timer after copy")
	}
	if copied.Text != "copy me" {
		t.Fatalf("expected clipboard copied text to be preserved, got %q", copied.Text)
	}
	if view := plain(got.status.View()); !strings.Contains(view, "Copied to clipboard") {
		t.Fatalf("expected copied flash in status row, got %q", view)
	}

	updated, _ = got.Update(flashClearMsg{Seq: got.flashSeq})
	got = updated.(Model)
	if view := plain(got.status.View()); strings.Contains(view, "Copied to clipboard") {
		t.Fatalf("expected copied flash to clear, got %q", view)
	}
}

func TestMouseSelectionRoutesToTranscriptAndCopiesOnRelease(t *testing.T) {
	if !theme.Set("nord") {
		t.Fatal("expected nord theme")
	}
	model := New(app.Config{})
	model.width = 80
	model.height = 24
	model.recalcLayout()
	model.messages.AppendAssistant("alpha beta gamma")
	model.editor.Blur()

	updated, _ := model.Update(tea.MouseClickMsg(tea.Mouse{X: 2, Y: model.header.Height() + 1, Button: tea.MouseLeft}))
	got := updated.(Model)

	updated, _ = got.Update(tea.MouseMotionMsg(tea.Mouse{X: 7, Y: model.header.Height() + 1, Button: tea.MouseLeft}))
	got = updated.(Model)

	updated, cmd := got.Update(tea.MouseReleaseMsg(tea.Mouse{X: 7, Y: model.header.Height() + 1, Button: tea.MouseLeft}))
	got = updated.(Model)
	if cmd == nil {
		t.Fatal("expected mouse release with selection to trigger copy command")
	}

	msg := cmd()
	copied, ok := msg.(ClipboardCopiedMsg)
	if !ok {
		t.Fatalf("expected ClipboardCopiedMsg, got %#v", msg)
	}
	if copied.Text != "alpha" {
		t.Fatalf("copied text = %q, want alpha", copied.Text)
	}

	updated, nextCmd := got.Update(msg)
	got = updated.(Model)
	if nextCmd == nil {
		t.Fatal("expected follow-up clipboard/flash command after copy")
	}
	if got.messages.HasSelection() {
		t.Fatal("expected transcript selection to clear after copy")
	}
	if view := plain(got.status.View()); !strings.Contains(view, "Copied to clipboard") {
		t.Fatalf("expected copied flash in status row, got %q", view)
	}
}

func TestInsertModeTogglePreservesTypingShortcuts(t *testing.T) {
	model := New(app.Config{})

	updated, _ := model.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEsc}))
	got := updated.(Model)
	if got.editor.Focused() {
		t.Fatal("expected esc to leave insert mode")
	}

	updated, cmd := got.Update(tea.KeyPressMsg(tea.Key{Code: 'i'}))
	got = updated.(Model)
	if cmd == nil {
		t.Fatal("expected i to refocus editor")
	}
	_ = cmd()
	if !got.editor.Focused() {
		t.Fatal("expected editor to be focused after i")
	}
}

func TestWindowSizeConfiguresSidebarLayout(t *testing.T) {
	model := New(app.Config{})
	model.sidebarVisible = true

	updated, _ := model.Update(tea.WindowSizeMsg{Width: 140, Height: 40})
	got := updated.(Model)

	if got.compactMode {
		t.Fatal("expected normal layout at width 140")
	}
	if got.mainWidth != 109 {
		t.Fatalf("expected main width 109, got %d", got.mainWidth)
	}
	if got.sidebarWidth != 30 {
		t.Fatalf("expected sidebar width 30, got %d", got.sidebarWidth)
	}
	if got.headerWidth != 109 {
		t.Fatalf("expected header width 109, got %d", got.headerWidth)
	}
	if got.statusWidth != 109 {
		t.Fatalf("expected status width 109, got %d", got.statusWidth)
	}
	if got.footerWidth != 109 {
		t.Fatalf("expected footer to match main pane width, got %d", got.footerWidth)
	}
	if got.messagesWidth != 107 {
		t.Fatalf("expected transcript width 107, got %d", got.messagesWidth)
	}
}

func TestCtrlDTogglesSidebarVisibilityAndPersistsState(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(t.TempDir(), "config"))

	model := New(app.Config{})
	model.sidebarVisible = true
	updated, cmd := model.Update(tea.WindowSizeMsg{Width: 140, Height: 40})
	got := updated.(Model)
	if got.sidebarVisible != true {
		t.Fatalf("expected sidebar visible in normal mode, got %v", got.sidebarVisible)
	}

	updated, cmd = got.Update(tea.KeyPressMsg(tea.Key{Code: 'd', Mod: tea.ModCtrl}))
	got = updated.(Model)
	if got.sidebarVisible {
		t.Fatal("expected ctrl+d to hide sidebar in normal mode")
	}
	if cmd == nil {
		t.Fatal("expected sidebar toggle to persist state")
	}
	_ = cmd()

	state := app.LoadState()
	if state.SidebarVisible == nil || *state.SidebarVisible {
		t.Fatalf("expected sidebar visibility to persist as false, got %#v", state.SidebarVisible)
	}
}

func TestCompactModeCtrlDTogglesOverlay(t *testing.T) {
	model := New(app.Config{})

	updated, _ := model.Update(tea.WindowSizeMsg{Width: 110, Height: 34})
	got := updated.(Model)
	if !got.compactMode {
		t.Fatal("expected compact mode for width 110")
	}
	if got.detailsOpen {
		t.Fatal("expected compact overlay to start closed")
	}

	updated, _ = got.Update(tea.KeyPressMsg(tea.Key{Code: 'd', Mod: tea.ModCtrl}))
	got = updated.(Model)
	if !got.detailsOpen {
		t.Fatal("expected ctrl+d to open compact overlay")
	}
	view := plain(got.View().Content)
	if !strings.Contains(view, "SESSION") || !strings.Contains(view, "CONTEXT") {
		t.Fatalf("expected compact overlay sections in view, got %q", view)
	}

	updated, _ = got.Update(tea.KeyPressMsg(tea.Key{Code: 'd', Mod: tea.ModCtrl}))
	got = updated.(Model)
	if got.detailsOpen {
		t.Fatal("expected ctrl+d to close compact overlay")
	}
}

func TestSidebarWidthRestoresAfterCompactCycle(t *testing.T) {
	model := New(app.Config{})
	model.sidebarVisible = true

	updated, _ := model.Update(tea.WindowSizeMsg{Width: 140, Height: 34})
	got := updated.(Model)
	if got.sidebarWidth != 30 {
		t.Fatalf("expected initial sidebar width 30, got %d", got.sidebarWidth)
	}

	updated, _ = got.Update(tea.WindowSizeMsg{Width: 110, Height: 34})
	got = updated.(Model)
	if !got.compactMode {
		t.Fatal("expected compact mode at width 110")
	}

	updated, _ = got.Update(tea.WindowSizeMsg{Width: 140, Height: 34})
	got = updated.(Model)
	if got.sidebarWidth != 30 {
		t.Fatalf("expected sidebar width to restore to 30 after compact cycle, got %d", got.sidebarWidth)
	}
}

func TestSidebarAreaClicksAreConsumed(t *testing.T) {
	model := New(app.Config{})

	updated, _ := model.Update(tea.WindowSizeMsg{Width: 140, Height: 40})
	got := updated.(Model)

	updated, cmd := got.Update(tea.MouseClickMsg(tea.Mouse{X: 120, Y: 5, Button: tea.MouseLeft}))
	if cmd != nil {
		t.Fatalf("expected sidebar click to be consumed, got %T", cmd)
	}
	if updated.(Model).mainWidth != got.mainWidth {
		t.Fatal("expected sidebar click not to change layout")
	}
}

func TestSidebarDataUpdatesFromStatusAndCompressionEvents(t *testing.T) {
	model := New(app.Config{
		MCPServers: []client.MCPServerInfo{
			{Name: "memory", Status: "configured", Tools: 3},
		},
	})
	model.app.Session = "tui:alt"
	model.app.Model = "model-a"

	updated, _ := model.Update(StatusLoadedMsg{
		Payload: client.StatusPayload{
			Model:   "model-b",
			Session: "tui:alt",
			Usage: client.UsageInfo{
				TotalTokens:   68000,
				ContextWindow: 131072,
			},
			Channels: []client.ChannelStatus{
				{Name: "WhatsApp", Status: "connected"},
			},
		},
	})
	got := updated.(Model)

	if headerView := plain(got.header.View()); !strings.Contains(headerView, "52%") {
		t.Fatalf("expected header context percent 52, got %q", headerView)
	}
	sidebarView := plain(got.sidebar.View())
	if !strings.Contains(sidebarView, "tui:alt") {
		t.Fatalf("expected sidebar session to update, got %q", sidebarView)
	}
	if !strings.Contains(sidebarView, "model-b") {
		t.Fatalf("expected sidebar model to update, got %q", sidebarView)
	}
	if !strings.Contains(sidebarView, "68K / 131.1K") {
		t.Fatalf("expected sidebar usage to update, got %q", sidebarView)
	}
	if !strings.Contains(sidebarView, "WhatsApp") {
		t.Fatalf("expected sidebar channels to update, got %q", sidebarView)
	}

	compPayload, _ := json.Marshal(client.CompressionInfo{
		Enabled:          true,
		OriginalTokens:   120000,
		CompressedTokens: 70000,
		ReductionPercent: 41.7,
	})
	updated, _ = got.Update(EventMsg{
		Event: client.Event{Type: client.FrameEvent, Event: "context.compressed", Payload: compPayload, Seq: 2},
	})
	got = updated.(Model)

	sidebarView = plain(got.sidebar.View())
	if !strings.Contains(sidebarView, "↓ 42% compacted") {
		t.Fatalf("expected sidebar compression to update, got %q", sidebarView)
	}

	updated, _ = got.Update(CronJobsLoadedMsg{
		Jobs: []client.CronJob{
			{Name: "backup", Schedule: "every 5m", Status: "active"},
		},
	})
	got = updated.(Model)

	sidebarView = plain(got.sidebar.View())
	if !strings.Contains(sidebarView, "memory") || !strings.Contains(sidebarView, "backup") {
		t.Fatalf("expected sidebar mcp and cron data to update, got %q", sidebarView)
	}
}

func TestHandleSlashCommandCompactCallsGatewayAndRendersSystemMessage(t *testing.T) {
	model := New(app.Config{})
	model.client = &fakeClient{
		compact: client.CompactResult{
			Compacted:        true,
			OriginalTokens:   12000,
			CompressedTokens: 7000,
			ReductionPercent: 42,
		},
	}
	model.width = 80
	model.footer.SetWidth(80)
	model.contextWarned = true

	updated, cmd := model.handleSlashCommand("/compact")
	got := updated.(Model)
	if !got.footer.IsCompacting() {
		t.Fatal("expected compaction spinner to start immediately")
	}
	if got.contextWarned {
		t.Fatal("expected compact command to clear context warning latch")
	}
	if cmd == nil {
		t.Fatal("expected compact command to return gateway cmd")
	}

	msg := cmd()
	updated, cmd = got.Update(msg)
	got = updated.(Model)
	if got.footer.IsCompacting() {
		t.Fatal("expected compaction spinner to stop after completion")
	}
	if cmd == nil {
		t.Fatal("expected compact completion to request status refresh")
	}

	view := plain(got.messages.View())
	if !strings.Contains(view, "Context compacted: 12K → 7K (42% reduction)") {
		t.Fatalf("expected compact result in transcript, got %q", view)
	}
	footer := plain(got.footer.View())
	if !strings.Contains(footer, "↓42%") {
		t.Fatalf("expected manual compact to update footer badge, got %q", footer)
	}
}

func TestHandleSlashCommandCompactNoOpShowsReason(t *testing.T) {
	model := New(app.Config{})
	model.client = &fakeClient{
		compact: client.CompactResult{
			Compacted: false,
			Reason:    "not enough history",
		},
	}

	updated, cmd := model.handleSlashCommand("/compact")
	got := updated.(Model)
	if cmd == nil {
		t.Fatal("expected compact command to return gateway cmd")
	}

	msg := cmd()
	updated, nextCmd := got.Update(msg)
	got = updated.(Model)
	if nextCmd != nil {
		t.Fatalf("expected no-op compaction to avoid status refresh, got %T", nextCmd)
	}
	view := plain(got.messages.View())
	if !strings.Contains(view, "Nothing to compact yet.") {
		t.Fatalf("expected no-op reason message, got %q", view)
	}
}

func TestContextCompressedEventAddsSystemMessage(t *testing.T) {
	model := New(app.Config{})
	model.messages.SetSize(80, 20)

	payload, _ := json.Marshal(client.CompressionInfo{
		Enabled:          true,
		OriginalTokens:   12000,
		CompressedTokens: 7000,
		ReductionPercent: 42,
	})
	updated, _ := model.Update(EventMsg{
		Event: client.Event{Type: client.FrameEvent, Event: "context.compressed", Payload: payload, Seq: 1},
	})
	got := updated.(Model)

	view := plain(got.messages.View())
	if !strings.Contains(view, "Context compacted: 12K → 7K (42% reduction)") {
		t.Fatalf("expected context.compressed to append system message, got %q", view)
	}
}

func TestUsageWarningIsAppendedOncePerSession(t *testing.T) {
	model := New(app.Config{})
	model.messages.SetSize(80, 20)
	model.width = 80
	model.footer.SetWidth(80)
	model.sidebar.SetSize(30, 20)

	// Establish a short model/session so the footer has room for full usage format.
	hydrated, _ := model.Update(StatusLoadedMsg{
		Payload: client.StatusPayload{Model: "openai/gpt-4o", Session: "test"},
	})
	model = hydrated.(Model)

	payload, _ := json.Marshal(client.UsagePayload{
		TotalTokens:   120000,
		ContextWindow: 131072,
	})
	updated, _ := model.Update(EventMsg{
		Event: client.Event{Type: client.FrameEvent, Event: "chat.usage", Payload: payload, Seq: 1},
	})
	got := updated.(Model)

	if headerView := plain(got.header.View()); !strings.Contains(headerView, "92%") {
		t.Fatalf("expected header context percent 92, got %q", headerView)
	}
	view := plain(got.messages.View())
	if !strings.Contains(view, "Context is 92% full. Use /compact to free space.") {
		t.Fatalf("expected warning on first threshold crossing, got %q", view)
	}
	footer := plain(got.footer.View())
	if !strings.Contains(footer, "92%") || !strings.Contains(footer, "120K/131.1K") {
		t.Fatalf("expected footer usage update, got %q", footer)
	}
	sidebar := plain(got.sidebar.View())
	if !strings.Contains(sidebar, "120K / 131.1K") {
		t.Fatalf("expected sidebar usage update, got %q", sidebar)
	}

	updated, _ = got.Update(EventMsg{
		Event: client.Event{Type: client.FrameEvent, Event: "chat.usage", Payload: payload, Seq: 2},
	})
	got = updated.(Model)
	view = plain(got.messages.View())
	if strings.Count(view, "Use /compact to free space.") != 1 {
		t.Fatalf("expected warning to appear once, got %q", view)
	}
}

func TestChatProgressIsBatchedUntilFlushTick(t *testing.T) {
	model := New(app.Config{})
	model.messages.SetSize(80, 20)

	updated, cmd := model.Update(ChatProgressMsg{Content: "hel"})
	got := updated.(Model)
	if cmd == nil {
		t.Fatal("expected first progress chunk to schedule flush")
	}
	if got.messages.GetProgress() != "" {
		t.Fatalf("expected progress to stay buffered before flush, got %q", got.messages.GetProgress())
	}

	updated, nextCmd := got.Update(ChatProgressMsg{Content: "lo"})
	got = updated.(Model)
	if nextCmd != nil {
		t.Fatalf("expected second chunk to reuse pending flush, got %T", nextCmd)
	}
	if got.messages.GetProgress() != "" {
		t.Fatalf("expected progress to remain buffered before flush, got %q", got.messages.GetProgress())
	}

	updated, _ = got.Update(flushProgressMsg{Seq: got.progressFlushSeq})
	got = updated.(Model)
	if got.messages.GetProgress() != "hello" {
		t.Fatalf("expected buffered chunks to flush once, got %q", got.messages.GetProgress())
	}
}

func TestClearResetsBufferedProgress(t *testing.T) {
	model := New(app.Config{})
	model.messages.SetSize(80, 20)

	updated, _ := model.Update(ChatProgressMsg{Content: "stale"})
	got := updated.(Model)
	staleSeq := got.progressFlushSeq

	updated, _ = got.handleSlashCommand("/clear")
	got = updated.(Model)
	if got.progressBuffer.Len() != 0 || got.progressFlushPending {
		t.Fatalf("expected /clear to reset buffered progress, got buffer=%q pending=%v", got.progressBuffer.String(), got.progressFlushPending)
	}

	updated, _ = got.Update(flushProgressMsg{Seq: staleSeq})
	got = updated.(Model)
	if got.messages.GetProgress() != "" {
		t.Fatalf("expected stale flush to be ignored after clear, got %q", got.messages.GetProgress())
	}
}

func TestSessionNewResetsBufferedProgress(t *testing.T) {
	model := New(app.Config{})
	model.messages.SetSize(80, 20)

	updated, _ := model.Update(ChatProgressMsg{Content: "stale"})
	got := updated.(Model)
	staleSeq := got.progressFlushSeq

	updated, _ = got.handleSlashCommand("/session new")
	got = updated.(Model)
	if got.progressBuffer.Len() != 0 || got.progressFlushPending {
		t.Fatalf("expected /session new to reset buffered progress, got buffer=%q pending=%v", got.progressBuffer.String(), got.progressFlushPending)
	}

	updated, _ = got.Update(flushProgressMsg{Seq: staleSeq})
	got = updated.(Model)
	if got.messages.GetProgress() != "" {
		t.Fatalf("expected stale flush to be ignored after session new, got %q", got.messages.GetProgress())
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
					ContextWindow: 131072,
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
	if headerView := plain(got.header.View()); !strings.Contains(headerView, "32%") {
		t.Fatalf("expected header context percent 32, got %q", headerView)
	}
	if !strings.Contains(view, "32% (42K/131.1K)") {
		t.Fatalf("expected footer to use current session usage, got %q", view)
	}
}

func TestModelSetUsesNormalizedGatewayCurrent(t *testing.T) {
	model := New(app.Config{})
	fake := &fakeClient{modelSetResult: "ollama/qwen3:8b"}
	model.client = fake

	updated, cmd := model.handleSlashCommand("/model ollama_chat/qwen3:8b")
	got := updated.(Model)
	msg := cmd()
	updated, _ = got.Update(msg)
	got = updated.(Model)

	if got.app.Model != "ollama/qwen3:8b" {
		t.Fatalf("expected normalized gateway model id, got %q", got.app.Model)
	}
	if len(fake.modelSets) != 1 || fake.modelSets[0] != "ollama_chat/qwen3:8b" {
		t.Fatalf("expected gateway client to receive requested model, got %#v", fake.modelSets)
	}
}

func TestModelSlashCommandUsesRealClientModelsSetContract(t *testing.T) {
	upgrader := websocket.Upgrader{CheckOrigin: func(*http.Request) bool { return true }}
	received := make(chan json.RawMessage, 1)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Fatalf("Upgrade: %v", err)
		}
		defer conn.Close()

		_, raw, err := conn.ReadMessage()
		if err != nil {
			t.Fatalf("Read hello: %v", err)
		}
		var hello client.Request
		if err := json.Unmarshal(raw, &hello); err != nil {
			t.Fatalf("Unmarshal hello: %v", err)
		}
		if err := conn.WriteJSON(client.Response{
			Type:    client.FrameRes,
			ID:      hello.ID,
			OK:      true,
			Payload: json.RawMessage(`{"server":"smolbot","version":"test","protocol":1,"methods":["models.set"],"events":[]}`),
		}); err != nil {
			t.Fatalf("Write hello response: %v", err)
		}

		_, raw, err = conn.ReadMessage()
		if err != nil {
			t.Fatalf("Read models.set: %v", err)
		}
		var wire struct {
			ID     string          `json:"id"`
			Method string          `json:"method"`
			Params json.RawMessage `json:"params"`
		}
		if err := json.Unmarshal(raw, &wire); err != nil {
			t.Fatalf("Unmarshal models.set: %v", err)
		}
		received <- append([]byte(nil), wire.Params...)
		if wire.Method != "models.set" {
			t.Fatalf("expected models.set request, got %#v", wire)
		}
		if err := conn.WriteJSON(client.Response{
			Type:    client.FrameRes,
			ID:      wire.ID,
			OK:      true,
			Payload: json.RawMessage(`{"current":"ollama/qwen3:8b","previous":"gpt-test"}`),
		}); err != nil {
			t.Fatalf("Write models.set response: %v", err)
		}
	}))
	defer srv.Close()

	cl := client.New("ws" + strings.TrimPrefix(srv.URL, "http") + "/ws")
	if _, err := cl.Connect(); err != nil {
		t.Fatalf("Connect: %v", err)
	}
	defer cl.Close()

	model := New(app.Config{})
	model.client = cl

	updated, cmd := model.handleSlashCommand("/model ollama_chat/qwen3:8b")
	got := updated.(Model)
	msg := cmd()
	updated, _ = got.Update(msg)
	got = updated.(Model)

	if got.app.Model != "ollama/qwen3:8b" {
		t.Fatalf("expected normalized gateway current model, got %q", got.app.Model)
	}

	var params struct {
		Model string `json:"model"`
		ID    string `json:"id"`
	}
	if err := json.Unmarshal(<-received, &params); err != nil {
		t.Fatalf("Unmarshal params: %v", err)
	}
	if params.Model != "ollama_chat/qwen3:8b" {
		t.Fatalf("expected canonical model field, got %#v", params)
	}
	if params.ID != "" {
		t.Fatalf("unexpected legacy id field in params: %#v", params)
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

	// Tool events are debounced; flush the pending batch.
	updated, _ = got.Update(flushToolsMsg{Seq: got.toolFlushSeq})
	got = updated.(Model)

	// Collapsed mode: read_file renders as a group summary
	view := got.messages.View()
	if !strings.Contains(view, "Read") {
		t.Fatalf("expected collapsed summary in view, got %q", view)
	}
	if !strings.Contains(view, "file") {
		t.Fatalf("expected 'file' count in view, got %q", view)
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

// TestSubmitDuringStreamingCurrentlyCallsChatSend characterizes the current TUI
// behavior where submitting a message while streaming=true calls ChatSend
// unconditionally. The gateway then rejects with "already active".
//
// TODO(queueing): After Tasks 2 and 5 this test should verify the TUI
// correctly handles a queued run response rather than an error.
func TestSubmitDuringStreamingCurrentlyCallsChatSend(t *testing.T) {
	fake := &fakeClient{}
	model := New(app.Config{})
	model.client = fake
	model.streaming = true
	model.currentRunID = "run-existing"

	// Simulate typing and submitting a follow-up while streaming.
	model.editor.SetValue("follow-up")
	updated, cmd := model.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}))
	model = updated.(Model)

	// Current behavior: sendChatCmd is returned (will call ChatSend).
	if cmd == nil {
		t.Fatal("REGRESSION: expected a command (sendChatCmd) when submitting during streaming")
	}
	// Run the cmd synchronously and check it calls ChatSend.
	msg := cmd()
	if _, ok := msg.(ChatStartedMsg); !ok {
		if errMsg, ok := msg.(ChatErrorMsg); ok {
			// This is the error path that will be replaced by queue behavior.
			// A "already active" error coming from the gateway.
			t.Logf("REGRESSION: got ChatErrorMsg %q — will become ChatStartedMsg with queued runId after queueing", errMsg.Message)
			return
		}
		t.Fatalf("expected ChatStartedMsg or ChatErrorMsg, got %T", msg)
	}
}

// TestChatStartedWhileStreamingDoesNotOverwriteRunID verifies that a
// ChatStartedMsg that arrives while already streaming (i.e. for a queued run)
// does not overwrite the active run's ID.
func TestChatStartedWhileStreamingDoesNotOverwriteRunID(t *testing.T) {
	model := New(app.Config{})
	model.streaming = true
	model.currentRunID = "run-active"

	updated, cmd := model.Update(ChatStartedMsg{RunID: "run-queued"})
	got := updated.(Model)

	if got.currentRunID != "run-active" {
		t.Fatalf("expected currentRunID to stay 'run-active', got %q", got.currentRunID)
	}
	if cmd != nil {
		t.Fatalf("expected no command when ignoring queued ChatStartedMsg, got %T", cmd)
	}
}

// TestAbortDuringQueuedRunUsesActiveRunID verifies that Ctrl+C while a
// queued run is pending aborts the active run, not the queued one.
func TestAbortDuringQueuedRunUsesActiveRunID(t *testing.T) {
	fake := &fakeClient{}
	model := New(app.Config{})
	model.client = fake
	model.streaming = true
	model.currentRunID = "run-active"

	// Simulate receiving a queued runID — must NOT overwrite currentRunID.
	updated, _ := model.Update(ChatStartedMsg{RunID: "run-queued"})
	model = updated.(Model)

	// Now Ctrl+C — must abort "run-active", not "run-queued".
	updated, _ = model.Update(CtrlCMsg{})
	model = updated.(Model)

	if len(fake.aborts) != 1 {
		t.Fatalf("expected one abort, got %d", len(fake.aborts))
	}
	if fake.aborts[0].runID != "run-active" {
		t.Fatalf("expected abort of 'run-active', got %q", fake.aborts[0].runID)
	}
}

func TestChatQueuedMsgShowsQueuedNotice(t *testing.T) {
	model := New(app.Config{})
	model.width = 80
	model.height = 24

	updated, cmd := model.Update(ChatQueuedMsg{RunID: "run-q1", Position: 1})
	got := updated.(Model)

	if cmd != nil {
		t.Fatalf("expected no command from ChatQueuedMsg, got %T", cmd)
	}
	view := plain(got.messages.View())
	if !strings.Contains(view, "queued") {
		t.Fatalf("expected queued notice in messages view, got %q", view)
	}
}

func TestChatDequeuedMsgActivatesStreaming(t *testing.T) {
	model := New(app.Config{})
	model.width = 80
	model.height = 24
	model.streaming = false
	model.currentRunID = ""

	updated, cmd := model.Update(ChatDequeuedMsg{RunID: "run-q1"})
	got := updated.(Model)

	if !got.streaming {
		t.Fatal("expected streaming=true after ChatDequeuedMsg")
	}
	if got.currentRunID != "run-q1" {
		t.Fatalf("expected currentRunID to be set, got %q", got.currentRunID)
	}
	if cmd == nil {
		t.Fatal("expected spinner commands from ChatDequeuedMsg")
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
	updated, _ := model.Update(tea.WindowSizeMsg{Width: 80, Height: 35})
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

	updated, _ := model.Update(tea.WindowSizeMsg{Width: 80, Height: 35})
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

	if !strings.Contains(plain(got.View().Content), "THEMES") {
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

	updated, _ := model.Update(tea.WindowSizeMsg{Width: 80, Height: 35})
	got := updated.(Model)
	got.messages.AppendUser("hello")
	got.messages.AppendAssistant("world")

	updated, _ = got.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyF1}))
	got = updated.(Model)

	view := plain(got.View().Content)
	if !strings.Contains(view, "//// MENU ////") {
		t.Fatalf("expected menu overlay in view, got %q", view)
	}
	if !strings.Contains(view, "USER") || !strings.Contains(view, "ASSISTANT") {
		t.Fatalf("expected transcript content to remain visible around overlay, got %q", view)
	}
}

func TestTranscriptFrameAddsSpacerBelowHeader(t *testing.T) {
	model := New(app.Config{})
	updated, _ := model.Update(tea.WindowSizeMsg{Width: 80, Height: 35})
	got := updated.(Model)
	got.messages.AppendUser("hello")

	lines := strings.Split(plain(got.View().Content), "\n")
	headerRow := -1
	contentRow := -1
	for i, line := range lines {
		if headerRow == -1 && strings.Contains(line, "▄▄▄▄ ▄") {
			headerRow = i
		}
		if contentRow == -1 && strings.Contains(line, "┃  USER") {
			contentRow = i
		}
	}
	if headerRow == -1 || contentRow == -1 {
		t.Fatalf("expected both header art and user message in view %q", plain(got.View().Content))
	}
	if contentRow-headerRow < 3 {
		t.Fatalf("expected info line + blank row between header and content, header row=%d content row=%d view=%q", headerRow, contentRow, plain(got.View().Content))
	}
}

func TestTranscriptAreaHasOwnBorder(t *testing.T) {
	model := New(app.Config{})
	updated, _ := model.Update(tea.WindowSizeMsg{Width: 80, Height: 35})
	got := updated.(Model)
	got.messages.AppendUser("hello")
	got.messages.AppendAssistant("world")

	view := plain(got.View().Content)
	if !strings.Contains(view, "●") {
		t.Fatalf("expected status row visible, got %q", view)
	}
	if !strings.Contains(view, "USER") || !strings.Contains(view, "ASSISTANT") {
		t.Fatalf("expected transcript content visible, got %q", view)
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
	if !strings.Contains(view, "THEMES") {
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
	model.height = 50
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
			{Key: "tui:10", Preview: "preview 10"},
			{Key: "tui:11", Preview: "preview 11"},
			{Key: "tui:12", Preview: "preview 12"},
		},
	}

	_, cmd := model.handleSlashCommand("/session")
	updated, _ := model.Update(cmd())
	got := updated.(Model)
	if !strings.Contains(plain(got.View().Content), "▼ more below") {
		t.Fatalf("expected session overflow cue below initial window, got %q", plain(got.View().Content))
	}
	for i := 0; i < 7; i++ {
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
	if !strings.Contains(view, "GPT-5") {
		t.Fatalf("expected model dialog to show model label and id, got %q", view)
	}
	if !strings.Contains(view, "openai") && !strings.Contains(view, "OpenAI") || !strings.Contains(view, "current") {
		t.Fatalf("expected model dialog to show provider and current marker, got %q", view)
	}
}

func TestModelDialogSpaceMarksPendingAndEnterSavesIt(t *testing.T) {
	model := New(app.Config{})
	model.width = 80
	model.height = 24
	fake := &fakeClient{
		models: []client.ModelInfo{
			{ID: "gpt-5", Name: "GPT-5", Provider: "openai", Selectable: true},
			{ID: "claude-3-7-sonnet", Name: "Claude 3.7 Sonnet", Provider: "anthropic", Selectable: true},
		},
		current: "gpt-5",
	}
	model.client = fake

	_, cmd := model.handleSlashCommand("/model")
	updated, _ := model.Update(cmd())
	got := updated.(Model)

	updated, _ = got.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyDown}))
	got = updated.(Model)
	updated, pendingCmd := got.Update(tea.KeyPressMsg(tea.Key{Code: ' ', Text: " "}))
	got = updated.(Model)
	if pendingCmd != nil {
		t.Fatal("expected space to mark a pending selection without saving")
	}

	view := plain(got.View().Content)
	if !strings.Contains(view, "pending") {
		t.Fatalf("expected pending marker in model dialog, got %q", view)
	}

	updated, _ = got.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyUp}))
	got = updated.(Model)
	updated, chooseCmd := got.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}))
	got = updated.(Model)
	if chooseCmd == nil {
		t.Fatal("expected enter to confirm the pending selection")
	}

	updated, saveCmd := got.Update(chooseCmd())
	got = updated.(Model)
	if saveCmd == nil {
		t.Fatal("expected model choice to trigger gateway save")
	}
	updated, _ = got.Update(saveCmd())
	got = updated.(Model)

	if got.app.Model != "claude-3-7-sonnet" {
		t.Fatalf("expected app model to update from pending selection, got %q", got.app.Model)
	}
	if len(fake.modelSets) != 1 || fake.modelSets[0] != "claude-3-7-sonnet" {
		t.Fatalf("expected pending model to be saved, got %#v", fake.modelSets)
	}
	if got.dialog != nil {
		t.Fatal("expected model dialog to close after saving")
	}
}

func TestModelDialogEnterUsesFocusedSelectionWithoutPending(t *testing.T) {
	model := New(app.Config{})
	model.width = 80
	model.height = 24
	fake := &fakeClient{
		models: []client.ModelInfo{
			{ID: "openrouter", Name: "OpenRouter", Provider: "openrouter", Description: "Configured provider", Source: "config", Selectable: false},
			{ID: "openrouter/auto", Name: "Auto", Provider: "openrouter", Selectable: true},
		},
		current: "openrouter/auto",
	}
	model.client = fake

	_, cmd := model.handleSlashCommand("/model")
	updated, _ := model.Update(cmd())
	got := updated.(Model)

	view := plain(got.View().Content)
	if !strings.Contains(strings.ToLower(view), "config") {
		t.Fatalf("expected provider info row to stay visible, got %q", view)
	}

	updated, chooseCmd := got.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}))
	got = updated.(Model)
	if chooseCmd == nil {
		t.Fatal("expected enter to save the focused selectable model")
	}
	updated, saveCmd := got.Update(chooseCmd())
	got = updated.(Model)
	if saveCmd == nil {
		t.Fatal("expected chosen model to trigger gateway save")
	}
	updated, _ = got.Update(saveCmd())
	got = updated.(Model)

	if got.app.Model != "openrouter/auto" {
		t.Fatalf("expected enter to save the selectable model, got %q", got.app.Model)
	}
	if len(fake.modelSets) != 1 || fake.modelSets[0] != "openrouter/auto" {
		t.Fatalf("expected selectable model to be sent to gateway, got %#v", fake.modelSets)
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
		if !strings.Contains(strings.ToLower(view), "esc") || !strings.Contains(strings.ToLower(view), "close") {
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

	checkOverlay("/theme", "THEMES")
	checkOverlay("/session", "Sessions")
	checkOverlay("/model", "MODELS")
}

func TestCompactLayoutOnShortTerminals(t *testing.T) {
	model := New(app.Config{})

	updated, _ := model.Update(tea.WindowSizeMsg{Width: 80, Height: 8})
	got := updated.(Model)
	view := plain(got.View().Content)

	if strings.Contains(view, "▄▄▄▄▄▄") {
		t.Fatalf("expected compact header treatment on short terminal, got %q", view)
	}
	if !strings.Contains(strings.ToLower(view), "smolbot") {
		t.Fatalf("expected compact layout to keep app identity visible, got %q", view)
	}
	if lines := strings.Count(view, "\n") + 1; lines > 8 {
		t.Fatalf("expected compact layout to stay within height 8, got %d lines", lines)
	}
}

func TestHeaderArtIsCenteredAcrossViewport(t *testing.T) {
	model := New(app.Config{})

	updated, _ := model.Update(tea.WindowSizeMsg{Width: 80, Height: 35})
	got := updated.(Model)

	lines := strings.Split(plain(got.View().Content), "\n")
	artLine := ""
	for _, line := range lines {
		if strings.Contains(line, "▄▄▄▄ ▄") {
			artLine = line
			break
		}
	}
	if artLine == "" {
		t.Fatalf("expected ascii header line in view %q", plain(got.View().Content))
	}
	if !strings.HasPrefix(strings.TrimSpace(artLine), "▄▄▄▄") {
		t.Fatalf("expected left-aligned header art, got %q", artLine)
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

func TestSidebarLayoutFillsViewportHeight(t *testing.T) {
	model := New(app.Config{})
	model.sidebarVisible = true
	model.app.Model = "gpt-5"
	model.app.Session = "tui:main"

	updated, _ := model.Update(tea.WindowSizeMsg{Width: 140, Height: 20})
	got := updated.(Model)
	got.status.SetConnected(true)

	lines := strings.Count(plain(got.View().Content), "\n") + 1
	if lines != 20 {
		t.Fatalf("expected composed layout to fill viewport height 20, got %d lines", lines)
	}
}

func TestEditorUsesThemeSurface(t *testing.T) {
	model := New(app.Config{Theme: "catppuccin"})
	model.width = 80
	model.height = 16
	view := model.editor.View()

	if !strings.Contains(view, "48;2;10;10;10") {
		t.Fatalf("expected editor to consume dark panel surface, got %q", view)
	}
}

func TestDialogsUsePanelSurface(t *testing.T) {
	model := New(app.Config{Theme: "catppuccin"})
	model.width = 80
	model.height = 24

	updated, _ := model.handleSlashCommand("/theme")
	got := updated.(Model)
	view := got.View().Content

	if !strings.Contains(view, "48;2;10;10;10") {
		t.Fatalf("expected dialogs to render on dark panel surface, got %q", view)
	}
}

func TestVisualSurfaceIntegration(t *testing.T) {
	model := New(app.Config{Theme: "catppuccin"})
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
	if strings.Count(view.Content, "48;2;10;10;10") < 2 {
		t.Fatalf("expected dark panel surfaces to appear across editor/dialog stack, got %q", view.Content)
	}
	if !strings.Contains(transcriptView, "USER") || !strings.Contains(transcriptView, "ASSISTANT") {
		t.Fatalf("expected transcript semantics to remain intact under overlay, got %q", transcriptView)
	}
	if !strings.Contains(plainView, "THEMES") {
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
	if !theme.Set("nord") {
		t.Fatal("expected nord theme")
	}
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
	if !theme.Set("nord") {
		t.Fatal("expected nord theme")
	}
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
	if !theme.Set("nord") {
		t.Fatal("expected nord theme")
	}
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

func TestF2CyclesToNextRecentModel(t *testing.T) {
model := New(app.Config{})
model.app.Model = "gpt-4o"
model.app.State.AddRecent("gpt-5")
model.app.State.AddRecent("gpt-4o")
model.client = &fakeClient{
models:  []client.ModelInfo{{ID: "gpt-5"}, {ID: "gpt-4o"}},
current: "gpt-4o",
}

updated, cmd := model.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyF2}))
got := updated.(Model)
_ = got
if cmd == nil {
t.Fatal("expected F2 to produce a command when recents available")
}
msg := cmd()
set, ok := msg.(ModelSetMsg)
if !ok {
t.Fatalf("expected ModelSetMsg, got %T", msg)
}
// recents are [gpt-5, gpt-4o]; current is gpt-4o → next is gpt-5
if set.ID != "gpt-5" {
t.Fatalf("expected F2 to cycle to gpt-5, got %q", set.ID)
}
}

func TestF2OpensDialogWhenNoRecents(t *testing.T) {
model := New(app.Config{})
model.app.Model = "gpt-4o"
model.client = &fakeClient{
models:  []client.ModelInfo{{ID: "gpt-4o", Provider: "openai", Selectable: true}},
current: "gpt-4o",
}

updated, cmd := model.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyF2}))
got := updated.(Model)
_ = got
if cmd == nil {
t.Fatal("expected F2 to produce a command when no recents")
}
msg := cmd()
if _, ok := msg.(ModelsLoadedMsg); !ok {
t.Fatalf("expected ModelsLoadedMsg (opens dialog) when no recents, got %T", msg)
}
}

func TestF2WrapsAroundRecentsList(t *testing.T) {
	model := New(app.Config{})
	// AddRecent prepends, so adding a/b/c gives recents = ["c","b","a"].
	// Set current to "a" (last element) so next wraps to index 0 = "c".
	model.app.State.AddRecent("a")
	model.app.State.AddRecent("b")
	model.app.State.AddRecent("c")
	model.app.Model = "a"
	model.client = &fakeClient{
		models:  []client.ModelInfo{{ID: "a"}, {ID: "b"}, {ID: "c"}},
		current: "a",
	}

	updated, cmd := model.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyF2}))
	_ = updated
	if cmd == nil {
		t.Fatal("expected F2 to produce a command")
	}
	msg := cmd()
	set, ok := msg.(ModelSetMsg)
	if !ok {
		t.Fatalf("expected ModelSetMsg on wrap, got %T", msg)
	}
	// recents = ["c","b","a"], current="a" at index 2 → next=(2+1)%3=0 → "c"
	if set.ID != "c" {
		t.Fatalf("expected F2 to wrap around to 'c', got %q", set.ID)
	}
}
