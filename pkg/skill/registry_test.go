package skill

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
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

	reg, err := NewRegistry(workspace)
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
	reg, err := NewRegistry(workspace)
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
