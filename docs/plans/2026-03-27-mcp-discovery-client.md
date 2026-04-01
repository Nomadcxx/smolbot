# MCP Discovery Client — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Implement a production `DiscoveryClient` that speaks the MCP protocol over stdio (JSON-RPC 2.0), enabling smolbot to discover and invoke tools from MCP servers like `hybrid-memory`.

**Root Cause:** `cmd/smolbot/runtime.go:103` passes `nil` to `mcp.NewManager(nil)`. No production implementation of the `DiscoveryClient` interface exists — only test fakes. `DiscoverAndRegister()` silently returns `nil, nil` when client is nil, so MCP servers appear connected in the TUI but their tools are never registered with the LLM.

**Architecture:** The implementation adds a stdio JSON-RPC transport layer beneath the existing (and working) `Manager` → `wrappedTool` → `Registry` pipeline. No changes needed to the agent loop, gateway, or TUI.

**Tech Stack:** Go 1.26, no external MCP library (the protocol is simple enough for a focused implementation)

---

## Current State Summary

| Component | Status | Notes |
|-----------|--------|-------|
| `config.MCPServerConfig` | Working | Parses command, args, env, url, headers, enabledTools |
| `DetectTransport()` | Working | Correctly identifies stdio/SSE/streamable HTTP |
| `ConnectionSpec` | Working | Fully populated from config |
| `Manager.DiscoverAndRegister()` | Working (with real client) | Tested with fakes, handles enabledTools filtering, warnings |
| `wrappedTool` | Working | Implements `tool.Tool`, delegates Execute to client.Invoke |
| `tool.Registry` | Working | Register/Definitions/Execute all solid |
| `runtime.go` wiring | **Broken** | Passes `nil` client → silent no-op |
| `DiscoveryClient` impl | **Missing** | Interface defined, no production implementation |

---

## File Map

| File | Change |
|------|--------|
| `pkg/mcp/stdio.go` | **New** — Stdio transport: subprocess management, JSON-RPC framing |
| `pkg/mcp/jsonrpc.go` | **New** — JSON-RPC 2.0 request/response types and helpers |
| `pkg/mcp/discovery.go` | **New** — Production `DiscoveryClient` implementation using transports |
| `pkg/mcp/client.go` | Modify — Add logging when client is nil |
| `cmd/smolbot/runtime.go` | Modify — Wire real `DiscoveryClient` into `newMCPMgr()` |
| `pkg/mcp/stdio_test.go` | **New** — Tests for stdio transport |
| `pkg/mcp/discovery_test.go` | **New** — Integration tests for discovery client |

---

## MCP Protocol Reference

MCP uses JSON-RPC 2.0. For stdio transport, the server is a subprocess. Communication is newline-delimited JSON over stdin/stdout. The key methods:

### Handshake: `initialize`

```json
→ {"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"smolbot","version":"1.0.0"}}}
← {"jsonrpc":"2.0","id":1,"result":{"protocolVersion":"2024-11-05","capabilities":{"tools":{}},"serverInfo":{"name":"hybrid-memory","version":"0.1.0"}}}
```

### Notification: `notifications/initialized`

```json
→ {"jsonrpc":"2.0","method":"notifications/initialized"}
```

(No response expected — this is a notification, not a request.)

### Tool Discovery: `tools/list`

```json
→ {"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}
← {"jsonrpc":"2.0","id":2,"result":{"tools":[{"name":"memory_store","description":"Store a memory","inputSchema":{"type":"object","properties":{"text":{"type":"string"}},"required":["text"]}}]}}
```

### Tool Invocation: `tools/call`

```json
→ {"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"memory_store","arguments":{"text":"hello"}}}
← {"jsonrpc":"2.0","id":3,"result":{"content":[{"type":"text","text":"Stored memory MEM-001"}],"isError":false}}
```

---

## Task 1: JSON-RPC 2.0 Types

**File:** `pkg/mcp/jsonrpc.go` (new)

Define the wire format for JSON-RPC 2.0 messages. These are intentionally minimal — just what MCP needs.

- [ ] **Step 1: Define request/response types**

```go
package mcp

import "encoding/json"

type jsonRPCRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      *int            `json:"id,omitempty"` // nil for notifications
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type jsonRPCResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      *int            `json:"id"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *jsonRPCError   `json:"error,omitempty"`
}

type jsonRPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func (e *jsonRPCError) Error() string {
	return e.Message
}
```

- [ ] **Step 2: Define MCP-specific result types**

```go
type mcpInitResult struct {
	ProtocolVersion string        `json:"protocolVersion"`
	Capabilities    map[string]any `json:"capabilities"`
	ServerInfo      struct {
		Name    string `json:"name"`
		Version string `json:"version"`
	} `json:"serverInfo"`
}

type mcpToolsListResult struct {
	Tools []mcpToolDef `json:"tools"`
}

type mcpToolDef struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	InputSchema map[string]any `json:"inputSchema"`
}

type mcpCallResult struct {
	Content []mcpContent `json:"content"`
	IsError bool         `json:"isError"`
}

type mcpContent struct {
	Type string `json:"type"`
	Text string `json:"text"`
}
```

---

## Task 2: Stdio Transport

**File:** `pkg/mcp/stdio.go` (new)

The stdio transport spawns a subprocess and communicates via newline-delimited JSON-RPC over stdin/stdout. Each MCP server gets its own subprocess, managed by `StdioTransport`.

- [ ] **Step 1: Implement StdioTransport**

```go
package mcp

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
	"sync"
	"sync/atomic"
)

type StdioTransport struct {
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	stdout *bufio.Scanner
	mu     sync.Mutex // serializes writes
	nextID atomic.Int64
}

func NewStdioTransport(ctx context.Context, command string, args []string, env map[string]string) (*StdioTransport, error) {
	cmd := exec.CommandContext(ctx, command, args...)

	// Build environment: inherit current env, overlay config env
	cmdEnv := cmd.Environ()
	for k, v := range env {
		cmdEnv = append(cmdEnv, k+"="+v)
	}
	cmd.Env = cmdEnv

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("stdin pipe: %w", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("stdout pipe: %w", err)
	}
	// Discard stderr to avoid blocking (could optionally log it)
	cmd.Stderr = io.Discard

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("start mcp server %q: %w", command, err)
	}

	scanner := bufio.NewScanner(stdout)
	scanner.Buffer(make([]byte, 0, 1024*1024), 10*1024*1024) // 10MB max line

	return &StdioTransport{
		cmd:    cmd,
		stdin:  stdin,
		stdout: scanner,
	}, nil
}
```

- [ ] **Step 2: Implement Send (request with response) and Notify (no response)**

```go
func (t *StdioTransport) Send(ctx context.Context, method string, params any) (json.RawMessage, error) {
	id := int(t.nextID.Add(1))
	return t.sendRequest(ctx, &id, method, params)
}

func (t *StdioTransport) Notify(ctx context.Context, method string, params any) error {
	_, err := t.sendRequest(ctx, nil, method, params)
	return err
}

func (t *StdioTransport) sendRequest(ctx context.Context, id *int, method string, params any) (json.RawMessage, error) {
	var rawParams json.RawMessage
	if params != nil {
		b, err := json.Marshal(params)
		if err != nil {
			return nil, fmt.Errorf("marshal params: %w", err)
		}
		rawParams = b
	}

	req := jsonRPCRequest{
		JSONRPC: "2.0",
		ID:      id,
		Method:  method,
		Params:  rawParams,
	}

	data, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	t.mu.Lock()
	_, err = t.stdin.Write(append(data, '\n'))
	t.mu.Unlock()
	if err != nil {
		return nil, fmt.Errorf("write to mcp server: %w", err)
	}

	// Notifications don't expect a response
	if id == nil {
		return nil, nil
	}

	// Read response (blocking — stdio is synchronous per-request)
	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		if !t.stdout.Scan() {
			if err := t.stdout.Err(); err != nil {
				return nil, fmt.Errorf("read from mcp server: %w", err)
			}
			return nil, fmt.Errorf("mcp server closed stdout")
		}

		var resp jsonRPCResponse
		if err := json.Unmarshal(t.stdout.Bytes(), &resp); err != nil {
			// Skip non-JSON lines (some servers emit logs to stdout)
			continue
		}

		// Skip notifications from server (no ID)
		if resp.ID == nil {
			continue
		}

		if *resp.ID != *id {
			// Mismatched ID — skip (shouldn't happen in serial mode)
			continue
		}

		if resp.Error != nil {
			return nil, resp.Error
		}

		return resp.Result, nil
	}
}
```

- [ ] **Step 3: Implement Close**

```go
func (t *StdioTransport) Close() error {
	t.stdin.Close()
	return t.cmd.Wait()
}
```

- [ ] **Step 4: Write tests**

Test with a mock subprocess (a small Go test helper that reads JSON-RPC from stdin and writes responses to stdout). Verify:
- Request/response round-trip
- Notification (no response expected)
- Large payloads (scanner buffer)
- Context cancellation
- Subprocess exit handling

---

## Task 3: Production DiscoveryClient

**File:** `pkg/mcp/discovery.go` (new)

This implements the `DiscoveryClient` interface. It manages per-server transports (lazy-initialized on first `Discover()` call) and translates between the MCP protocol and smolbot's `RemoteTool`/`tool.Result` types.

- [ ] **Step 1: Implement StdioDiscoveryClient**

```go
package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"sync"

	"github.com/Nomadcxx/smolbot/pkg/tool"
)

