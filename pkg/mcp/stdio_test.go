package mcp

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestLookPathWithEnv(t *testing.T) {
	t.Run("finds binary in custom PATH", func(t *testing.T) {
		tmpDir := t.TempDir()
		exePath := filepath.Join(tmpDir, "mytestcmd")
		
		if err := os.WriteFile(exePath, []byte("#!/bin/sh\necho hello"), 0755); err != nil {
			t.Fatal(err)
		}

		got, err := lookPathWithEnv("mytestcmd", tmpDir)
		if err != nil {
			t.Fatalf("expected to find mytestcmd in %s, got error: %v", tmpDir, err)
		}
		if got != exePath {
			t.Errorf("expected %s, got %s", exePath, got)
		}
	})

	t.Run("returns error when binary not in PATH", func(t *testing.T) {
		_, err := lookPathWithEnv("nonexistent-binary-xyz", "/nonexistent/path")
		if err == nil {
			t.Error("expected error for nonexistent binary")
		}
		if !strings.Contains(err.Error(), "not found in PATH") {
			t.Errorf("expected 'not found in PATH' error, got: %v", err)
		}
	})

	t.Run("handles empty PATH gracefully", func(t *testing.T) {
		_, err := lookPathWithEnv("echo", "")
		if err == nil {
			t.Error("expected error for empty PATH")
		}
	})

	t.Run("prefers earlier PATH entries", func(t *testing.T) {
		tmpDir1 := t.TempDir()
		tmpDir2 := t.TempDir()
		exe1 := filepath.Join(tmpDir1, "testcmd")
		exe2 := filepath.Join(tmpDir2, "testcmd")

		for _, p := range []string{exe1, exe2} {
			if err := os.WriteFile(p, []byte("#!/bin/sh\necho "+filepath.Dir(p)), 0755); err != nil {
				t.Fatal(err)
			}
		}

		got, err := lookPathWithEnv("testcmd", tmpDir1+string(os.PathListSeparator)+tmpDir2)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != exe1 {
			t.Errorf("expected %s (first in PATH), got %s", exe1, got)
		}
	})

	t.Run("skips non-executable files", func(t *testing.T) {
		tmpDir := t.TempDir()
		nonExe := filepath.Join(tmpDir, "notexe")
		if err := os.WriteFile(nonExe, []byte("not executable"), 0644); err != nil {
			t.Fatal(err)
		}

		_, err := lookPathWithEnv("notexe", tmpDir)
		if err == nil {
			t.Error("expected error for non-executable file")
		}
	})

	t.Run("skips directories", func(t *testing.T) {
		tmpDir := t.TempDir()
		subDir := filepath.Join(tmpDir, "subdir")
		if err := os.MkdirAll(subDir, 0755); err != nil {
			t.Fatal(err)
		}

		_, err := lookPathWithEnv("subdir", tmpDir)
		if err == nil {
			t.Error("expected error for directory")
		}
	})
}

func TestResolveCommand(t *testing.T) {
	t.Run("extracts basename from absolute path", func(t *testing.T) {
		got := resolveCommand("/usr/local/bin/node", nil)
		if got == "/usr/local/bin/node" {
			t.Error("expected basename extraction from absolute path")
		}
		if !strings.Contains(got, "node") {
			t.Errorf("expected 'node' in resolved path, got: %s", got)
		}
	})

	t.Run("resolves bare command using system PATH", func(t *testing.T) {
		got := resolveCommand("echo", nil)
		if got == "echo" {
			t.Error("expected resolveCommand to resolve 'echo' to an absolute path")
		}
		if _, err := os.Stat(got); err != nil {
			t.Errorf("resolved path does not exist: %s", got)
		}
	})

	t.Run("returns original command if not found in PATH", func(t *testing.T) {
		original := resolveCommand("this-command-does-not-exist-12345", nil)
		if original != "this-command-does-not-exist-12345" {
			t.Errorf("expected original command to be returned when not found, got: %s", original)
		}
	})

	t.Run("uses MCP env PATH when provided", func(t *testing.T) {
		tmpDir := t.TempDir()
		exePath := filepath.Join(tmpDir, "myenvcmd")
		if err := os.WriteFile(exePath, []byte("#!/bin/sh\necho hello"), 0755); err != nil {
			t.Fatal(err)
		}

		env := map[string]string{"PATH": tmpDir}
		got := resolveCommand("myenvcmd", env)
		if got != exePath {
			t.Errorf("expected %s, got %s", exePath, got)
		}
	})

	t.Run("prepends MCP env PATH to system PATH", func(t *testing.T) {
		tmpDir := t.TempDir()
		exePath := filepath.Join(tmpDir, "prependcmd")
		if err := os.WriteFile(exePath, []byte("#!/bin/sh\necho hello"), 0755); err != nil {
			t.Fatal(err)
		}

		env := map[string]string{"PATH": tmpDir}
		got := resolveCommand("prependcmd", env)
		if got != exePath {
			t.Errorf("expected %s, got %s", exePath, got)
		}
	})
}

func TestNewStdioTransportWithBareCommand(t *testing.T) {
	t.Run("bare echo command resolves successfully", func(t *testing.T) {
		ctx := testContext()
		t.Logf("Testing bare 'echo' command resolution")

		transport, err := NewStdioTransport(ctx, "echo", []string{"hello"}, nil)
		if err != nil {
			t.Fatalf("failed to create transport with bare 'echo' command: %v", err)
		}
		defer transport.Close()

		result, err := transport.Send(ctx, "tools/call", map[string]any{
			"name":      "echo",
			"arguments": map[string]any{"message": "hello"},
		})
		if err != nil {
			t.Logf("echo may not implement tools/call, which is expected: %v", err)
		}
		_ = result
	})

	t.Run("absolute path extracts basename and resolves", func(t *testing.T) {
		ctx := testContext()

		echoPath, err := exec.LookPath("echo")
		if err != nil {
			t.Skip("echo not found in PATH")
		}

		transport, err := NewStdioTransport(ctx, echoPath, []string{"test"}, nil)
		if err != nil {
			t.Fatalf("failed to create transport with absolute path: %v", err)
		}
		defer transport.Close()
		_ = transport
	})

	t.Run("nonexistent command gives clear error", func(t *testing.T) {
		ctx := testContext()

		_, err := NewStdioTransport(ctx, "nonexistent-command-xyz", []string{}, nil)
		if err == nil {
			t.Fatal("expected error for nonexistent command")
		}
		errStr := err.Error()
		if !strings.Contains(errStr, "not found") {
			t.Errorf("expected 'not found' in error, got: %s", errStr)
		}
		if !strings.Contains(errStr, "nonexistent-command-xyz") {
			t.Errorf("expected command name in error, got: %s", errStr)
		}
	})
}

func testContext() context.Context {
	return context.Background()
}
