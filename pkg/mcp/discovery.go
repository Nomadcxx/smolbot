package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/Nomadcxx/smolbot/pkg/tool"
)

type StdioDiscoveryClient struct {
	mu         sync.Mutex
	transports map[string]*StdioTransport
	connecting map[string]*connectCall
	closed     bool
	logger     *slog.Logger
	ctx        context.Context
	cancel     context.CancelFunc
}

type connectCall struct {
	done chan struct{}
	t    *StdioTransport
	err  error
}

type UnsupportedTransportError struct {
	Transport TransportKind
}

var errDiscoveryClientClosed = errors.New("mcp discovery client closed")

func (e *UnsupportedTransportError) Error() string {
	return fmt.Sprintf("transport %q not yet supported (only stdio implemented)", e.Transport)
}

func IsUnsupportedTransport(err error) bool {
	var target *UnsupportedTransportError
	return errors.As(err, &target)
}

func NewStdioDiscoveryClient(logger *slog.Logger) *StdioDiscoveryClient {
	if logger == nil {
		logger = slog.Default()
	}
	ctx, cancel := context.WithCancel(context.Background())
	return &StdioDiscoveryClient{
		transports: make(map[string]*StdioTransport),
		connecting: make(map[string]*connectCall),
		logger:     logger,
		ctx:        ctx,
		cancel:     cancel,
	}
}

func (c *StdioDiscoveryClient) getOrConnect(ctx context.Context, spec ConnectionSpec) (*StdioTransport, error) {
	for {
		c.mu.Lock()
		if c.closed {
			c.mu.Unlock()
			return nil, errDiscoveryClientClosed
		}
		if t, ok := c.transports[spec.Name]; ok {
			c.mu.Unlock()
			return t, nil
		}
		if call, ok := c.connecting[spec.Name]; ok {
			done := call.done
			c.mu.Unlock()
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-done:
				if call.err != nil {
					return nil, call.err
				}
				if call.t != nil {
					return call.t, nil
				}
			}
			continue
		}

		call := &connectCall{done: make(chan struct{})}
		c.connecting[spec.Name] = call
		c.mu.Unlock()

		t, err := c.connect(ctx, spec)

		c.mu.Lock()
		delete(c.connecting, spec.Name)
		if c.closed {
			if t != nil {
				_ = t.Close()
			}
			t = nil
			if err == nil {
				err = errDiscoveryClientClosed
			}
		} else if err == nil {
			c.transports[spec.Name] = t
		}
		call.t = t
		call.err = err
		close(call.done)
		c.mu.Unlock()

		return t, err
	}
}

func (c *StdioDiscoveryClient) connect(ctx context.Context, spec ConnectionSpec) (*StdioTransport, error) {
	initCtx, cancel := withDefaultTimeout(ctx, spec.ToolTimeout)
	defer cancel()

	c.logger.Info("starting mcp server", "name", spec.Name, "command", spec.Command)
	t, err := NewStdioTransport(c.ctx, spec.Command, spec.Args, spec.Env)
	if err != nil {
		return nil, fmt.Errorf("start mcp server %s: %w", spec.Name, err)
	}

	initParams := map[string]any{
		"protocolVersion": "2024-11-05",
		"capabilities":    map[string]any{},
		"clientInfo": map[string]any{
			"name":    "smolbot",
			"version": "1.0.0",
		},
	}
	result, err := t.Send(initCtx, "initialize", initParams)
	if err != nil {
		_ = t.Close()
		return nil, fmt.Errorf("mcp initialize %s: %w", spec.Name, err)
	}

	var initResult mcpInitResult
	if err := json.Unmarshal(result, &initResult); err != nil {
		_ = t.Close()
		return nil, fmt.Errorf("parse initialize result for %s: %w", spec.Name, err)
	}

	c.logger.Info(
		"mcp server initialized",
		"name", spec.Name,
		"serverName", initResult.ServerInfo.Name,
		"serverVersion", initResult.ServerInfo.Version,
		"protocol", initResult.ProtocolVersion,
	)

	if err := t.Notify(initCtx, "notifications/initialized", nil); err != nil {
		c.logger.Warn("failed to send initialized notification", "name", spec.Name, "error", err)
	}

	return t, nil
}

