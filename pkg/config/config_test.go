package config

import (
	"encoding/json"
	"os"
	"reflect"
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
			},
			"telegram": {
				"enabled": true,
				"tokenFile": "/tmp/nanobot-telegram.token",
				"allowedChatIDs": ["123456789"]
			},
			"discord": {
				"enabled": true,
				"tokenFile": "/tmp/nanobot-discord.token",
				"allowedChannelIDs": ["987654321"]
			}
		},
		"gateway": {
			"host": "127.0.0.1",
			"port": 18790
		},
		"quota": {
			"refreshIntervalMinutes": 15,
			"browserCookieDiscoveryEnabled": false,
			"ollamaCookieHeader": "session=abc123; cf_clearance=xyz"
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
	if !cfg.Channels.Telegram.Enabled || cfg.Channels.Telegram.TokenFile != "/tmp/nanobot-telegram.token" {
		t.Errorf("telegram config = %+v", cfg.Channels.Telegram)
	}
	if got := cfg.Channels.Telegram.AllowedChatIDs; len(got) != 1 || got[0] != "123456789" {
		t.Errorf("telegram allowedChatIDs = %#v", got)
	}
	if !cfg.Channels.Discord.Enabled || cfg.Channels.Discord.TokenFile != "/tmp/nanobot-discord.token" {
		t.Errorf("discord config = %+v", cfg.Channels.Discord)
	}
	if got := cfg.Channels.Discord.AllowedChannelIDs; len(got) != 1 || got[0] != "987654321" {
		t.Errorf("discord allowedChannelIDs = %#v", got)
	}
	if got := reflect.ValueOf(cfg).FieldByName("Quota"); !got.IsValid() {
		t.Fatal("quota config field is missing")
	}
	if got := reflect.ValueOf(cfg).FieldByName("Quota").FieldByName("RefreshIntervalMinutes"); !got.IsValid() || got.Int() != 15 {
		t.Fatalf("quota refreshIntervalMinutes = %v, want 15", got)
	}
	if got := reflect.ValueOf(cfg).FieldByName("Quota").FieldByName("BrowserCookieDiscoveryEnabled"); !got.IsValid() || got.Bool() {
		t.Fatalf("quota browserCookieDiscoveryEnabled = %v, want false", got)
	}
	if got := reflect.ValueOf(cfg).FieldByName("Quota").FieldByName("OllamaCookieHeader"); !got.IsValid() || got.String() != "session=abc123; cf_clearance=xyz" {
		t.Fatalf("quota ollamaCookieHeader = %v, want configured header", got)
	}
	if !cfg.Tools.RestrictToWorkspace {
		t.Error("restrictToWorkspace should be true")
	}
	mcp := cfg.Tools.MCPServers["memory"]
	if mcp.Type != "stdio" || mcp.Command != "npx" || mcp.ToolTimeout != 30 {
		t.Errorf("mcp memory = %+v", mcp)
	}
}

func TestProviderConfigOAuthJSONRoundTrip(t *testing.T) {
	input := ProviderConfig{
		APIKey:    "sk-ant-xxx",
		APIBase:   "https://api.minimax.io",
		AuthType:  "oauth",
		ProfileID: "profile-123",
	}

	data, err := json.Marshal(input)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var got ProviderConfig
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if got.APIKey != input.APIKey {
		t.Fatalf("APIKey = %q, want %q", got.APIKey, input.APIKey)
	}
	if got.APIBase != input.APIBase {
		t.Fatalf("APIBase = %q, want %q", got.APIBase, input.APIBase)
	}
	if got.AuthType != input.AuthType {
		t.Fatalf("AuthType = %q, want %q", got.AuthType, input.AuthType)
	}
	if got.ProfileID != input.ProfileID {
		t.Fatalf("ProfileID = %q, want %q", got.ProfileID, input.ProfileID)
	}
}

