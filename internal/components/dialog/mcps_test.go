package dialog

import (
	"strings"
	"testing"

	"github.com/Nomadcxx/smolbot/internal/client"
	"github.com/Nomadcxx/smolbot/internal/theme"
	_ "github.com/Nomadcxx/smolbot/internal/theme/themes"
)

func TestMCPServersModelShowsServersAndEmptyState(t *testing.T) {
	if !theme.Set("nord") {
		t.Fatal("expected nord theme")
	}

	model := NewMCPServers([]client.MCPServerInfo{
		{Name: "filesystem", Command: "npx server-filesystem", Status: "configured"},
	})
	view := plainDialog(model.View())
	if !strings.Contains(view, "filesystem") || !strings.Contains(view, "configured") {
		t.Fatalf("expected configured server in view, got %q", view)
	}

	empty := NewMCPServers(nil)
	view = plainDialog(empty.View())
	if !strings.Contains(view, "No MCP servers configured") {
		t.Fatalf("expected empty state, got %q", view)
	}
}
