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

func TestExecTool(t *testing.T) {
	workspace := t.TempDir()
	execDir := filepath.Join(workspace, "bin")
	if err := os.MkdirAll(execDir, 0o755); err != nil {
		t.Fatalf("mkdir exec dir: %v", err)
	}
	helper := filepath.Join(execDir, "showpath")
	if err := os.WriteFile(helper, []byte("#!/bin/sh\nprintf %s \"$PATH\"\n"), 0o755); err != nil {
		t.Fatalf("write helper: %v", err)
	}

	cfg := config.ExecToolConfig{
		DefaultTimeout: 1,
		MaxTimeout:     2,
		DenyPatterns:   []string{"rm -rf /"},
		PathAppend:     execDir,
	}
	tool := NewExecTool(cfg, true)

	t.Run("path append", func(t *testing.T) {
		raw, _ := json.Marshal(map[string]any{"command": "showpath"})
		result, err := tool.Execute(context.Background(), raw, ToolContext{Workspace: workspace})
		if err != nil {
			t.Fatalf("Execute: %v", err)
		}
		if !strings.Contains(firstNonEmpty(result.Output, result.Content), execDir) {
			t.Fatalf("PATH missing appended dir: %q", firstNonEmpty(result.Output, result.Content))
		}
	})

	t.Run("deny list", func(t *testing.T) {
		raw, _ := json.Marshal(map[string]any{"command": "rm -rf /"})
		result, err := tool.Execute(context.Background(), raw, ToolContext{Workspace: workspace})
		if err != nil {
			t.Fatalf("Execute: %v", err)
		}
		if !strings.Contains(result.Error, "denied") {
			t.Fatalf("expected deny-list error, got %q", result.Error)
		}
	})

	t.Run("ssrf rejection", func(t *testing.T) {
		raw, _ := json.Marshal(map[string]any{"command": "curl http://169.254.169.254/latest/meta-data"})
		result, err := tool.Execute(context.Background(), raw, ToolContext{Workspace: workspace})
		if err != nil {
			t.Fatalf("Execute: %v", err)
		}
		if !strings.Contains(result.Error, "ssrf") {
			t.Fatalf("expected ssrf error, got %q", result.Error)
		}
	})

	t.Run("workspace restriction", func(t *testing.T) {
		raw, _ := json.Marshal(map[string]any{"command": "cat /etc/passwd"})
		result, err := tool.Execute(context.Background(), raw, ToolContext{Workspace: workspace})
		if err != nil {
			t.Fatalf("Execute: %v", err)
		}
		if !strings.Contains(result.Error, "workspace") {
			t.Fatalf("expected workspace restriction error, got %q", result.Error)
		}
	})

	t.Run("timeout cap", func(t *testing.T) {
		raw, _ := json.Marshal(map[string]any{
			"command": "sleep 3",
			"timeout": 10,
		})
		result, err := tool.Execute(context.Background(), raw, ToolContext{Workspace: workspace})
		if err != nil {
			t.Fatalf("Execute: %v", err)
		}
		if !strings.Contains(result.Error, "timed out") {
			t.Fatalf("expected timeout error, got %q", result.Error)
		}
	})

	t.Run("head tail truncation", func(t *testing.T) {
		raw, _ := json.Marshal(map[string]any{
			"command": "python3 - <<'PY'\nprint('A'*11050)\nPY",
		})
		result, err := tool.Execute(context.Background(), raw, ToolContext{Workspace: workspace})
		if err != nil {
			t.Fatalf("Execute: %v", err)
		}
		output := firstNonEmpty(result.Output, result.Content)
		if !strings.Contains(output, "truncated") {
			t.Fatalf("expected truncation marker, got %q", output)
		}
		if !strings.HasPrefix(output, strings.Repeat("A", 100)) {
			t.Fatalf("missing head of output")
		}
		if !strings.HasSuffix(strings.TrimSpace(output), strings.Repeat("A", 100)) {
			t.Fatalf("missing tail of output")
		}
	})
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
