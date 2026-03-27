package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Config is the top-level configuration for the daemon and clients.
type Config struct {
	Agents    AgentsConfig              `json:"agents"`
	Providers map[string]ProviderConfig `json:"providers"`
	Channels  ChannelsConfig            `json:"channels"`
	Gateway   GatewayConfig             `json:"gateway"`
	Tools     ToolsConfig               `json:"tools"`
}

type AgentsConfig struct {
	Defaults AgentDefaults `json:"defaults"`
}

type CompressionConfig struct {
	Enabled          bool   `json:"enabled"`
	Mode             string `json:"mode"`
	ThresholdPercent int    `json:"thresholdPercent"`
}

type AgentDefaults struct {
	Model               string            `json:"model"`
	Provider            string            `json:"provider"`
	Workspace           string            `json:"workspace"`
	MaxTokens           int               `json:"maxTokens"`
	ContextWindowTokens int               `json:"contextWindowTokens"`
	Temperature         float64           `json:"temperature"`
	MaxToolIterations   int               `json:"maxToolIterations"`
	ReasoningEffort     string            `json:"reasoningEffort,omitempty"`
	Compression         CompressionConfig `json:"compression"`
}

type ProviderConfig struct {
	APIKey       string            `json:"apiKey,omitempty"`
	APIBase      string            `json:"apiBase,omitempty"`
	ExtraHeaders map[string]string `json:"extraHeaders,omitempty"`
}

type ChannelsConfig struct {
	SendProgress  bool                  `json:"sendProgress"`
	SendToolHints bool                  `json:"sendToolHints"`
	Signal        SignalChannelConfig   `json:"signal"`
	WhatsApp      WhatsAppChannelConfig `json:"whatsapp"`
}

type SignalChannelConfig struct {
	Enabled bool   `json:"enabled"`
	Account string `json:"account,omitempty"`
	CLIPath string `json:"cliPath,omitempty"`
	DataDir string `json:"dataDir,omitempty"`
}

type WhatsAppChannelConfig struct {
	Enabled        bool     `json:"enabled"`
	DeviceName     string   `json:"deviceName,omitempty"`
	StorePath      string   `json:"storePath,omitempty"`
	AllowedChatIDs []string `json:"allowedChatIDs,omitempty"`
}

type GatewayConfig struct {
	Host      string          `json:"host"`
	Port      int             `json:"port"`
	Heartbeat HeartbeatConfig `json:"heartbeat"`
}

type HeartbeatConfig struct {
	Enabled  bool   `json:"enabled"`
	Interval int    `json:"interval"`
	Channel  string `json:"channel"`
}

type ToolsConfig struct {
	Web                 WebToolConfig              `json:"web"`
	Exec                ExecToolConfig             `json:"exec"`
	RestrictToWorkspace bool                       `json:"restrictToWorkspace"`
	MCPServers          map[string]MCPServerConfig `json:"mcpServers"`
}

type ExecToolConfig struct {
	DefaultTimeout int      `json:"defaultTimeout"`
	MaxTimeout     int      `json:"maxTimeout"`
	DenyPatterns   []string `json:"denyPatterns"`
	PathAppend     string   `json:"pathAppend"`
}

type WebToolConfig struct {
	SearchBackend string `json:"searchBackend"`
	MaxResults    int    `json:"maxResults"`
	UserAgent     string `json:"userAgent"`
	Proxy         string `json:"proxy,omitempty"`
}

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

// DefaultConfig returns a config with sensible defaults.
func DefaultConfig() Config {
	home, _ := os.UserHomeDir()
	return Config{
		Agents: AgentsConfig{
			Defaults: AgentDefaults{
				Workspace:           filepath.Join(home, ".smolbot", "workspace"),
				MaxTokens:           8192,
				ContextWindowTokens: 200000,
				Temperature:         0.7,
				MaxToolIterations:   40,
				Compression: CompressionConfig{
					Enabled:          true,
					Mode:             "default",
					ThresholdPercent: 60,
				},
			},
		},
		Providers: make(map[string]ProviderConfig),
		Channels: ChannelsConfig{
			SendProgress: true,
			Signal: SignalChannelConfig{
				CLIPath: "signal-cli",
				DataDir: filepath.Join(home, ".smolbot", "signal"),
			},
			WhatsApp: WhatsAppChannelConfig{
				DeviceName: "smolbot",
				StorePath:  filepath.Join(home, ".smolbot", "whatsapp.db"),
			},
		},
		Gateway: GatewayConfig{
			Host: "127.0.0.1",
			Port: 18790,
		},
		Tools: ToolsConfig{
			Exec: ExecToolConfig{
				DefaultTimeout: 60,
				MaxTimeout:     600,
			},
			Web: WebToolConfig{
				SearchBackend: "duckduckgo",
				MaxResults:    5,
			},
			MCPServers: make(map[string]MCPServerConfig),
		},
	}
}

// Load reads config from disk and merges it onto defaults.
func Load(path string) (*Config, error) {
	cfg := DefaultConfig()

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}

	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}

	if strings.HasPrefix(cfg.Agents.Defaults.Workspace, "~/") {
		home, _ := os.UserHomeDir()
		cfg.Agents.Defaults.Workspace = filepath.Join(home, cfg.Agents.Defaults.Workspace[2:])
	}

	return &cfg, nil
}
