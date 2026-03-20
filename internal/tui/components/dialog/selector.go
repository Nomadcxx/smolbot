package dialog

import (
	"sort"
	"strings"

	tea "charm.land/bubbletea/v2"
	lipgloss "charm.land/lipgloss/v2"
	"github.com/Nomadcxx/nanobot-go/internal/tui/theme"
)

type selectorOption struct {
	Value       string
	Label       string
	Description string
}

type selectorState struct {
	title   string
	empty   string
	footer  string
	options []selectorOption
	filter  string
	filtered []selectorOption
	cursor  int
}

type selectorAction int

const (
	selectorActionNone selectorAction = iota
	selectorActionClose
	selectorActionSelect
)

func newSelectorState(title string, options []selectorOption, empty, footer string) selectorState {
	s := selectorState{
		title:   title,
		empty:   empty,
		footer:  footer,
		options: append([]selectorOption(nil), options...),
	}
	s.applyFilter()
	return s
}

func (s *selectorState) update(msg tea.Msg) selectorAction {
	key, ok := msg.(tea.KeyMsg)
	if !ok {
		return selectorActionNone
	}
	switch key.String() {
	case "esc":
		return selectorActionClose
	case "up", "k", "ctrl+p":
		if s.cursor > 0 {
			s.cursor--
		}
	case "down", "j", "ctrl+n":
		if s.cursor < len(s.filtered)-1 {
			s.cursor++
		}
	case "enter", "tab":
		if len(s.filtered) > 0 {
			return selectorActionSelect
		}
	case "backspace":
		if len(s.filter) > 0 {
			s.filter = s.filter[:len(s.filter)-1]
			s.applyFilter()
		}
	default:
		k := key.String()
		if len(k) == 1 && k >= " " {
			s.filter += k
			s.applyFilter()
		}
	}
	return selectorActionNone
}

func (s selectorState) current() (selectorOption, bool) {
	if len(s.filtered) == 0 || s.cursor < 0 || s.cursor >= len(s.filtered) {
		return selectorOption{}, false
	}
	return s.filtered[s.cursor], true
}

func (s selectorState) view() string {
	t := theme.Current()
	if t == nil {
		return strings.ToLower(s.title)
	}

	lines := []string{
		lipgloss.NewStyle().Foreground(t.Primary).Bold(true).Render(s.title),
		lipgloss.NewStyle().Foreground(t.TextMuted).Render("Filter: " + s.filter),
		"",
	}
	if len(s.filtered) == 0 {
		lines = append(lines, "  "+s.empty)
	} else {
		for i, option := range s.filtered {
			prefix := "  "
			if i == s.cursor {
				prefix = "› "
			}
			lines = append(lines, prefix+option.Label)
			if option.Description != "" {
				lines = append(lines, "  "+lipgloss.NewStyle().Foreground(t.TextMuted).Render(option.Description))
			}
		}
	}
	if s.footer != "" {
		lines = append(lines, "", lipgloss.NewStyle().Foreground(t.TextMuted).Render(s.footer))
	}

	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(t.BorderFocus).
		Padding(1, 2).
		Width(56).
		Render(strings.Join(lines, "\n"))
}

func (s *selectorState) setFilter(filter string) {
	s.filter = filter
	s.applyFilter()
}

func (s *selectorState) applyFilter() {
	s.filtered = s.filtered[:0]
	needle := strings.ToLower(strings.TrimSpace(s.filter))
	if needle == "" {
		s.filtered = append(s.filtered, s.options...)
		if s.cursor >= len(s.filtered) {
			s.cursor = max(0, len(s.filtered)-1)
		}
		return
	}

	type candidate struct {
		option selectorOption
		score  int
		order  int
	}
	candidates := make([]candidate, 0, len(s.options))
	for _, option := range s.options {
		primary := strings.ToLower(strings.Join([]string{option.Value, option.Label}, " "))
		if pos := strings.Index(primary, needle); pos >= 0 {
			candidates = append(candidates, candidate{option: option, score: pos, order: len(candidates)})
			continue
		}
		description := strings.ToLower(option.Description)
		if pos := strings.Index(description, needle); pos >= 0 {
			candidates = append(candidates, candidate{option: option, score: 1000 + pos, order: len(candidates)})
		}
	}
	sort.SliceStable(candidates, func(i, j int) bool {
		if candidates[i].score == candidates[j].score {
			return candidates[i].order < candidates[j].order
		}
		return candidates[i].score < candidates[j].score
	})
	for _, candidate := range candidates {
		s.filtered = append(s.filtered, candidate.option)
	}
	if s.cursor >= len(s.filtered) {
		s.cursor = max(0, len(s.filtered)-1)
	}
}
