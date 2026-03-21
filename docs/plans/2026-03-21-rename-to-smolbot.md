# Implementation Plan: Rename nanobot-go to smolbot

## Overview
Rename the project from "nanobot-go" to "smolbot" across all files, directories, and configurations.

## Phase 1: Module and Import Path Changes

### 1.1 Update go.mod
**File:** `go.mod`
- Change: `module github.com/Nomadcxx/nanobot-go`
- To: `module github.com/Nomadcxx/smolbot`

### 1.2 Update All Import Paths
**Files:** All `.go` files (100+ files)
- Change: `github.com/Nomadcxx/nanobot-go/...`
- To: `github.com/Nomadcxx/smolbot/...`

**Key directories:**
- `cmd/nanobot/` → `cmd/smolbot/`
- `cmd/nanobot-tui/` → `cmd/smolbot-tui/`
- `pkg/**/*`
- `internal/**/*`

## Phase 2: Binary and Command Names

### 2.1 Rename Directories
```bash
mv cmd/nanobot cmd/smolbot
mv cmd/nanobot-tui cmd/smolbot-tui
```

### 2.2 Update Binary Names in Code
**Files to update:**
- `cmd/installer/tasks.go`: Lines 40, 42, 46, 58, 60, 64, 84, 107
- `cmd/installer/utils.go`: Line 100
- `cmd/installer/views.go`: Lines 420, 424, 451, 452
- `cmd/smolbot/root.go` (was nanobot/root.go): Lines 18, 19
- `cmd/smolbot/run.go` (was nanobot/run.go): Line 14

**Changes:**
- `"nanobot"` → `"smolbot"`
- `"nanobot-tui"` → `"smolbot-tui"`

## Phase 3: Config Paths

### 3.1 Update Default Config Directory
**File:** `pkg/config/paths.go`
- Change: `~/.nanobot`
- To: `~/.smolbot`

### 3.2 Update All Config Path References
**Files:**
- `pkg/config/paths.go`: Lines 22, 25
- `pkg/config/config.go`: Lines 111, 123, 127
- `cmd/installer/main.go`: Lines 31, 32
- `cmd/installer/tasks.go`: Lines 152, 188, 193, 449, 472
- `cmd/installer/utils.go`: Line 120
- `cmd/installer/views.go`: Lines 436, 454, 456

**Changes:**
- `~/.nanobot` → `~/.smolbot`
- Keep `~/.nanobot-go` references for legacy migration

## Phase 4: Systemd Service Names

### 4.1 Update Service Names
**File:** `cmd/installer/tasks.go`

**Changes:**
- `nanobot-go.service` → `smolbot.service`
- `Description=nanobot-go` → `Description=smolbot`
- `ExecStart=%s/.local/bin/nanobot` → `ExecStart=%s/.local/bin/smolbot`
- Service commands: `nanobot-go` → `smolbot`

**Lines:** 310, 315, 324, 336, 347, 354, 367, 376, 426, 428, 437, 446, 447, 457

### 4.2 Update Service References
**Files:**
- `cmd/installer/utils.go`: Line 114
- `cmd/installer/views.go`: Lines 428, 430, 432

## Phase 5: ASCII Art and Branding

### 5.1 Update ASCII Art File
**File:** `internal/assets/NANOBOT.txt`
- Rename to: `internal/assets/SMOLBOT.txt`
- Replace content with ASCII from `/home/nomadx/SMOLBOT.txt`

### 5.2 Update Embed Reference
**File:** `internal/assets/header.go`
- Change: `//go:embed NANOBOT.txt`
- To: `//go:embed SMOLBOT.txt`

## Phase 6: Agent Identity and Personality

### 6.1 Update Agent Identity
**Files:**
- `pkg/agent/context.go`: Line 17 - `DefaultIdentityBlock`
- `templates/SOUL.md`: Line 5
- `templates/AGENTS.md`: Line 1
- `cmd/smolbot/runtime.go` (was nanobot/runtime.go): Line 328

**Changes:**
- `"You are nanobot"` → `"You are smolbot"`

### 6.2 Update Workspace Template Files
**File:** `cmd/installer/tasks.go` - Lines 135-147

The installer creates default SOUL.md and HEARTBEAT.md files. Update the content:

**Current SOUL.md content:**
```go
soulContent := "# Agent Personality\n\nYou are a helpful AI coding assistant..."
```

**Should reference smolbot instead of generic assistant.

**Current HEARTBEAT.md content:**
```go
heartbeatContent := "# Heartbeat Instructions\n\nPeriodic status checks..."
```

**Consider adding smolbot branding to these generated files.

## Phase 7: TUI and Client References

### 7.1 Update TUI State Path
**File:** `internal/app/state.go`: Line 20
- Change: `filepath.Join(dir, "nanobot-tui", "state.json")`
- To: `filepath.Join(dir, "smolbot-tui", "state.json")`

### 7.2 Update Client Identification
**File:** `internal/client/client.go`: Line 57
- Change: `Client: "nanobot-tui"`
- To: `Client: "smolbot-tui"`

### 7.3 Update Chat Display Name
**File:** `internal/components/chat/messages.go`: Line 156
- Change: `.Render("nanobot")`
- To: `.Render("smolbot")`

### 7.4 Update Test Files
**File:** `internal/client/protocol_test.go`: Line 17
- Change: `Client: "nanobot-tui"`
- To: `Client: "smolbot-tui"`

## Phase 8: Server, Channels, and Protocol

