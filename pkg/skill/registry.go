package skill

import (
	"encoding/xml"
	"fmt"
	"io/fs"
	"path/filepath"
	"sort"
	"strings"

	nanobotgo "github.com/Nomadcxx/nanobot-go"
)

type Registry struct {
	skills map[string]*Skill
}

func NewBuiltinRegistry() (*Registry, error) {
	skills, err := loadBuiltinSkills()
	if err != nil {
		return nil, err
	}
	return &Registry{skills: skills}, nil
}

func NewRegistry(workspace string) (*Registry, error) {
	builtin, err := loadBuiltinSkills()
	if err != nil {
		return nil, err
	}

	reg := &Registry{skills: builtin}
	workspaceSkills, err := LoadDir(filepath.Join(workspace, "skills"))
	if err != nil {
		return nil, err
	}
	for _, skill := range workspaceSkills {
		reg.skills[skill.Name] = skill
	}
	return reg, nil
}

func (r *Registry) Names() []string {
	names := make([]string, 0, len(r.skills))
	for name := range r.skills {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func (r *Registry) Get(name string) (*Skill, bool) {
	skill, ok := r.skills[name]
	return skill, ok
}

func (r *Registry) AlwaysOn() []*Skill {
	skills := make([]*Skill, 0)
	for _, name := range r.Names() {
		skill := r.skills[name]
		if skill.Always {
			skills = append(skills, skill)
		}
	}
	return skills
}

func (r *Registry) SummaryXML() string {
	type skillSummary struct {
		XMLName xml.Name `xml:"skill"`
		Name    string   `xml:"name,attr"`
		Status  string   `xml:"status,attr"`
		Reason  string   `xml:"reason,attr,omitempty"`
		Text    string   `xml:",chardata"`
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
			Text:   skill.Description,
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
			return nil, err
		}
		skill.Path = path
		skills[skill.Name] = skill
	}

	return skills, nil
}

func builtinSkillRoot() string {
	return strings.TrimSuffix("skills/", "/")
}
