package gateway

import (
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestQueueFullPath is an end-to-end test covering the complete lifecycle of
// two same-session messages when the second arrives while the first is running:
//
//  1. First send  → starts immediately, returns runId
//  2. Second send → queued (returns runId + chat.queued event)
//  3. First run   → finishes → chat.done
//  4. Second run  → starts automatically → chat.dequeued
//  5. Second run  → finishes → chat.done → chat.queue.drained
func TestQueueFullPath(t *testing.T) {
	processor := newBlockingAgent()
	server := NewServer(ServerDeps{Agent: processor})
	httpServer := httptest.NewServer(server.Handler())
	defer httpServer.Close()

	conn := dialWebsocket(t, httpServer.URL+"/ws")
	defer conn.Close()

	// 1. First send starts immediately.
	writeFrame(t, conn, RequestFrame{
		ID:     "fp1",
		Method: "chat.send",
		Params: json.RawMessage(`{"session":"fp","message":"first"}`),
	})
	resp1 := readResponseFrame(t, conn, "fp1")
	if resp1.Response.Error != nil {
		t.Fatalf("first send failed: %v", resp1.Response.Error)
	}
	firstRunID := extractRunID(t, resp1)
	if firstRunID == "" {
		t.Fatal("expected non-empty runId for first send")
	}

	// 2. Second send while first is active — must queue.
	writeFrame(t, conn, RequestFrame{
		ID:     "fp2",
		Method: "chat.send",
		Params: json.RawMessage(`{"session":"fp","message":"second"}`),
	})
	resp2 := readResponseFrame(t, conn, "fp2")
	if resp2.Response.Error != nil {
		t.Fatalf("second send must queue, not error: %v", resp2.Response.Error)
	}
	secondRunID := extractRunID(t, resp2)
	if secondRunID == "" {
		t.Fatal("expected non-empty runId for queued send")
	}
	if secondRunID == firstRunID {
		t.Fatalf("queued run must have distinct runId, both are %q", firstRunID)
	}

	// chat.queued event must arrive with the session and position.
	queued := readEventFrame(t, conn, "chat.queued")
	if !strings.Contains(string(queued.Event.Payload), `"session":"fp"`) {
		t.Fatalf("chat.queued missing session, payload: %s", queued.Event.Payload)
	}
	if !strings.Contains(string(queued.Event.Payload), `"position":1`) {
		t.Fatalf("chat.queued missing position:1, payload: %s", queued.Event.Payload)
	}

	// 3. Finish the first run.
	processor.finish("fp", "first result")
	firstDone := readEventFrame(t, conn, "chat.done")
	if !strings.Contains(string(firstDone.Event.Payload), "first result") {
		t.Fatalf("expected first result in chat.done, got %s", firstDone.Event.Payload)
	}

	// 4. Second run starts automatically.
	dequeued := readEventFrame(t, conn, "chat.dequeued")
	if !strings.Contains(string(dequeued.Event.Payload), secondRunID) {
		t.Fatalf("chat.dequeued must reference second runId %q, got %s", secondRunID, dequeued.Event.Payload)
	}

	// 5. Finish the second run.
	processor.finish("fp", "second result")
	secondDone := readEventFrame(t, conn, "chat.done")
	if !strings.Contains(string(secondDone.Event.Payload), "second result") {
		t.Fatalf("expected second result in chat.done, got %s", secondDone.Event.Payload)
	}

	// Queue is now exhausted.
	readEventFrame(t, conn, "chat.queue.drained")
}

// TestQueueOldAlreadyActiveBehaviorIsGone verifies that the original
// "already active" error no longer occurs for same-session sends.
func TestQueueOldAlreadyActiveBehaviorIsGone(t *testing.T) {
	processor := newBlockingAgent()
	server := NewServer(ServerDeps{Agent: processor})
	httpServer := httptest.NewServer(server.Handler())
	defer httpServer.Close()

	conn := dialWebsocket(t, httpServer.URL+"/ws")
	defer conn.Close()

	writeFrame(t, conn, RequestFrame{
		ID:     "old1",
		Method: "chat.send",
		Params: json.RawMessage(`{"session":"olds","message":"first"}`),
	})
	_ = readResponseFrame(t, conn, "old1")

	writeFrame(t, conn, RequestFrame{
		ID:     "old2",
		Method: "chat.send",
		Params: json.RawMessage(`{"session":"olds","message":"second"}`),
	})
	resp := readResponseFrame(t, conn, "old2")

	if resp.Response.Error != nil {
		t.Fatalf("REGRESSION: got 'already active' error — queueing is not working: %v", resp.Response.Error)
	}

	processor.finish("olds", "first done")
	readEventFrame(t, conn, "chat.done")
	readEventFrame(t, conn, "chat.dequeued")
	processor.finish("olds", "second done")
	readEventFrame(t, conn, "chat.done")
	readEventFrame(t, conn, "chat.queue.drained")
}
