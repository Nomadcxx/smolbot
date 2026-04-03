package themes

import (
	lipgloss "charm.land/lipgloss/v2"
	"github.com/Nomadcxx/smolbot/internal/theme"
)

func init() {
	register("dracula", [15]string{
		"#000000", "#000000", "#0a0a0a", "#44475A", "#BD93F9",
		"#BD93F9", "#8BE9FD", "#50FA7B", "#F8F8F2", "#6272A4",
		"#FF5555", "#F1FA8C", "#50FA7B", "#8BE9FD", "#44475A",
	}, func(t *theme.Theme) {
		t.SidebarBg = lipgloss.Color("#111111")
		t.DiffAdded = lipgloss.Color("#50FA7B")
		t.DiffRemoved = lipgloss.Color("#FF5555")
		t.DiffAddedBg = lipgloss.Color("#1a3a1a")
		t.DiffRemovedBg = lipgloss.Color("#3a1a1a")
	})
}
