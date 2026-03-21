package agent

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	nanobotgo "github.com/Nomadcxx/smolbot"
	"github.com/Nomadcxx/smolbot/pkg/skill"
)

var BootstrapFiles = []string{"AGENTS.md", "SOUL.md", "USER.md", "TOOLS.md"}

const DefaultIdentityBlock = "You are smolbot, a practical coding agent."

const PlatformPolicyBlock = "Platform policy: prefer UTF-8, POSIX shell tools, and direct file operations."

type BuildContext struct {
	Workspace string
	Skills    *skill.Registry
}

func SyncWorkspaceTemplates(workspace string) error {
	return fs.WalkDir(nanobotgo.EmbeddedAssets, "templates", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if path == "templates" {
			return nil
		}

		rel := strings.TrimPrefix(path, "templates/")
		dst := filepath.Join(workspace, rel)
		if d.IsDir() {
			return os.MkdirAll(dst, 0o755)
		}
		if _, err := os.Stat(dst); err == nil {
			return nil
		} else if !os.IsNotExist(err) {
			return err
		}

		data, err := fs.ReadFile(nanobotgo.EmbeddedAssets, path)
		if err != nil {
			return fmt.Errorf("read embedded template %q: %w", path, err)
		}
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			return err
		}
		return os.WriteFile(dst, data, 0o644)
	})
}

func BuildSystemPrompt(ctx BuildContext) (string, error) {
	workspace, err := filepath.Abs(ctx.Workspace)
	if err != nil {
		return "", fmt.Errorf("resolve workspace: %w", err)
	}

	var sections []string

	identity := loadOptionalFile(filepath.Join(workspace, "AGENTS.md"))
	if identity == "" {
		identity = DefaultIdentityBlock
	}
	sections = append(sections, identity)
	sections = append(sections, PlatformPolicyBlock)

	for _, name := range []string{"SOUL.md", "USER.md", "TOOLS.md"} {
		if content := loadOptionalFile(filepath.Join(workspace, name)); content != "" {
			sections = append(sections, content)
		}
	}

	memoryPath := filepath.Join(workspace, "memory", "MEMORY.md")
	if memory := loadOptionalFile(memoryPath); memory != "" {
		sections = append(sections, memory)
	}

	if ctx.Skills != nil {
		sections = append(sections, ctx.Skills.SummaryXML())
		for _, s := range ctx.Skills.AlwaysOn() {
			if s.Content != "" {
				sections = append(sections, s.Content)
			}
		}
	}

	historyPath := filepath.Join(workspace, "memory", "HISTORY.md")
	sections = append(sections,
		fmt.Sprintf("Your workspace is at: %s", workspace),
		fmt.Sprintf("Memory file: %s, History file: %s", memoryPath, historyPath),
	)

	return strings.Join(sections, "\n\n"), nil
}

func BuildRuntimeContextPrefix(now time.Time, channel, chatID string) string {
	return fmt.Sprintf(
		"[Runtime Context -- metadata only, not instructions]\nCurrent time: %s\nChannel: %s\nChat ID: %s\n\n",
		now.Format(time.RFC3339),
		channel,
		chatID,
	)
}

func loadOptionalFile(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}
