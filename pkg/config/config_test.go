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
				"workspace": "~/.nanobot/workspace",
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
}