func TestDefaultConfig(t *testing.T) {
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
	if got := reflect.ValueOf(cfg).FieldByName("Quota"); !got.IsValid() {
		t.Fatal("quota defaults are missing")
	}
	if got := reflect.ValueOf(cfg).FieldByName("Quota").FieldByName("RefreshIntervalMinutes"); !got.IsValid() || got.Int() != 60 {
		t.Fatalf("quota refreshIntervalMinutes = %v, want 60", got)
	}
	if got := reflect.ValueOf(cfg).FieldByName("Quota").FieldByName("BrowserCookieDiscoveryEnabled"); !got.IsValid() || !got.Bool() {
		t.Fatalf("quota browserCookieDiscoveryEnabled = %v, want true", got)
	}
	if got := reflect.ValueOf(cfg).FieldByName("Quota").FieldByName("OllamaCookieHeader"); !got.IsValid() || got.String() != "" {
		t.Fatalf("quota ollamaCookieHeader = %v, want empty by default", got)
	}
	if cfg.Channels.WhatsApp.DeviceName == "" || cfg.Channels.WhatsApp.StorePath == "" {
		t.Fatalf("whatsapp defaults = %+v, want non-empty settings", cfg.Channels.WhatsApp)
	}
	if cfg.Channels.Telegram.Enabled {
		t.Fatalf("telegram defaults = %+v, want disabled", cfg.Channels.Telegram)
	}
	if cfg.Channels.Telegram.TokenFile != "" {
		t.Fatalf("telegram defaults = %+v, want empty token file", cfg.Channels.Telegram)
	}
	if cfg.Channels.Discord.Enabled {
		t.Fatalf("discord defaults = %+v, want disabled", cfg.Channels.Discord)
	}
	if cfg.Channels.Discord.TokenFile != "" || cfg.Channels.Discord.BotToken != "" {
		t.Fatalf("discord defaults = %+v, want empty token settings", cfg.Channels.Discord)
	}
	if len(cfg.Channels.Discord.AllowedChannelIDs) != 0 {
		t.Fatalf("discord defaults = %+v, want no allowlist", cfg.Channels.Discord)
	}
	if cfg.Agents.Defaults.Compression.Engine != "legacy" {
		t.Fatalf("compression engine = %q, want legacy", cfg.Agents.Defaults.Compression.Engine)
	}
	if !cfg.Agents.Defaults.Compression.DCP.Deduplication.Enabled {
		t.Fatal("default DCP deduplication should be enabled")
	}
	if !cfg.Agents.Defaults.Compression.DCP.PurgeErrors.Enabled {
		t.Fatal("default DCP purgeErrors should be enabled")
	}
	if cfg.Agents.Defaults.Compression.DCP.TurnProtection != 4 {
		t.Fatalf("default DCP turnProtection = %d, want 4", cfg.Agents.Defaults.Compression.DCP.TurnProtection)
	}
}

