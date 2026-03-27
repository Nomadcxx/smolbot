package config

import (
	"encoding/json"
	"testing"
)

func TestConfigRoundTrip(t *testing.T) {
	input := `{
		"agents": {
			"defaults": {
				"model": "claude-sonnet-4-20250514",
				"provider": "anthropic",
				"workspace": "~/.smolbot/workspace",
				"maxTokens": 8192,
				"contextWindowTokens": 200000,
				"temperature": 0.7,
				"maxToolIterations": 40
			}
		},
		"providers": {
			"anthropic": {"apiKey": "sk-ant-xxx"},
			"openrouter": {"apiKey": "sk-or-xxx", "apiBase": "https://openrouter.ai/api/v1"}
		},
		"channels": {
			"sendProgress": true,
			"sendToolHints": false,
			"signal": {
				"enabled": true,
				"account": "+61400000000",
				"cliPath": "/usr/local/bin/signal-cli",
				"dataDir": "/tmp/nanobot-signal"
			},
			"whatsapp": {
				"enabled": true,
				"deviceName": "smolbot",
				"storePath": "/tmp/nanobot-whatsapp.db",
				"allowedChatIDs": ["example@s.whatsapp.net"]
			}
		},
		"gateway": {
			"host": "127.0.0.1",
			"port": 18790
		},
		"tools": {
			"restrictToWorkspace": true,
			"mcpServers": {
				"memory": {
					"type": "stdio",
					"command": "npx",
					"args": ["-y", "@anthropic/memory-server"],
					"toolTimeout": 30,
					"enabledTools": ["*"]
				}
			}
		}
	}`

	var cfg Config
	if err := json.Unmarshal([]byte(input), &cfg); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if cfg.Agents.Defaults.Model != "claude-sonnet-4-20250514" {
		t.Errorf("model = %q, want claude-sonnet-4-20250514", cfg.Agents.Defaults.Model)
	}
	if cfg.Providers["anthropic"].APIKey != "sk-ant-xxx" {
		t.Errorf("anthropic apiKey = %q, want sk-ant-xxx", cfg.Providers["anthropic"].APIKey)
	}
	if cfg.Providers["openrouter"].APIBase != "https://openrouter.ai/api/v1" {
		t.Errorf("openrouter apiBase = %q", cfg.Providers["openrouter"].APIBase)
	}
	if cfg.Gateway.Port != 18790 {
		t.Errorf("port = %d, want 18790", cfg.Gateway.Port)
	}
	if !cfg.Channels.Signal.Enabled || cfg.Channels.Signal.Account != "+61400000000" {
		t.Errorf("signal config = %+v", cfg.Channels.Signal)
	}
	if !cfg.Channels.WhatsApp.Enabled || cfg.Channels.WhatsApp.DeviceName != "smolbot" {
		t.Errorf("whatsapp config = %+v", cfg.Channels.WhatsApp)
	}
	if got := cfg.Channels.WhatsApp.AllowedChatIDs; len(got) != 1 || got[0] != "example@s.whatsapp.net" {
		t.Errorf("whatsapp allowedChatIDs = %#v", got)
	}
	if !cfg.Tools.RestrictToWorkspace {
		t.Error("restrictToWorkspace should be true")
	}
	mcp := cfg.Tools.MCPServers["memory"]
	if mcp.Type != "stdio" || mcp.Command != "npx" || mcp.ToolTimeout != 30 {
		t.Errorf("mcp memory = %+v", mcp)
	}
}

func TestConfigDefaults(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.Agents.Defaults.MaxTokens != 8192 {
		t.Errorf("default maxTokens = %d, want 8192", cfg.Agents.Defaults.MaxTokens)
	}
	if cfg.Agents.Defaults.MaxToolIterations != 40 {
		t.Errorf("default maxToolIterations = %d, want 40", cfg.Agents.Defaults.MaxToolIterations)
	}
	if cfg.Gateway.Port != 18790 {
		t.Errorf("default port = %d, want 18790", cfg.Gateway.Port)
	}
	if cfg.Agents.Defaults.Temperature != 0.7 {
		t.Errorf("default temperature = %f, want 0.7", cfg.Agents.Defaults.Temperature)
	}
	if !cfg.Channels.SendProgress {
		t.Error("default sendProgress should be true")
	}
	if cfg.Channels.Signal.CLIPath == "" || cfg.Channels.Signal.DataDir == "" {
		t.Fatalf("signal defaults = %+v, want non-empty paths", cfg.Channels.Signal)
	}
	if cfg.Channels.WhatsApp.DeviceName == "" || cfg.Channels.WhatsApp.StorePath == "" {
		t.Fatalf("whatsapp defaults = %+v, want non-empty settings", cfg.Channels.WhatsApp)
	}
}

func TestPaths(t *testing.T) {
	p := NewPaths("/home/test/.smolbot")
	if p.ConfigFile() != "/home/test/.smolbot/config.json" {
		t.Errorf("ConfigFile = %q", p.ConfigFile())
	}
	if p.Workspace() != "/home/test/.smolbot/workspace" {
		t.Errorf("Workspace = %q", p.Workspace())
	}
	if p.SessionsDB() != "/home/test/.smolbot/sessions.db" {
		t.Errorf("SessionsDB = %q", p.SessionsDB())
	}
	if p.JobsFile() != "/home/test/.smolbot/jobs.json" {
		t.Errorf("JobsFile = %q", p.JobsFile())
	}
	if p.MemoryDir() != "/home/test/.smolbot/workspace/memory" {
		t.Errorf("MemoryDir = %q", p.MemoryDir())
	}
}

func TestDefaultPaths(t *testing.T) {
	p := DefaultPaths()
	if p.root == "" {
		t.Error("root should not be empty")
	}
}
