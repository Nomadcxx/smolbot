package tool

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

type CronTool struct {
	service CronService
}

type cronArgs struct {
	Action    string `json:"action"`
	ID        string `json:"id"`
	Name      string `json:"name"`
	Schedule  string `json:"schedule"`
	Timezone  string `json:"timezone"`
	Reminder  string `json:"reminder"`
	Channel   string `json:"channel"`
	ChatID    string `json:"chat_id"`
	IsEnabled bool   `json:"isEnabled"`
}

func NewCronTool(service CronService) *CronTool {
	return &CronTool{service: service}
}

func (t *CronTool) Name() string {
	return "cron"
}

func (t *CronTool) Description() string {
	return "Create, inspect, update, and delete scheduled jobs."
}

func (t *CronTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"action":   map[string]any{"type": "string"},
			"id":       map[string]any{"type": "string"},
			"name":     map[string]any{"type": "string"},
			"schedule": map[string]any{"type": "string"},
			"timezone": map[string]any{"type": "string"},
			"reminder": map[string]any{"type": "string"},
			"channel":  map[string]any{"type": "string"},
			"chat_id":  map[string]any{"type": "string"},
		},
		"required": []string{"action"},
	}
}

func (t *CronTool) Execute(ctx context.Context, raw json.RawMessage, tctx ToolContext) (*Result, error) {
	args := cronArgs{}
	if err := json.Unmarshal(raw, &args); err != nil {
		return nil, fmt.Errorf("parse cron args: %w", err)
	}
	if t.service == nil {
		return &Result{Error: "cron service unavailable"}, nil
	}
	action := strings.ToLower(strings.TrimSpace(args.Action))
	if action == "" {
		return &Result{Error: "action is required"}, nil
	}
	if tctx.IsCronContext && action == "create" {
		return &Result{Error: "cron context cannot create new jobs"}, nil
	}
	switch action {
	case "create", "update", "list", "enable", "disable", "delete":
	default:
		return &Result{Error: fmt.Sprintf("unsupported cron action %q", action)}, nil
	}
	if tz := strings.TrimSpace(args.Timezone); tz != "" {
		if _, err := time.LoadLocation(tz); err != nil {
			return &Result{Error: fmt.Sprintf("invalid timezone %q", tz)}, nil
		}
	}

	response, err := t.service.Handle(ctx, CronRequest{
		Action:   action,
		ID:       args.ID,
		Name:     args.Name,
		Schedule: args.Schedule,
		Timezone: args.Timezone,
		Reminder: args.Reminder,
		Channel:  args.Channel,
		ChatID:   args.ChatID,
		Enabled:  args.IsEnabled,
	})
	if err != nil {
		return &Result{Error: fmt.Sprintf("cron action failed: %v", err)}, nil
	}
	return &Result{
		Output: response,
		Metadata: map[string]any{
			"action": action,
		},
	}, nil
}
