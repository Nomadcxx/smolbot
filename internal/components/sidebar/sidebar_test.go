package sidebar

import (
	"os"
	"strings"
	"testing"

	lipgloss "charm.land/lipgloss/v2"
	"github.com/Nomadcxx/smolbot/internal/client"
	"github.com/Nomadcxx/smolbot/internal/theme"
	_ "github.com/Nomadcxx/smolbot/internal/theme/themes"
	"github.com/charmbracelet/x/ansi"
)

func init() {
	_ = theme.Set("nord")
}

func plain(text string) string {
	return ansi.Strip(strings.NewReplacer("\r", "").Replace(text))
}

func TestRenderSectionHeaderUsesUppercaseAndSeparator(t *testing.T) {
	got := plain(renderSectionHeader("session", 24, theme.Current()))
	if !strings.HasPrefix(got, "SESSION") {
		t.Fatalf("expected uppercase title, got %q", got)
	}
	if !strings.Contains(got, "─") {
		t.Fatalf("expected separator line, got %q", got)
	}
}

func TestSessionSectionRendersPathAndModel(t *testing.T) {
	home, _ := os.UserHomeDir()
	section := SessionSection{
		sessionKey: "tui:main",
		cwd:        home + "/project",
		model:      "gpt-4o",
	}

	got := plain(section.Render(28, 0, theme.Current()))
	if !strings.Contains(got, "tui:main") || !strings.Contains(got, "~/project") || !strings.Contains(got, "gpt-4o") {
		t.Fatalf("unexpected session render: %q", got)
	}
}

func TestContextSectionRendersUsageAndCompression(t *testing.T) {
	section := ContextSection{
		usage: client.UsageInfo{
			TotalTokens:   78000,
			ContextWindow: 100000,
		},
		compression: &client.CompressionInfo{
			Enabled:          true,
			ReductionPercent: 42,
		},
	}

	got := plain(section.Render(28, 0, theme.Current()))
	if !strings.Contains(got, "78%") || !strings.Contains(got, "78K / 100K") || !strings.Contains(got, "↓ 42% compacted") {
		t.Fatalf("unexpected context render: %q", got)
	}
}

func TestUsageSectionRendersPersistedSummary(t *testing.T) {
	section := UsageSection{
		summary: &client.UsageSummary{
			ProviderID:    "ollama",
			ModelName:     "llama3.2",
			SessionTokens: 50,
			TodayTokens:   50,
			WeeklyTokens:  90,
			BudgetStatus:  "warning",
			WarningLevel:  "medium",
		},
	}

	got := plain(section.Render(28, 0, theme.Current()))
	if !strings.Contains(got, "ollama / llama3.2") || !strings.Contains(got, "session 50") || !strings.Contains(got, "week 90") || !strings.Contains(got, "warning medium") {
		t.Fatalf("unexpected usage render: %q", got)
	}
}

func TestUsageSectionAvoidsDuplicatingQualifiedProviderModel(t *testing.T) {
	section := UsageSection{
		summary: &client.UsageSummary{
			ProviderID:    "ollama",
			ModelName:     "ollama/llama3.2",
			SessionTokens: 50,
			TodayTokens:   50,
			WeeklyTokens:  90,
		},
	}

	got := plain(section.Render(28, 0, theme.Current()))
	if !strings.Contains(got, "ollama/llama3.2") {
		t.Fatalf("expected qualified provider/model label, got %q", got)
	}
	if strings.Contains(got, "ollama / ollama/llama3.2") {
		t.Fatalf("expected provider label to avoid duplication, got %q", got)
	}
}

func TestUsageSectionRendersEmptyState(t *testing.T) {
	section := UsageSection{}

	got := plain(section.Render(28, 0, theme.Current()))
	if got != "—" {
		t.Fatalf("expected empty usage state, got %q", got)
	}
}

func TestChannelsSectionTruncatesAndShowsOverflow(t *testing.T) {
	section := ChannelsSection{
		channels: []ChannelEntry{
			{Name: "WhatsApp", State: "connected"},
			{Name: "Signal", State: "starting"},
			{Name: "SMS", State: "error"},
			{Name: "Bridge", State: "connected"},
		},
	}

	got := plain(section.Render(28, 3, theme.Current()))
	if !strings.Contains(got, "WhatsApp") || !strings.Contains(got, "Signal") || !strings.Contains(got, "…and 1 more") {
		t.Fatalf("unexpected channels render: %q", got)
	}
}

func TestMCPsSectionShowsConfiguredToolsAndOverflow(t *testing.T) {
	section := MCPsSection{
		servers: []MCPEntry{
			{Name: "memory", Status: "connected", Tools: 3},
			{Name: "docs", Status: "configured", Tools: 2},
			{Name: "search", Status: "disabled"},
		},
	}

	got := plain(section.Render(28, 2, theme.Current()))
	if !strings.Contains(got, "memory") || !strings.Contains(got, "(3 tools)") || !strings.Contains(got, "…and 1 more") {
		t.Fatalf("unexpected mcps render: %q", got)
	}
}

