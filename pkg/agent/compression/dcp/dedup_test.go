package dcp

import (
	"reflect"
	"testing"

	"github.com/Nomadcxx/smolbot/pkg/provider"
)

func TestDedup_IdenticalCalls(t *testing.T) {
	msgs := []provider.Message{
		makeUserMsg("one"),
		makeAssistantWithToolCall("", "read_file", `{"path":"a.txt"}`, "tc1"),
		makeToolResult("tc1", "read_file", "A"),
		makeAssistantWithToolCall("", "read_file", `{"path":"a.txt"}`, "tc2"),
		makeToolResult("tc2", "read_file", "A"),
		makeAssistantWithToolCall("", "read_file", `{"path":"a.txt"}`, "tc3"),
		makeToolResult("tc3", "read_file", "A"),
	}

	count := DeduplicateToolCalls(msgs, DefaultConfig())
	if count != 2 {
		t.Fatalf("DeduplicateToolCalls() = %d, want 2", count)
	}
	if got := msgs[2].StringContent(); got != DedupPlaceholder {
		t.Fatalf("first result = %q, want placeholder", got)
	}
	if got := msgs[4].StringContent(); got != DedupPlaceholder {
		t.Fatalf("second result = %q, want placeholder", got)
	}
	if got := msgs[6].StringContent(); got != "A" {
		t.Fatalf("last result = %q, want preserved", got)
	}
}

func TestDedup_DifferentArgs(t *testing.T) {
	msgs := []provider.Message{
		makeAssistantWithToolCall("", "read_file", `{"path":"a.txt"}`, "tc1"),
		makeToolResult("tc1", "read_file", "A"),
		makeAssistantWithToolCall("", "read_file", `{"path":"b.txt"}`, "tc2"),
		makeToolResult("tc2", "read_file", "B"),
	}
	if got := DeduplicateToolCalls(msgs, DefaultConfig()); got != 0 {
		t.Fatalf("DeduplicateToolCalls() = %d, want 0", got)
	}
}

func TestDedup_JSONKeyOrder(t *testing.T) {
	msgs := []provider.Message{
		makeAssistantWithToolCall("", "read_file", `{"path":"a.txt","mode":"r"}`, "tc1"),
		makeToolResult("tc1", "read_file", "A"),
		makeAssistantWithToolCall("", "read_file", `{"mode":"r","path":"a.txt"}`, "tc2"),
		makeToolResult("tc2", "read_file", "A"),
	}
	if got := DeduplicateToolCalls(msgs, DefaultConfig()); got != 1 {
		t.Fatalf("DeduplicateToolCalls() = %d, want 1", got)
	}
}

func TestDedup_ProtectedToolSkipped(t *testing.T) {
	msgs := []provider.Message{
		makeAssistantWithToolCall("", "write_file", `{"path":"a.txt"}`, "tc1"),
		makeToolResult("tc1", "write_file", "ok"),
		makeAssistantWithToolCall("", "write_file", `{"path":"a.txt"}`, "tc2"),
		makeToolResult("tc2", "write_file", "ok"),
	}
	if got := DeduplicateToolCalls(msgs, DefaultConfig()); got != 0 {
		t.Fatalf("DeduplicateToolCalls() = %d, want 0", got)
	}
}

func TestDedup_OrphanToolCall(t *testing.T) {
	msgs := []provider.Message{
		makeAssistantWithToolCall("", "read_file", `{"path":"a.txt"}`, "tc1"),
		makeAssistantWithToolCall("", "read_file", `{"path":"a.txt"}`, "tc2"),
		makeToolResult("tc2", "read_file", "A"),
	}
	if got := DeduplicateToolCalls(msgs, DefaultConfig()); got != 0 {
		t.Fatalf("DeduplicateToolCalls() = %d, want 0", got)
	}
}

func TestDedup_OriginalUnmodified(t *testing.T) {
	original := []provider.Message{
		makeAssistantWithToolCall("", "read_file", `{"path":"a.txt"}`, "tc1"),
		makeToolResult("tc1", "read_file", "A"),
		makeAssistantWithToolCall("", "read_file", `{"path":"a.txt"}`, "tc2"),
		makeToolResult("tc2", "read_file", "A"),
	}
	copyMsgs := deepCopyMessages(original)

	DeduplicateToolCalls(copyMsgs, DefaultConfig())
	if !reflect.DeepEqual(original[1], makeToolResult("tc1", "read_file", "A")) {
		t.Fatalf("original mutated: %#v", original[1])
	}
}

func BenchmarkDedup100Messages(b *testing.B) {
	msgs := make([]provider.Message, 0, 100)
	for i := 0; i < 25; i++ {
		msgs = append(msgs,
			makeUserMsg("turn"),
			makeAssistantWithToolCall("", "read_file", `{"path":"a.txt"}`, "tc-a"+string(rune('a'+(i%5)))),
			makeToolResult("tc-a"+string(rune('a'+(i%5))), "read_file", "A"),
			makeAssistantMsg("done"),
		)
	}
	cfg := DefaultConfig()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		DeduplicateToolCalls(deepCopyMessages(msgs), cfg)
	}
}
