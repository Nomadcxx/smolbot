package security

import (
	"os"
	"path/filepath"
	"testing"
)

func TestValidatePath(t *testing.T) {
	workspace := t.TempDir()
	subdir := filepath.Join(workspace, "sub")
	if err := os.MkdirAll(subdir, 0o755); err != nil {
		t.Fatalf("mkdir subdir: %v", err)
	}

	outsideRoot := t.TempDir()
	outsideFile := filepath.Join(outsideRoot, "outside.txt")
	if err := os.WriteFile(outsideFile, []byte("x"), 0o644); err != nil {
		t.Fatalf("write outside file: %v", err)
	}

	escapeLink := filepath.Join(workspace, "escape-link")
	if err := os.Symlink(outsideRoot, escapeLink); err != nil {
		t.Fatalf("create symlink: %v", err)
	}

	tests := []struct {
		path    string
		allowed bool
	}{
		{path: filepath.Join(workspace, "file.txt"), allowed: true},
		{path: filepath.Join(workspace, "sub", "nested.go"), allowed: true},
		{path: filepath.Join(workspace, "..", "outside.txt"), allowed: false},
		{path: "/etc/passwd", allowed: false},
		{path: filepath.Join(workspace, "sub", "..", "..", "etc", "passwd"), allowed: false},
		{path: filepath.Join(escapeLink, "outside.txt"), allowed: false},
	}

	for _, tt := range tests {
		err := ValidatePath(tt.path, workspace)
		if tt.allowed && err != nil {
			t.Errorf("ValidatePath(%q, %q) should be allowed: %v", tt.path, workspace, err)
		}
		if !tt.allowed && err == nil {
			t.Errorf("ValidatePath(%q, %q) should be blocked", tt.path, workspace)
		}
	}
}

func TestExtractPathsFromCommand(t *testing.T) {
	tests := []struct {
		cmd   string
		paths []string
	}{
		{cmd: "cat /etc/passwd", paths: []string{"/etc/passwd"}},
		{cmd: "ls ~/Documents", paths: []string{"~/Documents"}},
		{cmd: "echo hello", paths: nil},
		{cmd: "cat /home/user/file.txt && rm /tmp/x", paths: []string{"/home/user/file.txt", "/tmp/x"}},
	}

	for _, tt := range tests {
		got := ExtractPathsFromCommand(tt.cmd)
		if len(got) != len(tt.paths) {
			t.Fatalf("ExtractPathsFromCommand(%q) len=%d want=%d (%v)", tt.cmd, len(got), len(tt.paths), got)
		}
		for i := range got {
			if got[i] != tt.paths[i] {
				t.Fatalf("ExtractPathsFromCommand(%q)[%d]=%q want=%q", tt.cmd, i, got[i], tt.paths[i])
			}
		}
	}
}

func TestValidateCommandPaths(t *testing.T) {
	workspace := t.TempDir()
	inside := filepath.Join(workspace, "file.txt")
	outside := "/etc/passwd"

	tests := []struct {
		name    string
		cmd     string
		allowed bool
	}{
		{name: "inside", cmd: "cat " + inside, allowed: true},
		{name: "outside", cmd: "cat " + outside, allowed: false},
		{name: "tilde path without traversal", cmd: "ls ~/Documents", allowed: true},
		{name: "tilde path with traversal", cmd: "cat ~/../.ssh/id_rsa", allowed: false},
	}

	for _, tt := range tests {
		err := ValidateCommandPaths(tt.cmd, workspace)
		if tt.allowed && err != nil {
			t.Errorf("%s should be allowed: %v", tt.name, err)
		}
		if !tt.allowed && err == nil {
			t.Errorf("%s should be blocked", tt.name)
		}
	}
}
