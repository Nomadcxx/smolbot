package config

import (
	"os"
	"path/filepath"
)

// Paths provides well-known runtime paths for config, storage, and workspace data.
type Paths struct {
	root      string
	workspace string
}

// NewPaths creates a set of runtime paths rooted at the provided directory.
func NewPaths(root string) *Paths {
	return &Paths{
		root:      root,
		workspace: filepath.Join(root, "workspace"),
	}
}

// DefaultPaths returns the default ~/.nanobot path set.
func DefaultPaths() *Paths {
	home, _ := os.UserHomeDir()
	return NewPaths(filepath.Join(home, ".nanobot"))
}

func (p *Paths) Root() string       { return p.root }
func (p *Paths) ConfigFile() string { return filepath.Join(p.root, "config.json") }
func (p *Paths) Workspace() string  { return p.workspace }
func (p *Paths) SessionsDB() string { return filepath.Join(p.root, "sessions.db") }
func (p *Paths) JobsFile() string   { return filepath.Join(p.root, "jobs.json") }
func (p *Paths) MemoryDir() string  { return filepath.Join(p.workspace, "memory") }
func (p *Paths) SkillsDir() string  { return filepath.Join(p.root, "skills") }
func (p *Paths) ChatHistory() string {
	return filepath.Join(p.root, "chat_history")
}

// SetWorkspace overrides the workspace directory when config points elsewhere.
func (p *Paths) SetWorkspace(workspace string) {
	p.workspace = workspace
}
