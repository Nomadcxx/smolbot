package gateway

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Nomadcxx/smolbot/pkg/agent"
	"github.com/gorilla/websocket"
)

func TestGatewayConcurrency(t *testing.T) {
	processor := newBlockingAgent()
	server := NewServer(ServerDeps{Agent: processor})
	httpServer := httptest.NewServer(server.Handler())
	defer httpServer.Close()

	conn := dialWebsocket(t, httpServer.URL+"/ws")
	defer conn.Close()

	t.Run("starting sessions blocks duplicate chat send", func(t *testing.T) {
		writeFrame(t, conn, RequestFrame{
			ID:     "1",
			Method: "chat.send",
			Params: json.RawMessage(`{"session":"dup","message":"first"}`),
		})
		first := readFrame(t, conn)
		if !strings.Contains(string(first.Response.Result), `"runId":"run-dup"`) {
			t.Fatalf("unexpected first send response %#v", first)
		}

		writeFrame(t, conn, RequestFrame{
			ID:     "2",
			Method: "chat.send",
			Params: json.RawMessage(`{"session":"dup","message":"second"}`),
		})
		second := readFrame(t, conn)
		if second.Response.Error == nil || !strings.Contains(second.Response.Error.Message, "already active") {
			t.Fatalf("expected duplicate-send rejection, got %#v", second)
		}

		processor.finish("dup", "done")
		readUntilEvent(t, conn, "chat.done")
	})

	t.Run("chat abort respects run id", func(t *testing.T) {
		writeFrame(t, conn, RequestFrame{
			ID:     "3",
			Method: "chat.send",
			Params: json.RawMessage(`{"session":"abort","message":"run"}`),
		})
		resp := readFrame(t, conn)
		if !strings.Contains(string(resp.Response.Result), `"runId":"run-abort"`) {
			t.Fatalf("unexpected run response %#v", resp)
		}

		writeFrame(t, conn, RequestFrame{
			ID:     "4",
			Method: "chat.abort",
			Params: json.RawMessage(`{"runId":"run-abort"}`),
		})
		abortResp := readFrame(t, conn)
		if abortResp.Response.Error != nil {
			t.Fatalf("unexpected abort error %#v", abortResp.Response.Error)
		}
		readUntilEvent(t, conn, "chat.error")
	})

	t.Run("event bridging and thinking aggregation", func(t *testing.T) {
		writeFrame(t, conn, RequestFrame{
			ID:     "5",
			Method: "chat.send",
			Params: json.RawMessage(`{"session":"events","message":"go"}`),
		})
		resp := readFrame(t, conn)
		if !strings.Contains(string(resp.Response.Result), `"runId":"run-events"`) {
			t.Fatalf("unexpected run response %#v", resp)
		}

		processor.emit("events", agent.Event{Type: agent.EventThinking, Content: "part1 "})
		processor.emit("events", agent.Event{Type: agent.EventThinking, Content: "part2"})
		processor.emit("events", agent.Event{Type: agent.EventProgress, Content: "working"})
		processor.emit("events", agent.Event{Type: agent.EventToolStart, Content: "message"})
		processor.emit("events", agent.Event{Type: agent.EventToolDone, Content: "message", Data: map[string]any{
			"deliveredToRequestTarget": true,
		}})
		processor.finish("events", "final text")

		progress := readUntilEvent(t, conn, "chat.progress")
		if !strings.Contains(string(progress.Event.Payload), `"content":"working"`) {
			t.Fatalf("unexpected progress event %#v", progress)
		}
		thinking := readUntilEvent(t, conn, "chat.thinking.done")
		if !strings.Contains(string(thinking.Event.Payload), `part1 part2`) {
			t.Fatalf("unexpected thinking aggregation %#v", thinking)
		}
		done := readUntilEvent(t, conn, "chat.done")
		if !strings.Contains(string(done.Event.Payload), `"content":"final text"`) {
			t.Fatalf("unexpected done event %#v", done)
		}
		if !server.completedDelivery["run-events"] {
			t.Fatalf("expected same-target delivery capture")
		}
	})

	t.Run("disconnect cancels websocket-owned runs", func(t *testing.T) {
		otherConn := dialWebsocket(t, httpServer.URL+"/ws")
		writeFrame(t, otherConn, RequestFrame{
			ID:     "6",
			Method: "chat.send",
			Params: json.RawMessage(`{"session":"disconnect","message":"go"}`),
		})
		_ = readFrame(t, otherConn)
		if err := otherConn.Close(); err != nil {
			t.Fatalf("Close: %v", err)
		}

		deadline := time.Now().Add(2 * time.Second)
		for time.Now().Before(deadline) {
			if processor.wasCancelled("disconnect") {
				return
			}
			time.Sleep(10 * time.Millisecond)
		}
		t.Fatal("expected disconnect cancellation")
	})
}

