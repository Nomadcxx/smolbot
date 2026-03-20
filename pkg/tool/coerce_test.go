package tool

import "testing"

func TestCoerceArgs(t *testing.T) {
	type args struct {
		Timeout int  `json:"timeout"`
		Force   bool `json:"force"`
	}

	got, err := CoerceArgs[args](map[string]any{
		"timeout": "60",
		"force":   "true",
	})
	if err != nil {
		t.Fatalf("CoerceArgs: %v", err)
	}
	if got.Timeout != 60 || !got.Force {
		t.Fatalf("coerced args = %+v", got)
	}
}

func TestCoerceArgsPreservesTypedMetadata(t *testing.T) {
	type args struct {
		Channel string `json:"channel"`
		ChatID  string `json:"chat_id"`
	}

	got, err := CoerceArgs[args](map[string]any{
		"channel": "gateway",
		"chat_id": "ws-client-1",
	})
	if err != nil {
		t.Fatalf("CoerceArgs: %v", err)
	}
	if got.Channel != "gateway" || got.ChatID != "ws-client-1" {
		t.Fatalf("coerced args = %+v", got)
	}
}
