package status

import (
	"time"

	tea "charm.land/bubbletea/v2"
	lipgloss "charm.land/lipgloss/v2"
	"github.com/Nomadcxx/smolbot/internal/theme"
)

const (
	SpinnerTickInterval = 80 * time.Millisecond
	SpinnerWidth        = 3
)

// Ellipsis frames: smooth wave animation.
var ellipsisFrames = []string{
	"∙∙∙",
	"●∙∙",
	"∙●∙",
	"∙∙●",
	"∙∙∙",
	"∙∙●",
	"∙●∙",
	"●∙∙",
}

type SpinnerTickMsg time.Time

type SpinnerModel struct {
	frame int
}

func NewSpinner() SpinnerModel {
	return SpinnerModel{}
}

func (s SpinnerModel) Init() tea.Cmd {
	return s.Tick()
}

func (s SpinnerModel) Tick() tea.Cmd {
	return tea.Tick(SpinnerTickInterval, func(t time.Time) tea.Msg {
		return SpinnerTickMsg(t)
	})
}

func (s SpinnerModel) Update(msg tea.Msg) (SpinnerModel, tea.Cmd) {
	if _, ok := msg.(SpinnerTickMsg); ok {
		s.frame = (s.frame + 1) % len(ellipsisFrames)
		return s, s.Tick()
	}
	return s, nil
}

func (s SpinnerModel) View() string {
	t := theme.Current()
	if t == nil {
		return ellipsisFrames[s.frame]
	}
	// Use primary/accent colors for the spinner animation
	return lipgloss.NewStyle().
		Foreground(t.Primary).
		Bold(true).
		Width(SpinnerWidth).
		Render(ellipsisFrames[s.frame])
}
