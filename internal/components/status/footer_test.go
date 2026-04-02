package status

import (
	"fmt"
	"image/color"
	"strings"
	"testing"

	"github.com/Nomadcxx/smolbot/internal/app"
	"github.com/Nomadcxx/smolbot/internal/client"
	"github.com/Nomadcxx/smolbot/internal/theme"
	_ "github.com/Nomadcxx/smolbot/internal/theme/themes"
	"github.com/stretchr/testify/require"
)

func stripANSIStatus(text string) string {
	text = strings.ReplaceAll(text, "\x1b[m", "")
	for {
		start := strings.Index(text, "\x1b[")
		if start == -1 {
			return text
		}
		end := strings.Index(text[start:], "m")
		if end == -1 {
			return text[:start]
		}
		text = text[:start] + text[start+end+1:]
	}
}

func TestStatusShowsActivityOnly(t *testing.T) {
	if !theme.Set("nord") {
		t.Fatal("expected nord theme")
	}

	a := app.New(app.Config{})
	a.Model = "gpt-5"
	a.Session = "tui:main"
	model := New(a)
	model.SetConnected(true)
	model.SetWidth(80)

	view := stripANSIStatus(model.View())
	if !strings.Contains(view, "connected") {
		t.Fatalf("expected activity status, got %q", view)
	}
	if strings.Contains(view, "gpt-5") || strings.Contains(view, "tui:main") {
		t.Fatalf("expected status row to omit persistent metadata, got %q", view)
	}
}

func TestStatusKeepsReconnectingAndStreamingExplicit(t *testing.T) {
	if !theme.Set("nord") {
		t.Fatal("expected nord theme")
	}

	a := app.New(app.Config{})
	model := New(a)
	model.SetWidth(80)

	model.SetReconnecting(true)
	view := stripANSIStatus(model.View())
	if !strings.Contains(view, "reconnecting") {
		t.Fatalf("expected reconnecting state to stay explicit, got %q", view)
	}

	model.SetConnected(true)
	model.SetStreaming(true)
	view = stripANSIStatus(model.View())
	if !strings.Contains(view, "working") {
		t.Fatalf("expected streaming state to stay explicit, got %q", view)
	}
}

func TestStatusShowsFlashMessage(t *testing.T) {
	if !theme.Set("nord") {
		t.Fatal("expected nord theme")
	}

	a := app.New(app.Config{})
	model := New(a)
	model.SetWidth(80)
	model.SetConnected(true)
	model.SetFlash("Copied to clipboard")

	view := stripANSIStatus(model.View())
	if !strings.Contains(view, "Copied to clipboard") {
		t.Fatalf("expected flash message in status row, got %q", view)
	}

	model.ClearFlash()
	view = stripANSIStatus(model.View())
	if strings.Contains(view, "Copied to clipboard") {
		t.Fatalf("expected flash message to clear, got %q", view)
	}
}

func TestFooterShowsPersistentMetadata(t *testing.T) {
	if !theme.Set("nord") {
		t.Fatal("expected nord theme")
	}

	a := app.New(app.Config{})
	a.Model = "gpt-5"
	a.Session = "tui:main"
	footer := NewFooter(a)
	footer.SetWidth(80)
	footer.SetMetadata("workspace ~/nanobot")

	view := stripANSIStatus(footer.View())
	if !strings.Contains(view, "model gpt-5") {
		t.Fatalf("expected model metadata in footer, got %q", view)
	}
	if !strings.Contains(view, "session tui:main") {
		t.Fatalf("expected session metadata in footer, got %q", view)
	}
	if !strings.Contains(view, "workspace ~/nanobot") {
		t.Fatalf("expected footer to allow future metadata, got %q", view)
	}
}

func TestFooterRightAlignsUsageMeter(t *testing.T) {
	if !theme.Set("nord") {
		t.Fatal("expected nord theme")
	}

	a := app.New(app.Config{})
	a.Model = "ollama/qwen3:8b"
	a.Session = "tui:main"
	footer := NewFooter(a)
	footer.SetWidth(80)
	footer.SetUsage(client.UsageInfo{TotalTokens: 68000, ContextWindow: 200000})

	view := stripANSIStatus(footer.View())
	if !strings.Contains(view, "model ollama/qwen3:8b | session tui:main") {
		t.Fatalf("expected left-side footer metadata, got %q", view)
	}
	if !strings.Contains(view, "34% (68K/200K)") {
		t.Fatalf("expected right-side usage meter, got %q", view)
	}
}

func TestFooterCollapsesUsageMeterOnNarrowWidths(t *testing.T) {
	if !theme.Set("nord") {
		t.Fatal("expected nord theme")
	}

	a := app.New(app.Config{})
	a.Model = "ollama/qwen3:8b"
	a.Session = "tui:main"
	footer := NewFooter(a)
	footer.SetWidth(24)
	footer.SetUsage(client.UsageInfo{TotalTokens: 68000, ContextWindow: 200000})

	view := stripANSIStatus(footer.View())
	if !strings.Contains(view, "34% (68K)") {
		t.Fatalf("expected compact usage meter, got %q", view)
	}
}

