package tool

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Nomadcxx/smolbot/pkg/config"
)

func TestToolRegistryIntegration(t *testing.T) {
	workspace := t.TempDir()
	registry := NewRegistry()
	registry.Register(NewExecTool(config.ExecToolConfig{DefaultTimeout: 1, MaxTimeout: 2}, true))
	registry.Register(NewReadFileTool(true))
	registry.Register(NewWriteFileTool(true))
	registry.Register(NewMessageTool())
	registry.Register(NewSpawnTool(func() string { return "child1" }))
	registry.Register(NewTaskTool(func() string { return "child2" }))

	defs := registry.Definitions()
	if len(defs) != 6 {
		t.Fatalf("expected six tool definitions, got %#v", defs)
	}
	if defs[0].Name != "exec" || defs[len(defs)-1].Name != "write_file" {
		t.Fatalf("expected stable sorted definitions, got %#v", defs)
	}

	writeRaw, _ := json.Marshal(map[string]any{
		"path":    filepath.Join(workspace, "notes.txt"),
		"content": "hello from registry",
	})
	writeResult, err := registry.Execute(context.Background(), "write_file", writeRaw, ToolContext{Workspace: workspace})
	if err != nil {
		t.Fatalf("write_file: %v", err)
	}
	if writeResult.Error != "" {
		t.Fatalf("unexpected write error %#v", writeResult)
	}

	readRaw, _ := json.Marshal(map[string]any{"path": filepath.Join(workspace, "notes.txt")})
	readResult, err := registry.Execute(context.Background(), "read_file", readRaw, ToolContext{Workspace: workspace})
	if err != nil {
		t.Fatalf("read_file: %v", err)
	}
	if !strings.Contains(firstNonEmpty(readResult.Output, readResult.Content), "1: hello from registry") {
		t.Fatalf("unexpected read output %#v", readResult)
	}

	escapedRaw, _ := json.Marshal(map[string]any{"command": "cat /etc/passwd"})
	escapedResult, err := registry.Execute(context.Background(), "exec", escapedRaw, ToolContext{Workspace: workspace})
	if err != nil {
		t.Fatalf("exec: %v", err)
	}
	if !strings.Contains(escapedResult.Error, "try a different approach") {
		t.Fatalf("expected retry hint on tool error, got %#v", escapedResult)
	}

	router := &fakeMessageRouter{}
	messageRaw, _ := json.Marshal(map[string]any{
		"channel": "discord",
		"chat_id": "abc",
		"content": "hello channel",
	})
	messageResult, err := registry.Execute(context.Background(), "message", messageRaw, ToolContext{MessageRouter: router})
	if err != nil {
		t.Fatalf("message: %v", err)
	}
	if router.calls != 1 || messageResult.Metadata["chatID"] != "abc" {
		t.Fatalf("unexpected message routing result %#v / %#v", router, messageResult)
	}

	spawner := &fakeSpawner{}
	spawnRaw, _ := json.Marshal(map[string]any{"message": "do work"})
	spawnResult, err := registry.Execute(context.Background(), "spawn", spawnRaw, ToolContext{SessionKey: "parent", Spawner: spawner})
	if err != nil {
		t.Fatalf("spawn: %v", err)
	}
	if firstNonEmpty(spawnResult.Output, spawnResult.Content) != "child finished" {
		t.Fatalf("unexpected spawn result %#v", spawnResult)
	}
	if spawnResult.Metadata["description"] == nil {
		t.Fatalf("expected structured spawn metadata, got %#v", spawnResult)
	}

	taskRaw, _ := json.Marshal(map[string]any{
		"description": "Spec review",
		"prompt":      "Review the working tree changes.",
		"agent_type":  "explorer",
	})
	taskResult, err := registry.Execute(context.Background(), "task", taskRaw, ToolContext{SessionKey: "parent", Spawner: spawner})
	if err != nil {
		t.Fatalf("task: %v", err)
	}
	if taskResult.Metadata["agentType"] != "explorer" {
		t.Fatalf("unexpected task result %#v", taskResult)
	}

	if _, err := os.Stat(filepath.Join(workspace, "notes.txt")); err != nil {
		t.Fatalf("expected written file to exist: %v", err)
	}
}
