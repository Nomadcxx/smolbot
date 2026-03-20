package agent

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Nomadcxx/nanobot-go/pkg/config"
	"github.com/Nomadcxx/nanobot-go/pkg/skill"
)

func TestAgentLoadsSkillOnDemand(t *testing.T) {
	// Setup test environment
	workspace := t.TempDir()

	// Create a test skill
	skillsDir := filepath.Join(workspace, "skills", "calculator")
	if err := os.MkdirAll(skillsDir, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	skillContent := `---
name: calculator
description: Use when performing mathematical calculations
---

# Calculator Skill

## When to Use
When user asks for calculations.

## How to Use
Simply provide the answer directly.
`
	skillPath := filepath.Join(skillsDir, "SKILL.md")
	if err := os.WriteFile(skillPath, []byte(skillContent), 0644); err != nil {
		t.Fatalf("write skill: %v", err)
	}

	// Create paths
	paths := config.NewPaths(workspace)
	paths.SetWorkspace(workspace)

	// Create registry
	reg, err := skill.NewRegistry(paths)
	if err != nil {
		t.Fatalf("NewRegistry: %v", err)
	}

	// Verify only metadata in SummaryXML
	summary := reg.SummaryXML()
	if !strings.Contains(summary, `name="calculator"`) {
		t.Error("Summary should contain calculator skill")
	}
	if strings.Contains(summary, "Calculator Skill") {
		t.Error("Summary should NOT contain full content")
	}
	if strings.Contains(summary, "mathematical calculations") {
		t.Error("Summary should NOT contain description in content section")
	}

	// Verify full content available via LoadContent
	content, err := reg.LoadContent("calculator")
	if err != nil {
		t.Fatalf("LoadContent: %v", err)
	}
	if !strings.Contains(content, "Calculator Skill") {
		t.Error("LoadContent should return full content")
	}
	if !strings.Contains(content, "## When to Use") {
		t.Error("LoadContent should contain the content section (after frontmatter)")
	}
}

func TestUserSkillsOverrideBuiltin(t *testing.T) {
	// Setup test environment
	workspace := t.TempDir()

	// Create a user skill that overrides a builtin
	userSkillsDir := filepath.Join(workspace, "skills", "test-skill")
	if err := os.MkdirAll(userSkillsDir, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	userSkillContent := `---
name: test-skill
description: User version of test skill
---

# User Test Skill

This is the user version.
`
	if err := os.WriteFile(filepath.Join(userSkillsDir, "SKILL.md"), []byte(userSkillContent), 0644); err != nil {
		t.Fatalf("write user skill: %v", err)
	}

	// Create paths
	paths := config.NewPaths(workspace)
	paths.SetWorkspace(workspace)

	// Create registry
	reg, err := skill.NewRegistry(paths)
	if err != nil {
		t.Fatalf("NewRegistry: %v", err)
	}

	// Load content and verify it's the user version
	content, err := reg.LoadContent("test-skill")
	if err != nil {
		t.Fatalf("LoadContent: %v", err)
	}
	if !strings.Contains(content, "User Test Skill") {
		t.Error("Should load user version of skill")
	}
}

func TestRegistryLoadsWorkspaceSkills(t *testing.T) {
	// Setup test environment
	workspace := t.TempDir()

	// Create a workspace skill
	workspaceSkillsDir := filepath.Join(workspace, "skills", "workspace-skill")
	if err := os.MkdirAll(workspaceSkillsDir, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	workspaceSkillContent := `---
name: workspace-skill
description: A workspace-specific skill
---

# Workspace Skill

This skill is from the workspace.
`
	if err := os.WriteFile(filepath.Join(workspaceSkillsDir, "SKILL.md"), []byte(workspaceSkillContent), 0644); err != nil {
		t.Fatalf("write workspace skill: %v", err)
	}

	// Create paths
	paths := config.NewPaths(workspace)
	paths.SetWorkspace(workspace)

	// Create registry
	reg, err := skill.NewRegistry(paths)
	if err != nil {
		t.Fatalf("NewRegistry: %v", err)
	}

	// Verify workspace skill is loaded
	content, err := reg.LoadContent("workspace-skill")
	if err != nil {
		t.Fatalf("LoadContent: %v", err)
	}
	if !strings.Contains(content, "Workspace Skill") {
		t.Error("Should load workspace skill")
	}
}
