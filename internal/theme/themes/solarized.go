package themes

import (
	lipgloss "charm.land/lipgloss/v2"
	"github.com/Nomadcxx/smolbot/internal/theme"
)

func init() {
	register("solarized", [15]string{
		"#000000", "#000000", "#0a0a0a", "#073642", "#268BD2",
		"#268BD2", "#2AA198", "#859900", "#FDF6E3", "#93A1A1",
		"#DC322F", "#B58900", "#859900", "#2AA198", "#073642",
	}, func(t *theme.Theme) {
		t.SidebarBg = lipgloss.Color("#111111")
	})
}
