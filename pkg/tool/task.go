package tool

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

const taskMaxIterations = 15

type TaskTool struct {
	newID func() string
}

type taskArgs struct {
	Description     string `json:"description"`
	Prompt          string `json:"prompt"`
	AgentType       string `json:"agent_type"`
	Model           string `json:"model"`
	ReasoningEffort string `json:"reasoning_effort"`
}

func NewTaskTool(newID func() string) *TaskTool {
	if newID == nil {
		newID = func() string { return "task" }
	}
	return &TaskTool{newID: newID}
}

func (t *TaskTool) Name() string {
	return "task"
}

func (t *TaskTool) Description() string {
	return "Delegate a structured task to a background child agent."
}

func (t *TaskTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"description":      map[string]any{"type": "string"},
			"prompt":           map[string]any{"type": "string"},
			"agent_type":       map[string]any{"type": "string"},
			"model":            map[string]any{"type": "string"},
			"reasoning_effort": map[string]any{"type": "string"},
		},
		"required": []string{"description", "prompt", "agent_type"},
	}
}

func (t *TaskTool) Execute(ctx context.Context, raw json.RawMessage, tctx ToolContext) (*Result, error) {
	args := taskArgs{}
	if err := json.Unmarshal(raw, &args); err != nil {
		return nil, fmt.Errorf("parse task args: %w", err)
	}
	if tctx.Spawner == nil {
		return &Result{Error: "spawner unavailable"}, nil
	}
	if strings.TrimSpace(tctx.SessionKey) == "" {
		return &Result{Error: "session key is required for task"}, nil
	}
	if strings.TrimSpace(args.Description) == "" {
		return &Result{Error: "description is required"}, nil
	}
	if strings.TrimSpace(args.Prompt) == "" {
		return &Result{Error: "prompt is required"}, nil
	}
	if strings.TrimSpace(args.AgentType) == "" {
		return &Result{Error: "agent_type is required"}, nil
	}

	childSessionKey := fmt.Sprintf("task:%s:%s", tctx.SessionKey, t.newID())
	spawned, err := tctx.Spawner.Spawn(ctx, SpawnRequest{
		ParentSessionKey: tctx.SessionKey,
		ChildSessionKey:  childSessionKey,
		Description:      args.Description,
		Prompt:           args.Prompt,
		AgentType:        args.AgentType,
		Model:            strings.TrimSpace(args.Model),
		ReasoningEffort:  strings.TrimSpace(args.ReasoningEffort),
		MaxIterations:    taskMaxIterations,
		DisabledTools:    []string{"message", "spawn", "task"},
		EmitEvent:        tctx.EmitEvent,
	})
	if err != nil {
		return &Result{Error: fmt.Sprintf("delegate task: %v", err)}, nil
	}
	if spawned == nil {
		return &Result{Error: "delegate task: empty result"}, nil
	}

	return &Result{
		Output: fmt.Sprintf("delegated %s to %s", spawned.Description, firstNonEmptyString(spawned.Name, spawned.ID)),
		Metadata: map[string]any{
			"agentID":         spawned.ID,
			"agentName":       spawned.Name,
			"agentType":       spawned.AgentType,
			"model":           spawned.Model,
			"reasoningEffort": spawned.ReasoningEffort,
			"description":     spawned.Description,
			"promptPreview":   spawned.PromptPreview,
		},
	}, nil
}
