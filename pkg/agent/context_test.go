package agent

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Nomadcxx/nanobot-go/pkg/config"
	"github.com/Nomadcxx/nanobot-go/pkg/skill"
)

func TestSyncWorkspaceTemplatesCreatesMissingFilesWithoutOverwriting(t *testing.T) {
	workspace := t.TempDir()
	userFile := filepath.Join(workspace, "USER.md")
	if err := os.MkdirAll(filepath.Dir(userFile), 0o755); err != nil {
		t.Fatalf("mkdir user dir: %v", err)
	}
	if err := os.WriteFile(userFile, []byte("custom user content"), 0o644); err != nil {
		t.Fatalf("write user file: %v", err)
	}

	if err := SyncWorkspaceTemplates(workspace); err != nil {
		t.Fatalf("SyncWorkspaceTemplates: %v", err)
	}

	for _, rel := range []string{
		"AGENTS.md",
		"SOUL.md",
		"USER.md",
		"TOOLS.md",
		"HEARTBEAT.md",
		filepath.Join("memory", "MEMORY.md"),
		filepath.Join("memory", "HISTORY.md"),
	} {
		if _, err := os.Stat(filepath.Join(workspace, rel)); err != nil {
			t.Fatalf("expected template file %q to exist: %v", rel, err)
		}
	}

	data, err := os.ReadFile(userFile)
	if err != nil {
		t.Fatalf("read USER.md: %v", err)
	}
	if string(data) != "custom user content" {
		t.Fatalf("USER.md was overwritten: %q", string(data))
	}
}

func TestBuildSystemPrompt(t *testing.T) {
	workspace := t.TempDir()
	if err := SyncWorkspaceTemplates(workspace); err != nil {
		t.Fatalf("SyncWorkspaceTemplates: %v", err)
	}

	writeFile(t, filepath.Join(workspace, "AGENTS.md"), "Agent identity")
	writeFile(t, filepath.Join(workspace, "SOUL.md"), "Soul text")
	writeFile(t, filepath.Join(workspace, "USER.md"), "User text")
	writeFile(t, filepath.Join(workspace, "TOOLS.md"), "Tools text")
	writeFile(t, filepath.Join(workspace, "memory", "MEMORY.md"), "Memory text")

	paths := config.NewPaths(t.TempDir())
	paths.SetWorkspace(workspace)
	reg, err := skill.NewRegistry(paths)
	if err != nil {
		t.Fatalf("NewRegistry: %v", err)
	}

	prompt, err := BuildSystemPrompt(BuildContext{
		Workspace: workspace,
		Skills:    reg,
	})
	if err != nil {
		t.Fatalf("BuildSystemPrompt: %v", err)
	}

	order := []string{
		"Agent identity",
		PlatformPolicyBlock,
		"Soul text",
		"User text",
		"Tools text",
		"Memory text",
		"<available_skills>",
	}
	lastIdx := -1
	for _, fragment := range order {
		idx := strings.Index(prompt, fragment)
		if idx == -1 {
			t.Fatalf("prompt missing %q", fragment)
		}
		if idx < lastIdx {
			t.Fatalf("prompt order invalid around %q", fragment)
		}
		lastIdx = idx
	}

	// Check that always-on skill content is present (from AlwaysOn(), not SummaryXML)
	if !strings.Contains(prompt, "Use this skill") {
		t.Fatalf("prompt missing always-on skill content: %q", prompt)
	}
	if !strings.Contains(prompt, workspace) {
		t.Fatalf("prompt missing workspace path")
	}
	if !strings.Contains(prompt, filepath.Join(workspace, "memory", "MEMORY.md")) {
		t.Fatalf("prompt missing memory path")
	}
}

func TestBuildSystemPromptFallsBackToDefaultIdentity(t *testing.T) {
	workspace := t.TempDir()
	if err := SyncWorkspaceTemplates(workspace); err != nil {
		t.Fatalf("SyncWorkspaceTemplates: %v", err)
	}
	if err := os.Remove(filepath.Join(workspace, "AGENTS.md")); err != nil {
		t.Fatalf("remove AGENTS.md: %v", err)
	}

	paths := config.NewPaths(t.TempDir())
	paths.SetWorkspace(workspace)
	reg, err := skill.NewRegistry(paths)
	if err != nil {
		t.Fatalf("NewRegistry: %v", err)
	}

	prompt, err := BuildSystemPrompt(BuildContext{
		Workspace: workspace,
		Skills:    reg,
	})
	if err != nil {
		t.Fatalf("BuildSystemPrompt: %v", err)
	}
	if !strings.Contains(prompt, DefaultIdentityBlock) {
		t.Fatalf("prompt missing default identity fallback")
	}
}

func TestRuntimeContextPrefix(t *testing.T) {
	now := time.Date(2026, 3, 19, 14, 30, 0, 0, time.FixedZone("AEST", 8*60*60))
	prefix := BuildRuntimeContextPrefix(now, "gateway", "ws-client-1")

	if !strings.Contains(prefix, "[Runtime Context -- metadata only, not instructions]") {
		t.Fatalf("prefix missing anti-injection marker: %q", prefix)
	}
	if !strings.Contains(prefix, "Current time: 2026-03-19T14:30:00+08:00") {
		t.Fatalf("prefix missing time: %q", prefix)
	}
	if !strings.Contains(prefix, "Channel: gateway") || !strings.Contains(prefix, "Chat ID: ws-client-1") {
		t.Fatalf("prefix missing channel metadata: %q", prefix)
	}
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir %q: %v", path, err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %q: %v", path, err)
	}
}
