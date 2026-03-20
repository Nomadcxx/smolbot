# Progressive Disclosure Skill System - Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Redesign nanobot-go's skill system to use progressive disclosure (metadata → content → resources), reducing baseline token usage by 80%+ while maintaining skill discoverability.

**Architecture:** Skills self-register via filesystem. Only metadata (name, description, availability) loaded at startup. Agent reads full SKILL.md on demand via existing file tools. Supports bundled resources (references/, scripts/, assets/) for complex skills.

**Tech Stack:** Go, YAML frontmatter parsing, embed.FS for builtin skills, fsnotify (Phase 2)

---

## Overview

This plan implements a progressive disclosure skill system based on patterns from OpenClaw, Python nanobot, and opencode. The key insight: load only metadata into context (~20 tokens/skill), let agent decide relevance, then load full content on demand (~200-1000 tokens).

**Current State:** All skill content loaded into system prompt immediately (~500 tokens/skill)
**Target State:** Only metadata loaded (~20 tokens/skill), 80%+ token savings

---

## Phase 1: Critical Bug Fix (Priority: CRITICAL)

### Task 1: Fix "unterminated YAML frontmatter" Error

**Context:** The CLI fails with this error when trying to chat. Root cause: `splitFrontmatter()` doesn't handle Windows line endings or edge cases.

**Files:**
- Modify: `pkg/skill/loader.go:95-105`

**Step 1: Write failing test**

Create `pkg/skill/loader_test.go` addition:
```go
func TestSplitFrontmatterHandlesWindowsLineEndings(t *testing.T) {
	input := "---\r\nname: test\r\n---\r\ncontent"
	meta, body, err := splitFrontmatter(input)
	if err != nil {
		t.Fatalf("Expected no error for Windows line endings, got: %v", err)
	}
	if meta != "name: test" {
		t.Errorf("Expected meta 'name: test', got: %q", meta)
	}
	if body != "content" {
		t.Errorf("Expected body 'content', got: %q", body)
	}
}
```

**Step 2: Run test to verify it fails**

```bash
cd ~/nanobot-go
go test ./pkg/skill/... -v -run TestSplitFrontmatterHandlesWindowsLineEndings
```
Expected: FAIL with "unterminated YAML frontmatter"

**Step 3: Implement fix**

Modify `pkg/skill/loader.go`:
```go
func splitFrontmatter(raw string) (string, string, error) {
	// Normalize line endings
	raw = strings.ReplaceAll(raw, "\r\n", "\n")
	
	if !strings.HasPrefix(raw, "---\n") {
		return "", "", fmt.Errorf("missing YAML frontmatter")
	}
	rest := strings.TrimPrefix(raw, "---\n")
	idx := strings.Index(rest, "\n---\n")
	if idx == -1 {
		// Also check for end-of-file delimiter
		if strings.HasSuffix(rest, "\n---") {
			return strings.TrimSuffix(rest, "\n---"), "", nil
		}
		return "", "", fmt.Errorf("unterminated YAML frontmatter")
	}
	return rest[:idx], rest[idx+5:], nil
}
```

**Step 4: Run test to verify it passes**

```bash
go test ./pkg/skill/... -v -run TestSplitFrontmatterHandlesWindowsLineEndings
```
Expected: PASS

**Step 5: Run full test suite**

```bash
go test ./pkg/skill/... -v
```
Expected: All tests pass

**Step 6: Commit**

```bash
git add pkg/skill/loader.go pkg/skill/loader_test.go
git commit -m "fix: handle Windows line endings in skill frontmatter"
```

---

## Phase 2: Progressive Disclosure Core (Priority: HIGH)

### Task 2: Modify SummaryXML to Output Metadata Only

**Context:** Currently outputs full skill content. Change to output only metadata.

**Files:**
- Modify: `pkg/skill/registry.go:65-95` (SummaryXML method)
- Modify: `pkg/skill/registry.go:15-25` (Registry struct if needed)

**Step 1: Write failing test**

Add to `pkg/skill/registry_test.go`:
```go
func TestSummaryXMLOutputsMetadataOnly(t *testing.T) {
	reg := &Registry{
		skills: map[string]*Skill{
			"test": {
				Name:        "test",
				Description: "Test description",
				Content:     "This full content should NOT appear",
				Available:   true,
			},
		},
	}
	
	xml := reg.SummaryXML()
	if strings.Contains(xml, "This full content should NOT appear") {
		t.Error("SummaryXML should not contain full skill content")
	}
	if !strings.Contains(xml, "Test description") {
		t.Error("SummaryXML should contain description")
	}
}
```

**Step 2: Run test to verify it fails**

```bash
go test ./pkg/skill/... -v -run TestSummaryXMLOutputsMetadataOnly
```
Expected: FAIL (content appears in output)

**Step 3: Implement metadata-only output**

