package dialog

import tea "charm.land/bubbletea/v2"

type CommandChosenMsg struct {
	Command string
}

type CommandsModel struct {
	selectorState
}

func NewCommands(commands []string) CommandsModel {
	options := make([]selectorOption, 0, len(commands))
	for _, command := range commands {
		options = append(options, selectorOption{
			Value:       command,
			Label:       command,
			Description: commandDescription(command),
		})
	}
	return CommandsModel{selectorState: newSelectorState("Commands", options, "No commands", "Enter=run  Tab=run  Esc=close")}
}

func (m CommandsModel) Update(msg tea.Msg) (CommandsModel, tea.Cmd) {
	switch m.update(msg) {
	case selectorActionClose:
		return m, func() tea.Msg { return CloseDialogMsg{} }
	case selectorActionSelect:
		current, ok := m.current()
		if !ok {
			return m, nil
		}
		return m, func() tea.Msg { return CommandChosenMsg{Command: current.Value} }
	}
	return m, nil
}

func (m CommandsModel) View() string { return m.view() }

func (m *CommandsModel) SetFilter(filter string) {
	m.setFilter(filter)
}

func (m CommandsModel) Current() string {
	current, ok := m.current()
	if !ok {
		return ""
	}
	return current.Value
}

func commandDescription(command string) string {
	switch command {
	case "/session":
		return "Browse and switch saved sessions"
	case "/session new":
		return "Create a fresh TUI session"
	case "/session reset":
		return "Reset the current session history"
	case "/model":
		return "Browse and switch the active model"
	case "/clear":
		return "Clear the visible transcript"
	case "/status":
		return "Show runtime metadata"
	case "/help":
		return "List available slash commands"
	case "/quit":
		return "Quit the TUI"
	case "/theme":
		return "Pick a theme"
	default:
		return ""
	}
}
