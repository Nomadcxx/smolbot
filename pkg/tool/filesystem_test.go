package tool

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFilesystemTools(t *testing.T) {
	workspace := t.TempDir()
	extraDir := t.TempDir()

	reader := NewReadFileTool(true)
	writer := NewWriteFileTool(true)
	editor := NewEditFileTool(true)
	lister := NewListDirTool(true)
	tctx := ToolContext{Workspace: workspace}

	t.Run("read file pagination and line numbers", func(t *testing.T) {
		path := filepath.Join(workspace, "notes.txt")
		if err := os.WriteFile(path, []byte("one\ntwo\nthree\nfour\n"), 0o644); err != nil {
			t.Fatalf("write file: %v", err)
		}

		raw, _ := json.Marshal(map[string]any{
			"path":   path,
			"offset": 1,
			"limit":  2,
		})
		result, err := reader.Execute(context.Background(), raw, tctx)
		if err != nil {
			t.Fatalf("Execute: %v", err)
		}
		output := firstNonEmpty(result.Output, result.Content)
		if strings.Contains(output, "1: one") {
			t.Fatalf("unexpected first line in paginated output: %q", output)
		}
		if !strings.Contains(output, "2: two") || !strings.Contains(output, "3: three") {
			t.Fatalf("missing line numbers in output: %q", output)
		}
	})

	t.Run("read file size cap after pagination", func(t *testing.T) {
		path := filepath.Join(workspace, "large.txt")
		var builder strings.Builder
		for builder.Len() < 130500 {
			builder.WriteString("abcdefghij")
		}
		if err := os.WriteFile(path, []byte(builder.String()), 0o644); err != nil {
			t.Fatalf("write file: %v", err)
		}

		raw, _ := json.Marshal(map[string]any{"path": path})
		result, err := reader.Execute(context.Background(), raw, tctx)
		if err != nil {
			t.Fatalf("Execute: %v", err)
		}
		output := firstNonEmpty(result.Output, result.Content)
		if len(output) > 128200 {
			t.Fatalf("expected capped output, got %d bytes", len(output))
		}
	})

	t.Run("read file extra allowed dirs", func(t *testing.T) {
		path := filepath.Join(extraDir, "skill.md")
		if err := os.WriteFile(path, []byte("external skill"), 0o644); err != nil {
			t.Fatalf("write extra file: %v", err)
		}

		raw, _ := json.Marshal(map[string]any{
			"path":             path,
			"extraAllowedDirs": []string{extraDir},
		})
		result, err := reader.Execute(context.Background(), raw, tctx)
		if err != nil {
			t.Fatalf("Execute: %v", err)
		}
		if !strings.Contains(firstNonEmpty(result.Output, result.Content), "1: external skill") {
			t.Fatalf("expected read from extra allowed dir, got %#v", result)
		}
	})

	t.Run("write file creates parent directories", func(t *testing.T) {
		path := filepath.Join(workspace, "nested", "child.txt")
		raw, _ := json.Marshal(map[string]any{
			"path":    path,
			"content": "created",
		})
		result, err := writer.Execute(context.Background(), raw, tctx)
		if err != nil {
			t.Fatalf("Execute: %v", err)
		}
		if result.Error != "" {
			t.Fatalf("unexpected write error: %q", result.Error)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read written file: %v", err)
		}
		if string(data) != "created" {
			t.Fatalf("unexpected file content %q", string(data))
		}
	})

	t.Run("edit file exact replacement", func(t *testing.T) {
		path := filepath.Join(workspace, "edit.txt")
		if err := os.WriteFile(path, []byte("hello world"), 0o644); err != nil {
			t.Fatalf("write file: %v", err)
		}

		raw, _ := json.Marshal(map[string]any{
			"path":       path,
			"old_string": "world",
			"new_string": "nanobot",
		})
		result, err := editor.Execute(context.Background(), raw, tctx)
		if err != nil {
			t.Fatalf("Execute: %v", err)
		}
		if result.Error != "" {
			t.Fatalf("unexpected edit error: %q", result.Error)
		}
		data, _ := os.ReadFile(path)
		if string(data) != "hello nanobot" {
			t.Fatalf("unexpected edit content %q", string(data))
		}
	})

	t.Run("edit file warns on multiple matches", func(t *testing.T) {
		path := filepath.Join(workspace, "dup.txt")
		if err := os.WriteFile(path, []byte("dup\ndup\n"), 0o644); err != nil {
			t.Fatalf("write file: %v", err)
		}

		raw, _ := json.Marshal(map[string]any{
			"path":        path,
			"old_string":  "dup",
			"new_string":  "only-once",
			"replace_all": false,
		})
		result, err := editor.Execute(context.Background(), raw, tctx)
		if err != nil {
			t.Fatalf("Execute: %v", err)
		}
		if !strings.Contains(result.Error, "multiple matches") {
			t.Fatalf("expected multiple match warning, got %#v", result)
		}
		data, _ := os.ReadFile(path)
		if string(data) != "dup\ndup\n" {
			t.Fatalf("file should not change on warning, got %q", string(data))
		}
	})

	t.Run("edit file preserves crlf", func(t *testing.T) {
		path := filepath.Join(workspace, "crlf.txt")
		if err := os.WriteFile(path, []byte("one\r\ntwo\r\n"), 0o644); err != nil {
			t.Fatalf("write file: %v", err)
		}

		raw, _ := json.Marshal(map[string]any{
			"path":       path,
			"old_string": "two",
			"new_string": "done",
		})
		result, err := editor.Execute(context.Background(), raw, tctx)
		if err != nil {
			t.Fatalf("Execute: %v", err)
		}
		if result.Error != "" {
			t.Fatalf("unexpected edit error: %q", result.Error)
		}
		data, _ := os.ReadFile(path)
		if !strings.Contains(string(data), "\r\n") {
			t.Fatalf("expected CRLF preservation, got %q", string(data))
		}
		if string(data) != "one\r\ndone\r\n" {
			t.Fatalf("unexpected CRLF edit result %q", string(data))
		}
	})

	t.Run("edit file fuzzy match fallback", func(t *testing.T) {
		path := filepath.Join(workspace, "fuzzy.txt")
		if err := os.WriteFile(path, []byte("alpha\n  beta\ncharlie\n"), 0o644); err != nil {
			t.Fatalf("write file: %v", err)
		}

		raw, _ := json.Marshal(map[string]any{
			"path":       path,
			"old_string": "alpha\nbeta\ncharlie",
			"new_string": "alpha\nbeta2\ncharlie",
		})
		result, err := editor.Execute(context.Background(), raw, tctx)
		if err != nil {
			t.Fatalf("Execute: %v", err)
		}
		if result.Error != "" {
			t.Fatalf("unexpected fuzzy edit error: %q", result.Error)
		}
		data, _ := os.ReadFile(path)
		if !strings.Contains(string(data), "beta2") {
			t.Fatalf("expected fuzzy replacement, got %q", string(data))
		}
	})

	t.Run("list dir shows size and type", func(t *testing.T) {
		if err := os.WriteFile(filepath.Join(workspace, "root.txt"), []byte("root"), 0o644); err != nil {
			t.Fatalf("write root file: %v", err)
		}
		if err := os.MkdirAll(filepath.Join(workspace, "subdir"), 0o755); err != nil {
			t.Fatalf("mkdir subdir: %v", err)
		}

		raw, _ := json.Marshal(map[string]any{"path": workspace})
		result, err := lister.Execute(context.Background(), raw, tctx)
		if err != nil {
			t.Fatalf("Execute: %v", err)
		}
		output := firstNonEmpty(result.Output, result.Content)
		if !strings.Contains(output, "root.txt [file, 4 B]") {
			t.Fatalf("missing file entry: %q", output)
		}
		if !strings.Contains(output, "subdir [dir") {
			t.Fatalf("missing dir entry: %q", output)
		}
	})

	t.Run("list dir recursive depth limiting and ignores", func(t *testing.T) {
		deepDir := filepath.Join(workspace, "deep", "level2")
		if err := os.MkdirAll(deepDir, 0o755); err != nil {
			t.Fatalf("mkdir deep dir: %v", err)
		}
		if err := os.WriteFile(filepath.Join(deepDir, "inside.txt"), []byte("inside"), 0o644); err != nil {
			t.Fatalf("write deep file: %v", err)
		}
		if err := os.MkdirAll(filepath.Join(workspace, "node_modules", "pkg"), 0o755); err != nil {
			t.Fatalf("mkdir ignored dir: %v", err)
		}
		if err := os.WriteFile(filepath.Join(workspace, "node_modules", "pkg", "skip.txt"), []byte("skip"), 0o644); err != nil {
			t.Fatalf("write ignored file: %v", err)
		}

		raw, _ := json.Marshal(map[string]any{
			"path":      filepath.Join(workspace, "deep"),
			"recursive": true,
			"max_depth": 1,
		})
		result, err := lister.Execute(context.Background(), raw, tctx)
		if err != nil {
			t.Fatalf("Execute: %v", err)
		}
		output := firstNonEmpty(result.Output, result.Content)
		if !strings.Contains(output, "level2 [dir") {
			t.Fatalf("expected level2 listing, got %q", output)
		}
		if strings.Contains(output, "inside.txt") {
			t.Fatalf("expected depth cap to hide nested file, got %q", output)
		}

		raw, _ = json.Marshal(map[string]any{
			"path":      workspace,
			"recursive": true,
			"max_depth": 3,
		})
		result, err = lister.Execute(context.Background(), raw, tctx)
		if err != nil {
			t.Fatalf("Execute: %v", err)
		}
		output = firstNonEmpty(result.Output, result.Content)
		if strings.Contains(output, "node_modules") || strings.Contains(output, "skip.txt") {
			t.Fatalf("expected ignored directories to be omitted, got %q", output)
		}
	})
}
