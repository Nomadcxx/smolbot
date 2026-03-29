package main

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/gorilla/websocket"
)

const wsURL = "ws://127.0.0.1:18791/ws"

type Frame struct {
	Type    string          `json:"type"`
	ID      string          `json:"id,omitempty"`
	Method  string          `json:"method,omitempty"`
	Params  json.RawMessage `json:"params,omitempty"`
	Event   string          `json:"event,omitempty"`
	Seq     int64           `json:"seq,omitempty"`
	Payload json.RawMessage `json:"payload,omitempty"`
	OK      bool            `json:"ok,omitempty"`
	Error   *ErrorFrame     `json:"error,omitempty"`
}

type ErrorFrame struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func main() {
	fmt.Println("=== TUI Integration Verification ===")

	// Connect
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		fmt.Printf("Connect failed: %v\n", err)
		os.Exit(1)
	}
	defer conn.Close()
	fmt.Printf("Connected to %s\n", wsURL)

	// Test 1: Hello
	fmt.Println("\n--- Test 1: Hello ---")
	if err := testHello(conn); err != nil {
		fmt.Printf("Hello failed: %v\n", err)
	} else {
		fmt.Println("Hello passed")
	}

	// Test 2: Status
	fmt.Println("\n--- Test 2: Status ---")
	if err := testStatus(conn); err != nil {
		fmt.Printf("Status failed: %v\n", err)
	} else {
		fmt.Println("Status passed")
	}

	// Test 3: Models.List
	fmt.Println("\n--- Test 3: Models.List ---")
	if err := testModelsList(conn); err != nil {
		fmt.Printf("Models.List failed: %v\n", err)
	} else {
		fmt.Println("Models.List passed")
	}

	// Test 4: Sessions.List
	fmt.Println("\n--- Test 4: Sessions.List ---")
	if err := testSessionsList(conn); err != nil {
		fmt.Printf("Sessions.List failed: %v\n", err)
	} else {
		fmt.Println("Sessions.List passed")
	}

	// Test 5: Chat.Send with Event Streaming
	fmt.Println("\n--- Test 5: Chat.Send with Event Streaming ---")
	if err := testChatSend(conn); err != nil {
		fmt.Printf("Chat.Send failed: %v\n", err)
	} else {
		fmt.Println("Chat.Send passed")
	}

	fmt.Println("\n=== Verification Complete ===")
}

func sendRequest(conn *websocket.Conn, method string, params any) (*Frame, error) {
	id := fmt.Sprintf("%d", time.Now().UnixNano())
	paramsJSON, _ := json.Marshal(params)

	req := Frame{
		Type:   "req",
		ID:     id,
		Method: method,
		Params: paramsJSON,
	}

	if err := conn.WriteJSON(req); err != nil {
		return nil, err
	}

	// Read response
	var res Frame
	if err := conn.ReadJSON(&res); err != nil {
		return nil, err
	}

	if res.Error != nil {
		return nil, fmt.Errorf("%s: %s", res.Error.Code, res.Error.Message)
	}

	return &res, nil
}

func testHello(conn *websocket.Conn) error {
	params, _ := json.Marshal(map[string]any{
		"client":   "test-client",
		"version":  "1.0.0",
		"protocol": 1,
		"platform": "linux",
	})

	req := Frame{
		Type:   "req",
		ID:     "hello-1",
		Method: "hello",
		Params: params,
	}

	if err := conn.WriteJSON(req); err != nil {
		return err
	}

	var res Frame
	if err := conn.ReadJSON(&res); err != nil {
		return err
	}

	if res.Error != nil {
		return fmt.Errorf("%s: %s", res.Error.Code, res.Error.Message)
	}

	var payload struct {
		Server   string   `json:"server"`
		Version  string   `json:"version"`
		Protocol int      `json:"protocol"`
		Methods  []string `json:"methods"`
		Events   []string `json:"events"`
	}
	if err := json.Unmarshal(res.Payload, &payload); err != nil {
		return fmt.Errorf("parse payload: %w", err)
	}

	fmt.Printf("  Server: %s@%s (protocol %d)\n", payload.Server, payload.Version, payload.Protocol)
	fmt.Printf("  Methods: %v\n", payload.Methods)
	fmt.Printf("  Events: %v\n", payload.Events)

	return nil
}

func testStatus(conn *websocket.Conn) error {
	res, err := sendRequest(conn, "status", map[string]string{})
	if err != nil {
		return err
	}

	var payload struct {
		Model            string         `json:"model"`
		Provider         string         `json:"provider"`
		UptimeSeconds    int            `json:"uptimeSeconds"`
		Channels         []string       `json:"channels"`
		ChannelStates    map[string]any `json:"channelStates"`
		ConnectedClients int            `json:"connectedClients"`
	}
	if err := json.Unmarshal(res.Payload, &payload); err != nil {
		return fmt.Errorf("parse payload: %w", err)
	}

	fmt.Printf("  Model: %s (%s)\n", payload.Model, payload.Provider)
	fmt.Printf("  Uptime: %ds, Clients: %d\n", payload.UptimeSeconds, payload.ConnectedClients)
	fmt.Printf("  Channels: %v\n", payload.Channels)

	return nil
}

func testModelsList(conn *websocket.Conn) error {
	res, err := sendRequest(conn, "models.list", map[string]string{})
	if err != nil {
		return err
	}

	var payload struct {
		Models  []map[string]string `json:"models"`
		Current string              `json:"current"`
	}
	if err := json.Unmarshal(res.Payload, &payload); err != nil {
		return fmt.Errorf("parse payload: %w", err)
	}

	fmt.Printf("  Current: %s\n", payload.Current)
	fmt.Printf("  Available: %d models\n", len(payload.Models))
	if len(payload.Models) > 2 {
		fmt.Printf("  First 3: %s, %s, %s...\n",
			payload.Models[0]["name"],
			payload.Models[1]["name"],
			payload.Models[2]["name"])
	}

	return nil
}

func testSessionsList(conn *websocket.Conn) error {
	res, err := sendRequest(conn, "sessions.list", map[string]string{})
	if err != nil {
		return err
	}

	var payload struct {
		Sessions []map[string]string `json:"sessions"`
	}
	if err := json.Unmarshal(res.Payload, &payload); err != nil {
		return fmt.Errorf("parse payload: %w", err)
	}

	fmt.Printf("  Sessions: %d\n", len(payload.Sessions))

	return nil
}

func testChatSend(conn *websocket.Conn) error {
	// First send the chat request
	params, _ := json.Marshal(map[string]any{
		"session": "test-session",
		"message": "What is 2+2?",
	})

	req := Frame{
		Type:   "req",
		ID:     "chat-1",
		Method: "chat.send",
		Params: params,
	}

	if err := conn.WriteJSON(req); err != nil {
		return err
	}

	// Read response (should contain runId)
	var res Frame
	if err := conn.ReadJSON(&res); err != nil {
		return err
	}

	if res.Error != nil {
		return fmt.Errorf("%s: %s", res.Error.Code, res.Error.Message)
	}

	var payload struct {
		RunID string `json:"runId"`
	}
	if err := json.Unmarshal(res.Payload, &payload); err != nil {
		return fmt.Errorf("parse chat.send payload: %w", err)
	}

	fmt.Printf("  RunID: %s\n", payload.RunID)
	return nil
}