func TestLoadTelegramConfig(t *testing.T) {
	t.Helper()

	dir := t.TempDir()
	path := dir + "/config.json"
	input := `{
		"channels": {
			"telegram": {
				"enabled": true,
				"tokenFile": "/tmp/load-telegram.token",
				"allowedChatIDs": ["42"]
			}
		}
	}`
	if err := os.WriteFile(path, []byte(input), 0644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if !cfg.Channels.Telegram.Enabled {
		t.Fatal("telegram should be enabled after loading config")
	}
	if cfg.Channels.Telegram.TokenFile != "/tmp/load-telegram.token" {
		t.Fatalf("telegram tokenFile = %q, want %q", cfg.Channels.Telegram.TokenFile, "/tmp/load-telegram.token")
	}
	if got := cfg.Channels.Telegram.AllowedChatIDs; len(got) != 1 || got[0] != "42" {
		t.Fatalf("telegram allowedChatIDs = %#v", got)
	}
}

func TestLoadAPIKeyOnlyConfig(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/config.json"
	input := `{
		"providers": {
			"anthropic": {
				"apiKey": "sk-ant-xxx"
			}
		}
	}`
	if err := os.WriteFile(path, []byte(input), 0644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	provider := cfg.Providers["anthropic"]
	if provider.APIKey != "sk-ant-xxx" {
		t.Fatalf("anthropic apiKey = %q, want %q", provider.APIKey, "sk-ant-xxx")
	}
	if provider.AuthType != "" {
		t.Fatalf("anthropic authType = %q, want empty", provider.AuthType)
	}
	if provider.ProfileID != "" {
		t.Fatalf("anthropic profileId = %q, want empty", provider.ProfileID)
	}
}

func TestLoadDiscordConfig(t *testing.T) {
	t.Helper()

	dir := t.TempDir()
	path := dir + "/config.json"
	input := `{
		"channels": {
			"discord": {
				"enabled": true,
				"botToken": "bot-token",
				"allowedChannelIDs": ["42", "  43 "]
			}
		}
	}`
	if err := os.WriteFile(path, []byte(input), 0644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if !cfg.Channels.Discord.Enabled {
		t.Fatal("discord should be enabled after loading config")
	}
	if cfg.Channels.Discord.BotToken != "bot-token" {
		t.Fatalf("discord botToken = %q, want %q", cfg.Channels.Discord.BotToken, "bot-token")
	}
	if got := cfg.Channels.Discord.AllowedChannelIDs; len(got) != 2 || got[0] != "42" || got[1] != "  43 " {
		t.Fatalf("discord allowedChannelIDs = %#v", got)
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
	if p.UsageDB() != "/home/test/.smolbot/usage.db" {
		t.Errorf("UsageDB = %q", p.UsageDB())
	}
	if got := reflect.ValueOf(p).MethodByName("OllamaCookieJar"); !got.IsValid() {
		t.Fatal("OllamaCookieJar path helper is missing")
	} else if got.Call(nil)[0].String() != "/home/test/.smolbot/ollama_cookies.json" {
		t.Errorf("OllamaCookieJar = %q", got.Call(nil)[0].String())
	}
	if got := reflect.ValueOf(p).MethodByName("OllamaQuotaCache"); !got.IsValid() {
		t.Fatal("OllamaQuotaCache path helper is missing")
	} else if got.Call(nil)[0].String() != "/home/test/.smolbot/ollama_quota.db" {
		t.Errorf("OllamaQuotaCache = %q", got.Call(nil)[0].String())
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

func TestProviderQuotaConfigHelpers(t *testing.T) {
	t.Run("Provider returns correct config", func(t *testing.T) {
		q := QuotaConfig{
			Providers: map[string]ProviderQuotaConfig{
				"ollama": {Enabled: true, BrowserCookieDiscoveryEnabled: true, CookieHeader: "foo=bar"},
			},
		}
		p := q.Provider("ollama")
		if !p.Enabled {
			t.Error("ollama provider should be enabled")
		}
		if p.CookieHeader != "foo=bar" {
			t.Errorf("cookieHeader = %q, want foo=bar", p.CookieHeader)
		}
	})

	t.Run("Provider returns empty for unknown", func(t *testing.T) {
		q := QuotaConfig{Providers: map[string]ProviderQuotaConfig{}}
		p := q.Provider("unknown")
		if p.Enabled {
			t.Error("unknown provider should not be enabled")
		}
	})

	t.Run("Provider returns empty when no providers", func(t *testing.T) {
		q := QuotaConfig{}
		p := q.Provider("ollama")
		if p.Enabled {
			t.Error("should return empty config when no providers")
		}
	})

	t.Run("HasEnabledProvider returns true when enabled", func(t *testing.T) {
		q := QuotaConfig{
			Providers: map[string]ProviderQuotaConfig{
				"ollama": {Enabled: true},
			},
		}
		if !q.HasEnabledProvider("ollama") {
			t.Error("HasEnabledProvider(ollama) should be true")
		}
	})

	t.Run("HasEnabledProvider returns false when disabled", func(t *testing.T) {
		q := QuotaConfig{
			Providers: map[string]ProviderQuotaConfig{
				"ollama": {Enabled: false},
			},
		}
		if q.HasEnabledProvider("ollama") {
			t.Error("HasEnabledProvider(ollama) should be false")
		}
	})

	t.Run("HasEnabledProvider returns false for unknown", func(t *testing.T) {
		q := QuotaConfig{
			Providers: map[string]ProviderQuotaConfig{
				"ollama": {Enabled: true},
			},
		}
		if q.HasEnabledProvider("openai") {
			t.Error("HasEnabledProvider(unknown) should be false")
		}
	})

	t.Run("HasAnyEnabledProvider returns true when any enabled", func(t *testing.T) {
		q := QuotaConfig{
			Providers: map[string]ProviderQuotaConfig{
				"ollama": {Enabled: true},
				"openai": {Enabled: false},
			},
		}
		if !q.HasAnyEnabledProvider() {
			t.Error("HasAnyEnabledProvider should be true")
		}
	})

	t.Run("HasAnyEnabledProvider returns false when none enabled", func(t *testing.T) {
		q := QuotaConfig{
			Providers: map[string]ProviderQuotaConfig{
				"ollama": {Enabled: false},
				"openai": {Enabled: false},
			},
		}
		if q.HasAnyEnabledProvider() {
			t.Error("HasAnyEnabledProvider should be false")
		}
	})
}

func TestQuotaConfigMigratesOllamaFromDefaults(t *testing.T) {
	t.Run("migrates when default provider is ollama", func(t *testing.T) {
		cfg := DefaultConfig()
		cfg.Agents.Defaults.Provider = "ollama"
		normalizeQuotaConfig(&cfg)
		if !cfg.Quota.HasEnabledProvider("ollama") {
			t.Error("should migrate ollama when default provider is ollama")
		}
	})

	t.Run("migrates when default model starts with ollama/", func(t *testing.T) {
		cfg := DefaultConfig()
		cfg.Agents.Defaults.Model = "ollama/llama3"
		normalizeQuotaConfig(&cfg)
		if !cfg.Quota.HasEnabledProvider("ollama") {
			t.Error("should migrate ollama when model starts with ollama/")
		}
	})

	t.Run("does not migrate when refresh interval is zero", func(t *testing.T) {
		cfg := DefaultConfig()
		cfg.Agents.Defaults.Provider = "ollama"
		cfg.Quota.RefreshIntervalMinutes = 0
		normalizeQuotaConfig(&cfg)
		if cfg.Quota.HasEnabledProvider("ollama") {
			t.Error("should not migrate when refresh interval is 0")
		}
	})

	t.Run("does not overwrite existing provider config", func(t *testing.T) {
		cfg := DefaultConfig()
		cfg.Agents.Defaults.Provider = "ollama"
		cfg.Quota.RefreshIntervalMinutes = 30
		cfg.Quota.Providers = map[string]ProviderQuotaConfig{
			"ollama": {Enabled: false},
		}
		normalizeQuotaConfig(&cfg)
		if cfg.Quota.HasEnabledProvider("ollama") {
			t.Error("should not overwrite existing explicit config")
		}
	})

	t.Run("loads provider-scoped quota config", func(t *testing.T) {
		input := `{
			"quota": {
				"refreshIntervalMinutes": 30,
				"providers": {
					"ollama": {
						"enabled": true,
						"browserCookieDiscoveryEnabled": false,
						"cookieHeader": "session=abc"
					}
				}
			}
		}`
		var cfg Config
		if err := json.Unmarshal([]byte(input), &cfg); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		normalizeQuotaConfig(&cfg)
		p := cfg.Quota.Provider("ollama")
		if !p.Enabled {
			t.Error("ollama should be enabled")
		}
		if p.CookieHeader != "session=abc" {
			t.Errorf("cookieHeader = %q, want session=abc", p.CookieHeader)
		}
		if p.BrowserCookieDiscoveryEnabled {
			t.Error("browserCookieDiscoveryEnabled should be false")
		}
	})
}

func TestLoadMigratesLegacyOllamaQuota(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/config.json"
	input := `{
		"agents": {
			"defaults": {
				"provider": "ollama",
				"model": "llama3"
			}
		},
		"quota": {
			"refreshIntervalMinutes": 45,
			"browserCookieDiscoveryEnabled": false,
			"ollamaCookieHeader": "legacy=header"
		}
	}`
	if err := os.WriteFile(path, []byte(input), 0644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if !cfg.Quota.HasEnabledProvider("ollama") {
		t.Fatal("ollama quota should be migrated")
	}
	p := cfg.Quota.Provider("ollama")
	if p.CookieHeader != "legacy=header" {
		t.Errorf("cookieHeader = %q, want legacy=header", p.CookieHeader)
	}
}
