package dcp

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/Nomadcxx/smolbot/pkg/provider"
)

var (
	dcpIDPattern       = regexp.MustCompile(`<dcp-id>([^<]+)</dcp-id>`)
	dcpReminderPattern = regexp.MustCompile(`<dcp-reminder>.*?</dcp-reminder>`)
)

func AssignMessageIDs(messages []provider.Message, state *State, cfg Config) {
	if state == nil {
		return
	}
	state.ProtectedIndexes = make(map[int]bool)
	state.ToolPairs = buildToolPairs(messages)
	for i, msg := range messages {
		if msg.Role == "system" {
			continue
		}
		if existing := extractDCPID(msg.StringContent()); existing != "" {
			if strings.HasPrefix(existing, "m") {
				state.MessageIDs.ByMsgIndex[i] = existing
				state.MessageIDs.ByRef[existing] = i
			}
			if existing == "PROTECTED" {
				state.ProtectedIndexes[i] = true
			}
			continue
		}
		if IsMessageProtected(msg, i, messages, state.CurrentTurn, cfg) {
			messages[i].Content = appendDCPTag(messages[i].Content, "PROTECTED")
			state.ProtectedIndexes[i] = true
			continue
		}
		ref := state.MessageIDs.ByMsgIndex[i]
		if ref == "" {
			ref = fmt.Sprintf("m%04d", state.MessageIDs.NextRef)
			state.MessageIDs.ByMsgIndex[i] = ref
			state.MessageIDs.ByRef[ref] = i
			state.MessageIDs.NextRef++
		}
		messages[i].Content = appendDCPTag(messages[i].Content, ref)
	}
}

func buildToolPairs(messages []provider.Message) []ToolPairState {
	callIndexes := make(map[string]int)
	pairs := make([]ToolPairState, 0)
	for i, msg := range messages {
		if msg.Role == "assistant" {
			for _, call := range msg.ToolCalls {
				callIndexes[call.ID] = i
			}
			continue
		}
		if msg.Role != "tool" || msg.ToolCallID == "" {
			continue
		}
		callIndex, ok := callIndexes[msg.ToolCallID]
		if !ok {
			continue
		}
		pairs = append(pairs, ToolPairState{
			ToolCallID:  msg.ToolCallID,
			CallIndex:   callIndex,
			ResultIndex: i,
		})
	}
	return pairs
}

func StripDCPTags(content string) string {
	content = dcpIDPattern.ReplaceAllString(content, "")
	content = dcpReminderPattern.ReplaceAllString(content, "")
	return strings.TrimSpace(content)
}

// StripMessages returns a copy of messages with all DCP tags removed from content.
func StripMessages(messages []provider.Message) []provider.Message {
	out := make([]provider.Message, len(messages))
	for i, msg := range messages {
		out[i] = msg
		out[i].Content = stripContent(msg.Content)
	}
	return out
}

func stripContent(content any) any {
	switch value := content.(type) {
	case string:
		stripped := StripDCPTags(value)
		if stripped == "" && value != "" {
			return value
		}
		return stripped
	case []provider.ContentBlock:
		blocks := make([]provider.ContentBlock, len(value))
		for i, b := range value {
			blocks[i] = b
			if b.Type == "text" || b.Type == "input_text" || b.Type == "output_text" {
				blocks[i].Text = StripDCPTags(b.Text)
			}
		}
		return blocks
	default:
		return content
	}
}

func extractDCPID(content string) string {
	matches := dcpIDPattern.FindStringSubmatch(content)
	if len(matches) != 2 {
		return ""
	}
	return matches[1]
}

func appendDCPTag(content any, tag string) any {
	wrapped := "<dcp-id>" + tag + "</dcp-id>"
	switch value := content.(type) {
	case nil:
		return wrapped
	case string:
		if extractDCPID(value) != "" {
			return value
		}
		if value == "" {
			return wrapped
		}
		return value + " " + wrapped
	case []provider.ContentBlock:
		blocks := append([]provider.ContentBlock(nil), value...)
		if len(blocks) == 0 {
			return []provider.ContentBlock{{Type: "text", Text: wrapped}}
		}
		for i := len(blocks) - 1; i >= 0; i-- {
			if blocks[i].Type == "text" || blocks[i].Type == "input_text" || blocks[i].Type == "output_text" {
				if extractDCPID(blocks[i].Text) != "" {
					return blocks
				}
				blocks[i].Text += " " + wrapped
				return blocks
			}
		}
		return append(blocks, provider.ContentBlock{Type: "text", Text: wrapped})
	default:
		text := provider.Message{Content: value}.StringContent()
		if text == "" {
			return wrapped
		}
		return text + " " + wrapped
	}
}
