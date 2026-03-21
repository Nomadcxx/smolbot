package client

import (
	"encoding/json"
	"testing"
)

func TestRequestJSONRoundTrip(t *testing.T) {
	req := Request{
		Type:   FrameReq,
		ID:     "1",
		Method: "hello",
		Params: HelloParams{
			Client:   "smolbot-tui",
			Version:  "0.1.0",
			Protocol: ProtocolVersion,
			Platform: "linux",
		},
	}

	raw, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}

	var got Request
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("unmarshal request: %v", err)
	}

	if got.Type != FrameReq || got.Method != "hello" || got.ID != "1" {
		t.Fatalf("unexpected request round trip: %#v", got)
	}
}

func TestResponseJSONRoundTrip(t *testing.T) {
	raw := []byte(`{"type":"res","id":"1","ok":true,"payload":{"runId":"abc"}}`)

	var res Response
	if err := json.Unmarshal(raw, &res); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if !res.OK || res.ID != "1" || res.Type != FrameRes {
		t.Fatalf("unexpected response frame: %#v", res)
	}

	var payload ChatSendPayload
	if err := json.Unmarshal(res.Payload, &payload); err != nil {
		t.Fatalf("unmarshal payload: %v", err)
	}
	if payload.RunID != "abc" {
		t.Fatalf("unexpected run id: %q", payload.RunID)
	}
}
