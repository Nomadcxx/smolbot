<div align="center">
  <img src="assets/smolbot_header.png" alt="SMOLBOT" width="600" />
  
  <p>
    <a href="https://github.com/Nomadcxx/smolbot/releases"><img src="https://img.shields.io/github/v/release/Nomadcxx/smolbot?include_prereleases&style=flat-square" alt="Release"></a>
    <img src="https://img.shields.io/badge/go-≥1.21-blue?style=flat-square" alt="Go">
    <a href="LICENSE"><img src="https://img.shields.io/badge/license-BSD--3--Clause-blue?style=flat-square" alt="License"></a>
  </p>
</div>

A terminal-based AI assistant that runs on your own hardware. Chat with local or cloud AI models, manage persistent sessions, and automate tasks without relying on external services.

```bash
curl -sSL https://raw.githubusercontent.com/Nomadcxx/smolbot/main/install.sh | bash
```

## What It Does

SMOLBOT provides a self-hosted AI assistant that lives in your terminal. Unlike cloud-based assistants, you control the entire stack: the AI provider, your data, and where the assistant runs.

**Core capabilities:**

- **Local-first**: Run entirely offline with Ollama, or connect to cloud providers
- **Persistent sessions**: Conversations saved to SQLite, resume anytime
- **Multi-provider**: Switch between Ollama, OpenAI, Anthropic, Azure, or custom endpoints
- **TUI interface**: Full terminal UI for interactive chat
- **Channel integration**: Optional Signal/WhatsApp for notifications
- **Tool system**: Extensible tools for file operations and MCP servers

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

## Quick Start

**1. Initialize**

The installer walks you through configuration:

```bash
./install-smolbot
```

Or if already installed:

```bash
smolbot onboard
```

**2. Configure** (`~/.smolbot/config.json`)

Minimal config for Ollama (local):

```json
{
  "agents": {
    "defaults": {
      "model": "llama3.1:8b",
      "provider": "ollama"
    }
  },
  "providers": {
    "ollama": {
      "apiBase": "http://localhost:11434/v1"
    }
  }
}
```

**3. Start**

```bash
# Terminal 1: Start the gateway
smolbot run

# Terminal 2: Launch TUI
smolbot-tui
```

## Configuration

### Providers

| Provider | Config | Auth |
|----------|--------|------|
| **Ollama** (local) | `apiBase: "http://localhost:11434/v1"` | None required |
| **OpenAI** | `apiKey: "sk-..."` | API key |
| **Anthropic** | `apiKey: "sk-ant-..."` | API key |
| **Azure** | `apiKey: "...", endpoint: "..."` | API key + endpoint |
| **Custom** | `apiBase: "...", apiKey: "..."` | OpenAI-compatible |

**Example - OpenAI:**

```json
{
  "providers": {
    "openai": {
      "apiKey": "sk-..."
    }
  },
  "agents": {
    "defaults": {
      "model": "gpt-4",
      "provider": "openai"
    }
  }
}
```

### Channels

Optional messaging integrations:

| Channel | Setup | Purpose |
|---------|-------|---------|
| **Signal** | signal-cli required | Notifications |
| **WhatsApp** | QR code scan | Notifications |

Enable in `config.json`:

```json
{
  "channels": {
    "signal": {
      "enabled": true,
      "cliPath": "signal-cli"
    },
    "whatsapp": {
      "enabled": true
    }
  }
}
```

### Tools

Built-in tool categories:

- **Web**: Search with Brave, Tavily, DuckDuckGo, or SearXNG
- **Exec**: Shell command execution (respects `restrictToWorkspace`)
- **MCP**: Model Context Protocol servers for external tools

Configure in `config.json`:

```json
{
  "tools": {
    "restrictToWorkspace": true,
    "web": {
      "searchBackend": "duckduckgo",
      "maxResults": 5
    },
    "exec": {
      "denyPatterns": ["rm -rf /"]
    }
  }
}
```

### Hybrid Memory

smolbot bundles the `hybrid-memory` MCP server as a first-class optional component. On fresh install or upgrade, the installer will:

- initialize the `mcp/hybrid-memory` submodule
- install the Node.js dependencies in `mcp/hybrid-memory/mcp`
- create `~/.smolbot/memory`
- add the `hybrid-memory` MCP server to `tools.mcpServers` when Node.js and npm are available

The bundled server exposes these wrapped tools:

- `mcp_hybrid-memory_memory_store`
- `mcp_hybrid-memory_memory_search`
- `mcp_hybrid-memory_memory_semantic`
- `mcp_hybrid-memory_memory_get`
- `mcp_hybrid-memory_memory_delete`
- `mcp_hybrid-memory_memory_stats`
- `mcp_hybrid-memory_memory_cleanup`

The builtin `memory` skill is designed around a 5-stage workflow: startup gate, small recall, mid-session triggers, harvest gate, and distilled store. The full operating guidance lives in [skills/memory/SKILL.md](./skills/memory/SKILL.md) and its reference files under [skills/memory/references](./skills/memory/references).

Default generated config:

```json
{
  "tools": {
    "mcpServers": {
      "hybrid-memory": {
        "enabled": true,
        "type": "stdio",
        "command": "node",
        "args": ["/path/to/smolbot/mcp/hybrid-memory/mcp/mcp-server.js"],
        "env": {
          "HYBRID_MEMORY_DIR": "/home/you/.smolbot/memory",
          "OLLAMA_HOST": "http://localhost:11434",
          "OLLAMA_EMBED_MODEL": "mxbai-embed-large"
        },
        "toolTimeout": 30,
        "enabledTools": ["*"]
      }
    }
  }
}
```

To disable it without removing the config, set:

```json
{
  "tools": {
    "mcpServers": {
      "hybrid-memory": {
        "enabled": false
      }
    }
  }
}
```

Notes:

- `HYBRID_MEMORY_DIR` controls where SQLite and LanceDB data live. The default smolbot location is `~/.smolbot/memory`.
- Ollama is optional. If it is unavailable, keyword search still works and semantic search degrades gracefully.
- You can customize `OLLAMA_EMBED_MODEL` and `OLLAMA_HOST` in the MCP server `env` block.

Troubleshooting:

- If Node.js is missing or older than 18, the installer skips hybrid-memory and leaves the rest of the install intact.
- If `npm install --production` fails, install build tooling and rerun the installer.
- If semantic search returns weak or empty results, verify Ollama is running and the embed model is available.
- If no memory tools appear after install, check `tools.mcpServers.hybrid-memory.enabled` and the `mcp-server.js` path in `~/.smolbot/config.json`.
- If the memory directory is not writable, the MCP server will fail to start and smolbot will continue without those tools.

### Quota Tracking

For Ollama providers, you can enable quota tracking to monitor your API usage against your Ollama account limits.

**Supported browsers for auto-discovery:**

- Chromium-based: Chrome, Brave, Edge, Vivaldi, Zen
- Firefox-based: Firefox, Zen

**Configuration:**

```json
{
  "quota": {
    "refreshIntervalMinutes": 60,
    "providers": {
      "ollama": {
        "enabled": true,
        "browserCookieDiscoveryEnabled": true
      }
    }
  }
}
```

Or with manual cookie fallback:

```json
{
  "quota": {
    "refreshIntervalMinutes": 60,
    "providers": {
      "ollama": {
        "enabled": true,
        "browserCookieDiscoveryEnabled": false,
        "cookieHeader": "session=abc123; cf_clearance=xyz"
      }
    }
  }
}
```

**Setup options:**

1. **Installer**: Toggle quota tracking during installation (Ollama only)
2. **Onboarding**: Run `smolbot onboard` and enable when prompted
3. **Manual**: Edit `~/.smolbot/config.json` directly

**Sidebar display:**

- `Quota` only appears when configured and available
- Percentage values show severity: green (<60%), yellow (60-80%), red (>80%)
- `Observed` shows smolbot's local usage; `Quota` shows account-backed usage

## Security

**Workspace Sandboxing:**

Set `tools.exec.restrictToWorkspace: true` to prevent the agent from accessing files outside the workspace directory.

**DM Pairing:**

For channel integrations, unknown senders receive a pairing code instead of immediate responses. Approve with:

```bash
smolbot pairing approve signal +1234567890
```

## Workspace Structure

```
~/.smolbot/
├── config.json          # Main configuration
├── sessions.db          # Chat history
└── workspace/
    ├── memory/          # Agent memory
    ├── SOUL.md          # Personality definition
    └── HEARTBEAT.md     # Periodic tasks
```

## Development

```bash
# Build daemon
go build -o smolbot ./cmd/smolbot

# Build TUI
go build -o smolbot-tui ./cmd/smolbot-tui

# Build installer
go build -o install-smolbot ./cmd/installer
```

## License

BSD 3-Clause License - see LICENSE file for details