func TestCronSectionRendersTwoLineJobs(t *testing.T) {
	section := CronSection{
		jobs: []client.CronJob{
			{ID: "job-1", Name: "backup", Schedule: "every 5m", Status: "active"},
			{ID: "job-2", Name: "pause", Schedule: "daily 02:00", Status: "paused"},
		},
	}

	got := plain(section.Render(28, 2, theme.Current()))
	if !strings.Contains(got, "backup") || !strings.Contains(got, "every 5m") || !strings.Contains(got, "⏸ pause") {
		t.Fatalf("unexpected cron render: %q", got)
	}
}

func TestSidebarDynamicLimitsAllocateByHeight(t *testing.T) {
	model := New()
	model.SetSize(30, 24)
	model.SetChannels([]ChannelEntry{
		{Name: "WhatsApp", State: "connected"},
		{Name: "Signal", State: "connected"},
		{Name: "SMS", State: "connected"},
		{Name: "Telegram", State: "connected"},
	})
	model.SetMCPs([]MCPEntry{
		{Name: "memory", Status: "connected"},
		{Name: "docs", Status: "configured"},
	})
	model.SetCronJobs([]client.CronJob{
		{Name: "backup", Schedule: "every 5m", Status: "active"},
		{Name: "rotate", Schedule: "daily 02:00", Status: "paused"},
	})

	limits := model.getDynamicLimits()
	if got := limits["CHANNELS"]; got == 0 {
		t.Fatalf("expected channel limit > 0")
	}
	if got := limits["MCPS"]; got == 0 {
		t.Fatalf("expected mcp limit > 0")
	}
	if got := limits["SCHEDULED"]; got == 0 {
		t.Fatalf("expected cron limit > 0")
	}
}

func TestSidebarViewAndCompactView(t *testing.T) {
	model := New()
	model.SetSize(84, 24)
	model.SetSession("tui:main")
	home, _ := os.UserHomeDir()
	model.SetCWD(home + "/project")
	model.SetModel("gpt-4o")
	model.SetUsage(client.UsageInfo{TotalTokens: 78000, ContextWindow: 100000})
	model.SetPersistedUsage(&client.UsageSummary{
		ProviderID:    "ollama",
		ModelName:     "llama3.2",
		SessionTokens: 50,
		TodayTokens:   50,
		WeeklyTokens:  90,
	})
	model.SetCompression(&client.CompressionInfo{Enabled: true, ReductionPercent: 42})
	model.SetChannels([]ChannelEntry{{Name: "WhatsApp", State: "connected"}})
	model.SetMCPs([]MCPEntry{{Name: "memory", Status: "connected"}})

	view := plain(model.View())
	if !strings.Contains(view, "SESSION") || !strings.Contains(view, "CONTEXT") || !strings.Contains(view, "USAGE") {
		t.Fatalf("expected sidebar view to include sections, got %q", view)
	}

	compact := plain(model.CompactView())
	if !strings.Contains(compact, "SESSION") || !strings.Contains(compact, "USAGE") || !strings.Contains(compact, "CHANNELS") {
		t.Fatalf("expected compact view to include multiple columns, got %q", compact)
	}
}

func TestSidebarViewRespectsHeightBudget(t *testing.T) {
	model := New()
	model.SetSize(30, 8)
	model.SetSession("tui:main")
	model.SetModel("gpt-4o")
	model.SetUsage(client.UsageInfo{TotalTokens: 78000, ContextWindow: 100000})
	model.SetPersistedUsage(&client.UsageSummary{
		ProviderID:    "ollama",
		ModelName:     "llama3.2",
		SessionTokens: 50,
		TodayTokens:   50,
		WeeklyTokens:  90,
	})
	model.SetChannels([]ChannelEntry{
		{Name: "WhatsApp", State: "connected"},
		{Name: "Signal", State: "connected"},
		{Name: "SMS", State: "connected"},
	})
	model.SetMCPs([]MCPEntry{{Name: "memory", Status: "configured"}})
	model.SetCronJobs([]client.CronJob{{Name: "backup", Schedule: "every 5m", Status: "active"}})

	if got := lipgloss.Height(model.View()); got > 8 {
		t.Fatalf("expected sidebar view to fit height 8, got %d lines: %q", got, plain(model.View()))
	}
}

func TestSidebarVisibleToggle(t *testing.T) {
	model := New()
	if !model.Visible() {
		t.Fatal("expected default visible sidebar")
	}
	model.Toggle()
	if model.Visible() {
		t.Fatal("expected toggle to hide sidebar")
	}
	model.SetVisible(true)
	if !model.Visible() {
		t.Fatal("expected SetVisible to show sidebar")
	}
}

func TestTruncateVisibleHandlesANSI(t *testing.T) {
	text := lipgloss.NewStyle().Foreground(lipgloss.Color("#ffffff")).Render("123456")
	got := plain(truncateVisible(text, 4))
	if len(got) == 0 || strings.Contains(got, "\x1b") {
		t.Fatalf("expected ansi-safe truncation, got %q", got)
	}
}
