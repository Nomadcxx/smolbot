<div align="center">
  <img src="assets/smolbot_header.png" alt="SMOLBOT" width="600" />
</div>

---

> **AI-POWERED CODING ASSISTANT FOR YOUR TERMINAL**

A lightweight Go-based AI assistant that runs in your terminal. Chat with local or cloud AI models, manage coding sessions, and get help with your projects — all from the command line.

```bash
curl -sSL https://raw.githubusercontent.com/Nomadcxx/smolbot/main/install.sh | bash
```

## Features

- **Multiple AI Providers**: Ollama (local), OpenAI, Anthropic Claude, Azure OpenAI, custom OpenAI-compatible endpoints
- **TUI Interface**: Full-featured terminal UI for interactive chat sessions
- **Session Management**: Persistent chat history with SQLite backend
- **Workspace Integration**: Organized project workspaces with memory and context
- **Systemd Service**: Run as a background service with user-level systemd integration
- **Channel Support**: Optional Signal and WhatsApp integration for notifications
- **Tool Calling**: Extensible tool system for file operations, web search, and MCP servers

## Installation

### One-Liner

```bash
curl -sSL https://raw.githubusercontent.com/Nomadcxx/smolbot/main/install.sh | bash
```

### Manual

```bash
git clone https://github.com/Nomadcxx/smolbot.git
cd smolbot
go build -o install-smolbot ./cmd/installer
./install-smolbot
```

**Requirements:** Go 1.21+

## Usage

### Start the Daemon

```bash
nanobot run              # Start the API daemon
nanobot run --daemon     # Run as background daemon
```

### Interactive TUI

```bash
nanobot-tui              # Launch the TUI client
```

### CLI Commands

```bash
# Quick chat from command line
nanobot chat "Explain Go interfaces"

# Check daemon status
nanobot status

# Stop the daemon
nanobot stop

# View logs
nanobot logs

# Reconfigure settings
nanobot onboard
```

## Configuration

Configuration lives at `~/.nanobot/config.json`

```json
{
  "agents": {
    "defaults": {
      "model": "llama3.1:8b",
      "provider": "ollama",
      "workspace": "/home/user/.nanobot/workspace",
      "maxTokens": 8192,
      "temperature": 0.7
    }
  },
  "providers": {
    "ollama": {
      "apiBase": "http://localhost:11434/v1"
    }
  },
  "gateway": {
    "host": "127.0.0.1",
    "port": 18791
  }
}
```

### Providers

**Ollama (Local)**
```json
"providers": {
  "ollama": {
    "apiBase": "http://localhost:11434/v1"
  }
}
```

**OpenAI**
```json
"providers": {
  "openai": {
    "apiKey": "sk-...",
    "apiBase": "https://api.openai.com/v1"
  }
}
```

**Anthropic**
```json
"providers": {
  "anthropic": {
    "apiKey": "sk-ant-..."
  }
}
```

## Workspace Structure

```
~/.nanobot/
├── config.json          # Main configuration
├── sessions.db          # Chat history database
└── workspace/
    ├── memory/          # Agent memory files
    ├── SOUL.md          # Agent personality definition
    └── HEARTBEAT.md     # Scheduled task instructions
```

## Service Management

The installer sets up a systemd user service:

```bash
# Check status
systemctl --user status nanobot-go

# Stop service
systemctl --user stop nanobot-go

# Restart service
systemctl --user restart nanobot-go

# View logs
journalctl --user -u nanobot-go -f
```

## Architecture

```
┌─────────────┐     ┌─────────────┐     ┌─────────────┐
│  nanobot    │────▶│   API GW    │────▶│    TUI      │
│  (daemon)   │     │  (port      │     │  (client)   │
│             │◀────│   18791)    │◀────│             │
└─────────────┘     └─────────────┘     └─────────────┘
       │
       ▼
┌─────────────┐     ┌─────────────┐
│  SQLite DB  │     │   Config    │
│  (sessions) │     │   (JSON)    │
└─────────────┘     └─────────────┘
```

## Development

```bash
# Build daemon
make build

# Build TUI
cd cmd/nanobot-tui && go build

# Run tests
make test

# Build installer
go build -o install-smolbot ./cmd/installer
```

## License

BSD 3-Clause License - see LICENSE file for details
