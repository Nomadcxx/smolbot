package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"time"

	"github.com/Nomadcxx/smolbot/pkg/config"
	"github.com/Nomadcxx/smolbot/pkg/tool"
)

type TransportKind string

const (
	TransportSSE            TransportKind = "sse"
	TransportStdio          TransportKind = "stdio"
	TransportStreamableHTTP TransportKind = "streamable_http"
)

type ConnectionSpec struct {
	Name         string
	Transport    TransportKind
	Command      string
	Args         []string
	Env          map[string]string
	URL          string
	Headers      map[string]string
	ToolTimeout  time.Duration
	EnabledTools []string
}

type RemoteTool struct {
	Name        string
	Description string
	InputSchema map[string]any
}

type DiscoveryClient interface {
	Discover(ctx context.Context, spec ConnectionSpec) ([]RemoteTool, error)
	Invoke(ctx context.Context, spec ConnectionSpec, toolName string, args json.RawMessage) (*tool.Result, error)
}

type Manager struct {
	client DiscoveryClient
}

func NewManager(client DiscoveryClient) *Manager {
	return &Manager{client: client}
}

func DetectTransport(cfg config.MCPServerConfig) TransportKind {
	if strings.EqualFold(cfg.Type, "stdio") || strings.TrimSpace(cfg.Command) != "" {
		return TransportStdio
	}
	if strings.HasSuffix(strings.TrimRight(strings.TrimSpace(cfg.URL), "/"), "/sse") {
		return TransportSSE
	}
	return TransportStreamableHTTP
}

func WrapName(serverName, toolName string) string {
	return fmt.Sprintf("mcp_%s_%s", serverName, toolName)
}

func (m *Manager) DiscoverAndRegister(ctx context.Context, registry *tool.Registry, servers map[string]config.MCPServerConfig) ([]string, error) {
	if m.client == nil {
		slog.Warn("mcp manager has no discovery client, skipping tool registration", "servers", len(servers))
		return nil, nil
	}

	serverNames := make([]string, 0, len(servers))
	for name := range servers {
		serverNames = append(serverNames, name)
	}
	sort.Strings(serverNames)

	var warnings []string
	for _, serverName := range serverNames {
		cfg := servers[serverName]
		spec := connectionSpec(serverName, cfg)
		remoteTools, err := m.client.Discover(ctx, spec)
		if err != nil {
			if IsUnsupportedTransport(err) {
				warnings = append(warnings, fmt.Sprintf("mcp server %s uses unsupported transport %q; skipping", serverName, spec.Transport))
				continue
			}
			return warnings, fmt.Errorf("discover mcp tools for %s: %w", serverName, err)
		}

		available := make(map[string]struct{}, len(remoteTools)*2)
		for _, remoteTool := range remoteTools {
			available[remoteTool.Name] = struct{}{}
			available[WrapName(serverName, remoteTool.Name)] = struct{}{}
			if !toolEnabled(cfg.EnabledTools, serverName, remoteTool.Name) {
				continue
			}
			registry.Register(&wrappedTool{
				spec:       spec,
				client:     m.client,
				rawName:    remoteTool.Name,
				wrapped:    WrapName(serverName, remoteTool.Name),
				desc:       remoteTool.Description,
				parameters: remoteTool.InputSchema,
			})
		}

		for _, name := range cfg.EnabledTools {
			if name == "*" {
				continue
			}
			if _, ok := available[name]; !ok {
				warnings = append(warnings, fmt.Sprintf("mcp server %s enabledTools entry %q did not match any discovered tool", serverName, name))
			}
		}
	}

	return warnings, nil
}

func connectionSpec(serverName string, cfg config.MCPServerConfig) ConnectionSpec {
	headers := make(map[string]string, len(cfg.Headers))
	for key, value := range cfg.Headers {
		headers[key] = value
	}
	env := make(map[string]string, len(cfg.Env))
	for key, value := range cfg.Env {
		env[key] = value
	}
	args := append([]string(nil), cfg.Args...)
	enabled := append([]string(nil), cfg.EnabledTools...)

	timeout := time.Duration(cfg.ToolTimeout) * time.Second
	if timeout <= 0 {
		timeout = 30 * time.Second
	}

	return ConnectionSpec{
		Name:         serverName,
		Transport:    DetectTransport(cfg),
		Command:      cfg.Command,
		Args:         args,
		Env:          env,
		URL:          cfg.URL,
		Headers:      headers,
		ToolTimeout:  timeout,
		EnabledTools: enabled,
	}
}

func toolEnabled(enabled []string, serverName, rawName string) bool {
	if len(enabled) == 0 {
		return true
	}

	wrapped := WrapName(serverName, rawName)
	for _, entry := range enabled {
		if entry == "*" || entry == rawName || entry == wrapped {
			return true
		}
	}
	return false
}

type wrappedTool struct {
	spec       ConnectionSpec
	client     DiscoveryClient
	rawName    string
	wrapped    string
	desc       string
	parameters map[string]any
}

func (t *wrappedTool) Name() string {
	return t.wrapped
}

func (t *wrappedTool) Description() string {
	return t.desc
}

func (t *wrappedTool) Parameters() map[string]any {
	if t.parameters == nil {
		return map[string]any{"type": "object"}
	}
	return t.parameters
}

func (t *wrappedTool) Execute(ctx context.Context, args json.RawMessage, _ tool.ToolContext) (*tool.Result, error) {
	timeoutCtx, cancel := context.WithTimeout(ctx, t.spec.ToolTimeout)
	defer cancel()
	return t.client.Invoke(timeoutCtx, t.spec, t.rawName, args)
}