func TestFooterWarnsOnHighUsage(t *testing.T) {
	if !theme.Set("nord") {
		t.Fatal("expected nord theme")
	}

	tokyo := theme.Current()
	if tokyo == nil {
		t.Fatal("expected theme current")
	}

	a := app.New(app.Config{})
	footer := NewFooter(a)
	footer.SetWidth(80)
	footer.SetUsage(client.UsageInfo{TotalTokens: 170000, ContextWindow: 200000})

	view := footer.View()
	if !strings.Contains(stripANSIStatus(view), "85% (170K/200K)") {
		t.Fatalf("expected warning usage text, got %q", stripANSIStatus(view))
	}
	if !strings.Contains(view, ansiRGB(tokyo.Warning)) {
		t.Fatalf("expected warning color in high-usage meter, got %q", view)
	}
}

func ansiRGB(c color.Color) string {
	r, g, b, _ := c.RGBA()
	return fmt.Sprintf("38;2;%d;%d;%d", r>>8, g>>8, b>>8)
}

func TestFooterCompressionIndicator(t *testing.T) {
	if !theme.Set("nord") {
		t.Fatal("expected nord theme")
	}

	a := app.New(app.Config{})
	footer := NewFooter(a)
	footer.SetWidth(100)

	footer.SetCompression(&client.CompressionInfo{
		Enabled:          true,
		ReductionPercent: 35.0,
	})

	view := stripANSIStatus(footer.View())
	require.Contains(t, view, "↓35%")
}

func TestFooterCompressionHighReduction(t *testing.T) {
	if !theme.Set("nord") {
		t.Fatal("expected nord theme")
	}

	currentTheme := theme.Current()
	if currentTheme == nil {
		t.Fatal("expected theme current")
	}

	a := app.New(app.Config{})
	footer := NewFooter(a)
	footer.SetWidth(100)

	footer.SetCompression(&client.CompressionInfo{
		Enabled:          true,
		ReductionPercent: 65.0, // High - should use warning color
	})

	view := footer.View()
	require.Contains(t, stripANSIStatus(view), "↓65%")
}

func TestFooterCompressionDisabled(t *testing.T) {
	if !theme.Set("nord") {
		t.Fatal("expected nord theme")
	}

	a := app.New(app.Config{})
	footer := NewFooter(a)
	footer.SetWidth(100)

	footer.SetCompression(&client.CompressionInfo{Enabled: false})
	view := stripANSIStatus(footer.View())

	// Should not show compression indicator
	require.NotContains(t, view, "↓")
}

func TestFooterShowsCompactingSpinner(t *testing.T) {
	if !theme.Set("nord") {
		t.Fatal("expected nord theme")
	}

	a := app.New(app.Config{})
	footer := NewFooter(a)
	footer.SetWidth(100)
	footer.SetCompacting(true)
	footer.SetCompactionFrame(2)

	view := stripANSIStatus(footer.View())
	require.Contains(t, view, "compacting...")
	require.Contains(t, view, "⣻")
}

func TestFooterCriticalUsageWarnings(t *testing.T) {
	if !theme.Set("nord") {
		t.Fatal("expected nord theme")
	}

	a := app.New(app.Config{})
	footer := NewFooter(a)
	footer.SetWidth(100)

	footer.SetUsage(client.UsageInfo{
		TotalTokens:   9000,
		ContextWindow: 10000,
	})
	view := stripANSIStatus(footer.View())
	require.Contains(t, view, "90%")
	require.Contains(t, view, "⚠")
	require.NotContains(t, view, "/compact")

	footer.SetUsage(client.UsageInfo{
		TotalTokens:   9500,
		ContextWindow: 10000,
	})
	view = stripANSIStatus(footer.View())
	require.Contains(t, view, "95%")
	require.Contains(t, view, "⚠ /compact")
}

func TestFooterTokenUsageColorCoding(t *testing.T) {
	if !theme.Set("nord") {
		t.Fatal("expected nord theme")
	}

	a := app.New(app.Config{})
	footer := NewFooter(a)
	footer.SetWidth(100)

	// Test high usage (>90%)
	footer.SetUsage(client.UsageInfo{
		TotalTokens:   9000,
		ContextWindow: 10000,
	})
	view := stripANSIStatus(footer.View())
	require.Contains(t, view, "90%")

	// Test medium usage (60-80%)
	footer.SetUsage(client.UsageInfo{
		TotalTokens:   7000,
		ContextWindow: 10000,
	})
	view = stripANSIStatus(footer.View())
	require.Contains(t, view, "70%")
}