type StdioDiscoveryClient struct {
	mu         sync.Mutex
	transports map[string]*StdioTransport // keyed by server name
	logger     *slog.Logger
}

func NewStdioDiscoveryClient(logger *slog.Logger) *StdioDiscoveryClient {
	if logger == nil {
		logger = slog.Default()
	}
	return &StdioDiscoveryClient{
		transports: make(map[string]*StdioTransport),
		logger:     logger,
	}
}
```

- [ ] **Step 2: Implement transport lifecycle (connect + initialize)**

```go
func (c *StdioDiscoveryClient) getOrConnect(ctx context.Context, spec ConnectionSpec) (*StdioTransport, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if t, ok := c.transports[spec.Name]; ok {
		return t, nil
	}

	c.logger.Info("starting mcp server", "name", spec.Name, "command", spec.Command)

	t, err := NewStdioTransport(ctx, spec.Command, spec.Args, spec.Env)
	if err != nil {
		return nil, fmt.Errorf("start mcp server %s: %w", spec.Name, err)
	}

	// MCP handshake: initialize
	initParams := map[string]any{
		"protocolVersion": "2024-11-05",
		"capabilities":    map[string]any{},
		"clientInfo": map[string]any{
			"name":    "smolbot",
			"version": "1.0.0",
		},
	}

	result, err := t.Send(ctx, "initialize", initParams)
	if err != nil {
		t.Close()
		return nil, fmt.Errorf("mcp initialize %s: %w", spec.Name, err)
	}

	var initResult mcpInitResult
	if err := json.Unmarshal(result, &initResult); err != nil {
		t.Close()
		return nil, fmt.Errorf("parse initialize result for %s: %w", spec.Name, err)
	}

	c.logger.Info("mcp server initialized",
		"name", spec.Name,
		"serverName", initResult.ServerInfo.Name,
		"serverVersion", initResult.ServerInfo.Version,
		"protocol", initResult.ProtocolVersion,
	)

	// Send initialized notification
	if err := t.Notify(ctx, "notifications/initialized", nil); err != nil {
		c.logger.Warn("failed to send initialized notification", "name", spec.Name, "error", err)
		// Non-fatal — some servers don't require this
	}

	c.transports[spec.Name] = t
	return t, nil
}
```

- [ ] **Step 3: Implement Discover (tools/list)**

```go
func (c *StdioDiscoveryClient) Discover(ctx context.Context, spec ConnectionSpec) ([]RemoteTool, error) {
	if spec.Transport != TransportStdio {
		c.logger.Warn("unsupported transport for discovery", "name", spec.Name, "transport", spec.Transport)
		return nil, fmt.Errorf("transport %q not yet supported (only stdio implemented)", spec.Transport)
	}

	t, err := c.getOrConnect(ctx, spec)
	if err != nil {
		return nil, err
	}

	result, err := t.Send(ctx, "tools/list", map[string]any{})
	if err != nil {
		return nil, fmt.Errorf("tools/list for %s: %w", spec.Name, err)
	}

	var listResult mcpToolsListResult
	if err := json.Unmarshal(result, &listResult); err != nil {
		return nil, fmt.Errorf("parse tools/list result for %s: %w", spec.Name, err)
	}

	tools := make([]RemoteTool, len(listResult.Tools))
	for i, t := range listResult.Tools {
		tools[i] = RemoteTool{
			Name:        t.Name,
			Description: t.Description,
			InputSchema: t.InputSchema,
		}
	}

	c.logger.Info("discovered mcp tools", "name", spec.Name, "count", len(tools))
	return tools, nil
}
```

- [ ] **Step 4: Implement Invoke (tools/call)**

```go
func (c *StdioDiscoveryClient) Invoke(ctx context.Context, spec ConnectionSpec, toolName string, args json.RawMessage) (*tool.Result, error) {
	t, err := c.getOrConnect(ctx, spec)
	if err != nil {
		return nil, err
	}

	params := map[string]any{
		"name":      toolName,
		"arguments": json.RawMessage(args),
	}

	result, err := t.Send(ctx, "tools/call", params)
	if err != nil {
		return nil, fmt.Errorf("tools/call %s on %s: %w", toolName, spec.Name, err)
	}

	var callResult mcpCallResult
	if err := json.Unmarshal(result, &callResult); err != nil {
		return nil, fmt.Errorf("parse tools/call result for %s/%s: %w", spec.Name, toolName, err)
	}

	// Concatenate text content blocks
	var output strings.Builder
	for _, content := range callResult.Content {
		if content.Type == "text" {
			if output.Len() > 0 {
				output.WriteString("\n")
			}
			output.WriteString(content.Text)
		}
	}

	if callResult.IsError {
		return &tool.Result{Error: output.String()}, nil
	}

	return &tool.Result{Output: output.String()}, nil
}
```

- [ ] **Step 5: Implement Close (shutdown all transports)**

```go
func (c *StdioDiscoveryClient) Close() {
	c.mu.Lock()
	defer c.mu.Unlock()

	for name, t := range c.transports {
		c.logger.Info("stopping mcp server", "name", name)
		t.Close()
	}
	c.transports = make(map[string]*StdioTransport)
}
```

- [ ] **Step 6: Write tests**

Test with a real subprocess (create a `testdata/echo_mcp_server.sh` or use a Go test binary):
- Discover returns tools from a mock MCP server
- Invoke returns tool results
- Error responses map to tool.Result.Error
- Connection reuse (second Discover on same server reuses transport)
- Timeout handling

---

## Task 4: Wire Into Runtime

**Files:**
- Modify: `cmd/smolbot/runtime.go`
- Modify: `pkg/mcp/client.go`

- [ ] **Step 1: Fix `newMCPMgr()` to use real client**

In `cmd/smolbot/runtime.go`, replace the nil client:

```go
var newMCPMgr = func() mcpDiscoveryManager {
	client := mcp.NewStdioDiscoveryClient(slog.Default())
	return mcp.NewManager(client)
}
```

Import `log/slog` and `github.com/Nomadcxx/smolbot/pkg/mcp` (already imported for `mcp.NewManager`).

- [ ] **Step 2: Add cleanup on runtime shutdown**

The `StdioDiscoveryClient` owns subprocess lifetimes. We need to close it when the runtime shuts down.

Option A: Add a `Closer` interface to `mcpDiscoveryManager`:

```go
type mcpDiscoveryManager interface {
	DiscoverAndRegister(ctx context.Context, registry *tool.Registry, servers map[string]config.MCPServerConfig) ([]string, error)
}
```

Since `Manager` doesn't own the client lifecycle, add a shutdown hook in the runtime:

```go
// After DiscoverAndRegister succeeds, store client for cleanup
var mcpClient *mcp.StdioDiscoveryClient

