# smolbot Installer Specification

**Date:** 2026-03-21  
**Status:** Revised  
**Security Model:** User-local only (NO root/system-wide)

---

## 1. Overview

**Purpose:** Build a TUI-based installer for smolbot that guides users through setup without requiring root privileges.

**Key Principles:**
- User-local installation only (no root, no system-wide)
- Binaries to `~/.local/bin`
- Config to `~/.smolbot/`
- Workspace to `~/.smolbot/workspace/`
- Systemd **user** service (not system service)

---

## 2. Architecture

### 2.1 Two-Stage Installation

```
Stage 1: Bootstrap Shell Script
├── Checks prerequisites (Go, Git)
├── Creates temp directory with trap cleanup
├── Clones/builds installer binary
└── Runs installer

Stage 2: Bubble Tea TUI Installer
└── Wizard: Welcome → Provider → Model → Channels → Service → Install → Complete
```

### 2.2 Security Model

| Component | Location | Permissions |
|-----------|----------|-------------|
| Binaries | `~/.local/bin/` | 0755 (executable by user) |
| Config | `~/.smolbot/config.json` | 0644 (user readable) |
| Workspace | `~/.smolbot/workspace/` | 0755 |
| Sessions DB | `~/.smolbot/sessions.db` | 0644 |
| Skills | `~/.smolbot/skills/` | 0755 |
| Systemd | `~/.config/systemd/user/` | 0755 |

**No root required** - everything under user's home directory.

---

## 3. Installer Steps

### Step 1: Welcome
- Detect existing installation (config exists at `~/.smolbot/config.json`)
- Options:
  - **Fresh Install** - New installation
  - **Update** - Upgrade existing installation (preserve config)
  - **Reconfigure** - Modify existing config
  - **Uninstall** - Remove smolbot

### Step 2: Provider Selection
- Auto-detect available providers:
  - **OpenAI** - Requires API key
  - **Anthropic** - Requires API key
  - **Ollama** - Local, auto-detect running instance
  - **Azure OpenAI** - Requires endpoint + key
  - **Custom** - Generic OpenAI-compatible
- Show Ollama status if available (running/not running)

### Step 3: Model Configuration
- If **Cloud provider selected:**
  - List available models for provider
  - Allow custom model name
  - Show pricing info if available

- If **Ollama selected:**
  - Detect running Ollama at `localhost:11434`
  - List installed models via `ollama list`
  - Allow pulling new models
  - Show recommended models

### Step 4: Ollama Detection (if applicable)
- Check if Ollama is installed: `which ollama`
- Check if Ollama is running: `curl localhost:11434/api/tags`
- If not running: Offer to start Ollama
- If no models: Offer to pull a default model
- Show connection URL (default: `http://localhost:11434/v1`)

### Step 5: Channel Setup (Optional)
Signal and WhatsApp configuration - can be skipped.

**Signal:**
- Check if `signal-cli` is installed
- If not: Show installation instructions
- Offer to link device (requires QR code interaction)
- Store: `~/.smolbot/signal/`

**WhatsApp:**
- Check if whatsmeow-compatible setup possible
- Show instructions for WhatsApp linking
- Store: `~/.smolbot/whatsapp.db`

### Step 6: Service Setup
- Offer to create systemd user service
- Service file: `~/.config/systemd/user/smolbot.service`
- Options:
  - Enable on install: Yes/No
  - Start now: Yes/No
  - Gateway port: (default 18791)

### Step 7: Installation
Tasks:
1. Create directories (`~/.smolbot/`, `~/.local/bin/`)
2. Build binaries (if building from source) OR copy pre-built
3. Write config file
4. Create systemd user service
5. Enable/start service (if selected)

### Step 8: Complete
- Show success message
- Show commands to run:
  - `smolbot run` - Start daemon
  - `smolbot-tui` - Start TUI client
  - `smolbot status` - Check status
- Show workspace location

---

## 4. Configuration File Format

**Location:** `~/.smolbot/config.json`

```json
{
  "agents": {
    "defaults": {
      "model": "qwen2.5-coder:7b",
      "provider": "ollama",
      "workspace": "/home/user/.smolbot/workspace",
      "maxTokens": 8192,
      "contextWindowTokens": 128000,
      "temperature": 0.7,
      "maxToolIterations": 40
    }
  },
  "providers": {
    "ollama": {
      "apiBase": "http://localhost:11434/v1"
    },
    "openai": {
      "apiKey": ""
    }
  },
  "channels": {
    "sendProgress": true,
    "sendToolHints": true,
    "signal": {
      "enabled": false,
      "account": "",
      "cliPath": "signal-cli",
      "dataDir": "/home/user/.smolbot/signal"
    },
    "whatsapp": {
      "enabled": false,
      "deviceName": "smolbot",
      "storePath": "/home/user/.smolbot/whatsapp.db"
    }
  },
  "gateway": {
    "host": "127.0.0.1",
    "port": 18791,
    "heartbeat": {
      "enabled": true,
      "interval": 60,
      "channel": ""
    }
  },
  "tools": {
    "web": {
      "searchBackend": "duckduckgo",
      "maxResults": 5
    },
    "exec": {
      "defaultTimeout": 60,
      "maxTimeout": 600,
      "denyPatterns": ["rm -rf /", "dd if="],
      "restrictToWorkspace": true
    },
    "mcpServers": {}
  }
}
```

---

## 5. Systemd User Service

**File:** `~/.config/systemd/user/smolbot.service`

