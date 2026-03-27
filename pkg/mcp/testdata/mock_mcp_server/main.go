package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"
)

type request struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      *int            `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type response struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      *int            `json:"id,omitempty"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   any             `json:"error,omitempty"`
}

func main() {
	scanner := bufio.NewScanner(os.Stdin)
	scanner.Buffer(make([]byte, 0, 1024*1024), 12*1024*1024)
	for scanner.Scan() {
		var req request
		if err := json.Unmarshal(scanner.Bytes(), &req); err != nil {
			continue
		}
		switch req.Method {
		case "initialize":
			if os.Getenv("MOCK_MCP_DELAY_INIT") == "1" {
				time.Sleep(200 * time.Millisecond)
			}
			writeResult(req.ID, map[string]any{
				"protocolVersion": "2024-11-05",
				"capabilities":    map[string]any{"tools": map[string]any{}},
				"serverInfo": map[string]any{
					"name":    "mock-mcp",
					"version": "0.1.0",
				},
			})
			if path := os.Getenv("MOCK_MCP_INIT_LOG"); path != "" {
				f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
				if err == nil {
					_, _ = fmt.Fprintln(f, "init")
					_ = f.Close()
				}
			}
		case "notifications/initialized":
		case "tools/list":
			if os.Getenv("MOCK_MCP_DELAY_LIST") == "1" {
				time.Sleep(200 * time.Millisecond)
			}
			if os.Getenv("MOCK_MCP_NOTIFY_BURST") == "1" {
				for i := 0; i < 3; i++ {
					writeNotification("notifications/message", map[string]any{
						"level": "info",
						"data":  fmt.Sprintf("burst-%d", i),
					})
				}
			}
			if os.Getenv("MOCK_MCP_LARGE") == "1" {
				writeResult(req.ID, map[string]any{
					"tools": []map[string]any{
						{
							"name":        "echo",
							"description": strings.Repeat("large-", 70_000),
							"inputSchema": map[string]any{"type": "object"},
						},
					},
				})
				continue
			}
			writeResult(req.ID, map[string]any{
				"tools": []map[string]any{
					{
						"name":        "echo",
						"description": "Echo input",
						"inputSchema": map[string]any{"type": "object"},
					},
				},
			})
		case "tools/call":
			if os.Getenv("MOCK_MCP_EXIT_ON_CALL") == "1" {
				os.Exit(3)
			}
			var params struct {
				Name      string          `json:"name"`
				Arguments json.RawMessage `json:"arguments"`
			}
			_ = json.Unmarshal(req.Params, &params)
			argText := string(params.Arguments)
			if os.Getenv("MOCK_MCP_ERROR_ON_CALL") == "1" {
				writeResult(req.ID, map[string]any{
					"content": []map[string]any{
						{"type": "text", "text": "boom"},
					},
					"isError": true,
				})
				continue
			}
			if os.Getenv("MOCK_MCP_UNSUPPORTED_CONTENT") == "1" {
				writeResult(req.ID, map[string]any{
					"content": []map[string]any{
						{"type": "image", "text": ""},
					},
					"isError": false,
				})
				continue
			}
			if strings.Contains(argText, "hello") {
				writeResult(req.ID, map[string]any{
					"content": []map[string]any{
						{"type": "text", "text": "echoed: hello"},
					},
					"isError": false,
				})
				continue
			}
			writeResult(req.ID, map[string]any{
				"content": []map[string]any{
					{"type": "text", "text": "echoed: " + argText},
				},
				"isError": false,
			})
		}
	}
}

func writeResult(id *int, result any) {
	data, err := json.Marshal(result)
	if err != nil {
		return
	}
	resp := response{
		JSONRPC: "2.0",
		ID:      id,
		Result:  data,
	}
	out, err := json.Marshal(resp)
	if err != nil {
		return
	}
	_, _ = fmt.Fprintln(os.Stdout, string(out))
}

func writeNotification(method string, params any) {
	payload, err := json.Marshal(params)
	if err != nil {
		return
	}
	out, err := json.Marshal(map[string]any{
		"jsonrpc": "2.0",
		"method":  method,
		"params":  json.RawMessage(payload),
	})
	if err != nil {
		return
	}
	_, _ = fmt.Fprintln(os.Stdout, string(out))
}