var newMCPMgr = func() (mcpDiscoveryManager, func()) {
	client := mcp.NewStdioDiscoveryClient(slog.Default())
	cleanup := func() { client.Close() }
	return mcp.NewManager(client), cleanup
}
```

Call `cleanup()` when the runtime shuts down (wherever the existing cleanup/defer chain is).

- [ ] **Step 3: Fix silent nil failure in client.go**

In `pkg/mcp/client.go`, add a log warning when client is nil:

```go
func (m *Manager) DiscoverAndRegister(ctx context.Context, registry *tool.Registry, servers map[string]config.MCPServerConfig) ([]string, error) {
	if m.client == nil {
		slog.Warn("mcp manager has no discovery client, skipping tool registration",
			"servers", len(servers))
		return nil, nil
	}
	// ... rest unchanged
}
```

- [ ] **Step 4: Log discovered tool names**

After `DiscoverAndRegister` returns in runtime.go, log what was registered:

```go
if len(cfg.Tools.MCPServers) > 0 {
	mgr, mcpCleanup := newMCPMgr()
	defer mcpCleanup()
	warnings, err := mgr.DiscoverAndRegister(context.Background(), tools, cfg.Tools.MCPServers)
	if err != nil {
		_ = sessions.Close()
		return nil, fmt.Errorf("mcp discovery: %w", err)
	}
	for _, w := range warnings {
		slog.Warn("mcp discovery warning", "msg", w)
	}
}
```

- [ ] **Step 5: Verify with hybrid-memory**

Build and run smolbot. Confirm:
1. Daemon log shows "starting mcp server" and "mcp server initialized" for hybrid-memory
2. Daemon log shows "discovered mcp tools" with correct count
3. In TUI, send a message that would trigger tool use (e.g. "remember that my favorite color is blue")
4. The LLM should see `mcp_hybrid-memory_memory_store` in its available tools
5. Tool invocation should succeed and return a result

---

## Task 5: Graceful Error Handling

**Files:**
- Modify: `pkg/mcp/discovery.go`
- Modify: `pkg/mcp/stdio.go`

- [ ] **Step 1: Handle subprocess crashes**

If the MCP server process exits unexpectedly during an Invoke, detect it and remove the cached transport so the next call attempts a reconnect:

```go
func (c *StdioDiscoveryClient) Invoke(ctx context.Context, spec ConnectionSpec, toolName string, args json.RawMessage) (*tool.Result, error) {
	t, err := c.getOrConnect(ctx, spec)
	if err != nil {
		return nil, err
	}

	// ... send request ...

	result, err := t.Send(ctx, "tools/call", params)
	if err != nil {
		// Check if subprocess died
		c.mu.Lock()
		if t.cmd.ProcessState != nil { // process exited
			delete(c.transports, spec.Name)
			c.logger.Warn("mcp server crashed, will reconnect on next call", "name", spec.Name)
		}
		c.mu.Unlock()
		return nil, fmt.Errorf("tools/call %s on %s: %w", toolName, spec.Name, err)
	}
	// ...
}
```

- [ ] **Step 2: Startup timeout**

Add a timeout for MCP server initialization. Some servers may hang on startup:

```go
func (c *StdioDiscoveryClient) getOrConnect(ctx context.Context, spec ConnectionSpec) (*StdioTransport, error) {
	// ... lock, check cache ...

	// Use a 30-second timeout for initialization
	initCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	t, err := NewStdioTransport(initCtx, spec.Command, spec.Args, spec.Env)
	// ...
}
```

- [ ] **Step 3: Handle malformed tool responses**

If `tools/call` returns unexpected content types (e.g., `image` instead of `text`), handle gracefully:

```go
for _, content := range callResult.Content {
	switch content.Type {
	case "text":
		output.WriteString(content.Text)
	default:
		output.WriteString(fmt.Sprintf("[unsupported content type: %s]", content.Type))
	}
}
```

---

## Task 6: Tests

**Files:**
- New: `pkg/mcp/stdio_test.go`
- New: `pkg/mcp/discovery_test.go`
- Modify: `pkg/mcp/client_test.go`

- [ ] **Step 1: Create test MCP server helper**

A minimal Go test helper that acts as an MCP server over stdio:

```go
// pkg/mcp/testdata/mock_server.go
// Build with: go build -o mock_server ./pkg/mcp/testdata/
//
// Or use inline in tests via exec.Command("go", "run", "testdata/mock_server.go")