func (c *StdioDiscoveryClient) Discover(ctx context.Context, spec ConnectionSpec) ([]RemoteTool, error) {
	if spec.Transport != TransportStdio {
		c.logger.Warn("unsupported transport for discovery", "name", spec.Name, "transport", spec.Transport)
		return nil, &UnsupportedTransportError{Transport: spec.Transport}
	}

	t, err := c.getOrConnect(ctx, spec)
	if err != nil {
		return nil, err
	}

	reqCtx, cancel := withDefaultTimeout(ctx, spec.ToolTimeout)
	defer cancel()

	result, err := t.Send(reqCtx, "tools/list", map[string]any{})
	if err != nil {
		if shouldDropTransport(err) {
			c.dropTransport(spec.Name, t, err)
		}
		return nil, fmt.Errorf("tools/list for %s: %w", spec.Name, err)
	}

	var listResult mcpToolsListResult
	if err := json.Unmarshal(result, &listResult); err != nil {
		return nil, fmt.Errorf("parse tools/list result for %s: %w", spec.Name, err)
	}

	tools := make([]RemoteTool, len(listResult.Tools))
	for i, toolDef := range listResult.Tools {
		tools[i] = RemoteTool{
			Name:        toolDef.Name,
			Description: toolDef.Description,
			InputSchema: toolDef.InputSchema,
		}
	}

	c.logger.Info("discovered mcp tools", "name", spec.Name, "count", len(tools))
	return tools, nil
}

func (c *StdioDiscoveryClient) Invoke(ctx context.Context, spec ConnectionSpec, toolName string, args json.RawMessage, _ tool.ToolContext) (*tool.Result, error) {
	t, err := c.getOrConnect(ctx, spec)
	if err != nil {
		return nil, err
	}
	reqCtx, cancel := withDefaultTimeout(ctx, spec.ToolTimeout)
	defer cancel()

	params := map[string]any{
		"name":      toolName,
		"arguments": json.RawMessage(args),
	}
	result, err := t.Send(reqCtx, "tools/call", params)
	if err != nil {
		if shouldDropTransport(err) {
			c.dropTransport(spec.Name, t, err)
		}
		return nil, fmt.Errorf("tools/call %s on %s: %w", toolName, spec.Name, err)
	}

	var callResult mcpCallResult
	if err := json.Unmarshal(result, &callResult); err != nil {
		return nil, fmt.Errorf("parse tools/call result for %s/%s: %w", spec.Name, toolName, err)
	}

	var output strings.Builder
	for _, content := range callResult.Content {
		if output.Len() > 0 {
			output.WriteByte('\n')
		}
		switch content.Type {
		case "text":
			output.WriteString(content.Text)
		default:
			output.WriteString(fmt.Sprintf("[unsupported content type: %s]", content.Type))
		}
	}

	if callResult.IsError {
		return &tool.Result{Error: output.String()}, nil
	}
	return &tool.Result{Output: output.String()}, nil
}

func (c *StdioDiscoveryClient) Close() {
	if c == nil {
		return
	}
	if c.cancel != nil {
		c.cancel()
	}
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return
	}
	c.closed = true
	transports := c.transports
	c.transports = make(map[string]*StdioTransport)
	c.mu.Unlock()

	for name, t := range transports {
		c.logger.Info("stopping mcp server", "name", name)
		_ = t.Close()
	}
}

func (c *StdioDiscoveryClient) dropTransport(name string, t *StdioTransport, err error) {
	c.mu.Lock()
	current, ok := c.transports[name]
	if ok && current == t {
		delete(c.transports, name)
	}
	c.mu.Unlock()
	if ok {
		c.logger.Warn("mcp transport dropped; will reconnect on next call", "name", name, "error", err)
		_ = t.Close()
	}
}

func shouldDropTransport(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, errTransportWriteInterrupted) {
		return true
	}
	return !errors.Is(err, context.Canceled) && !errors.Is(err, context.DeadlineExceeded)
}

func withDefaultTimeout(ctx context.Context, timeout time.Duration) (context.Context, context.CancelFunc) {
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	if deadline, ok := ctx.Deadline(); ok {
		if time.Until(deadline) <= timeout {
			return ctx, func() {}
		}
	}
	return context.WithTimeout(ctx, timeout)
}