```ini
[Unit]
Description=Nanobot-Go AI Assistant
After=network.target

[Service]
Type=simple
ExecStart=%h/.local/bin/smolbot run --config %h/.smolbot/config.json --workspace %h/.smolbot/workspace --port 18791
Restart=on-failure
RestartSec=5s
Environment=HOME=%h

[Install]
WantedBy=default.target
```

**Commands:**
```bash
systemctl --user daemon-reload
systemctl --user enable smolbot
systemctl --user start smolbot
```

---

## 6. Directory Structure

```
~/.smolbot/
├── config.json          # Main config
├── sessions.db          # Chat sessions
├── workspace/
│   ├── memory/          # Agent memory
│   ├── SOUL.md          # Agent personality
│   └── HEARTBEAT.md     # Heartbeat instructions
├── skills/              # Custom skills
└── signal/              # Signal data (if enabled)

~/.local/bin/
├── smolbot              # Daemon + CLI
└── smolbot-tui          # TUI client

~/.config/systemd/user/
└── smolbot.service
```

---

## 7. Upgrade Flow

When existing installation detected:

1. Check current version from binary
2. Stop running daemon (`systemctl --user stop smolbot` or kill process)
3. Backup config: `~/.smolbot/config.json → ~/.smolbot/config.json.bak`
4. Install new binary
5. Ask: Keep existing config or re-run setup?
6. Restart service if it was running

---

## 8. Uninstall Flow

1. Stop service: `systemctl --user stop smolbot`
2. Disable service: `systemctl --user disable smolbot`
3. Remove binaries: `rm ~/.local/bin/smolbot ~/.local/bin/smolbot-tui`
4. Ask: Remove config/workspace?
   - Keep config/workspace
   - Remove all `~/.smolbot/`

---

## 9. Ollama Integration

### Auto-Detection
```bash
# Check if ollama command exists
which ollama

# Check if running
curl -s http://localhost:11434/api/tags 2>/dev/null | jq -r '.models[].name'

# Get API base (add /v1 if missing)
```

### Model Listing
Parse output of `ollama list`:
```
NAME                           ID           SIZE      MODIFIED    
qwen2.5-coder:7b               a8c5e8...    4.7GB     2 days ago
llama3.1:8b                    b7a3e1...    4.9GB     1 week ago
```

### Pulling Models
```bash
ollama pull <model-name>
```

---

## 10. Error Handling

| Error | Handling |
|-------|----------|
| Go not installed | Show error with install instructions |
| Git not installed | Show error with install instructions |
| Build fails | Show error with build log location |
| Config write fails | Show error, retry option |
| Systemd user service fails | Show warning, explain manual setup |
| Ollama not running | Offer to start it |
| Ollama has no models | Offer to pull default |

---

## 11. Implementation Structure

```
cmd/installer/
├── main.go           # Entry, model, Init, Update, main
├── types.go          # Types, constants, step enum
├── tasks.go          # Installation tasks
├── view.go           # Main View() renderer  
├── screens.go        # Step-specific renderers
├── inputs.go         # Text input handling
├── ollama.go         # Ollama detection and model listing
├── utils.go          # Path helpers, config read/write
├── theme.go          # Colors and styles
├── update.go         # Upgrade/backup logic
└── uninstall.go      # Uninstall tasks
```

---

## 12. Reference Implementations

- **Bubble Tea patterns:** `/home/nomadx/Documents/jellywatch/cmd/installer/`
- **sysc-greet patterns:** `/home/nomadx/go/sysc-greet/cmd/installer/main.go`
- **Existing onboard flow:** `/home/nomadx/smolbot/cmd/smolbot/onboard.go`
- **Config structure:** `/home/nomadx/smolbot/pkg/config/config.go`

---

## 13. Bootstrap Script

```bash
#!/bin/bash
# smolbot one-line installer
# Usage: curl -fsSL https://raw.githubusercontent.com/Nomadcxx/smolbot/main/install.sh | bash

set -e

echo "smolbot installer"
echo ""

# Check prerequisites
command -v go >/dev/null || { echo "Error: Go is not installed"; exit 1; }
command -v git >/dev/null || { echo "Error: Git is not installed"; exit 1; }

# Create temp directory with cleanup trap
TEMP_DIR=$(mktemp -d)
trap "rm -rf '$TEMP_DIR'" EXIT

cd "$TEMP_DIR"

echo "Cloning smolbot..."
git clone --depth 1 https://github.com/Nomadcxx/smolbot.git
cd smolbot

echo "Building installer..."
go build -o install-smolbot ./cmd/installer/

echo "Starting installer..."
exec ./install-smolbot
```

---

## 14. Tasks (Implementation Order)

### Phase 1: Foundation
1. Create `cmd/installer/` directory structure
2. Write `types.go` - Model, Step enum, Task types
3. Write `theme.go` - Colors matching smolbot-tui
4. Write `utils.go` - Path helpers, config read/write

### Phase 2: Core Installer
5. Write `main.go` - Entry, Init, Update, main loop
6. Write `screens.go` - Welcome, Provider, Model, Channels, Service screens
7. Write `inputs.go` - Text input handling
8. Write `view.go` - Main View() compositor

### Phase 3: Ollama Integration
9. Write `ollama.go` - Ollama detection, model listing, pull

### Phase 4: Installation Tasks
10. Write `tasks.go` - Directory creation, binary build, config write, service setup

### Phase 5: Upgrade/Uninstall
11. Write `update.go` - Upgrade detection, backup, restore
12. Write `uninstall.go` - Clean removal

### Phase 6: Bootstrap
13. Write `install.sh` - Bootstrap shell script

### Phase 7: Testing
14. Write tests for core functions
15. Build and test installer
