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

// TestDuplicateSendQueuesInsteadOfRejecting verifies that a same-session
// chat.send while a run is active is queued rather than rejected.
func TestDuplicateSendQueuesInsteadOfRejecting(t *testing.T) {
	processor := newBlockingAgent()
	server := NewServer(ServerDeps{Agent: processor})
	httpServer := httptest.NewServer(server.Handler())
	defer httpServer.Close()

	conn := dialWebsocket(t, httpServer.URL+"/ws")
	defer conn.Close()

	writeFrame(t, conn, RequestFrame{
		ID:     "reg-1",
		Method: "chat.send",
		Params: json.RawMessage(`{"session":"reg","message":"first"}`),
	})
	first := readResponseFrame(t, conn, "reg-1")
	if first.Response.Error != nil {
		t.Fatalf("unexpected error on first send: %v", first.Response.Error)
	}

	writeFrame(t, conn, RequestFrame{
		ID:     "reg-2",
		Method: "chat.send",
		Params: json.RawMessage(`{"session":"reg","message":"second"}`),
	})
	second := readResponseFrame(t, conn, "reg-2")
	if second.Response.Error != nil {
		t.Fatalf("second send must queue, not error; got: %v", second.Response.Error)
	}
	if !strings.Contains(string(second.Response.Result), "runId") {
		t.Fatalf("expected runId in queued response, got %#v", second)
	}

	// chat.queued event must arrive.
	readEventFrame(t, conn, "chat.queued")

	// Finish the first run; the queued run starts automatically.
	processor.finish("reg", "first done")
	readEventFrame(t, conn, "chat.done")
	readEventFrame(t, conn, "chat.dequeued")

	// Finish the second run.
	processor.finish("reg", "second done")
	readEventFrame(t, conn, "chat.done")
	readEventFrame(t, conn, "chat.queue.drained")
}

