package themes

import (
	lipgloss "charm.land/lipgloss/v2"
	"github.com/Nomadcxx/smolbot/internal/theme"
)

func init() {
	register("material", [15]string{
		"#000000", "#000000", "#0a0a0a", "#37474F", "#80CBC4",
		"#80CBC4", "#64B5F6", "#FFAB40", "#ECEFF1", "#90A4AE",
		"#F44336", "#FFB300", "#80CBC4", "#64B5F6", "#37474F",
	}, func(t *theme.Theme) {
		t.SidebarBg = lipgloss.Color("#111111")
	})
}
