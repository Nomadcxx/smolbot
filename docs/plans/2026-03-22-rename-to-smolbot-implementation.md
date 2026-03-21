# Rename nanobot-go to smolbot - Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Rename the entire project from "nanobot-go" to "smolbot", including module name, binaries, config paths, service names, and all references.

**Architecture:** Rename Go module, update all import paths, rename cmd directories, update binary names, change config directory from ~/.nanobot to ~/.smolbot, update service name to smolbot.service, and update all branding/references.

**Tech Stack:** Go 1.26, systemd, WebSocket gateway, TUI

---

## Pre-Implementation Checklist

- [ ] Backup any unsaved work
- [ ] Ensure git is clean (commit or stash changes)
- [ ] Verify working directory is /home/nomadx/nanobot-go

---

## Task 1: Update go.mod Module Name

**Files:**
- Modify: `go.mod:1`

**Step 1: Change module name**

```bash
sed -i 's|module github.com/Nomadcxx/nanobot-go|module github.com/Nomadcxx/smolbot|' go.mod
```

**Step 2: Verify change**

```bash
head -1 go.mod
```
Expected: `module github.com/Nomadcxx/smolbot`

**Step 3: Commit**

```bash
git add go.mod
git commit -m "chore: rename module to smolbot"
```

---

## Task 2: Update All Import Paths

**Files:**
- Modify: All .go files (~100 files)

**Step 1: Update imports across all Go files**

```bash
find . -name "*.go" -type f -exec sed -i 's|github.com/Nomadcxx/nanobot-go|github.com/Nomadcxx/smolbot|g' {} +
```

**Step 2: Verify no old imports remain**

```bash
grep -r "github.com/Nomadcxx/nanobot-go" --include="*.go" . || echo "No old imports found"
```

**Step 3: Commit**

```bash
git add -A
git commit -m "chore: update all import paths"
```

---

## Task 3: Rename cmd Directories

**Files:**
- Rename: `cmd/nanobot/` → `cmd/smolbot/`
- Rename: `cmd/nanobot-tui/` → `cmd/smolbot-tui/`

**Step 1: Rename directories**

```bash
cd /home/nomadx/nanobot-go
mv cmd/nanobot cmd/smolbot
mv cmd/nanobot-tui cmd/smolbot-tui
```

**Step 2: Verify renames**

```bash
ls -la cmd/
```
Expected: `smolbot/`, `smolbot-tui/`, `installer/`

**Step 3: Commit**

```bash
git add -A
git commit -m "chore: rename cmd directories"
```

---

## Task 4: Update Binary Names in Code

**Files:**
- Modify: `cmd/installer/tasks.go:40,42,46,58,60,64,84,107`
- Modify: `cmd/installer/utils.go:100`
- Modify: `cmd/installer/views.go:420,424,451,452`
- Modify: `cmd/smolbot/root.go:18,19` (was nanobot/root.go)
- Modify: `cmd/smolbot/run.go:14` (was nanobot/run.go)

**Step 1: Update binary names in installer**

```bash
sed -i 's/"nanobot"/"smolbot"/g' cmd/installer/tasks.go
sed -i 's/"nanobot-tui"/"smolbot-tui"/g' cmd/installer/tasks.go
sed -i 's/\.local", "bin", "nanobot"/".local", "bin", "smolbot"/' cmd/installer/utils.go
```

**Step 2: Update CLI command names**

```bash
sed -i 's/Use: "nanobot"/Use: "smolbot"/' cmd/smolbot/root.go
sed -i 's/Short: "nanobot daemon/Short: "smolbot daemon/' cmd/smolbot/root.go
sed -i 's/Short: "Start the nanobot daemon"/Short: "Start the smolbot daemon"/' cmd/smolbot/run.go
```

**Step 3: Update quick start guide**

```bash
sed -i 's/nanobot-tui/smolbot-tui/g' cmd/installer/views.go
sed -i 's/nanobot run/smolbot run/g' cmd/installer/views.go
sed -i 's/~\/.local\/bin\/nanobot/~\/.local\/bin\/smolbot/g' cmd/installer/views.go
```

**Step 4: Commit**

```bash
git add -A
git commit -m "chore: update binary names"
```

---

## Task 5: Update Config Paths

**Files:**
- Modify: `pkg/config/paths.go:22,25`
- Modify: `pkg/config/config.go:111,123,127`
- Modify: `cmd/installer/main.go:31,32`
- Modify: `cmd/installer/tasks.go:152,188,193,449,472`
- Modify: `cmd/installer/utils.go:120,126`
- Modify: `cmd/installer/views.go:436,454,456`