### 8.1 Update Server Identification
**File:** `pkg/gateway/server.go`: Line 179
- Change: `"server": "nanobot-go"`
- To: `"server": "smolbot"`

### 8.2 Update WhatsApp Device Name
**Files:**
- `pkg/config/config.go`: Line 126
- `cmd/installer/tasks.go`: Line 192

**Change:**
- `"nanobot-go"` → `"smolbot"`

### 8.3 Update Channel Error Messages
**File:** `pkg/channel/whatsapp/adapter.go`
- Lines 225, 318: Update command references
- Change: `nanobot channels login whatsapp`
- To: `smolbot channels login whatsapp`

### 8.4 Update Signal CLI Device Name
**File:** `pkg/channel/signal/adapter.go`: Line 156
- Change: `"nanobot-go"` in link command
- To: `"smolbot"`

## Phase 9: Package Declaration

### 9.1 Update Package Name
**File:** `embed.go`
- Change: `package nanobotgo`
- To: `package smolbot`

## Phase 10: Shell Scripts and Documentation

### 10.1 Update install.sh
**File:** `install.sh`
- Line 2: Comment change
- Line 8: Echo text change

### 10.2 Update .gitignore
**File:** `.gitignore`
- `nanobot` → `smolbot`
- `./nanobot-tui` → `./smolbot-tui`

### 10.3 Update README.md
Update all command examples and references.

## Phase 13: Documentation Files (Additional)

### 13.1 Update Plan Documents
**Files:**
- `docs/plans/2026-03-21-installer-design.md`
- `docs/plans/2026-03-21-installer-implementation.md`
- `docs/plans/2026-03-21-progressive-disclosure-skills.md`
- `docs/plans/2026-03-21-installer-ui-fix.md`

### 13.2 Update Skills Documentation
**Files:**
- `docs/skills/creating-skills.md`
- `docs/skill-system/*.md` (all files)

### 13.3 Update Skill Definitions
**Files:**
- `skills/*/SKILL.md` (all skill documentation files)

## Phase 11: Test Files

### 11.1 Update Test Configurations
**Files with test data:**
- `pkg/config/config_test.go`
- `cmd/smolbot/onboard_test.go` (was nanobot/onboard_test.go)
- `cmd/smolbot/onboard_validation_test.go`
- `cmd/smolbot/runtime_test.go`
- `pkg/channel/whatsapp/adapter_test.go`
- `pkg/channel/signal/adapter_test.go`
- `pkg/gateway/server_test.go`
- `pkg/tool/web_test.go`
- `pkg/tool/filesystem_test.go`
- `cmd/smolbot/chat_readline_test.go`
- `cmd/smolbot/chat_test.go`
- `cmd/smolbot/runtime_chat_test.go`

**Changes:**
- Test paths: `~/.nanobot` → `~/.smolbot`
- Test device names: `nanobot-go` → `smolbot`
- Test agent names: `nanobot-test-agent` → `smolbot-test-agent`

## Phase 12: Error Messages and Comments

### 12.1 Update Error Messages
**File:** `pkg/channel/whatsapp/adapter.go`
- Lines 225, 318: Command references

### 12.2 Update Comments
**File:** `embed.go`: Line 5
**File:** `pkg/skill/registry.go`: Lines 32, 43

## Execution Order

1. **Phase 1**: Module and imports (affects all files)
2. **Phase 2**: Binary names and directory renames
3. **Phase 3**: Config paths
4. **Phase 4**: Service names
5. **Phase 5**: ASCII art
6. **Phase 6**: Agent identity
7. **Phase 7**: TUI references
8. **Phase 8**: Server/protocol
9. **Phase 9**: Package declaration
10. **Phase 10**: Scripts and docs
11. **Phase 11**: Test files
12. **Phase 12**: Error messages

## Verification Steps

1. Build both binaries: `go build ./cmd/smolbot` and `go build ./cmd/smolbot-tui`
2. Run tests: `go test ./...`
3. Test installer: `./install-smolbot` and verify uninstall removes `~/.smolbot`
4. Verify config is written to `~/.smolbot/config.json`
5. Verify service is named `smolbot.service`
6. Verify ASCII art displays "SMOLBOT"

## Runtime Implications and Migration Strategy

### User Data Migration
**Issue:** Existing users have data in `~/.nanobot/` and `~/.nanobot-go/`
**Solution:** 
- The uninstaller already removes both legacy directories
- Fresh installs will use `~/.smolbot/` only
- Users must manually migrate or reinstall

### Service Conflicts
**Issue:** Existing systemd service `nanobot-go.service` may conflict with new `smolbot.service`
**Solution:**
- The uninstaller stops and disables the old service
- New service uses different name `smolbot.service`
- No automatic migration of running services

### TUI State Migration
**Issue:** TUI state stored in `~/.config/nanobot-tui/state.json`
**Solution:**
- New TUI uses `~/.config/smolbot-tui/state.json`
- Users lose session history and settings
- Consider adding migration prompt in installer

### Database Compatibility
**Good news:** No database schema changes needed
- SQLite database format remains compatible
- Only file paths change

### API Compatibility
**Good news:** Gateway protocol unchanged
- WebSocket endpoint remains `/ws`
- Frame protocol identical
- Client identification string changes but doesn't affect protocol

## Notes

- Keep `~/.nanobot-go` references for legacy migration support
- The repository URL already points to `smolbot` - no change needed
- Update GitHub repository name after code changes are complete
- Consider adding migration assistant in future release
