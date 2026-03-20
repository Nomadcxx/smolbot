package skill

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/Nomadcxx/nanobot-go/pkg/config"
)

func TestBuiltinSkillNames(t *testing.T) {
	reg, err := NewBuiltinRegistry()
	if err != nil {
		t.Fatalf("NewBuiltinRegistry: %v", err)
	}

	names := reg.Names()
	for _, want := range []string{
		"github", "cron", "weather", "skill-creator",
		"tmux", "summarize", "clawhub", "memory",
	} {
		if !slices.Contains(names, want) {
			t.Fatalf("missing builtin skill %q", want)
		}
	}
}

func TestRegistryWorkspaceOverrideAndAlwaysOn(t *testing.T) {
	workspace := t.TempDir()
	paths := config.NewPaths(t.TempDir())
	paths.SetWorkspace(workspace)

	overrideDir := filepath.Join(workspace, "skills", "github")
	if err := os.MkdirAll(overrideDir, 0o755); err != nil {
		t.Fatalf("mkdir override dir: %v", err)
	}
	overridePath := filepath.Join(overrideDir, "SKILL.md")
	override := `---
name: github
description: Workspace override
always: true
---
workspace github skill
`
	if err := os.WriteFile(overridePath, []byte(override), 0o644); err != nil {
		t.Fatalf("write override skill: %v", err)
	}

	reg, err := NewRegistry(paths)
	if err != nil {
		t.Fatalf("NewRegistry: %v", err)
	}

	skill, ok := reg.Get("github")
	if !ok {
		t.Fatalf("github skill missing")
	}
	if skill.Description != "Workspace override" {
		t.Fatalf("description = %q, want workspace override", skill.Description)
	}
	if !strings.Contains(skill.Path, overridePath) {
		t.Fatalf("path = %q, want workspace path", skill.Path)
	}

	alwaysOn := reg.AlwaysOn()
	if len(alwaysOn) == 0 || !strings.Contains(alwaysOn[0].Content, "workspace github skill") {
		t.Fatalf("always-on skill content missing override")
	}
}

func TestRegistrySummaryXML(t *testing.T) {
	workspace := t.TempDir()
	paths := config.NewPaths(t.TempDir())
	paths.SetWorkspace(workspace)

	reg, err := NewRegistry(paths)
	if err != nil {
		t.Fatalf("NewRegistry: %v", err)
	}

	summary := reg.SummaryXML()
	if !strings.Contains(summary, "<available_skills>") {
		t.Fatalf("summary missing wrapper: %q", summary)
	}
	if !strings.Contains(summary, `name="github"`) {
		t.Fatalf("summary missing github entry: %q", summary)
	}
}

func TestRegistrySummaryXMLMetadataOnly(t *testing.T) {
	workspace := t.TempDir()
	paths := config.NewPaths(t.TempDir())
	paths.SetWorkspace(workspace)

	reg, err := NewRegistry(paths)
	if err != nil {
		t.Fatalf("NewRegistry: %v", err)
	}

	summary := reg.SummaryXML()

	// Should NOT contain full skill content
	if strings.Contains(summary, "Use this skill for") {
		t.Error("SummaryXML should not contain full skill content, only metadata")
	}

	// Should contain description (metadata)
	if !strings.Contains(summary, `name="github"`) {
		t.Error("SummaryXML should contain skill names")
	}
}

func TestRegistryLoadContent(t *testing.T) {
	reg, err := NewBuiltinRegistry()
	if err != nil {
		t.Fatalf("NewBuiltinRegistry: %v", err)
	}

	// Test loading existing skill
	content, err := reg.LoadContent("github")
	if err != nil {
		t.Fatalf("LoadContent: %v", err)
	}
	// Content should be just the body (without frontmatter)
	if !strings.Contains(content, "GitHub-oriented") {
		t.Errorf("LoadContent should return skill content, got: %q", content)
	}

	// Test loading non-existent skill
	_, err = reg.LoadContent("non-existent")
	if err == nil {
		t.Error("LoadContent should return error for non-existent skill")
	}
}

func TestRegistryThreadSafety(t *testing.T) {
	workspace := t.TempDir()
	paths := config.NewPaths(t.TempDir())
	paths.SetWorkspace(workspace)

	reg, err := NewRegistry(paths)
	if err != nil {
		t.Fatalf("NewRegistry: %v", err)
	}

	// Concurrent access test
	done := make(chan bool, 3)

	go func() {
		_ = reg.Names()
		done <- true
	}()

	go func() {
		_ = reg.SummaryXML()
		done <- true
	}()

	go func() {
		_, _ = reg.LoadContent("github")
		done <- true
	}()

	for i := 0; i < 3; i++ {
		select {
		case <-done:
		case <-time.After(time.Second):
			t.Fatal("Timeout waiting for concurrent operations")
		}
	}
}
