package tool

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/Nomadcxx/nanobot-go/pkg/config"
	"github.com/Nomadcxx/nanobot-go/pkg/security"
)

const (
	execOutputLimit     = 10000
	execOutputHeadBytes = 5000
	execOutputTailBytes = 5000
)

type ExecTool struct {
	cfg                 config.ExecToolConfig
	restrictToWorkspace bool
}

type execArgs struct {
	Command string `json:"command"`
	Timeout int    `json:"timeout"`
}

func NewExecTool(cfg config.ExecToolConfig, restrictToWorkspace bool) *ExecTool {
	return &ExecTool{
		cfg:                 cfg,
		restrictToWorkspace: restrictToWorkspace,
	}
}

func (t *ExecTool) Name() string {
	return "exec"
}

func (t *ExecTool) Description() string {
	return "Run a shell command inside the configured workspace restrictions."
}

func (t *ExecTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"command": map[string]any{
				"type":        "string",
				"description": "Shell command to execute.",
			},
			"timeout": map[string]any{
				"type":        "integer",
				"description": "Timeout in seconds; capped by configuration.",
			},
		},
		"required": []string{"command"},
	}
}

func (t *ExecTool) Execute(ctx context.Context, args json.RawMessage, tctx ToolContext) (*Result, error) {
	parsed := execArgs{}
	if err := json.Unmarshal(args, &parsed); err != nil {
		return nil, fmt.Errorf("parse exec args: %w", err)
	}
	command := strings.TrimSpace(parsed.Command)
	if command == "" {
		return &Result{Error: "command is required"}, nil
	}

	if err := t.validateCommand(command, tctx.Workspace); err != nil {
		return &Result{Error: err.Error()}, nil
	}

	timeout := t.effectiveTimeout(parsed.Timeout)
	runCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	cmd := exec.CommandContext(runCtx, "/bin/sh", "-c", command)
	cmd.Dir = firstNonEmptyString(strings.TrimSpace(tctx.Workspace), ".")
	cmd.Env = t.commandEnv()

	output, err := cmd.CombinedOutput()
	if runCtx.Err() == context.DeadlineExceeded {
		return &Result{Error: fmt.Sprintf("command timed out after %s", timeout)}, nil
	}
	if err != nil {
		text := firstNonEmptyString(strings.TrimSpace(string(output)), err.Error())
		return &Result{Error: truncateOutput(text)}, nil
	}

	return &Result{Output: truncateOutput(string(output))}, nil
}

func (t *ExecTool) validateCommand(command, workspace string) error {
	for _, pattern := range t.cfg.DenyPatterns {
		if pattern != "" && strings.Contains(command, pattern) {
			return fmt.Errorf("command denied by policy")
		}
	}

	if err := security.ContainsInternalURL(command); err != nil {
		return fmt.Errorf("ssrf blocked: %w", err)
	}

	if t.restrictToWorkspace {
		if strings.TrimSpace(workspace) == "" {
			return fmt.Errorf("workspace restriction enabled but no workspace was provided")
		}
		if err := security.ValidateCommandPaths(command, workspace); err != nil {
			return fmt.Errorf("workspace restriction: %w", err)
		}
	}

	return nil
}

func (t *ExecTool) effectiveTimeout(requestedSeconds int) time.Duration {
	seconds := requestedSeconds
	if seconds <= 0 {
		seconds = t.cfg.DefaultTimeout
	}
	if seconds <= 0 {
		seconds = 60
	}
	if t.cfg.MaxTimeout > 0 && seconds > t.cfg.MaxTimeout {
		seconds = t.cfg.MaxTimeout
	}
	return time.Duration(seconds) * time.Second
}

func (t *ExecTool) commandEnv() []string {
	env := os.Environ()
	appendPath := strings.TrimSpace(t.cfg.PathAppend)
	if appendPath == "" {
		return env
	}

	resolved := appendPath
	if !filepath.IsAbs(resolved) {
		if abs, err := filepath.Abs(resolved); err == nil {
			resolved = abs
		}
	}

	for i, entry := range env {
		if !strings.HasPrefix(entry, "PATH=") {
			continue
		}
		env[i] = entry + string(os.PathListSeparator) + resolved
		return env
	}

	return append(env, "PATH="+resolved)
}

func truncateOutput(output string) string {
	if len(output) <= execOutputLimit {
		return output
	}

	head := output[:execOutputHeadBytes]
	tail := output[len(output)-execOutputTailBytes:]
	return fmt.Sprintf("%s\n\n... truncated %d bytes ...\n\n%s", head, len(output)-execOutputLimit, tail)
}

func firstNonEmptyString(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
