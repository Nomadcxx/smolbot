package theme

import "image/color"

type Theme struct {
	Name        string
	Background  color.Color
	Panel       color.Color
	Element     color.Color
	Border      color.Color
	BorderFocus color.Color
	Primary     color.Color
	Secondary   color.Color
	Accent      color.Color
	Text        color.Color
	TextMuted   color.Color
	Error       color.Color
	Warning     color.Color
	Success     color.Color
	Info        color.Color
	ToolBorder  color.Color
	ToolName    color.Color
}