**Step 1: Update default config directory**

```bash
sed -i 's/\.nanobot/.smolbot/g' pkg/config/paths.go
sed -i 's/\.nanobot/.smolbot/g' pkg/config/config.go
```

**Step 2: Update installer paths**

```bash
sed -i 's/\.nanobot/.smolbot/g' cmd/installer/main.go
sed -i 's/Creating ~\/.nanobot/Creating ~\/.smolbot/' cmd/installer/tasks.go
sed -i 's/Removing ~\/.nanobot/Removing ~\/.smolbot/' cmd/installer/tasks.go
```

**Step 3: Update remaining paths (but keep legacy migration)**

```bash
# Keep nanobot-go for legacy migration in utils.go
sed -i 's/\.nanobot-go/.smolbot-go/' cmd/installer/utils.go
```

**Step 4: Update views**

```bash
sed -i 's/nano ~\/.nanobot/nano ~\/.smolbot/' cmd/installer/views.go
sed -i 's/~\/.nanobot/~\/.smolbot/g' cmd/installer/views.go
```

**Step 5: Commit**

```bash
git add -A
git commit -m "chore: update config paths"
```

---

## Task 6: Update Service Names

**Files:**
- Modify: `cmd/installer/tasks.go:310,315,324,336,347,354,367,376,426,428,437,446,447,457`
- Modify: `cmd/installer/utils.go:114`
- Modify: `cmd/installer/views.go:428,430,432`

**Step 1: Update systemd service references**

```bash
sed -i 's/nanobot-go.service/smolbot.service/g' cmd/installer/tasks.go
sed -i 's/Description=nanobot-go/Description=smolbot/' cmd/installer/tasks.go
sed -i 's/nanobot-go"/smolbot"/g' cmd/installer/tasks.go
sed -i 's/nanobot-go"/smolbot"/g' cmd/installer/utils.go
```

**Step 2: Update views**

```bash
sed -i 's/nanobot-go/smolbot/g' cmd/installer/views.go
```

**Step 3: Commit**

```bash
git add -A
git commit -m "chore: update service names"
```

---

## Task 7: Update ASCII Art

**Files:**
- Rename: `internal/assets/NANOBOT.txt` → `internal/assets/SMOLBOT.txt`
- Modify: `internal/assets/header.go:5`

**Step 1: Replace ASCII art content**

```bash
cat > internal/assets/SMOLBOT.txt << 'EOF'
 ▄▄▄▄▄▄▄ ▄▄    ▄▄  ▄▄▄▄▄▄  ▄▄       ▄▄▄▄▄▄▄    ▄▄▄▄▄▄  ▄▄▄▄▄▄▄▄
██▀▀▀▀▀▀ ███▄▄███ ██▀▀▀▀██ ██       ██▀▀▀▀██  ██▀▀▀▀██ ▀▀▀██▀▀▀
▀██████▄ ██▀██▀██ ██    ██ ██       ████████  ██    ██    ██   
▄▄▄▄▄▄██ ██    ██ ██▄▄▄▄██ ██▄▄▄▄▄▄ ██▄▄▄▄██  ██▄▄▄▄██    ██   
▀▀▀▀▀▀▀  ▀▀    ▀▀  ▀▀▀▀▀▀  ▀▀▀▀▀▀▀▀ ▀▀▀▀▀▀▀    ▀▀▀▀▀▀     ▀▀   
EOF
rm -f internal/assets/NANOBOT.txt
```

**Step 2: Update embed reference**

```bash
sed -i 's/NANOBOT.txt/SMOLBOT.txt/' internal/assets/header.go
```

**Step 3: Commit**

```bash
git add -A
git commit -m "chore: update ASCII art"
```

---

## Task 8: Update Agent Identity and Templates

**Files:**
- Modify: `pkg/agent/context.go:17`
- Modify: `templates/SOUL.md:5`
- Modify: `templates/AGENTS.md:1`
- Modify: `cmd/smolbot/runtime.go:328`

**Step 1: Update identity strings**

```bash
sed -i 's/You are nanobot/You are smolbot/' pkg/agent/context.go
sed -i 's/You are nanobot/You are smolbot/' templates/SOUL.md
sed -i 's/You are nanobot/You are smolbot/' templates/AGENTS.md
sed -i 's/You are nanobot/You are smolbot/' cmd/smolbot/runtime.go
```

