package theme

import (
	"image/color"
	"sync"
)

// AgentColorName identifies one of 8 distinct agent colors.
type AgentColorName string

const (
	AgentColorBlue   AgentColorName = "blue"
	AgentColorGreen  AgentColorName = "green"
	AgentColorPurple AgentColorName = "purple"
	AgentColorOrange AgentColorName = "orange"
	AgentColorCyan   AgentColorName = "cyan"
	AgentColorPink   AgentColorName = "pink"
	AgentColorYellow AgentColorName = "yellow"
	AgentColorRed    AgentColorName = "red"
)

// agentColorOrder is the assignment sequence for new agent types.
var agentColorOrder = []AgentColorName{
	AgentColorBlue, AgentColorGreen, AgentColorPurple, AgentColorOrange,
	AgentColorCyan, AgentColorPink, AgentColorYellow, AgentColorRed,
}

var (
	agentColorMu          sync.RWMutex
	agentColorAssignments = make(map[string]AgentColorName)
	agentNextColorIndex   = 0
)

// GetAgentColor returns a consistent AgentColorName for the given agent type.
// The same type always receives the same color within a session.
func GetAgentColor(agentType string) AgentColorName {
	agentColorMu.Lock()
	defer agentColorMu.Unlock()

	if c, ok := agentColorAssignments[agentType]; ok {
		return c
	}

	c := agentColorOrder[agentNextColorIndex%len(agentColorOrder)]
	agentColorAssignments[agentType] = c
	agentNextColorIndex++
	return c
}

// GetAgentThemeColor resolves an AgentColorName to the color.Color from the given theme.
// Returns nil if t is nil or the name is unrecognised.
func GetAgentThemeColor(t *Theme, name AgentColorName) color.Color {
	if t == nil {
		return nil
	}
	switch name {
	case AgentColorRed:
		return t.AgentRed
	case AgentColorBlue:
		return t.AgentBlue
	case AgentColorGreen:
		return t.AgentGreen
	case AgentColorYellow:
		return t.AgentYellow
	case AgentColorPurple:
		return t.AgentPurple
	case AgentColorOrange:
		return t.AgentOrange
	case AgentColorPink:
		return t.AgentPink
	case AgentColorCyan:
		return t.AgentCyan
	default:
		return t.AgentBlue
	}
}

// ResetAgentColors clears all color assignments. Useful in tests.
func ResetAgentColors() {
	agentColorMu.Lock()
	defer agentColorMu.Unlock()
	agentColorAssignments = make(map[string]AgentColorName)
	agentNextColorIndex = 0
}
