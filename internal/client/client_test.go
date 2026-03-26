package client

import (
	"encoding/json"
	"testing"
)

func TestCloseResetsLastSequence(t *testing.T) {
	c := New("ws://127.0.0.1/ws")
	c.lastSeq = 9

	c.Close()

	if c.lastSeq != 0 {
		t.Fatalf("expected close to reset sequence tracking, got %d", c.lastSeq)
	}
}

func TestCronJobsResponseDecoding(t *testing.T) {
	raw := []byte(`{"type":"res","id":"1","ok":true,"payload":{"jobs":[{"id":"job-1","name":"Daily cleanup","schedule":"every 5m","status":"active","nextRun":"2026-03-27T10:00:00Z"},{"id":"job-2","name":"Paused sync","schedule":"daily 02:00","status":"paused","nextRun":""}]}}`)

	var res Response
	if err := json.Unmarshal(raw, &res); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if !res.OK || res.Type != FrameRes {
		t.Fatalf("unexpected response frame: %#v", res)
	}

	var payload struct {
		Jobs []CronJob `json:"jobs"`
	}
	if err := json.Unmarshal(res.Payload, &payload); err != nil {
		t.Fatalf("unmarshal cron jobs payload: %v", err)
	}
	if len(payload.Jobs) != 2 {
		t.Fatalf("expected 2 jobs, got %#v", payload.Jobs)
	}
	if payload.Jobs[0].ID != "job-1" || payload.Jobs[0].Status != "active" || payload.Jobs[1].Status != "paused" {
		t.Fatalf("unexpected decoded cron jobs: %#v", payload.Jobs)
	}
}