**Step 2: Commit**

```bash
git add -A
git commit -m "chore: update agent identity"
```

---

## Task 9: Update TUI References

**Files:**
- Modify: `internal/app/state.go:20`
- Modify: `internal/client/client.go:57`
- Modify: `internal/client/protocol_test.go:17`
- Modify: `internal/components/chat/messages.go:156`

**Step 1: Update TUI paths and client ID**

```bash
sed -i 's/nanobot-tui/smolbot-tui/' internal/app/state.go
sed -i 's/nanobot-tui/smolbot-tui/' internal/client/client.go
sed -i 's/nanobot-tui/smolbot-tui/' internal/client/protocol_test.go
sed -i 's/"nanobot"/"smolbot"/' internal/components/chat/messages.go
```

**Step 2: Commit**

```bash
git add -A
git commit -m "chore: update TUI references"
```

---

## Task 10: Update Server and Protocol

**Files:**
- Modify: `pkg/gateway/server.go:179`
- Modify: `pkg/config/config.go:126`
- Modify: `cmd/installer/tasks.go:192`

**Step 1: Update server identification**

```bash
sed -i 's/"server": "nanobot-go"/"server": "smolbot"/' pkg/gateway/server.go
sed -i 's/DeviceName: "nanobot-go"/DeviceName: "smolbot"/' pkg/config/config.go
sed -i 's/"deviceName": "nanobot-go"/"deviceName": "smolbot"/' cmd/installer/tasks.go
```

**Step 2: Commit**

```bash
git add -A
git commit -m "chore: update server and device names"
```

---

## Task 11: Update Channel Error Messages and Device Names

**Files:**
- Modify: `pkg/channel/whatsapp/adapter.go:225,318`
- Modify: `pkg/channel/signal/adapter.go:156`
- Modify: `pkg/channel/whatsapp/adapter_test.go`
- Modify: `pkg/channel/signal/adapter_test.go`

**Step 1: Update command references**

```bash
sed -i 's/nanobot channels login/smolbot channels login/' pkg/channel/whatsapp/adapter.go
sed -i 's/-n nanobot-go/-n smolbot/' pkg/channel/signal/adapter.go
```

**Step 2: Update test files**

```bash
sed -i 's/nanobot-go/smolbot/g' pkg/channel/whatsapp/adapter_test.go
sed -i 's/nanobot-go/smolbot/g' pkg/channel/signal/adapter_test.go
```

**Step 3: Commit**

```bash
git add -A
git commit -m "chore: update channel error messages"
```

---

## Task 12: Update Package Declaration

**Files:**
- Modify: `embed.go:1`

**Step 1: Update package name**

```bash
sed -i 's/package nanobotgo/package smolbot/' embed.go
```

**Step 2: Commit**

```bash
git add -A
git commit -m "chore: update package declaration"
```

---

## Task 13: Update Test Files

**Files:**
- Modify: `pkg/config/config_test.go`
- Modify: `cmd/smolbot/*_test.go` (all test files)
- Modify: `pkg/channel/whatsapp/adapter_test.go`
- Modify: `pkg/channel/signal/adapter_test.go`
- Modify: `pkg/gateway/server_test.go`
- Modify: `pkg/tool/web_test.go`
- Modify: `pkg/tool/filesystem_test.go`

**Step 1: Update test paths and names**

```bash
sed -i 's/\.nanobot/.smolbot/g' pkg/config/config_test.go
sed -i 's/nanobot-go/smolbot/g' pkg/channel/whatsapp/adapter_test.go
sed -i 's/nanobot-go/smolbot/g' pkg/channel/signal/adapter_test.go
sed -i 's/"server":"nanobot-go"/"server":"smolbot"/' pkg/gateway/server_test.go
sed -i 's/nanobot-test-agent/smolbot-test-agent/' pkg/tool/web_test.go
sed -i 's/nanobot/smolbot/' pkg/tool/filesystem_test.go
```

**Step 2: Commit**

```bash
git add -A
git commit -m "chore: update test files"
```

---

## Task 14: Update Shell Scripts

**Files:**
- Modify: `install.sh:2,8`
- Modify: `.gitignore`

**Step 1: Update install script**

```bash
sed -i 's/nanobot-go/smolbot/g' install.sh
sed -i 's/nanobot/smolbot/g' install.sh
```

**Step 2: Update .gitignore**

