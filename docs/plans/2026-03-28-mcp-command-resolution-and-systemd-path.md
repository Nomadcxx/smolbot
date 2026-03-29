# MCP Command Resolution & Systemd PATH Hardening

> **For agentic workers:** This is a standalone task. Do not assume prior context. Read every file referenced below before making changes.

## Problem Statement

The smolbot daemon runs as a systemd user service (`~/.config/systemd/user/smolbot.service`). MCP (Model Context Protocol) servers are external tool providers configured in `~/.smolbot/config.json` under `tools.mcpServers`. Each MCP server has a `command` field that specifies the executable to spawn as a subprocess (e.g., `node` for a Node.js-based MCP server).

**The bug:** The installer (`cmd/installer/`) writes the *absolute path* of the command as resolved in the user's current shell session. When the user uses **fnm** (Fast Node Manager), this path points to an ephemeral per-shell directory:

```
/run/user/1000/fnm_multishells/<session_id>_<timestamp>/bin/node
```

This path is destroyed when the shell exits or the system reboots. The systemd service then fails to start the MCP server subprocess because the binary no longer exists at that path. Since MCP discovery failure was previously fatal (`return nil, fmt.Errorf(...)`), the entire daemon crash-looped with exit code 1, restarting every 5 seconds indefinitely.

**Current temporary fix (already deployed, do not revert):**
1. Config changed from the ephemeral absolute path to bare `"command": "node"`
2. Systemd service unit has a hardcoded `Environment=PATH=...` line that includes the fnm stable path
3. The `mcp-startup-hardening` branch changed MCP discovery failure from fatal to warn-and-continue

**What this task fixes:** Make the system robust so this class of problem cannot recur, regardless of the user's node version manager (fnm, nvm, volta, asdf, etc.) or how they install tools.

---

## Architecture Overview

### How smolbot runs

```
systemd --user
  └─ smolbot run --config ~/.smolbot/config.json --workspace ~/.smolbot/workspace --port 18791
       └─ node /home/nomadx/.opencode/hybrid-memory/mcp-server.js   (MCP subprocess)
```

- **Daemon binary:** `~/.local/bin/smolbot` (Go binary)
- **Config:** `~/.smolbot/config.json` (JSON, user-editable)
- **Systemd unit:** `~/.config/systemd/user/smolbot.service`
- **Installer:** `cmd/installer/` (Bubble Tea TUI wizard, builds and installs everything)

### How MCP servers are spawned

1. Config is loaded: `tools.mcpServers` map of `MCPServerConfig`
2. `cmd/smolbot/runtime.go` calls `mgr.DiscoverAndRegister(ctx, tools, cfg.Tools.MCPServers)`
3. For each server, `pkg/mcp/discovery.go` calls `getOrConnect()` which calls `NewStdioTransport()`
4. `pkg/mcp/stdio.go` → `NewStdioTransport()` runs `exec.CommandContext(ctx, command, args...)` with env overlaid
5. The subprocess speaks JSON-RPC 2.0 over stdin/stdout (MCP protocol)

### Key config structure

```json
{
  "tools": {
    "mcpServers": {
      "hybrid-memory": {
        "command": "node",
        "args": ["/home/nomadx/.opencode/hybrid-memory/mcp-server.js"],
        "env": {
          "OPENCODE_MEMORY_DIR": "/home/nomadx/.opencode/hybrid-memory/data",
          "OLLAMA_HOST": "http://localhost:11434",
          "OLLAMA_EMBED_MODEL": "mxbai-embed-large"
        }
      }
    }
  }
}
```

### Key types

```go
// pkg/config/config.go
type MCPServerConfig struct {
    Type         string            `json:"type"`
    Command      string            `json:"command,omitempty"`
    Args         []string          `json:"args,omitempty"`
    Env          map[string]string `json:"env,omitempty"`
    URL          string            `json:"url,omitempty"`
    Headers      map[string]string `json:"headers,omitempty"`
    ToolTimeout  int               `json:"toolTimeout"`
    EnabledTools []string          `json:"enabledTools"`
}
```

---

## Files to Read First

Read ALL of these before making any changes:

