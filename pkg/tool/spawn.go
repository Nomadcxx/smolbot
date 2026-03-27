package tool

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

const spawnMaxIterations = 15

type SpawnTool struct {
	newID func() string
}

type spawnArgs struct {
	Message string `json:"message"`
}

func NewSpawnTool(newID func() string) *SpawnTool {
	if newID == nil {
		newID = func() string { return "spawn" }
	}
	return &SpawnTool{newID: newID}
}

func (t *SpawnTool) Name() string {
	return "spawn"
}

func (t *SpawnTool) Description() string {
	return "Spawn a child agent session with reduced tool access."
}

func (t *SpawnTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"message": map[string]any{"type": "string"},
		},
		"required": []string{"message"},
	}
}

func (t *SpawnTool) Execute(ctx context.Context, raw json.RawMessage, tctx ToolContext) (*Result, error) {
	args := spawnArgs{}
	if err := json.Unmarshal(raw, &args); err != nil {
		return nil, fmt.Errorf("parse spawn args: %w", err)
	}
	if tctx.Spawner == nil {
		return &Result{Error: "spawner unavailable"}, nil
	}
	if strings.TrimSpace(tctx.SessionKey) == "" {
		return &Result{Error: "session key is required for spawn"}, nil
	}
	if strings.TrimSpace(args.Message) == "" {
		return &Result{Error: "message is required"}, nil
	}

	childSessionKey := fmt.Sprintf("spawn:%s:%s", tctx.SessionKey, t.newID())
	req := SpawnRequest{
		ParentSessionKey: tctx.SessionKey,
		ChildSessionKey:  childSessionKey,
		Prompt:           args.Message,
		Description:      args.Message,
		MaxIterations:    spawnMaxIterations,
		DisabledTools:    []string{"message", "spawn", "task"},
	}
	output, err := tctx.Spawner.ProcessDirect(ctx, req)
	if err != nil {
		return &Result{Error: fmt.Sprintf("spawn child: %v", err)}, nil
	}
	return &Result{
		Output: strings.TrimSpace(output),
		Metadata: map[string]any{
			"description":   strings.TrimSpace(args.Message),
			"promptPreview": summarizeInlinePrompt(args.Message),
		},
	}, nil
}

func summarizeInlinePrompt(prompt string) string {
	prompt = strings.Join(strings.Fields(strings.TrimSpace(prompt)), " ")
	if len(prompt) <= 160 {
		return prompt
	}
	return strings.TrimSpace(prompt[:157]) + "..."
}