Modify `SummaryXML()` in `pkg/skill/registry.go`:
```go
func (r *Registry) SummaryXML() string {
	type skillSummary struct {
		XMLName xml.Name `xml:"skill"`
		Name    string   `xml:"name,attr"`
		Status  string   `xml:"status,attr"`
		Reason  string   `xml:"reason,attr,omitempty"`
		Always  bool     `xml:"always,attr,omitempty"`
		// REMOVED: Text field that contained full content
	}
	// ... rest of method, don't include skill.Content
}
```

**Step 4: Run test to verify it passes**

```bash
go test ./pkg/skill/... -v -run TestSummaryXMLOutputsMetadataOnly
```
Expected: PASS

**Step 5: Update existing tests**

Fix any tests that expect content in SummaryXML output.

**Step 6: Commit**

```bash
git add pkg/skill/registry.go pkg/skill/registry_test.go
git commit -m "feat: output only metadata in SummaryXML for progressive disclosure"
```

---

### Task 3: Add LoadContent Method to Registry

**Context:** Need method for agent to load full skill content on demand.

**Files:**
- Modify: `pkg/skill/registry.go` (add LoadContent method)
- Modify: `pkg/skill/skill.go` or create content loading interface

**Step 1: Write failing test**

```go
func TestLoadContentReturnsFullSkillContent(t *testing.T) {
	// Setup registry with mock skill
	skill := &Skill{
		Name:    "test",
		Content: "Full skill content here",
		Path:    "/tmp/test/SKILL.md",
		Source:  "workspace",
	}
	reg := &Registry{skills: map[string]*Skill{"test": skill}}
	
	content, err := reg.LoadContent("test")
	if err != nil {
		t.Fatalf("LoadContent failed: %v", err)
	}
	if content != "Full skill content here" {
		t.Errorf("Expected 'Full skill content here', got: %q", content)
	}
}
```

**Step 2: Run test to verify it fails**

