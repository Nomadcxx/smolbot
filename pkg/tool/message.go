package tool

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

type MessageTool struct{}

type messageArgs struct {
	Channel string `json:"channel"`
	ChatID  string `json:"chat_id"`
	Content string `json:"content"`
}

func NewMessageTool() *MessageTool {
	return &MessageTool{}
}

func (t *MessageTool) Name() string {
	return "message"
}

func (t *MessageTool) Description() string {
	return "Send a message through the configured channel router."
}

// ConcurrencySafe: each message is routed independently.
func (t *MessageTool) IsConcurrencySafe() bool { return true }

// DeferredTool: message is specialized and hidden until discovered.
func (t *MessageTool) IsDeferred() bool          { return true }
func (t *MessageTool) IsAlwaysLoad() bool         { return false }
func (t *MessageTool) DeferredKeywords() []string {
	return []string{"message", "channel", "send", "notify", "whatsapp", "telegram", "signal"}
}

func (t *MessageTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"channel": map[string]any{"type": "string"},
			"chat_id": map[string]any{"type": "string"},
			"content": map[string]any{"type": "string"},
		},
		"required": []string{"channel", "chat_id", "content"},
	}
}

func (t *MessageTool) Execute(ctx context.Context, raw json.RawMessage, tctx ToolContext) (*Result, error) {
	args := messageArgs{}
	if err := json.Unmarshal(raw, &args); err != nil {
		return nil, fmt.Errorf("parse message args: %w", err)
	}
	if tctx.MessageRouter == nil {
		return &Result{Error: "message router unavailable"}, nil
	}
	args.Channel = strings.TrimSpace(args.Channel)
	args.ChatID = strings.TrimSpace(args.ChatID)
	args.Content = strings.TrimSpace(args.Content)
	if args.Channel == "" || args.ChatID == "" || args.Content == "" {
		return &Result{Error: "channel, chat_id, and content are required"}, nil
	}
	if err := tctx.MessageRouter.Route(ctx, args.Channel, args.ChatID, args.Content); err != nil {
		return &Result{Error: fmt.Sprintf("route message: %v", err)}, nil
	}
	return &Result{
		Output: "message sent",
		Metadata: map[string]any{
			"channel": args.Channel,
			"chatID":  args.ChatID,
		},
	}, nil
}
