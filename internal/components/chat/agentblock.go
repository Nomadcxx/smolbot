package chat

import (
	"fmt"
	"strings"

	lipgloss "charm.land/lipgloss/v2"
	"github.com/Nomadcxx/smolbot/internal/theme"
)

type AgentArtifactKind string

const (
	SpawnedAgentArtifact   AgentArtifactKind = "spawned"
	WaitingAgentsArtifact  AgentArtifactKind = "waiting"
	FinishedWaitingArtifact AgentArtifactKind = "finished_waiting"
)

type AgentArtifact struct {
	Kind   AgentArtifactKind
	Count  int
	Agents []AgentArtifactAgent
}

type AgentArtifactAgent struct {
	ID              string
	Name            string
	AgentType       string
	Model           string
	ReasoningEffort string
	Description     string
	PromptPreview   string
	Status          string
	Summary         string
	Error           string
}

func renderAgentArtifact(artifact AgentArtifact, width int) string {
	t := theme.Current()
	if t == nil {
		return strings.Join(plainAgentLines(artifact, width), "\n")
	}

	lines := plainAgentLines(artifact, width)
	if len(lines) == 0 {
		return ""
	}

	headStyle := lipgloss.NewStyle().Foreground(t.Accent).Bold(true)
	bodyStyle := lipgloss.NewStyle().Foreground(t.TextMuted)
	detailStyle := lipgloss.NewStyle().Foreground(t.Text)

	rendered := make([]string, 0, len(lines))
	for i, line := range lines {
		switch {
		case i == 0:
			rendered = append(rendered, headStyle.Render(line))
		case strings.HasPrefix(line, "  └ "):
			rendered = append(rendered, detailStyle.Render(line))
		default:
			rendered = append(rendered, bodyStyle.Render(line))
		}
	}
	return strings.Join(rendered, "\n")
}

func plainAgentLines(artifact AgentArtifact, width int) []string {
	contentWidth := max(24, cappedWidth(width))
	switch artifact.Kind {
	case SpawnedAgentArtifact:
		if len(artifact.Agents) == 0 {
			return nil
		}
		agent := artifact.Agents[0]
		lines := []string{fmt.Sprintf("• Spawned %s", agentIdentity(agent))}
		preview := firstNonEmpty(strings.TrimSpace(agent.Description), strings.TrimSpace(agent.PromptPreview))
		lines = append(lines, wrapWithPrefixes(preview, "  └ ", "    ", contentWidth)...)
		return lines
	case WaitingAgentsArtifact:
		lines := []string{fmt.Sprintf("• Waiting for %d agents", max(artifact.Count, len(artifact.Agents)))}
		for i, agent := range artifact.Agents {
			prefix := "    "
			if i == 0 {
				prefix = "  └ "
			}
			lines = append(lines, wrapWithPrefixes(agentNameAndType(agent), prefix, "    ", contentWidth)...)
		}
		return lines
	case FinishedWaitingArtifact:
		lines := []string{"• Finished waiting"}
		for i, agent := range artifact.Agents {
			prefix := "    "
			if i == 0 {
				prefix = "  └ "
			}
			summary := agentCompletionLine(agent)
			lines = append(lines, wrapWithPrefixes(summary, prefix, "    ", contentWidth)...)
		}
		return lines
	default:
		return nil
	}
}

func agentIdentity(agent AgentArtifactAgent) string {
	identity := firstNonEmpty(strings.TrimSpace(agent.Name), "Agent")
	role := strings.TrimSpace(agent.AgentType)
	if role != "" {
		identity += " [" + role + "]"
	}
	model := strings.TrimSpace(agent.Model)
	if model != "" {
		identity += " (" + modelModelSuffix(model, agent.ReasoningEffort) + ")"
	}
	return identity
}

func modelModelSuffix(model, reasoning string) string {
	reasoning = strings.TrimSpace(reasoning)
	if reasoning == "" {
		return model
	}
	return model + " " + reasoning
}

func agentNameAndType(agent AgentArtifactAgent) string {
	name := firstNonEmpty(strings.TrimSpace(agent.Name), "Agent")
	if role := strings.TrimSpace(agent.AgentType); role != "" {
		return name + " [" + role + "]"
	}
	return name
}

func agentCompletionLine(agent AgentArtifactAgent) string {
	status := strings.TrimSpace(agent.Status)
	if status == "" {
		status = "completed"
	}
	status = strings.ToUpper(status[:1]) + status[1:]
	detail := firstNonEmpty(strings.TrimSpace(agent.Summary), strings.TrimSpace(agent.Error), firstNonEmpty(strings.TrimSpace(agent.Description), strings.TrimSpace(agent.PromptPreview)))
	if detail == "" {
		return fmt.Sprintf("%s: %s", agentNameAndType(agent), status)
	}
	return fmt.Sprintf("%s: %s - %s", agentNameAndType(agent), status, detail)
}

func wrapWithPrefixes(text, firstPrefix, nextPrefix string, width int) []string {
	text = strings.Join(strings.Fields(strings.TrimSpace(text)), " ")
	if text == "" {
		return nil
	}
	if width <= 0 {
		width = 24
	}
	lineWidth := max(12, width-lipgloss.Width(firstPrefix))
	wrapped := wrapPlainText(text, lineWidth)
	lines := make([]string, 0, len(wrapped))
	for i, line := range wrapped {
		prefix := nextPrefix
		if i == 0 {
			prefix = firstPrefix
		}
		lines = append(lines, prefix+truncatePreview(line, lineWidth))
	}
	return lines
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
