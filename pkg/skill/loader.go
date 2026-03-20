package skill

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

type Skill struct {
	Name              string
	Description       string
	Requires          Requires
	Always            bool
	Content           string
	Path              string
	Source            string
	Available         bool
	UnavailableReason string
}

type Requires struct {
	Bins []string `yaml:"bins"`
	Env  []string `yaml:"env"`
}

type frontmatter struct {
	Name        string   `yaml:"name"`
	Description string   `yaml:"description"`
	Requires    Requires `yaml:"requires"`
	Always      bool     `yaml:"always"`
}

func LoadFile(path string) (*Skill, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read skill file: %w", err)
	}
	return parseSkill(string(data), path, "workspace")
}

func LoadDir(root string) ([]*Skill, error) {
	entries, err := os.ReadDir(root)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read skill directory: %w", err)
	}

	skills := make([]*Skill, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		skillPath := filepath.Join(root, entry.Name(), "SKILL.md")
		skill, err := LoadFile(skillPath)
		if err != nil {
			return nil, err
		}
		skills = append(skills, skill)
	}
	return skills, nil
}

func parseSkill(raw, path, source string) (*Skill, error) {
	meta, body, err := splitFrontmatter(raw)
	if err != nil {
		return nil, err
	}

	var fm frontmatter
	if err := yaml.Unmarshal([]byte(meta), &fm); err != nil {
		return nil, fmt.Errorf("parse frontmatter: %w", err)
	}

	skill := &Skill{
		Name:        fm.Name,
		Description: fm.Description,
		Requires:    fm.Requires,
		Always:      fm.Always,
		Content:     strings.TrimSpace(body),
		Path:        path,
		Source:      source,
		Available:   true,
	}
	skill.checkAvailability()
	return skill, nil
}

func splitFrontmatter(raw string) (string, string, error) {
	// Normalize line endings to handle Windows-style line endings
	raw = strings.ReplaceAll(raw, "\r\n", "\n")
	
	if !strings.HasPrefix(raw, "---\n") {
		return "", "", fmt.Errorf("missing YAML frontmatter: file must start with '---'")
	}
	rest := strings.TrimPrefix(raw, "---\n")
	idx := strings.Index(rest, "\n---\n")
	if idx == -1 {
		// Check if file ends with just "\n---" (no trailing newline)
		if strings.HasSuffix(rest, "\n---") {
			return strings.TrimSuffix(rest, "\n---"), "", nil
		}
		return "", "", fmt.Errorf("unterminated YAML frontmatter: missing closing '---'")
	}
	return rest[:idx], rest[idx+5:], nil
}

func (s *Skill) checkAvailability() {
	for _, bin := range s.Requires.Bins {
		if _, err := exec.LookPath(bin); err != nil {
			s.Available = false
			s.UnavailableReason = "missing bin: " + bin
			return
		}
	}
	for _, env := range s.Requires.Env {
		if os.Getenv(env) == "" {
			s.Available = false
			s.UnavailableReason = "missing env: " + env
			return
		}
	}
}
