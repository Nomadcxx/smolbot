package tool

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

type WaitTool struct{}

type waitArgs struct {
	AgentIDs []string `json:"agent_ids"`
}

func NewWaitTool() *WaitTool {
	return &WaitTool{}
}

func (t *WaitTool) Name() string {
	return "wait"
}

func (t *WaitTool) Description() string {
	return "Wait for outstanding delegated child agents to finish and return compact summaries."
}

func (t *WaitTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"agent_ids": map[string]any{
				"type":  "array",
				"items": map[string]any{"type": "string"},
			},
		},
	}
}

func (t *WaitTool) Execute(ctx context.Context, raw json.RawMessage, tctx ToolContext) (*Result, error) {
	args := waitArgs{}
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &args); err != nil {
			return nil, fmt.Errorf("parse wait args: %w", err)
		}
	}
	if tctx.Spawner == nil {
		return &Result{Error: "spawner unavailable"}, nil
	}
	if strings.TrimSpace(tctx.SessionKey) == "" {
		return &Result{Error: "session key is required for wait"}, nil
	}

	waited, err := tctx.Spawner.Wait(ctx, WaitRequest{
		ParentSessionKey: tctx.SessionKey,
		AgentIDs:         append([]string(nil), args.AgentIDs...),
		EmitEvent:        tctx.EmitEvent,
	})
	if err != nil {
		return &Result{Error: fmt.Sprintf("wait for child agents: %v", err)}, nil
	}
	if waited == nil || waited.Count == 0 {
		return &Result{
			Output: "",
			Metadata: map[string]any{
				"count":   0,
				"results": []WaitResultItem{},
			},
		}, nil
	}

	return &Result{
		Output: fmt.Sprintf("finished waiting for %d agent(s)", waited.Count),
		Metadata: map[string]any{
			"count":   waited.Count,
			"results": waited.Results,
		},
	}, nil
}
