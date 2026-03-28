package client

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gorilla/websocket"
)

func TestModelsSetUsesCanonicalModelField(t *testing.T) {
	upgrader := websocket.Upgrader{CheckOrigin: func(*http.Request) bool { return true }}
	received := make(chan json.RawMessage, 1)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Fatalf("Upgrade: %v", err)
		}
		defer conn.Close()

		if _, raw, err := conn.ReadMessage(); err != nil {
			t.Fatalf("Read hello: %v", err)
		} else {
			var hello Request
			if err := json.Unmarshal(raw, &hello); err != nil {
				t.Fatalf("Unmarshal hello: %v", err)
			}
			if hello.Method != "hello" {
				t.Fatalf("expected hello handshake, got %#v", hello)
			}
			if err := conn.WriteJSON(Response{
				Type:    FrameRes,
				ID:      hello.ID,
				OK:      true,
				Payload: json.RawMessage(`{"server":"smolbot","version":"test","protocol":1,"methods":["models.set"],"events":[]}`),
			}); err != nil {
				t.Fatalf("Write hello response: %v", err)
			}
		}

		if _, raw, err := conn.ReadMessage(); err != nil {
			t.Fatalf("Read models.set: %v", err)
		} else {
			var wire struct {
				Type   string          `json:"type"`
				ID     string          `json:"id"`
				Method string          `json:"method"`
				Params json.RawMessage `json:"params"`
			}
			if err := json.Unmarshal(raw, &wire); err != nil {
				t.Fatalf("Unmarshal models.set: %v", err)
			}
			received <- append([]byte(nil), wire.Params...)
			if wire.Method != "models.set" {
				t.Fatalf("expected models.set request, got %#v", wire)
			}
			if err := conn.WriteJSON(Response{
				Type:    FrameRes,
				ID:      wire.ID,
				OK:      true,
				Payload: json.RawMessage(`{"previous":"gpt-test"}`),
			}); err != nil {
				t.Fatalf("Write models.set response: %v", err)
			}
		}
	}))
	defer srv.Close()

	c := New("ws" + strings.TrimPrefix(srv.URL, "http") + "/ws")
	defer c.Close()

	if _, err := c.Connect(); err != nil {
		t.Fatalf("Connect: %v", err)
	}

	previous, err := c.ModelsSet("claude-sonnet")
	if err != nil {
		t.Fatalf("ModelsSet: %v", err)
	}
	if previous != "gpt-test" {
		t.Fatalf("previous model = %q, want gpt-test", previous)
	}

	var params struct {
		Model string `json:"model"`
		ID    string `json:"id"`
	}
	if err := json.Unmarshal(<-received, &params); err != nil {
		t.Fatalf("Unmarshal params: %v", err)
	}
	if params.Model != "claude-sonnet" {
		t.Fatalf("models.set params = %#v, want canonical model field", params)
	}
	if params.ID != "" {
		t.Fatalf("models.set params unexpectedly included id field: %#v", params)
	}
}
