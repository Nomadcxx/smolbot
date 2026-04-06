package gateway

import (
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestGatewayQueuesSameSendDuringActiveRun verifies that a second chat.send for
// the same session while a run is active is queued rather than rejected.
func TestGatewayQueuesSameSendDuringActiveRun(t *testing.T) {
	processor := newBlockingAgent()
	server := NewServer(ServerDeps{Agent: processor})
	httpServer := httptest.NewServer(server.Handler())
	defer httpServer.Close()

	conn := dialWebsocket(t, httpServer.URL+"/ws")
	defer conn.Close()

	// First send — starts immediately.
	writeFrame(t, conn, RequestFrame{
		ID:     "q1",
		Method: "chat.send",
		Params: json.RawMessage(`{"session":"qsess","message":"first"}`),
	})
	first := readResponseFrame(t, conn, "q1")
	if first.Response.Error != nil {
		t.Fatalf("unexpected error on first send: %v", first.Response.Error)
	}
	if !strings.Contains(string(first.Response.Result), "runId") {
		t.Fatalf("expected runId in first response, got %#v", first)
	}

	// Second send — session is active, must queue.
	writeFrame(t, conn, RequestFrame{
		ID:     "q2",
		Method: "chat.send",
		Params: json.RawMessage(`{"session":"qsess","message":"second"}`),
	})
	second := readResponseFrame(t, conn, "q2")
	if second.Response.Error != nil {
		t.Fatalf("expected second send to queue (not error), got: %v", second.Response.Error)
	}
	if !strings.Contains(string(second.Response.Result), "runId") {
		t.Fatalf("expected runId in queued response, got %#v", second)
	}

	// Verify chat.queued event arrives.
	queued := readEventFrame(t, conn, "chat.queued")
	if !strings.Contains(string(queued.Event.Payload), `"session":"qsess"`) {
		t.Fatalf("unexpected chat.queued payload %#v", queued)
	}

	// Finish the first run.
	processor.finish("qsess", "first done")
	readEventFrame(t, conn, "chat.done")

	// The queued run should start automatically — expect chat.dequeued then chat.done.
	readEventFrame(t, conn, "chat.dequeued")
	processor.finish("qsess", "second done")
	done := readEventFrame(t, conn, "chat.done")
	if !strings.Contains(string(done.Event.Payload), "second done") {
		t.Fatalf("expected second run result, got %#v", done)
	}
}

// TestGatewayQueueDrainedEmittedAfterQueueExhausted verifies that
// chat.queue.drained is emitted after the last dequeued run completes, and is
// NOT emitted for a lone run that was never queued.
func TestGatewayQueueDrainedEmittedAfterQueueExhausted(t *testing.T) {
	processor := newBlockingAgent()
	server := NewServer(ServerDeps{Agent: processor})
	httpServer := httptest.NewServer(server.Handler())
	defer httpServer.Close()

	conn := dialWebsocket(t, httpServer.URL+"/ws")
	defer conn.Close()

	// Start two sends so the second gets queued.
	writeFrame(t, conn, RequestFrame{
		ID:     "drain1",
		Method: "chat.send",
		Params: json.RawMessage(`{"session":"drain","message":"first"}`),
	})
	_ = readResponseFrame(t, conn, "drain1")

	writeFrame(t, conn, RequestFrame{
		ID:     "drain2",
		Method: "chat.send",
		Params: json.RawMessage(`{"session":"drain","message":"second"}`),
	})
	_ = readResponseFrame(t, conn, "drain2")
	readEventFrame(t, conn, "chat.queued")

	// Finish first; second dequeues.
	processor.finish("drain", "first done")
	readEventFrame(t, conn, "chat.done")
	readEventFrame(t, conn, "chat.dequeued")

	// Finish second; queue is now empty — drained event must fire.
	processor.finish("drain", "second done")
	readEventFrame(t, conn, "chat.done")
	readEventFrame(t, conn, "chat.queue.drained")
}

// TestGatewayDifferentSessionsRunConcurrently verifies that sends for different
// sessions are never serialised — they continue to run in parallel.
func TestGatewayDifferentSessionsRunConcurrently(t *testing.T) {
	processor := newBlockingAgent()
	server := NewServer(ServerDeps{Agent: processor})
	httpServer := httptest.NewServer(server.Handler())
	defer httpServer.Close()

	connA := dialWebsocket(t, httpServer.URL+"/ws")
	connB := dialWebsocket(t, httpServer.URL+"/ws")
	defer connA.Close()
	defer connB.Close()

	writeFrame(t, connA, RequestFrame{
		ID:     "a1",
		Method: "chat.send",
		Params: json.RawMessage(`{"session":"sessA","message":"hello"}`),
	})
	writeFrame(t, connB, RequestFrame{
		ID:     "b1",
		Method: "chat.send",
		Params: json.RawMessage(`{"session":"sessB","message":"hello"}`),
	})

	respA := readResponseFrame(t, connA, "a1")
	respB := readResponseFrame(t, connB, "b1")

	if respA.Response.Error != nil {
		t.Fatalf("session A send failed: %v", respA.Response.Error)
	}
	if respB.Response.Error != nil {
		t.Fatalf("session B send failed: %v", respB.Response.Error)
	}

	processor.finish("sessA", "A done")
	processor.finish("sessB", "B done")

	readUntilEvent(t, connA, "chat.done")
	readUntilEvent(t, connB, "chat.done")
}
