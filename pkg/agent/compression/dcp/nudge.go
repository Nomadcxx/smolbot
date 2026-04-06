package dcp

import (
	"fmt"
	"strings"

	"github.com/Nomadcxx/smolbot/pkg/provider"
	"github.com/Nomadcxx/smolbot/pkg/tokenizer"
)

func InjectNudges(messages []provider.Message, state *State, cfg Config, tok *tokenizer.Tokenizer, contextWindow int) int {
	tier, usagePct := pendingNudge(messages, state, cfg, tok, contextWindow)
	if tier == "" {
		return 0
	}

	// Find last user message to attach nudge — avoids injecting into
	// assistant tool-call messages which some providers reject.
	target := -1
	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i].Role == "user" {
			target = i
			break
		}
	}
	if target < 0 {
		return 0
	}
	messages[target].Content = appendReminder(messages[target].Content, nudgeText(tier, usagePct))
	return 1
}

func pendingNudge(messages []provider.Message, state *State, cfg Config, tok *tokenizer.Tokenizer, contextWindow int) (string, int) {
	if len(messages) == 0 || tok == nil || cfg.Nudge.NudgeFrequency <= 0 {
		return "", 0
	}
	if state.RequestCount == 0 || state.RequestCount%cfg.Nudge.NudgeFrequency != 0 {
		return "", 0
	}
	for _, msg := range messages {
		if strings.Contains(msg.StringContent(), "<dcp-reminder>") {
			return "", 0
		}
	}

	tokenUsage := tok.EstimatePromptTokens(messages)
	msgsSinceUser := countMessagesSinceLastUser(messages)
	usagePct := 0
	if contextWindow > 0 {
		usagePct = (tokenUsage * 100) / contextWindow
	}

	switch {
	case shouldNudgeCritical(tokenUsage, cfg.Nudge.MaxContextLimit):
		return "critical", usagePct
	case shouldNudgeTurn(tokenUsage, cfg.Nudge.MinContextLimit, isNewUserTurn(messages)):
		return "turn", usagePct
	case shouldNudgeIteration(tokenUsage, cfg.Nudge.MinContextLimit, msgsSinceUser, cfg.Nudge.IterationNudgeThreshold):
		return "iteration", usagePct
	default:
		return "", 0
	}
}

func shouldNudgeCritical(tokenUsage, maxLimit int) bool {
	return maxLimit > 0 && tokenUsage >= maxLimit
}

func shouldNudgeTurn(tokenUsage, minLimit int, isNewUserTurn bool) bool {
	return minLimit > 0 && tokenUsage >= minLimit && isNewUserTurn
}

func shouldNudgeIteration(tokenUsage, minLimit int, msgsSinceUser int, threshold int) bool {
	return minLimit > 0 && tokenUsage >= minLimit && msgsSinceUser >= threshold
}

func nudgeText(tier string, usagePct int) string {
	switch tier {
	case "critical":
		return fmt.Sprintf("<dcp-reminder>CRITICAL: Context limit reached (%d%% used). You MUST use the compress tool NOW to summarize old conversation ranges and free space. Use the message IDs (m0001, m0002...) shown in <dcp-id> tags to specify ranges.</dcp-reminder>", usagePct)
	case "turn":
		return fmt.Sprintf("<dcp-reminder>Context is growing (%d%% used). Consider using the compress tool to summarize completed work. Target old tool call sequences that are no longer relevant.</dcp-reminder>", usagePct)
	default:
		return "<dcp-reminder>Extended iteration without user input. If tool output from earlier iterations is no longer needed, use compress to free context space.</dcp-reminder>"
	}
}

func countMessagesSinceLastUser(messages []provider.Message) int {
	count := 0
	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i].Role == "user" {
			break
		}
		count++
	}
	return count
}

func isNewUserTurn(messages []provider.Message) bool {
	return len(messages) > 0 && messages[len(messages)-1].Role == "user"
}

func appendReminder(content any, reminder string) any {
	switch value := content.(type) {
	case nil:
		return reminder
	case string:
		if value == "" {
			return reminder
		}
		return value + " " + reminder
	case []provider.ContentBlock:
		blocks := append([]provider.ContentBlock(nil), value...)
		if len(blocks) == 0 {
			return []provider.ContentBlock{{Type: "text", Text: reminder}}
		}
		for i := len(blocks) - 1; i >= 0; i-- {
			if blocks[i].Type == "text" || blocks[i].Type == "input_text" || blocks[i].Type == "output_text" {
				blocks[i].Text += " " + reminder
				return blocks
			}
		}
		return append(blocks, provider.ContentBlock{Type: "text", Text: reminder})
	default:
		text := provider.Message{Content: value}.StringContent()
		if text == "" {
			return reminder
		}
		return text + " " + reminder
	}
}