```bash
sed -i 's/^nanobot$/smolbot/' .gitignore
sed -i 's/\.\/nanobot-tui/.\/smolbot-tui/' .gitignore
```

**Step 3: Commit**

```bash
git add -A
git commit -m "chore: update scripts and gitignore"
```

---

## Task 15: Update Documentation Files

**Files:**
- Modify: `README.md`
- Modify: `docs/plans/*.md`
- Modify: `docs/skills/*.md`
- Modify: `docs/skill-system/*.md`

**Step 1: Update main documentation**

```bash
sed -i 's/nanobot-go/smolbot/g' README.md
sed -i 's/nanobot/smolbot/g' README.md
sed -i 's/~\/.nanobot/~\/.smolbot/g' README.md
```

**Step 2: Update plan documents (batch)**

```bash
for f in docs/plans/*.md; do
  sed -i 's/nanobot-go/smolbot/g' "$f"
  sed -i 's/nanobot/smolbot/g' "$f"
  sed -i 's/~\/.nanobot/~\/.smolbot/g' "$f"
done
```

**Step 3: Update skills docs**

```bash
for f in docs/skills/*.md docs/skill-system/*.md; do
  if [ -f "$f" ]; then
    sed -i 's/nanobot-go/smolbot/g' "$f"
    sed -i 's/nanobot/smolbot/g' "$f"
  fi
done
```

**Step 4: Commit**

```bash
git add -A
git commit -m "docs: update documentation"
```

---

## Task 16: Update Workspace Templates

**Files:**
- Modify: `cmd/installer/tasks.go:135-147`

**Step 1: Update SOUL.md generation in installer**

```bash
sed -i 's/You are a helpful AI coding assistant/You are smolbot, a practical coding assistant/' cmd/installer/tasks.go
```

**Step 2: Update HEARTBEAT.md generation (if it mentions nanobot)**

```bash
# Check if HEARTBEAT content needs update
grep -n "HEARTBEAT" cmd/installer/tasks.go
```

**Step 3: Commit**

```bash
git add -A
git commit -m "chore: update workspace templates"
```

---

## Task 17: Verify Build

**Step 1: Clean and build**

```bash
cd /home/nomadx/nanobot-go
go clean -cache
go build -o /tmp/smolbot-test ./cmd/smolbot
```

**Step 2: Build TUI**

```bash
go build -o /tmp/smolbot-tui-test ./cmd/smolbot-tui
```

**Step 3: Run tests**

```bash
go test ./... 2>&1 | head -50
```

**Step 4: Verify binaries**

```bash
/tmp/smolbot-test --help | head -5
/tmp/smolbot-tui-test --help | head -5
```

**Step 5: Final commit if all good**

```bash
git status
```

## Task 17: Update Workspace Template Generation

**Files:**
- Modify: `cmd/installer/tasks.go:135-147`

**Step 1: Update installer workspace templates**

Update the createWorkspace function to generate smolbot-branded files:

```bash
# Update SOUL.md content
cat > /tmp/soul_patch.txt << 'EOF'
# Agent Personality

You are smolbot, a practical coding assistant with a clear working style.

## Tone
- Be direct and calm.
- Prefer clarity over flourish.
- Stay technically precise and grounded in what is actually implemented.
EOF
```

**Step 2: Update the installer code**

```go
// In createWorkspace function, update:
soulContent := "# Agent Personality\n\nYou are smolbot, a practical coding assistant. You help users write code, debug issues, and understand complex systems.\n"
```

**Step 3: Commit**

```bash
git add -A
git commit -m "chore: update workspace templates"
```

---

## Verification Checklist

- [ ] Module name changed to smolbot
- [ ] All imports updated
- [ ] cmd directories renamed
- [ ] Binary names updated
- [ ] Config paths use ~/.smolbot
- [ ] Service name is smolbot.service
- [ ] ASCII art shows SMOLBOT
- [ ] Agent identity says smolbot
- [ ] TUI client ID is smolbot-tui
- [ ] Server reports smolbot
- [ ] WhatsApp device is smolbot
- [ ] Channel error messages updated
- [ ] Package declaration updated
- [ ] Tests updated
- [ ] Scripts updated
- [ ] Documentation updated
- [ ] Both binaries build successfully
- [ ] Tests pass

---

## Post-Implementation Notes

1. Update GitHub repository name after code changes
2. Notify users about breaking change
3. Legacy ~/.nanobot-go directory kept for migration
4. Users need to manually migrate or reinstall
5. TUI state will reset (new location)
