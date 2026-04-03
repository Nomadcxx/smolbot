package themes

import (
	lipgloss "charm.land/lipgloss/v2"
	"github.com/Nomadcxx/smolbot/internal/theme"
)

func init() {
	register("nord", [15]string{
		"#000000", "#000000", "#0a0a0a", "#3B4252", "#81A1C1",
		"#81A1C1", "#88C0D0", "#8FBCBB", "#ECEFF4", "#D8DEE9",
		"#BF616A", "#EBCB8B", "#8FBCBB", "#88C0D0", "#3B4252",
	}, func(t *theme.Theme) {
		t.SidebarBg = lipgloss.Color("#111111")
		t.DiffAdded = lipgloss.Color("#a3be8c")
		t.DiffRemoved = lipgloss.Color("#bf616a")
		t.DiffAddedBg = lipgloss.Color("#1a2a1a")
		t.DiffRemovedBg = lipgloss.Color("#2a1a1a")
	})
}
