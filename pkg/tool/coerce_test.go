package tool

import "testing"

func TestCoerceArgsBoolFromNumericString(t *testing.T) {
	type args struct {
		Recursive bool `json:"recursive"`
	}

	tests := []struct {
		name  string
		input string
		want  bool
	}{
		{"string_true", "true", true},
		{"string_false", "false", false},
		{"string_True", "True", true},
		{"string_1", "1", true},
		{"string_0", "0", false},
		{"string_yes", "yes", true},
		{"string_no", "no", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := CoerceArgs[args](map[string]any{"recursive": tt.input})
			if err != nil {
				t.Fatalf("CoerceArgs(%q): %v", tt.input, err)
			}
			if got.Recursive != tt.want {
				t.Fatalf("CoerceArgs(%q).Recursive = %v, want %v", tt.input, got.Recursive, tt.want)
			}
		})
	}
}

func TestCoerceArgsMixedTypes(t *testing.T) {
	type args struct {
		Path      string `json:"path"`
		Recursive bool   `json:"recursive"`
		MaxDepth  int    `json:"max_depth"`
	}

	got, err := CoerceArgs[args](map[string]any{
		"path":      "/tmp",
		"recursive": "1",
		"max_depth": "3",
	})
	if err != nil {
		t.Fatalf("CoerceArgs: %v", err)
	}
	if got.Path != "/tmp" || !got.Recursive || got.MaxDepth != 3 {
		t.Fatalf("got %+v", got)
	}
}

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
