package client

import (
	"encoding/json"
	"strings"
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

func TestToolDonePayloadDecodesDeliveredField(t *testing.T) {
	raw := []byte(`{"name":"web_search","output":"results","id":"call-1","deliveredToRequestTarget":true}`)
	var p ToolDonePayload
	if err := json.Unmarshal(raw, &p); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if !p.DeliveredToRequestTarget {
		t.Fatal("DeliveredToRequestTarget was not decoded: field missing from struct")
	}
	if p.Name != "web_search" || p.ID != "call-1" {
		t.Fatalf("unexpected payload %#v", p)
	}
}

func TestToolDonePayloadOmitsDeliveredWhenFalse(t *testing.T) {
	p := ToolDonePayload{Name: "tool", Output: "out", ID: "id-1"}
	raw, err := json.Marshal(p)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	if strings.Contains(string(raw), "deliveredToRequestTarget") {
		t.Fatalf("deliveredToRequestTarget should be omitted when false, got: %s", raw)
	}
}
