package tool

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/Nomadcxx/nanobot-go/pkg/security"
)

const readFileMaxChars = 128000

var ignoredDirs = map[string]struct{}{
	".coverage":     {},
	".git":          {},
	".mypy_cache":   {},
	".pytest_cache": {},
	".ruff_cache":   {},
	".tox":          {},
	".venv":         {},
	"__pycache__":   {},
	"build":         {},
	"dist":          {},
	"htmlcov":       {},
	"node_modules":  {},
	"venv":          {},
}

type ReadFileTool struct {
	restrictToWorkspace bool
}

type WriteFileTool struct {
	restrictToWorkspace bool
}

type EditFileTool struct {
	restrictToWorkspace bool
}

type ListDirTool struct {
	restrictToWorkspace bool
}

type readFileArgs struct {
	Path             string   `json:"path"`
	Offset           int      `json:"offset"`
	Limit            int      `json:"limit"`
	ExtraAllowedDirs []string `json:"extraAllowedDirs"`
}

type writeFileArgs struct {
	Path    string `json:"path"`
	Content string `json:"content"`
}

type editFileArgs struct {
	Path       string `json:"path"`
	OldString  string `json:"old_string"`
	NewString  string `json:"new_string"`
	ReplaceAll bool   `json:"replace_all"`
}

type listDirArgs struct {
	Path      string `json:"path"`
	Recursive bool   `json:"recursive"`
	MaxDepth  int    `json:"max_depth"`
}

func NewReadFileTool(restrictToWorkspace bool) *ReadFileTool {
	return &ReadFileTool{restrictToWorkspace: restrictToWorkspace}
}

func NewWriteFileTool(restrictToWorkspace bool) *WriteFileTool {
	return &WriteFileTool{restrictToWorkspace: restrictToWorkspace}
}

func NewEditFileTool(restrictToWorkspace bool) *EditFileTool {
	return &EditFileTool{restrictToWorkspace: restrictToWorkspace}
}

func NewListDirTool(restrictToWorkspace bool) *ListDirTool {
	return &ListDirTool{restrictToWorkspace: restrictToWorkspace}
}

func (t *ReadFileTool) Name() string        { return "read_file" }
func (t *WriteFileTool) Name() string       { return "write_file" }
func (t *EditFileTool) Name() string        { return "edit_file" }
func (t *ListDirTool) Name() string         { return "list_dir" }
func (t *ReadFileTool) Description() string { return "Read file content with optional pagination." }
func (t *WriteFileTool) Description() string {
	return "Create or overwrite a file, creating parent directories when needed."
}
func (t *EditFileTool) Description() string {
	return "Replace content inside a file with exact or fuzzy matching."
}
func (t *ListDirTool) Description() string {
	return "List directory contents with file size and type information."
}

func (t *ReadFileTool) Parameters() map[string]any {
	return filesystemParameters("path", "offset", "limit", "extraAllowedDirs")
}

func (t *WriteFileTool) Parameters() map[string]any {
	return filesystemParameters("path", "content")
}

func (t *EditFileTool) Parameters() map[string]any {
	return filesystemParameters("path", "old_string", "new_string", "replace_all")
}

func (t *ListDirTool) Parameters() map[string]any {
	return filesystemParameters("path", "recursive", "max_depth")
}

func (t *ReadFileTool) Execute(_ context.Context, raw json.RawMessage, tctx ToolContext) (*Result, error) {
	args := readFileArgs{}
	if err := json.Unmarshal(raw, &args); err != nil {
		return nil, fmt.Errorf("parse read_file args: %w", err)
	}
	path, err := validateFilesystemPath(args.Path, tctx.Workspace, t.restrictToWorkspace, args.ExtraAllowedDirs)
	if err != nil {
		return &Result{Error: err.Error()}, nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return &Result{Error: fmt.Sprintf("read file: %v", err)}, nil
	}

	lines := splitLines(strings.ReplaceAll(string(data), "\r\n", "\n"))
	start := clamp(args.Offset, 0, len(lines))
	end := len(lines)
	if args.Limit > 0 && start+args.Limit < end {
		end = start + args.Limit
	}

	var builder strings.Builder
	for i := start; i < end; i++ {
		line := fmt.Sprintf("%d: %s\n", i+1, lines[i])
		if builder.Len()+len(line) > readFileMaxChars {
			remaining := readFileMaxChars - builder.Len()
			if remaining > 0 {
				builder.WriteString(line[:remaining])
			}
			builder.WriteString("\n... truncated ...")
			break
		}
		builder.WriteString(line)
	}

	return &Result{Output: strings.TrimRight(builder.String(), "\n")}, nil
}