package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
)

func main() {
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		var req map[string]any
		json.Unmarshal(scanner.Bytes(), &req)

		id := req["id"]
		method := req["method"].(string)

		switch method {
		case "initialize":
			respond(id, map[string]any{
				"protocolVersion": "2024-11-05",
				"capabilities":   map[string]any{"tools": map[string]any{}},
				"serverInfo":     map[string]any{"name": "mock", "version": "0.1.0"},
			})
		case "notifications/initialized":
			// no response
		case "tools/list":
			respond(id, map[string]any{
				"tools": []map[string]any{
					{"name": "echo", "description": "Echo input", "inputSchema": map[string]any{"type": "object"}},
				},
			})
		case "tools/call":
			params := req["params"].(map[string]any)
			respond(id, map[string]any{
				"content": []map[string]any{
					{"type": "text", "text": fmt.Sprintf("echoed: %v", params["arguments"])},
				},
				"isError": false,
			})
		}
	}
}

func respond(id any, result any) {
	resp := map[string]any{"jsonrpc": "2.0", "id": id, "result": result}
	data, _ := json.Marshal(resp)
	fmt.Println(string(data))
}
```

- [ ] **Step 2: Write stdio transport tests**

```go
func TestStdioTransportRoundTrip(t *testing.T) {
	// Start mock server
	// Send initialize → get response
	// Send tools/list → get tools
	// Send tools/call → get result
	// Close transport → verify subprocess exits
}

