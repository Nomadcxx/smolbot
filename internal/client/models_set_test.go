package client

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

func TestModelsSetSendsCanonicalModelPayload(t *testing.T) {
	type capturedRequest struct {
		frame Request
		raw   []byte
	}

	captured := make(chan capturedRequest, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upgrader := websocket.Upgrader{CheckOrigin: func(*http.Request) bool { return true }}
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Errorf("upgrade websocket: %v", err)
			return
		}
		defer conn.Close()

		_, raw, err := conn.ReadMessage()
		if err != nil {
			t.Errorf("read hello request: %v", err)
			return
		}
		var req Request
		if err := json.Unmarshal(raw, &req); err != nil {
			t.Errorf("unmarshal hello request: %v", err)
			return
		}

		helloPayload, err := json.Marshal(HelloPayload{
			Server:   "smolbot",
			Version:  "test",
			Protocol: ProtocolVersion,
			Methods:  []string{"models.list", "models.set"},
			Events:   []string{},
		})
		if err != nil {
			t.Errorf("marshal hello payload: %v", err)
			return
		}
		helloResponse, err := json.Marshal(Response{
			Type:    FrameRes,
			ID:      req.ID,
			OK:      true,
			Payload: helloPayload,
		})
		if err != nil {
			t.Errorf("marshal hello response: %v", err)
			return
		}
		if err := conn.WriteMessage(websocket.TextMessage, helloResponse); err != nil {
			t.Errorf("write hello response: %v", err)
			return
		}

		_, raw, err = conn.ReadMessage()
		if err != nil {
			t.Errorf("read models.set request: %v", err)
			return
		}
		if err := json.Unmarshal(raw, &req); err != nil {
			t.Errorf("unmarshal models.set request: %v", err)
			return
		}

		captured <- capturedRequest{frame: req, raw: raw}

		resultPayload, err := json.Marshal(map[string]string{"previous": "gpt-test"})
		if err != nil {
			t.Errorf("marshal models.set response: %v", err)
			return
		}
		response, err := json.Marshal(Response{
			Type:    FrameRes,
			ID:      req.ID,
			OK:      true,
			Payload: resultPayload,
		})
		if err != nil {
			t.Errorf("marshal models.set response frame: %v", err)
			return
		}
		if err := conn.WriteMessage(websocket.TextMessage, response); err != nil {
			t.Errorf("write models.set response: %v", err)
			return
		}
	}))
	defer server.Close()

	c := New("ws" + strings.TrimPrefix(server.URL, "http"))
	hello, err := c.Connect()
	if err != nil {
		t.Fatalf("Connect: %v", err)
	}
	defer c.Close()

	if hello.Server != "smolbot" {
		t.Fatalf("unexpected hello payload: %#v", hello)
	}

	previous, err := c.ModelsSet("claude-sonnet")
	if err != nil {
		t.Fatalf("ModelsSet: %v", err)
	}
	if previous != "gpt-test" {
		t.Fatalf("previous model = %q, want gpt-test", previous)
	}

	select {
	case got := <-captured:
		if got.frame.Method != "models.set" {
			t.Fatalf("expected models.set request, got %#v", got.frame)
		}
		if !bytes.Contains(got.raw, []byte(`"model":"claude-sonnet"`)) {
			t.Fatalf("expected canonical model payload, got %s", got.raw)
		}
		paramsRaw, err := json.Marshal(got.frame.Params)
		if err != nil {
			t.Fatalf("marshal models.set params: %v", err)
		}
		var params ModelsSetParams
		if err := json.Unmarshal(paramsRaw, &params); err != nil {
			t.Fatalf("unmarshal models.set params: %v", err)
		}
		if params.Model != "claude-sonnet" {
			t.Fatalf("models.set model = %q, want claude-sonnet", params.Model)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for models.set request")
	}
}