func TestGatewayShutdownCancelsRunsAndClosesClients(t *testing.T) {
	processor := newBlockingAgent()
	server := NewServer(ServerDeps{Agent: processor})
	httpServer := httptest.NewServer(server.Handler())
	defer httpServer.Close()

	conn := dialWebsocket(t, httpServer.URL+"/ws")

	writeFrame(t, conn, RequestFrame{
		ID:     "shutdown-1",
		Method: "chat.send",
		Params: json.RawMessage(`{"session":"shutdown","message":"go"}`),
	})
	_ = readFrame(t, conn)

	if err := server.Shutdown(context.Background()); err != nil {
		t.Fatalf("Shutdown: %v", err)
	}

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if processor.wasCancelled("shutdown") {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if !processor.wasCancelled("shutdown") {
		t.Fatal("expected active run cancellation on shutdown")
	}

	_ = conn.SetReadDeadline(time.Now().Add(500 * time.Millisecond))
	for {
		if _, _, err := conn.ReadMessage(); err != nil {
			break
		}
	}
}

type blockingAgent struct {
	mu       sync.Mutex
	runs     map[string]*runControl
	canceled map[string]bool
}

type runControl struct {
	cb     agent.EventCallback
	ctx    context.Context
	done   chan string
	cancel context.CancelFunc
}

func newBlockingAgent() *blockingAgent {
	return &blockingAgent{
		runs:     make(map[string]*runControl),
		canceled: make(map[string]bool),
	}
}

func (b *blockingAgent) ProcessDirect(ctx context.Context, req agent.Request, cb agent.EventCallback) (string, error) {
	runCtx, cancel := context.WithCancel(ctx)
	control := &runControl{cb: cb, ctx: runCtx, done: make(chan string, 1), cancel: cancel}
	b.mu.Lock()
	b.runs[req.SessionKey] = control
	b.mu.Unlock()

	select {
	case <-runCtx.Done():
		b.mu.Lock()
		b.canceled[req.SessionKey] = true
		b.mu.Unlock()
		return "", runCtx.Err()
	case result := <-control.done:
		return result, nil
	}
}

func (b *blockingAgent) CancelSession(sessionKey string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if run, ok := b.runs[sessionKey]; ok {
		run.cancel()
	}
}

func (b *blockingAgent) emit(sessionKey string, event agent.Event) {
	b.mu.Lock()
	run := b.runs[sessionKey]
	b.mu.Unlock()
	if run != nil && run.cb != nil {
		run.cb(event)
	}
}

func (b *blockingAgent) finish(sessionKey, result string) {
	b.mu.Lock()
	run := b.runs[sessionKey]
	delete(b.runs, sessionKey)
	b.mu.Unlock()
	if run != nil {
		run.done <- result
	}
}

func (b *blockingAgent) wasCancelled(sessionKey string) bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.canceled[sessionKey]
}

func readUntilEvent(t *testing.T, conn *websocket.Conn, name string) *DecodedFrame {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	_ = conn.SetReadDeadline(deadline)
	for {
		_, data, err := conn.ReadMessage()
		if err != nil {
			t.Fatalf("ReadMessage: %v", err)
		}
		frame, err := DecodeFrame(data)
		if err != nil {
			t.Fatalf("DecodeFrame: %v", err)
		}
		if frame.Kind == FrameEvent && frame.Event.EventName == name {
			return frame
		}
	}
}
