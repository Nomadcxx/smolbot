package themes

import (
	lipgloss "charm.land/lipgloss/v2"
	"github.com/Nomadcxx/smolbot/internal/theme"
)

func init() {
	register("catppuccin", [15]string{
		"#000000", "#000000", "#0a0a0a", "#313244", "#CBA6F7",
		"#CBA6F7", "#89B4FA", "#A6E3A1", "#CDD6F4", "#A6ADC8",
		"#F38BA8", "#F9E2AF", "#A6E3A1", "#89B4FA", "#313244",
	}, func(t *theme.Theme) {
		t.SidebarBg = lipgloss.Color("#111111")
	})
}
