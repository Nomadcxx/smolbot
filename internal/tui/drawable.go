package tui

import uv "github.com/charmbracelet/ultraviolet"

type Drawable interface {
	Draw(scr uv.Screen, area uv.Rectangle)
}
