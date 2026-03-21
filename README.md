<div align="center">
  <img src="assets/smolbot_header.png" alt="SMOLBOT" width="600" />
</div>

A terminal-based AI assistant. Chat with local or cloud AI models, manage sessions, and automate tasks from the command line.

```bash
curl -sSL https://raw.githubusercontent.com/Nomadcxx/smolbot/main/install.sh | bash
```

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
nanobot chat "Explain this code"

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

## License

BSD 3-Clause License - see LICENSE file for details
