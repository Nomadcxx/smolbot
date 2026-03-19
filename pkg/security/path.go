package security

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var pathPattern = regexp.MustCompile(`(?:~/[^\s"'<>|&;]+|/[^\s"'<>|&;]+)`)

// ValidatePath checks that the target path resolves within the workspace,
// including through existing symlink ancestors.
func ValidatePath(target, workspace string) error {
	resolvedWorkspace, err := resolveForValidation(workspace)
	if err != nil {
		return fmt.Errorf("resolve workspace: %w", err)
	}

	resolvedTarget, err := resolveForValidation(target)
	if err != nil {
		return fmt.Errorf("resolve target: %w", err)
	}

	if !isWithinWorkspace(resolvedTarget, resolvedWorkspace) {
		return fmt.Errorf("path traversal: %q resolves outside workspace %q", target, workspace)
	}

	return nil
}

func ExtractPathsFromCommand(cmd string) []string {
	matches := pathPattern.FindAllString(cmd, -1)
	if len(matches) == 0 {
		return nil
	}

	paths := make([]string, 0, len(matches))
	for _, match := range matches {
		if match == "/" || match == "//" {
			continue
		}
		paths = append(paths, match)
	}

	return paths
}

func ValidateCommandPaths(cmd, workspace string) error {
	for _, candidate := range ExtractPathsFromCommand(cmd) {
		if strings.HasPrefix(candidate, "~/") {
			if strings.Contains(candidate, "..") {
				return fmt.Errorf("path traversal: %q", candidate)
			}
			continue
		}
		if err := ValidatePath(candidate, workspace); err != nil {
			return err
		}
	}
	return nil
}

func resolveForValidation(path string) (string, error) {
	expanded, err := expandHome(path)
	if err != nil {
		return "", err
	}

	absPath, err := filepath.Abs(expanded)
	if err != nil {
		return "", err
	}
	cleanPath := filepath.Clean(absPath)

	current := cleanPath
	var remainder []string
	for {
		resolved, err := filepath.EvalSymlinks(current)
		if err == nil {
			if len(remainder) == 0 {
				return filepath.Clean(resolved), nil
			}
			parts := append([]string{resolved}, remainder...)
			return filepath.Clean(filepath.Join(parts...)), nil
		}
		if !os.IsNotExist(err) {
			return "", err
		}

		parent := filepath.Dir(current)
		if parent == current {
			return cleanPath, nil
		}

		remainder = append([]string{filepath.Base(current)}, remainder...)
		current = parent
	}
}

func isWithinWorkspace(target, workspace string) bool {
	if target == workspace {
		return true
	}
	rel, err := filepath.Rel(workspace, target)
	if err != nil {
		return false
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

func expandHome(path string) (string, error) {
	if path == "~" || strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("resolve home: %w", err)
		}
		if path == "~" {
			return home, nil
		}
		return filepath.Join(home, path[2:]), nil
	}
	return path, nil
}
