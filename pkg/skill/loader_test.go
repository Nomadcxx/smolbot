package skill

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadFileParsesFrontmatterAndRequirements(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "SKILL.md")
	content := `---
name: weather
description: Check weather forecasts
requires:
  bins: [sh]
  env: [WEATHER_API_KEY]
always: true
---
skill content
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write skill file: %v", err)
	}

	t.Setenv("WEATHER_API_KEY", "test-key")
	skill, err := LoadFile(path)
	if err != nil {
		t.Fatalf("LoadFile: %v", err)
	}
	if skill.Name != "weather" || skill.Description != "Check weather forecasts" {
		t.Fatalf("loaded skill = %+v", skill)
	}
	if !skill.Always {
		t.Fatalf("expected always=true")
	}
	if !skill.Available {
		t.Fatalf("expected skill to be available")
	}
}

func TestLoadFileUnavailableForMissingRequirements(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "SKILL.md")
	content := `---
name: github
description: GitHub operations
requires:
  bins: [definitely_missing_binary_123]
  env: [DEFINITELY_MISSING_ENV_123]
---
skill content
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write skill file: %v", err)
	}

	skill, err := LoadFile(path)
	if err != nil {
		t.Fatalf("LoadFile: %v", err)
	}
	if skill.Available {
		t.Fatalf("skill should be unavailable")
	}
	if !strings.Contains(skill.UnavailableReason, "missing") {
		t.Fatalf("unavailable reason = %q", skill.UnavailableReason)
	}
}
