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
				Payload: json.RawMessage(`{"current":"claude-sonnet","previous":"gpt-test"}`),
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

	current, err := c.ModelsSet("claude-sonnet")
	if err != nil {
		t.Fatalf("ModelsSet: %v", err)
	}
	if current != "claude-sonnet" {
		t.Fatalf("current model = %q, want claude-sonnet", current)
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

func TestModelsSetRejectsResponseWithoutCurrentModel(t *testing.T) {
	upgrader := websocket.Upgrader{CheckOrigin: func(*http.Request) bool { return true }}

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
			var wire Request
			if err := json.Unmarshal(raw, &wire); err != nil {
				t.Fatalf("Unmarshal models.set: %v", err)
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

	if _, err := c.ModelsSet("claude-sonnet"); err == nil || !strings.Contains(err.Error(), "missing current model") {
		t.Fatalf("expected missing-current error, got %v", err)
	}
}

func TestSessionsResetSendsSessionParam(t *testing.T) {
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
			if err := conn.WriteJSON(Response{
				Type:    FrameRes,
				ID:      hello.ID,
				OK:      true,
				Payload: json.RawMessage(`{"server":"smolbot","version":"test","protocol":1,"methods":["sessions.reset"],"events":[]}`),
			}); err != nil {
				t.Fatalf("Write hello: %v", err)
			}
		}

		if _, raw, err := conn.ReadMessage(); err != nil {
			t.Fatalf("Read sessions.reset: %v", err)
		} else {
			var wire struct {
				Method string          `json:"method"`
				ID     string          `json:"id"`
				Params json.RawMessage `json:"params"`
			}
			if err := json.Unmarshal(raw, &wire); err != nil {
				t.Fatalf("Unmarshal wire: %v", err)
			}
			received <- append([]byte(nil), wire.Params...)
			if err := conn.WriteJSON(Response{
				Type:    FrameRes,
				ID:      wire.ID,
				OK:      true,
				Payload: json.RawMessage(`{"ok":true}`),
			}); err != nil {
				t.Fatalf("Write response: %v", err)
			}
		}
	}))
	defer srv.Close()

	c := New("ws" + strings.TrimPrefix(srv.URL, "http") + "/ws")
	defer c.Close()
	if _, err := c.Connect(); err != nil {
		t.Fatalf("Connect: %v", err)
	}

	if err := c.SessionsReset("my-session"); err != nil {
		t.Fatalf("SessionsReset: %v", err)
	}

	var params map[string]string
	if err := json.Unmarshal(<-received, &params); err != nil {
		t.Fatalf("Unmarshal params: %v", err)
	}
	if got, ok := params["session"]; !ok || got != "my-session" {
		t.Fatalf("sessions.reset params = %#v, want {\"session\":\"my-session\"}", params)
	}
	if _, hasKey := params["key"]; hasKey {
		t.Fatalf("sessions.reset still sends old 'key' field: %#v", params)
	}
}