| File | Why |
|------|-----|
| `pkg/mcp/stdio.go` | Where `exec.CommandContext` is called — the point where command resolution happens |
| `pkg/mcp/discovery.go` | `StdioDiscoveryClient.getOrConnect()` — creates transports, handles errors |
| `pkg/mcp/client.go` | `ConnectionSpec` struct, `DetectTransport()`, `Manager.DiscoverAndRegister()` |
| `cmd/smolbot/runtime.go` | `newMCPMgr()` factory, `buildRuntime()` where MCP is wired in |
| `cmd/installer/tasks.go` | Where config JSON is written — look for how MCP `command` field is populated |
| `cmd/installer/types.go` | Installer model fields |
| `~/.config/systemd/user/smolbot.service` | Current systemd unit (already has temporary PATH fix) |
| `pkg/config/config.go` | `MCPServerConfig` struct definition |

Also check:
- `cmd/smolbot/runtime.go` for `collectOnboardConfig()` — the CLI onboarding flow that also writes MCP config
- `cmd/installer/tasks.go` for `setupSystemd()` — where the systemd unit file is generated

---

## Requirements

### 1. Config should store bare command names, not absolute paths

**Change:** When the installer or onboarding writes an MCP server config, it should store the bare command name (e.g., `"node"`, `"python3"`, `"npx"`) rather than the resolved absolute path.

**Why:** Absolute paths break when:
- fnm/nvm/volta multishell paths are ephemeral
- Node is upgraded to a new version (path includes version number)
- User switches between node version managers
- System packages are updated

**Where to change:**
- `cmd/installer/tasks.go` — wherever MCP server command is written to config
- `cmd/smolbot/runtime.go` — wherever onboarding writes MCP config
- Any other place that writes `MCPServerConfig.Command`

If the command is already bare (no `/` prefix), leave it alone. If it's an absolute path, extract the basename. If the installer is capturing the command from `which` or `exec.LookPath`, just store the basename instead.

### 2. Systemd unit generator should capture user's PATH at install time

**Change:** When the installer generates `~/.config/systemd/user/smolbot.service`, it should capture the user's current `$PATH` and write it into the unit file's `Environment=` directive.

**Why:** Systemd user services inherit a minimal PATH (`/usr/local/bin:/usr/bin:/bin`). Tools installed via fnm, nvm, volta, cargo, go, pip, etc. are typically in directories that are only in the user's interactive shell PATH. The daemon needs these paths to find commands like `node`, `python3`, `npx`, etc.

**Where to change:**
- `cmd/installer/tasks.go` — the `setupSystemd()` function (or equivalent) that writes the `.service` file

**Implementation:**
```go
// Capture current PATH at install time
currentPath := os.Getenv("PATH")

// Write into service unit
fmt.Fprintf(f, "Environment=PATH=%s\n", currentPath)
```

This captures whatever PATH the user has when they run the installer, which will include fnm/nvm/volta/cargo/go paths since the installer runs in an interactive shell.

### 3. `NewStdioTransport` should resolve commands using MCP env overlay

**Change:** In `pkg/mcp/stdio.go`, before calling `exec.CommandContext`, resolve bare commands (no `/` prefix) using `exec.LookPath` with the MCP server's `env` map overlaid on the process environment. This allows MCP configs to include a `PATH` override in their `env` that helps find the right binary.

**Why:** Even with the systemd PATH fix, individual MCP servers might need different environments. For example, one server might need a specific Python virtualenv. The `env` field in `MCPServerConfig` already supports arbitrary env vars — we should use them during command resolution too, not just pass them to the subprocess.

**Where to change:** `pkg/mcp/stdio.go` → `NewStdioTransport()`

**Implementation sketch:**
```go
func NewStdioTransport(ctx context.Context, command string, args []string, env map[string]string) (*StdioTransport, error) {
    // Resolve bare commands using the overlaid environment
    resolvedCommand := command
    if !strings.Contains(command, string(os.PathSeparator)) {
        // Build effective PATH: MCP env PATH (if set) takes precedence, then process PATH
        effectivePath := os.Getenv("PATH")
        if envPath, ok := env["PATH"]; ok {
            effectivePath = envPath + string(os.PathListSeparator) + effectivePath
        }
        if resolved, err := lookPathWithEnv(command, effectivePath); err == nil {
            resolvedCommand = resolved
        }
        // If resolution fails, let exec.CommandContext try anyway — it will produce
        // a clear "executable file not found in $PATH" error
    }

    procCtx, cancel := context.WithCancel(ctx)
    cmd := exec.CommandContext(procCtx, resolvedCommand, args...)
    // ... rest unchanged
}

func lookPathWithEnv(file, pathEnv string) (string, error) {
    for _, dir := range filepath.SplitList(pathEnv) {
        if dir == "" {
            dir = "."
        }
        path := filepath.Join(dir, file)
        if fi, err := os.Stat(path); err == nil && !fi.IsDir() && fi.Mode()&0111 != 0 {
            return path, nil
        }
    }
    return "", fmt.Errorf("%s not found in PATH", file)
}
```

