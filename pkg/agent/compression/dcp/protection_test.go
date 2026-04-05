package dcp

import "testing"

func TestIsToolProtected_ExactMatch(t *testing.T) {
	if !IsToolProtected("write_file", nil) {
		t.Fatal("write_file should be protected")
	}
}

func TestIsToolProtected_GlobPattern(t *testing.T) {
	if !IsToolProtected("mcp_github_search", []string{"mcp_*"}) {
		t.Fatal("mcp_* should match mcp_github_search")
	}
}

func TestIsTurnProtected(t *testing.T) {
	if IsTurnProtected(1, 5, 2) {
		t.Fatal("older turn should not be protected")
	}
	if !IsTurnProtected(4, 5, 2) {
		t.Fatal("recent turn should be protected")
	}
}
