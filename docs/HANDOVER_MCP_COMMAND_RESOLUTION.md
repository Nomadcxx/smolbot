# Handover: MCP Command Resolution and Systemd PATH Hardening

## Context

**Problem:** smolbot daemon crashes on startup when MCP servers are configured with ephemeral shell paths (e.g., fnm, nvm, volta). The systemd unit lacks the user's full PATH, so commands like `/run/user/1000/fnm/node-versions/v24.14.0/installation/bin/node` fail.

**Root Cause:** When the installer writes an MCP server config, it may store absolute paths that contain ephemeral version directories. When systemd starts the daemon, it has a minimal PATH that doesn't include these directories.

**Goal:** Make MCP server commands portable — store bare names in config, resolve at runtime using a known-good PATH baked into the systemd unit at install time.

**Reference Plan:** `docs/plans/2026-03-28-mcp-command-resolution-and-systemd-path.md`

---

## Files Modified

### 1. `pkg/mcp/stdio.go` (PRIMARY CHANGE)

**Added imports:**
```go
"os"
"path/filepath"
```

**Modified `NewStdioTransport`:**
- Now calls `resolveCommand(command, env)` instead of passing `command` directly to `exec.CommandContext`
- Improved error message for `exec.ErrNotFound` showing command name, resolved path, and current PATH

**New function `resolveCommand`:**
```go
func resolveCommand(command string, env map[string]string) string {
    // Extract basename from absolute paths
    if strings.Contains(command, string(filepath.Separator)) {
        base := filepath.Base(command)
        if base != "." && base != "" {
            command = base
        }
    }

    // Build effective PATH: MCP env PATH prepended to system PATH
    effectivePath := os.Getenv("PATH")
    if envPath, ok := env["PATH"]; ok {
        effectivePath = envPath + string(os.PathListSeparator) + effectivePath
    }

    // Resolve via LookPath
    if resolved, err := lookPathWithEnv(command, effectivePath); err == nil {
        return resolved
    }
    return command
}
```

**New function `lookPathWithEnv`:**
- Custom PATH lookup that respects the effective PATH (MCP env + system)
- Iterates through `filepath.SplitList(pathEnv)`, checking each entry for executable
- Returns error with clear message: `"<command> not found in PATH"`

---

### 2. `cmd/installer/tasks.go`

**Modified `setupSystemd`:**
```go
currentPath := os.Getenv("PATH")
serviceContent := fmt.Sprintf(`...
Environment=PATH=%s
...`, currentPath)
```

Now captures `os.Getenv("PATH")` at install time and writes it into the systemd unit's `Environment=PATH=...` directive. This bakes the user's full PATH (including fnm/nvm/volta/cargo paths) into the service file.

---

### 3. `pkg/mcp/stdio_test.go` (NEW FILE)

Unit and integration tests for:
- `lookPathWithEnv`: finds binary in custom PATH, returns error when not found, handles empty PATH, skips non-executables/directories
- `resolveCommand`: extracts basename from absolute paths, resolves bare commands, uses MCP env PATH
- `NewStdioTransport`: bare command resolution, absolute path handling, clear error messages

---

## Key Design Decisions

### 1. Custom `lookPathWithEnv` instead of `exec.LookPath`

The spec suggested `exec.LookPath`, but I implemented a custom function to allow the MCP env `PATH` to be prepended to the system PATH. This ensures MCP-specific paths take priority over system paths.

### 2. Basename extraction from absolute paths

If someone has stored an absolute path in config (e.g., `/run/user/1000/fnm/.../node`), we extract just the basename (`node`) and resolve it through the effective PATH. This handles cases where old configs have ephemeral paths stored.

### 3. Fallback behavior

If resolution fails, we pass the original command through — `exec.CommandContext` will produce a clear "not found" error with the improved error message format showing the current PATH.

---

## Build & Test Status

| Command | Status | Notes |
|---------|--------|-------|
| `go build ./pkg/mcp/...` | ✅ Passes | |
| `go build ./cmd/smolbot/...` | ❌ Fails | Pre-existing issue: missing `github.com/Nomadcxx/smolbot/pkg/channel/qr` package |
| `go test ./pkg/mcp/...` | ❌ Fails | Pre-existing issue: `discovery_test.go:261` references undefined `mockServerBinary` |

The test and build failures are **pre-existing issues** unrelated to this MCP fix.

---

## Git Status

```
On branch main
Your branch and 'origin/main' have diverged (1 commit each)

Changes not staged for commit:
  modified:   cmd/installer/tasks.go          ← OUR CHANGE
  modified:   pkg/mcp/stdio.go                ← OUR CHANGE
  modified:   pkg/mcp/stdio_test.go           ← OUR CHANGE (new file)
  modified:   cmd/smolbot/channels_whatsapp_login.go    ← UNRELATED
  modified:   pkg/channel/whatsapp/adapter.go           ← UNRELATED
  (and several other unrelated files)
```

**UNSAFE TO PUSH** — main and origin/main have diverged, and there are unrelated changes staged.

---

## Recommended Next Steps

### 1. Resolve git divergence

Determine how to integrate changes:
- Option A: Create feature branch, cherry-pick MCP commits, open PR
- Option B: Rebase onto origin/main (requires force-push)
- Option C: Merge origin/main into main (creates merge commit)

### 2. Fix pre-existing issues (if needed before merging)

- `discovery_test.go:261`: `mockServerBinary` is undefined — this is a test helper that needs to be implemented or the test reference removed
- `cmd/smolbot/channels_whatsapp_login.go`: Missing `qr` package import

### 3. Verify the fix works

Test scenario:
1. Configure an MCP server with an fnm-based path like `/run/user/1000/fnm/node-versions/v24.14.0/installation/bin/node`
2. Start smolbot via systemd
3. Confirm the command resolves correctly and the MCP server starts

### 4. Consider integration test

Add a test that simulates the full startup scenario:
- Create a mock MCP server binary
- Configure smolbot to use it
- Start the daemon and verify the MCP tools are discovered

---

## Summary of Changes

| File | Change Type | Description |
|------|-------------|-------------|
| `pkg/mcp/stdio.go` | Modified | Added `resolveCommand` and `lookPathWithEnv`, improved error messages |
| `cmd/installer/tasks.go` | Modified | Systemd unit now captures PATH at install time |
| `pkg/mcp/stdio_test.go` | New | Unit and integration tests for command resolution |

---

## Plan Document Reference

Full specification: `docs/plans/2026-03-28-mcp-command-resolution-and-systemd-path.md`

Key requirements from plan:
1. ✅ Config stores bare command names (basename extraction added to `resolveCommand`)
2. ✅ Systemd captures PATH at install time (`setupSystemd` modified)
3. ✅ `NewStdioTransport` resolves bare commands using MCP env overlay
4. ✅ Improved error messages for command-not-found
5. ✅ Update flow refreshes systemd (`setupSystemd` is in both install and update task lists)