### 4. Improve error messages for command-not-found

**Change:** When `cmd.Start()` fails with "no such file or directory" or "executable file not found", wrap the error with actionable context.

**Where:** `pkg/mcp/stdio.go` → `NewStdioTransport()`

**Example:**
```go
if err := cmd.Start(); err != nil {
    cancel()
    if errors.Is(err, exec.ErrNotFound) || strings.Contains(err.Error(), "no such file or directory") {
        return nil, fmt.Errorf("start mcp server %q: command %q not found — ensure it is installed and in PATH (current PATH: %s): %w",
            command, resolvedCommand, os.Getenv("PATH"), err)
    }
    return nil, fmt.Errorf("start mcp server %q: %w", command, err)
}
```

### 5. Installer update flow should refresh systemd PATH

**Change:** When the installer runs in **update mode** (not fresh install), it should also regenerate the systemd unit file with the current PATH, not just the binary.

**Why:** If the user upgrades node or changes their version manager, the next `installer update` should pick up the new PATH.

**Where:** `cmd/installer/tasks.go` — the update task list should include `setupSystemd()` (verify it already does; if not, add it).

---

## Testing

### Unit tests

1. **`pkg/mcp/stdio_test.go`** — Test `lookPathWithEnv`:
   - Finds binary in custom PATH
   - Returns error when binary not in PATH
   - Handles empty PATH gracefully
   - MCP env PATH prepended to process PATH

2. **`pkg/mcp/stdio_test.go`** — Test `NewStdioTransport` with bare command:
   - Verify bare `"echo"` resolves to `/usr/bin/echo` (or wherever it is)
   - Verify absolute path is used as-is
   - Verify MCP `env["PATH"]` override is respected

### Integration test

3. **`cmd/smolbot/runtime_tools_test.go`** — Test MCP config with bare command:
   - Configure an MCP server with `"command": "echo"` (bare name)
   - Verify `DiscoverAndRegister` doesn't fail with "not found"

### Manual verification

After making changes:
```bash
# 1. Rebuild and deploy
go build -o ~/.local/bin/smolbot ./cmd/smolbot/

# 2. Verify config uses bare command
cat ~/.smolbot/config.json | grep '"command"'
# Should show: "command": "node"

# 3. Verify systemd unit has PATH
cat ~/.config/systemd/user/smolbot.service | grep PATH
# Should show: Environment=PATH=/home/nomadx/.local/share/fnm/...:/usr/local/bin:...

# 4. Restart and verify
systemctl --user daemon-reload
systemctl --user restart smolbot
systemctl --user status smolbot
# Should show: active (running), node subprocess visible in CGroup

# 5. Check logs for successful MCP discovery
journalctl --user -u smolbot -n 10 --no-pager
# Should show: "INFO discovered mcp tools name=hybrid-memory count=7"
```

---

## What NOT to change

- **Do not modify `pkg/mcp/client.go`** — the `DiscoverAndRegister` logic and `ConnectionSpec` are correct
- **Do not modify `pkg/mcp/discovery.go`** — the `getOrConnect` / `Discover` / `Invoke` pipeline is correct
- **Do not change the MCP protocol handling** — JSON-RPC 2.0, `initialize`, `tools/list`, `tools/call` are all working
- **Do not change the warn-and-continue behavior** in `runtime.go` lines 580-581 — MCP failure should remain non-fatal
- **Do not revert the current config** — `"command": "node"` is correct, keep it bare
- **Do not revert the current systemd PATH** — it's a valid fix, just make the installer generate it automatically

---

## Priority

This is a **P1 reliability fix**. Without it, the daemon crash-loops after every reboot on any system using fnm, nvm, or similar version managers. The temporary fix is holding, but it's fragile (hardcoded PATH, breaks on node version upgrade).

## Branches

- Work off `mcp-startup-hardening` branch (already has the non-fatal MCP error handling and the full stdio transport implementation)
- The main branch may not build cleanly due to pending worktree merges — do not use main as base