func TestStdioTransportContextCancel(t *testing.T) {
	// Start server, cancel context mid-request
	// Verify error returned
}
```

- [ ] **Step 3: Write discovery client integration tests**

```go
func TestStdioDiscoveryClientDiscover(t *testing.T) {
	// Create client with mock server config
	// Discover → verify RemoteTool list
	// Discover again → verify transport reuse (no second subprocess)
}

func TestStdioDiscoveryClientInvoke(t *testing.T) {
	// Discover first, then Invoke a tool
	// Verify tool.Result output
}

func TestStdioDiscoveryClientErrorTool(t *testing.T) {
	// Invoke a tool that returns isError: true
	// Verify tool.Result.Error is set
}
```

- [ ] **Step 4: Update existing client_test.go**

Add a test that verifies `DiscoverAndRegister` logs a warning when client is nil:

```go
func TestDiscoverAndRegisterNilClient(t *testing.T) {
	manager := NewManager(nil)
	registry := tool.NewRegistry()
	warnings, err := manager.DiscoverAndRegister(context.Background(), registry, map[string]config.MCPServerConfig{
		"test": {Command: "echo"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if warnings != nil {
		t.Fatalf("expected nil warnings, got %v", warnings)
	}
	if len(registry.Definitions()) != 0 {
		t.Fatalf("expected no tools registered")
	}
}
```

---

## Priority Order

| Priority | Task | Rationale |
|----------|------|-----------|
| P0 | Task 1: JSON-RPC types | Foundation — everything else depends on this |
| P0 | Task 2: Stdio transport | Core transport — hybrid-memory uses stdio |
| P0 | Task 3: Discovery client | Implements the missing interface |
| P0 | Task 4: Wire into runtime | Makes it work end-to-end |
| P1 | Task 5: Error handling | Robustness for production use |
| P1 | Task 6: Tests | Verify correctness and catch regressions |

---

## Testing Strategy

- **Unit tests:** JSON-RPC marshaling, transport send/receive, tool result mapping
- **Integration tests:** Full Discover → Invoke cycle with mock MCP server subprocess
- **Manual verification:** Build smolbot, start with hybrid-memory configured, verify tools appear in LLM function calls and invocations succeed
- **Regression:** Existing `TestDiscoverAndRegister` and `TestEnabledToolsWildcard` must still pass

## Out of Scope (Future Work)

1. **SSE transport** — For remote MCP servers accessible via HTTP SSE. Not needed for hybrid-memory (stdio).
2. **Streamable HTTP transport** — Newer MCP transport. Not needed currently.
3. **Resource discovery** — MCP `resources/list` and `resources/read`. Not needed for tool-focused servers.
4. **Prompt discovery** — MCP `prompts/list`. Not needed currently.
5. **Server-initiated notifications** — MCP servers can send notifications. Would require a background reader goroutine.
6. **Connection pooling** — Current design: one transport per server, kept alive for the daemon lifetime. Sufficient for now.

## Risks

1. **Stdio blocking:** The current implementation reads stdout synchronously. If a server sends unexpected output (logs, progress) before the JSON-RPC response, the scanner may consume it. Mitigation: skip lines that don't parse as JSON-RPC responses.
2. **Process lifetime:** MCP server processes live as long as the daemon. If the daemon crashes without cleanup, orphan processes may remain. Mitigation: use `exec.CommandContext` with the daemon's context.
3. **Large tool responses:** Some MCP tools may return large payloads. Mitigation: 10MB scanner buffer limit.
4. **Multiple transports:** The `DiscoveryClient` interface doesn't distinguish transports — only `StdioDiscoveryClient` is implemented. If a config has an SSE server, it will return an error. This is acceptable for now; the error message is clear.
