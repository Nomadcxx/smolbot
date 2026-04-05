package dcp

import (
	"testing"

	"github.com/Nomadcxx/smolbot/pkg/provider"
)

func TestPurge_StaleError(t *testing.T) {
	cfg := DefaultConfig()
	msgs := []provider.Message{
		makeUserMsg("turn1"),
		makeAssistantWithToolCall("", "exec", `{"cmd":"bad"}`, "tc1"),
		makeToolError("tc1", "exec", "Error: boom"),
		makeUserMsg("turn2"),
		makeAssistantMsg("a"),
		makeUserMsg("turn3"),
		makeAssistantMsg("b"),
		makeUserMsg("turn4"),
		makeAssistantMsg("c"),
		makeUserMsg("turn5"),
		makeAssistantMsg("d"),
	}

	if got := PurgeErroredInputs(msgs, 5, cfg); got != 1 {
		t.Fatalf("PurgeErroredInputs() = %d, want 1", got)
	}
	if got := msgs[1].ToolCalls[0].Function.Arguments; got != ErrorInputPlaceholder {
		t.Fatalf("arguments = %q, want placeholder", got)
	}
}

func TestPurge_RecentError(t *testing.T) {
	cfg := DefaultConfig()
	msgs := []provider.Message{
		makeUserMsg("turn1"),
		makeAssistantWithToolCall("", "exec", `{"cmd":"bad"}`, "tc1"),
		makeToolError("tc1", "exec", "Error: boom"),
		makeUserMsg("turn2"),
		makeAssistantMsg("a"),
	}
	if got := PurgeErroredInputs(msgs, 2, cfg); got != 0 {
		t.Fatalf("PurgeErroredInputs() = %d, want 0", got)
	}
}

func TestPurge_ErrorMessagePreserved(t *testing.T) {
	cfg := DefaultConfig()
	msgs := []provider.Message{
		makeUserMsg("turn1"),
		makeAssistantWithToolCall("", "exec", `{"cmd":"bad"}`, "tc1"),
		makeToolError("tc1", "exec", "panic: boom"),
		makeUserMsg("turn2"),
		makeAssistantMsg("a"),
		makeUserMsg("turn3"),
		makeAssistantMsg("b"),
		makeUserMsg("turn4"),
		makeAssistantMsg("c"),
		makeUserMsg("turn5"),
		makeAssistantMsg("d"),
	}
	_ = PurgeErroredInputs(msgs, 5, cfg)
	if got := msgs[2].StringContent(); got != "panic: boom" {
		t.Fatalf("tool result = %q, want preserved", got)
	}
}

func TestPurge_ProtectedToolSkipped(t *testing.T) {
	cfg := DefaultConfig()
	msgs := []provider.Message{
		makeUserMsg("turn1"),
		makeAssistantWithToolCall("", "write_file", `{"path":"bad"}`, "tc1"),
		makeToolError("tc1", "write_file", "Error: boom"),
		makeUserMsg("turn2"),
		makeAssistantMsg("a"),
		makeUserMsg("turn3"),
		makeAssistantMsg("b"),
		makeUserMsg("turn4"),
		makeAssistantMsg("c"),
		makeUserMsg("turn5"),
		makeAssistantMsg("d"),
	}
	if got := PurgeErroredInputs(msgs, 5, cfg); got != 0 {
		t.Fatalf("PurgeErroredInputs() = %d, want 0", got)
	}
}

func TestPurge_ErrorDetection(t *testing.T) {
	cases := []string{
		"Error: boom",
		"ERROR: boom",
		"error: boom",
		"FAIL something",
		"command failed with exit code 1",
		"panic: boom",
		"fatal: boom",
	}
	for _, tc := range cases {
		if !isErrorResult(tc) {
			t.Fatalf("isErrorResult(%q) = false, want true", tc)
		}
	}
}

func TestPurge_SuccessNotDetected(t *testing.T) {
	cases := []string{"success", "ok", "done", "exit code 0"}
	for _, tc := range cases {
		if isErrorResult(tc) {
			t.Fatalf("isErrorResult(%q) = true, want false", tc)
		}
	}
}
