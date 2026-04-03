package themes

import (
	lipgloss "charm.land/lipgloss/v2"
	"github.com/Nomadcxx/smolbot/internal/theme"
)

func init() {
	register("gruvbox", [15]string{
		"#000000", "#000000", "#0a0a0a", "#665C54", "#FE8019",
		"#FE8019", "#8EC07C", "#FABD2F", "#EBDBB2", "#BDAE93",
		"#CC241D", "#D79921", "#8EC07C", "#8EC07C", "#665C54",
	}, func(t *theme.Theme) {
		t.SidebarBg = lipgloss.Color("#111111")
	})
}