func (t *WriteFileTool) Execute(_ context.Context, raw json.RawMessage, tctx ToolContext) (*Result, error) {
	args := writeFileArgs{}
	if err := json.Unmarshal(raw, &args); err != nil {
		return nil, fmt.Errorf("parse write_file args: %w", err)
	}
	path, err := validateFilesystemPath(args.Path, tctx.Workspace, t.restrictToWorkspace, nil)
	if err != nil {
		return &Result{Error: err.Error()}, nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return &Result{Error: fmt.Sprintf("create parent directories: %v", err)}, nil
	}
	if err := os.WriteFile(path, []byte(args.Content), 0o644); err != nil {
		return &Result{Error: fmt.Sprintf("write file: %v", err)}, nil
	}
	return &Result{Output: fmt.Sprintf("wrote %s", path)}, nil
}

func (t *EditFileTool) Execute(_ context.Context, raw json.RawMessage, tctx ToolContext) (*Result, error) {
	args := editFileArgs{}
	if err := json.Unmarshal(raw, &args); err != nil {
		return nil, fmt.Errorf("parse edit_file args: %w", err)
	}
	path, err := validateFilesystemPath(args.Path, tctx.Workspace, t.restrictToWorkspace, nil)
	if err != nil {
		return &Result{Error: err.Error()}, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return &Result{Error: fmt.Sprintf("read file: %v", err)}, nil
	}

	original := string(data)
	hasCRLF := strings.Contains(original, "\r\n")
	content := strings.ReplaceAll(original, "\r\n", "\n")
	oldString := strings.ReplaceAll(args.OldString, "\r\n", "\n")
	newString := strings.ReplaceAll(args.NewString, "\r\n", "\n")

	updated, warning, replaced := replaceContent(content, oldString, newString, args.ReplaceAll)
	if warning != "" {
		return &Result{Error: warning}, nil
	}
	if !replaced {
		return &Result{Error: buildFuzzyMismatch(content, oldString)}, nil
	}
	if hasCRLF {
		updated = strings.ReplaceAll(updated, "\n", "\r\n")
	}
	if err := os.WriteFile(path, []byte(updated), 0o644); err != nil {
		return &Result{Error: fmt.Sprintf("write file: %v", err)}, nil
	}
	return &Result{Output: fmt.Sprintf("updated %s", path)}, nil
}

func (t *ListDirTool) Execute(_ context.Context, raw json.RawMessage, tctx ToolContext) (*Result, error) {
	args := listDirArgs{}
	if err := json.Unmarshal(raw, &args); err != nil {
		return nil, fmt.Errorf("parse list_dir args: %w", err)
	}
	path, err := validateFilesystemPath(args.Path, tctx.Workspace, t.restrictToWorkspace, nil)
	if err != nil {
		return &Result{Error: err.Error()}, nil
	}

	entries, err := renderDirectory(path, args.Recursive, args.MaxDepth)
	if err != nil {
		return &Result{Error: err.Error()}, nil
	}
	return &Result{Output: strings.Join(entries, "\n")}, nil
}

func filesystemParameters(fields ...string) map[string]any {
	properties := map[string]any{}
	for _, field := range fields {
		properties[field] = map[string]any{"type": "string"}
	}
	return map[string]any{
		"type":       "object",
		"properties": properties,
	}
}

func validateFilesystemPath(path, workspace string, restrict bool, extraAllowedDirs []string) (string, error) {
	if strings.TrimSpace(path) == "" {
		return "", fmt.Errorf("path is required")
	}
	expanded, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("resolve path: %w", err)
	}
	if !restrict {
		return expanded, nil
	}
	if strings.TrimSpace(workspace) == "" {
		return "", fmt.Errorf("workspace restriction enabled but no workspace was provided")
	}
	if err := security.ValidatePath(expanded, workspace); err == nil {
		return expanded, nil
	}
	for _, allowedDir := range extraAllowedDirs {
		if strings.TrimSpace(allowedDir) == "" {
			continue
		}
		if err := security.ValidatePath(expanded, allowedDir); err == nil {
			return expanded, nil
		}
	}
	return "", fmt.Errorf("workspace restriction: %s", path)
}

func splitLines(content string) []string {
	lines := strings.Split(content, "\n")
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	return lines
}

