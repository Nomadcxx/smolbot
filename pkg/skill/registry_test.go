package skill

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/Nomadcxx/smolbot/pkg/config"
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
	if !strings.Contains(content, "GitHub") {
		t.Errorf("LoadContent should return skill content, got: %q", content)
	}

	// Test loading non-existent skill
	_, err = reg.LoadContent("non-existent")
	if err == nil {
		t.Error("LoadContent should return error for non-existent skill")
	}
}

func TestMemorySkillLoads(t *testing.T) {
	reg, err := NewBuiltinRegistry()
	if err != nil {
		t.Fatalf("NewBuiltinRegistry: %v", err)
	}

	skill, ok := reg.Get("memory")
	if !ok {
		t.Fatal("memory skill missing")
	}
	if skill.Source != "builtin" {
		t.Fatalf("memory skill source = %q, want builtin", skill.Source)
	}
	if strings.TrimSpace(skill.Content) == "" {
		t.Fatal("memory skill content is empty")
	}
}

func TestMemorySkillReferences(t *testing.T) {
	reg, err := NewBuiltinRegistry()
	if err != nil {
		t.Fatalf("NewBuiltinRegistry: %v", err)
	}

	for _, resource := range []string{
		"references/recall.md",
		"references/triggers.md",
		"references/harvest.md",
	} {
		if !reg.HasResource("memory", resource) {
			t.Fatalf("missing memory skill resource %q", resource)
		}
	}
}

func TestMemorySkillLoadContent(t *testing.T) {
	reg, err := NewBuiltinRegistry()
	if err != nil {
		t.Fatalf("NewBuiltinRegistry: %v", err)
	}

	content, err := reg.LoadContent("memory")
	if err != nil {
		t.Fatalf("LoadContent(memory): %v", err)
	}
	for _, phrase := range []string{"5-stage", "startup gate"} {
		if !strings.Contains(strings.ToLower(content), strings.ToLower(phrase)) {
			t.Fatalf("memory skill content missing %q", phrase)
		}
	}
}

func TestMemorySkillRequiresNode(t *testing.T) {
	reg, err := NewBuiltinRegistry()
	if err != nil {
		t.Fatalf("NewBuiltinRegistry: %v", err)
	}

	skill, ok := reg.Get("memory")
	if !ok {
		t.Fatal("memory skill missing")
	}
	if !slices.Contains(skill.Requires.Bins, "node") {
		t.Fatalf("memory skill bins = %#v, want node", skill.Requires.Bins)
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

func TestNewRegistryLoadsUserSkills(t *testing.T) {
	tmp := t.TempDir()
	skillsDir := filepath.Join(tmp, "skills")
	skillDir := filepath.Join(skillsDir, "my-user-skill")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	skillContent := `---
name: my-user-skill
description: A test skill
---
# My Skill`
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(skillContent), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	paths := config.NewPaths(tmp)
	reg, err := NewRegistry(paths)
	if err != nil {
		t.Fatalf("NewRegistry: %v", err)
	}

	if _, ok := reg.Get("my-user-skill"); !ok {
		t.Fatal("user skill 'my-user-skill' not loaded — K3: user skills dir checked against embedded FS")
	}
}

func TestHasResourceNonBuiltinDoesNotPanic(t *testing.T) {
	tmp := t.TempDir()
	skillFile := filepath.Join(tmp, "my-skill.xml")
	os.WriteFile(skillFile, []byte(`<?xml version="1.0"?><skill><name>my-skill</name><description>test</description></skill>`), 0o644)
	resourceFile := filepath.Join(tmp, "data.txt")
	os.WriteFile(resourceFile, []byte("resource"), 0o644)

	reg := &Registry{
		skills: map[string]*Skill{
			"my-skill": {
				Name:        "my-skill",
				Path:        skillFile,
				Source:      "user",
				Description: "test",
			},
		},
	}

	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("HasResource panicked: %v — C14: fs.Stat(nil, path) panics", r)
		}
	}()
	if !reg.HasResource("my-skill", "data.txt") {
		t.Fatal("HasResource returned false for existing resource")
	}
}
