package skill

import (
	"encoding/xml"
	"fmt"
	"io/fs"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/Nomadcxx/smolbot/pkg/config"
	nanobotgo "github.com/Nomadcxx/smolbot"
)

// Registry manages loaded skills from builtin, user, and workspace sources.
type Registry struct {
	skills map[string]*Skill
	mu     sync.RWMutex
}

// NewBuiltinRegistry creates a registry with only builtin skills.
func NewBuiltinRegistry() (*Registry, error) {
	skills, err := loadBuiltinSkills()
	if err != nil {
		return nil, err
	}
	return &Registry{skills: skills}, nil
}

// NewRegistry creates a registry with builtin, user, and workspace skills.
// User skills (~/.nanobot-go/skills/) override builtin.
// Workspace skills override user skills.
func NewRegistry(paths *config.Paths) (*Registry, error) {
	// Load builtin skills
	builtin, err := loadBuiltinSkills()
	if err != nil {
		return nil, err
	}

	reg := &Registry{skills: builtin}

	// Load user skills from ~/.nanobot-go/skills/
	userSkillsDir := paths.SkillsDir()
	if _, err := fs.Stat(nanobotgo.EmbeddedAssets, userSkillsDir); err == nil {
		userSkills, err := LoadDir(userSkillsDir)
		if err != nil {
			return nil, fmt.Errorf("load user skills: %w", err)
		}
		for _, skill := range userSkills {
			reg.skills[skill.Name] = skill
		}
	}

	// Load workspace skills
	workspaceSkillsDir := filepath.Join(paths.Workspace(), "skills")
	workspaceSkills, err := LoadDir(workspaceSkillsDir)
	if err != nil {
		return nil, fmt.Errorf("load workspace skills: %w", err)
	}
	for _, skill := range workspaceSkills {
		reg.skills[skill.Name] = skill
	}

	return reg, nil
}

// Names returns all skill names in sorted order.
func (r *Registry) Names() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	names := make([]string, 0, len(r.skills))
	for name := range r.skills {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// Get retrieves a skill by name.
func (r *Registry) Get(name string) (*Skill, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	skill, ok := r.skills[name]
	return skill, ok
}

// AlwaysOn returns skills marked with always: true.
func (r *Registry) AlwaysOn() []*Skill {
	r.mu.RLock()
	defer r.mu.RUnlock()

	skills := make([]*Skill, 0)
	for _, name := range r.Names() {
		skill := r.skills[name]
		if skill.Always {
			skills = append(skills, skill)
		}
	}
	return skills
}

// LoadContent returns the full content of a skill by name.
// Note: This returns the already-loaded content. Skills are pre-loaded at startup.
func (r *Registry) LoadContent(name string) (string, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	skill, ok := r.skills[name]
	if !ok {
		return "", fmt.Errorf("skill not found: %s", name)
	}
	return skill.Content, nil
}

// HasResource checks if a skill has a resource file.
func (r *Registry) HasResource(skillName, resource string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	skill, ok := r.skills[skillName]
	if !ok {
		return false
	}

	// Check if resource exists in skill directory
	resourcePath := filepath.Join(filepath.Dir(skill.Path), resource)
	if skill.Source == "builtin" {
		_, err := fs.Stat(nanobotgo.EmbeddedAssets, resourcePath)
		return err == nil
	}

	_, err := fs.Stat(nil, resourcePath)
	return err == nil
}

// GetResourcePath returns the full path to a skill resource.
func (r *Registry) GetResourcePath(skillName, resource string) (string, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	skill, ok := r.skills[skillName]
	if !ok {
		return "", fmt.Errorf("skill not found: %s", skillName)
	}

	return filepath.Join(filepath.Dir(skill.Path), resource), nil
}

// SummaryXML returns XML summary of available skills for system prompt.
// Only includes metadata (name, description, status) - not full content.
func (r *Registry) SummaryXML() string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	type skillSummary struct {
		XMLName xml.Name `xml:"skill"`
		Name    string   `xml:"name,attr"`
		Status  string   `xml:"status,attr"`
		Reason  string   `xml:"reason,attr,omitempty"`
		Always  bool     `xml:"always,attr,omitempty"`
		// Note: Text field removed - only metadata included
	}
	type wrapper struct {
		XMLName xml.Name       `xml:"available_skills"`
		Skills  []skillSummary `xml:"skill"`
	}

	out := wrapper{}
	for _, name := range r.Names() {
		skill := r.skills[name]
		status := "available"
		reason := ""
		if !skill.Available {
			status = "unavailable"
			reason = skill.UnavailableReason
		}
		out.Skills = append(out.Skills, skillSummary{
			Name:   skill.Name,
			Status: status,
			Reason: reason,
			Always: skill.Always,
		})
	}

	data, err := xml.MarshalIndent(out, "", "  ")
	if err != nil {
		return "<available_skills></available_skills>"
	}
	return string(data)
}

func loadBuiltinSkills() (map[string]*Skill, error) {
	skills := make(map[string]*Skill)
	entries, err := fs.ReadDir(nanobotgo.EmbeddedAssets, "skills")
	if err != nil {
		return nil, fmt.Errorf("read embedded skills: %w", err)
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		path := filepath.ToSlash(filepath.Join("skills", entry.Name(), "SKILL.md"))
		data, err := fs.ReadFile(nanobotgo.EmbeddedAssets, path)
		if err != nil {
			return nil, fmt.Errorf("read embedded skill %q: %w", path, err)
		}
		skill, err := parseSkill(string(data), path, "builtin")
		if err != nil {
			return nil, fmt.Errorf("parse builtin skill %q: %w", path, err)
		}
		skill.Path = path
		skills[skill.Name] = skill
	}

	return skills, nil
}

func builtinSkillRoot() string {
	return strings.TrimSuffix("skills/", "/")
}