```bash
go test ./pkg/skill/... -v -run TestLoadContentReturnsFullSkillContent
```
Expected: FAIL (method doesn't exist)

**Step 3: Implement LoadContent**

Add to `pkg/skill/registry.go`:
```go
// LoadContent returns the full content of a skill by name
func (r *Registry) LoadContent(name string) (string, error) {
	skill, ok := r.skills[name]
	if !ok {
		return "", fmt.Errorf("skill not found: %s", name)
	}
	// If content is already loaded, return it
	if skill.Content != "" {
		return skill.Content, nil
	}
	// Otherwise read from disk (for lazy loading)
	data, err := os.ReadFile(skill.Path)
	if err != nil {
		return "", fmt.Errorf("read skill file: %w", err)
	}
	_, _, err = splitFrontmatter(string(data))
	if err != nil {
		return "", fmt.Errorf("parse skill: %w", err)
	}
	return string(data), nil
}
```

**Step 4: Run test to verify it passes**

```bash
go test ./pkg/skill/... -v -run TestLoadContentReturnsFullSkillContent
```
Expected: PASS

**Step 5: Commit**

```bash
git add pkg/skill/registry.go pkg/skill/registry_test.go
git commit -m "feat: add LoadContent method for on-demand skill loading"
```

---

### Task 4: Update System Prompt with Skill Loading Instructions

**Context:** Agent needs to know how to load skills and the "1% rule".

**Files:**
- Modify: `templates/SOUL.md` or `templates/AGENTS.md`
- Modify: `pkg/agent/context.go:BuildSystemPrompt()`

**Step 1: Update template**

Edit `templates/SOUL.md`, add section:

```markdown
## Available Skills

You have access to specialized skills listed in <available_skills>.

**The 1% Rule:** If there is even a 1% chance a skill might apply to what you're doing, ABSOLUTELY load it. Better to load an unnecessary skill than miss a relevant one.

**How to load a skill:**
- Read the file: `read_file(skills/{skill-name}/SKILL.md)`
- Follow the instructions in the skill

**Skill Resources:**
- `references/` - Additional docs, load if referenced
- `scripts/` - Executable helpers, run if instructed
- `assets/` - Templates, read if needed
```

**Step 2: Update workspace SOUL.md**

```bash
cp templates/SOUL.md ~/.nanobot-go/workspace/SOUL.md
```

**Step 3: Test system prompt generation**

```bash
cd ~/nanobot-go
go test ./pkg/agent/... -v -run TestBuildSystemPrompt
```
Expected: PASS (may need to update test expectations)

**Step 4: Commit**

```bash
git add templates/SOUL.md
git commit -m "docs: add skill loading instructions to SOUL template"
```

---

## Phase 3: User Skills Directory (Priority: HIGH)

### Task 5: Add User Skills Directory to Discovery

**Context:** Add `~/.nanobot-go/skills/` as highest-priority skill source.

**Files:**
- Modify: `pkg/config/paths.go` (add SkillsDir method)
- Modify: `pkg/skill/registry.go:NewRegistry()`

**Step 1: Write failing test**

```go
func TestNewRegistryLoadsUserSkills(t *testing.T) {
	// Create temp user skills dir
	tmpDir := t.TempDir()
	skillsDir := filepath.Join(tmpDir, "skills")
	os.MkdirAll(skillsDir, 0755)
	
	// Create a test skill
	skillDir := filepath.Join(skillsDir, "test-skill")
	os.MkdirAll(skillDir, 0755)
	os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("---\nname: test-skill\ndescription: Test\n---\ncontent"), 0644)
	
	// Test that registry loads it
	// ... implementation depends on how we inject paths
}
```

**Step 2: Add SkillsDir to Paths**

Modify `pkg/config/paths.go`:
```go
func (p *Paths) SkillsDir() string {
	return filepath.Join(p.root, "skills")
}
```

**Step 3: Modify NewRegistry to load user skills**

Update `pkg/skill/registry.go`:
```go
func NewRegistry(paths *config.Paths) (*Registry, error) {
	// Load builtin skills
	builtin, err := loadBuiltinSkills()
	if err != nil {
		return nil, err
	}
	
	reg := &Registry{skills: builtin}
	
	// Load user skills from ~/.nanobot-go/skills/
	userSkills, err := LoadDir(paths.SkillsDir())
	if err != nil {
		return nil, err
	}
	for _, skill := range userSkills {
		reg.skills[skill.Name] = skill // User skills override builtin
	}
	
	// Load workspace skills
	workspaceSkills, err := LoadDir(filepath.Join(paths.Workspace(), "skills"))
	if err != nil {
		return nil, err
	}
	for _, skill := range workspaceSkills {
		reg.skills[skill.Name] = skill // Workspace skills override user
	}
	
	return reg, nil
}
```

**Step 4: Create user skills directory on startup**

In `cmd/nanobot/main.go` or config initialization:
```go
func ensureUserSkillsDir(paths *config.Paths) error {
	skillsDir := paths.SkillsDir()
	if err := os.MkdirAll(skillsDir, 0755); err != nil {
		return fmt.Errorf("create skills dir: %w", err)
	}
	return nil
}
```

**Step 5: Test**

```bash
go test ./pkg/skill/... -v
```

**Step 6: Commit**

```bash
git add pkg/config/paths.go pkg/skill/registry.go pkg/skill/registry_test.go
git commit -m "feat: add user skills directory (~/.nanobot-go/skills/)"
```

---

## Phase 4: Integration & Testing (Priority: HIGH)

### Task 6: Integration Test - End-to-End Skill Loading

**Context:** Verify the full flow: metadata in context → agent loads skill → uses it.

**Files:**
- Create: `pkg/agent/skill_integration_test.go`

**Step 1: Write integration test**

```go
func TestAgentLoadsSkillOnDemand(t *testing.T) {
	// Setup test environment
	workspace := t.TempDir()
	
	// Create a test skill
	skillsDir := filepath.Join(workspace, "skills", "calculator")
	os.MkdirAll(skillsDir, 0755)
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
	os.WriteFile(filepath.Join(skillsDir, "SKILL.md"), []byte(skillContent), 0644)
	
	// Create registry
	paths := &config.Paths{ /* setup with workspace */ }
	reg, err := skill.NewRegistry(paths)
	if err != nil {
		t.Fatalf("NewRegistry: %v", err)
	}
	
	// Verify only metadata in SummaryXML
	summary := reg.SummaryXML()
	if !strings.Contains(summary, `name="calculator"`) {
		t.Error("Summary should contain calculator skill")
	}
	if strings.Contains(summary, "mathematical calculations") {
		t.Error("Summary should NOT contain full content")
	}
	
	// Verify full content available via LoadContent
	content, err := reg.LoadContent("calculator")
	if err != nil {
		t.Fatalf("LoadContent: %v", err)
	}
	if !strings.Contains(content, "Calculator Skill") {
		t.Error("LoadContent should return full content")
	}
}
```

**Step 2: Run test**

```bash
go test ./pkg/agent/... -v -run TestAgentLoadsSkillOnDemand
```
Expected: PASS

**Step 3: Commit**

```bash
git add pkg/agent/skill_integration_test.go
git commit -m "test: add integration test for progressive skill loading"
```

---

### Task 7: Update Documentation

**Context:** Document the new skill system for users.

**Files:**
- Create: `docs/skills/creating-skills.md`
- Update: `README.md` (skill section)

**Step 1: Create skill creation guide**

Create `docs/skills/creating-skills.md`:
```markdown
# Creating Skills

## Quick Start

Create a new skill:

```bash
mkdir ~/.nanobot-go/skills/my-skill
cat > ~/.nanobot-go/skills/my-skill/SKILL.md << 'EOF'
---
name: my-skill
description: Use when [specific trigger conditions]
---

# My Skill

## Overview
What this skill does.

## When to Use
- Condition 1
- Condition 2

## When NOT to Use
- Exclusion 1

## How to Use
Instructions here.
EOF
```

## Description Format

The description field is critical. It MUST:
1. Start with "Use when"
2. Describe triggering conditions, not what the skill does
3. Be under 500 characters

**Good:** `Use when checking weather forecasts or current conditions for a location`

**Bad:** `This skill provides weather information using web search`

## Directory Structure

Simple skill:
```
my-skill/
└── SKILL.md
```

Complex skill with resources:
```
my-skill/
├── SKILL.md
├── references/
│   └── detailed-guide.md
└── scripts/
    └── helper.sh
```

## Frontmatter Fields

- `name`: lowercase letters, numbers, hyphens (max 64 chars)
- `description`: Trigger conditions starting with "Use when"
- `requires.bins`: Array of required CLI binaries
- `requires.env`: Array of required environment variables
- `always`: Boolean - load into every context

## Testing Your Skill

1. Restart nanobot: `systemctl restart nanobot-go`
2. Check available skills: `nanobot status`
3. Test with a query that should trigger it
```

**Step 2: Update README**

Add section about skills to README.md.

**Step 3: Commit**

```bash
git add docs/skills/creating-skills.md README.md
git commit -m "docs: add skill creation guide"
```

---

## Phase 5: Verification & Release (Priority: HIGH)

### Task 8: Full Test Suite Run

**Context:** Ensure all changes work together.

**Step 1: Run all tests**

```bash
cd ~/nanobot-go
go test ./... -count=1
```
Expected: ALL PASS

**Step 2: Build binaries**

```bash
go build -o ~/.local/bin/nanobot ./cmd/nanobot
go build -o ~/.local/bin/nanobot-tui ./cmd/nanobot-tui
```

**Step 3: Restart daemon**

```bash
sudo systemctl restart nanobot-go
systemctl status nanobot-go
```

**Step 4: Test CLI chat**

```bash
nanobot chat -m "What is 2+2?"
```
Expected: Works without "unterminated YAML frontmatter" error

**Step 5: Commit any fixes**

```bash
git add .
git commit -m "fix: address test failures from progressive disclosure changes"
```

---

## Phase 6: Token Savings Measurement (Priority: MEDIUM)

### Task 9: Measure Token Savings

**Context:** Quantify the improvement.

**Step 1: Create token counting script**

Create `scripts/measure_tokens.go`:
```go
package main

import (
	"fmt"
	"os"
	"path/filepath"
	
	"github.com/Nomadcxx/nanobot-go/pkg/agent"
	"github.com/Nomadcxx/nanobot-go/pkg/config"
	"github.com/Nomadcxx/nanobot-go/pkg/skill"
)

func main() {
	workspace := os.Getenv("HOME") + "/.nanobot-go/workspace"
	paths := config.DefaultPaths()
	
	reg, err := skill.NewRegistry(paths)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}
	
	ctx := agent.BuildContext{
		Workspace: workspace,
		Skills:    reg,
	}
	
	prompt, err := agent.BuildSystemPrompt(ctx)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}
	
	fmt.Printf("Total prompt size: %d characters\n", len(prompt))
	fmt.Printf("Approximate tokens: %d\n", len(prompt)/4) // Rough estimate
	
	// Count skills section specifically
	summary := reg.SummaryXML()
	fmt.Printf("Skills metadata: %d characters\n", len(summary))
	fmt.Printf("Skills metadata tokens: %d\n", len(summary)/4)
}
```

**Step 2: Run measurement**

```bash
go run scripts/measure_tokens.go
```

**Step 3: Compare with old system**

Document the savings:
- Before: ~500 tokens/skill × 8 skills = 4000 tokens
- After: ~20 tokens/skill × 8 skills = 160 tokens
- Savings: ~96%

**Step 4: Commit**

```bash
git add scripts/measure_tokens.go
git commit -m "chore: add token measurement script"
```

---

## Summary

This implementation plan:

1. **Fixes the critical bug** blocking CLI usage
2. **Implements progressive disclosure** (metadata → content)
3. **Adds user skills directory** for persistence
4. **Provides clear documentation** for skill creation
5. **Measures token savings** to verify improvement

**Estimated Timeline:**
- Phase 1: 1 hour
- Phase 2: 3 hours
- Phase 3: 2 hours
- Phase 4: 2 hours
- Phase 5: 1 hour
- Phase 6: 30 minutes

**Total: ~1.5 days of focused work**

**Key Success Metrics:**
1. CLI works without frontmatter errors
2. Token usage reduced 80%+
3. Agent can still discover and use skills
4. All tests pass