func TestGatewayConcurrency(t *testing.T) {
	processor := newBlockingAgent()
	server := NewServer(ServerDeps{Agent: processor})
	httpServer := httptest.NewServer(server.Handler())
	defer httpServer.Close()

	conn := dialWebsocket(t, httpServer.URL+"/ws")
	defer conn.Close()

	t.Run("same-session duplicate send is queued not rejected", func(t *testing.T) {
		writeFrame(t, conn, RequestFrame{
			ID:     "1",
			Method: "chat.send",
			Params: json.RawMessage(`{"session":"dup","message":"first"}`),
		})
		first := readResponseFrame(t, conn, "1")
		if first.Response.Error != nil {
			t.Fatalf("unexpected error on first send: %v", first.Response.Error)
		}
		if !strings.Contains(string(first.Response.Result), "runId") {
			t.Fatalf("unexpected first send response %#v", first)
		}

		writeFrame(t, conn, RequestFrame{
			ID:     "2",
			Method: "chat.send",
			Params: json.RawMessage(`{"session":"dup","message":"second"}`),
		})
		second := readResponseFrame(t, conn, "2")
		if second.Response.Error != nil {
			t.Fatalf("second send must queue not error, got: %v", second.Response.Error)
		}
		if !strings.Contains(string(second.Response.Result), "runId") {
			t.Fatalf("expected runId in queued response, got %#v", second)
		}
		readEventFrame(t, conn, "chat.queued")

		// Finish first run; second starts automatically.
		processor.finish("dup", "first done")
		readEventFrame(t, conn, "chat.done")
		readEventFrame(t, conn, "chat.dequeued")

		// Finish second run and drain.
		processor.finish("dup", "second done")
		readEventFrame(t, conn, "chat.done")
		readEventFrame(t, conn, "chat.queue.drained")
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

// TestAbortLeavesQueueIntact verifies that aborting the active run does not
// discard the queue — remaining queued messages are preserved for later.
func TestAbortLeavesQueueIntact(t *testing.T) {
	processor := newBlockingAgent()
	server := NewServer(ServerDeps{Agent: processor})
	httpServer := httptest.NewServer(server.Handler())
	defer httpServer.Close()

	conn := dialWebsocket(t, httpServer.URL+"/ws")
	defer conn.Close()

	// Start a run.
	writeFrame(t, conn, RequestFrame{
		ID:     "a1",
		Method: "chat.send",
		Params: json.RawMessage(`{"session":"abq","message":"first"}`),
	})
	first := readResponseFrame(t, conn, "a1")
	firstRunID := extractRunID(t, first)

	// Queue a second message.
	writeFrame(t, conn, RequestFrame{
		ID:     "a2",
		Method: "chat.send",
		Params: json.RawMessage(`{"session":"abq","message":"second"}`),
	})
	_ = readResponseFrame(t, conn, "a2")
	readEventFrame(t, conn, "chat.queued")

	// Abort the active run — queue must survive.
	writeFrame(t, conn, RequestFrame{
		ID:     "a3",
		Method: "chat.abort",
		Params: json.RawMessage(`{"runId":"` + firstRunID + `"}`),
	})
	abortResp := readResponseFrame(t, conn, "a3")
	if abortResp.Response.Error != nil {
		t.Fatalf("abort failed: %v", abortResp.Response.Error)
	}
	// chat.error may arrive before or after the abort response due to goroutine
	// scheduling. read until chat.dequeued which always follows chat.error in the
	// same goroutine — this implicitly proves the active run was cancelled.
	readEventFrame(t, conn, "chat.dequeued")

	// Finish the queued run.
	processor.finish("abq", "queued done")
	done := readEventFrame(t, conn, "chat.done")
	if !strings.Contains(string(done.Event.Payload), "queued done") {
		t.Fatalf("expected queued run result, got %#v", done)
	}
	readEventFrame(t, conn, "chat.queue.drained")
}

// TestDisconnectClearsOwnedQueuedRequests verifies that when a client
// disconnects its queued (not-yet-started) requests are removed from the queue.
func TestDisconnectClearsOwnedQueuedRequests(t *testing.T) {
	processor := newBlockingAgent()
	server := NewServer(ServerDeps{Agent: processor})
	httpServer := httptest.NewServer(server.Handler())
	defer httpServer.Close()

	// Start a run on connA.
	connA := dialWebsocket(t, httpServer.URL+"/ws")
	writeFrame(t, connA, RequestFrame{
		ID:     "d1",
		Method: "chat.send",
		Params: json.RawMessage(`{"session":"disq","message":"first"}`),
	})
	_ = readResponseFrame(t, connA, "d1")

	// Queue a second message on connA — it is owned by connA.
	writeFrame(t, connA, RequestFrame{
		ID:     "d2",
		Method: "chat.send",
		Params: json.RawMessage(`{"session":"disq","message":"second"}`),
	})
	_ = readResponseFrame(t, connA, "d2")
	readEventFrame(t, connA, "chat.queued")

	// Close connA — the queued request should be dropped.
	connA.Close()

	// Allow cancellation to propagate.
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if processor.wasCancelled("disq") {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if !processor.wasCancelled("disq") {
		t.Fatal("expected active run to be cancelled on disconnect")
	}

	// Verify the queue is now empty — no second run should appear.
	server.mu.Lock()
	qLen := len(server.sessionQueue["disq"])
	server.mu.Unlock()
	if qLen != 0 {
		t.Fatalf("expected empty queue after disconnect, got %d", qLen)
	}
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
		delete(b.runs, req.SessionKey)
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

// waitForSession blocks until ProcessDirect has been called for sessionKey.
// This prevents a race where finish is called before the goroutine registers.
func (b *blockingAgent) waitForSession(sessionKey string) {
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		b.mu.Lock()
		_, ok := b.runs[sessionKey]
		b.mu.Unlock()
		if ok {
			return
		}
		time.Sleep(time.Millisecond)
	}
}

func (b *blockingAgent) finish(sessionKey, result string) {
	b.waitForSession(sessionKey)
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

func extractRunID(t *testing.T, frame *DecodedFrame) string {
	t.Helper()
	var result struct {
		RunID string `json:"runId"`
	}
	if err := json.Unmarshal(frame.Response.Result, &result); err != nil {
		t.Fatalf("extractRunID: %v", err)
	}
	if result.RunID == "" {
		t.Fatalf("extractRunID: empty runId in %s", string(frame.Response.Result))
	}
	return result.RunID
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