func clamp(value, minValue, maxValue int) int {
	if value < minValue {
		return minValue
	}
	if value > maxValue {
		return maxValue
	}
	return value
}

func replaceContent(content, oldString, newString string, replaceAll bool) (updated, warning string, replaced bool) {
	if oldString == "" {
		return content, "old_string is required", false
	}

	exactMatches := strings.Count(content, oldString)
	if exactMatches > 1 && !replaceAll {
		return content, fmt.Sprintf("multiple matches found for old_string (%d occurrences)", exactMatches), false
	}
	if exactMatches > 0 {
		if replaceAll {
			return strings.ReplaceAll(content, oldString, newString), "", true
		}
		return strings.Replace(content, oldString, newString, 1), "", true
	}

	fuzzy, ok := fuzzyReplace(content, oldString, newString)
	if !ok {
		return content, "", false
	}
	return fuzzy, "", true
}

func fuzzyReplace(content, oldString, newString string) (string, bool) {
	contentLines := splitLines(content)
	oldLines := splitLines(oldString)
	if len(oldLines) == 0 || len(oldLines) > len(contentLines) {
		return "", false
	}

	target := make([]string, len(oldLines))
	for i, line := range oldLines {
		target[i] = strings.TrimSpace(line)
	}

	for start := 0; start+len(oldLines) <= len(contentLines); start++ {
		match := true
		for i := range oldLines {
			if strings.TrimSpace(contentLines[start+i]) != target[i] {
				match = false
				break
			}
		}
		if !match {
			continue
		}

		replacementLines := splitLines(newString)
		updatedLines := append([]string{}, contentLines[:start]...)
		updatedLines = append(updatedLines, replacementLines...)
		updatedLines = append(updatedLines, contentLines[start+len(oldLines):]...)
		return strings.Join(updatedLines, "\n"), true
	}

	return "", false
}

func buildFuzzyMismatch(content, oldString string) string {
	contentLines := splitLines(content)
	oldLines := splitLines(oldString)
	bestIndex := -1
	bestScore := 0.0

	for start := 0; start+len(oldLines) <= len(contentLines); start++ {
		score := similarityScore(contentLines[start:start+len(oldLines)], oldLines)
		if score > bestScore {
			bestScore = score
			bestIndex = start
		}
	}

	if bestIndex >= 0 && bestScore >= 0.5 {
		actual := strings.Join(contentLines[bestIndex:bestIndex+len(oldLines)], "\n")
		return fmt.Sprintf("exact match not found; closest match similarity %.0f%%\n--- expected\n%s\n+++ actual\n%s", bestScore*100, oldString, actual)
	}
	return "exact match not found"
}

func similarityScore(actual, expected []string) float64 {
	if len(expected) == 0 {
		return 0
	}
	matched := 0
	for i := range expected {
		if i < len(actual) && strings.TrimSpace(actual[i]) == strings.TrimSpace(expected[i]) {
			matched++
		}
	}
	return float64(matched) / float64(len(expected))
}

func renderDirectory(root string, recursive bool, maxDepth int) ([]string, error) {
	rootInfo, err := os.Stat(root)
	if err != nil {
		return nil, fmt.Errorf("stat path: %w", err)
	}
	if !rootInfo.IsDir() {
		return nil, fmt.Errorf("path is not a directory")
	}

	var lines []string
	err = walkDirectory(root, root, recursive, maxDepth, 0, &lines)
	return lines, err
}

func walkDirectory(root, current string, recursive bool, maxDepth, depth int, lines *[]string) error {
	entries, err := os.ReadDir(current)
	if err != nil {
		return fmt.Errorf("read directory: %w", err)
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })

	for _, entry := range entries {
		if _, ignored := ignoredDirs[entry.Name()]; ignored {
			continue
		}

		fullPath := filepath.Join(current, entry.Name())
		info, err := entry.Info()
		if err != nil {
			return fmt.Errorf("stat entry: %w", err)
		}

		rel, err := filepath.Rel(root, fullPath)
		if err != nil {
			return fmt.Errorf("relative path: %w", err)
		}

		entryType := "file"
		if info.IsDir() {
			entryType = "dir"
		}
		*lines = append(*lines, fmt.Sprintf("%s [%s, %d B]", rel, entryType, info.Size()))

		if !recursive || !info.IsDir() {
			continue
		}
		if maxDepth > 0 && depth+1 >= maxDepth {
			continue
		}
		if err := walkDirectory(root, fullPath, recursive, maxDepth, depth+1, lines); err != nil {
			return err
		}
	}

	return nil
}
