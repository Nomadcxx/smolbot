package dcp

import (
	"strings"
	"testing"

	"github.com/Nomadcxx/smolbot/pkg/provider"
)

func TestAssignMessageIDs_BasicSequence(t *testing.T) {
	msgs := []provider.Message{
		{Role: "system", Content: "sys"},
		makeUserMsg("u1"),
		makeAssistantMsg("a1"),
		makeToolResult("tc1", "exec", "out"),
		makeUserMsg("u2"),
		makeAssistantMsg("a2"),
	}
	state := NewState("s1")

	AssignMessageIDs(msgs, state, DefaultConfig())

	if strings.Contains(msgs[0].StringContent(), "<dcp-id>") {
		t.Fatal("system message should not be tagged")
	}
	for i, want := range []string{"m0001", "m0002", "m0003", "m0004", "m0005"} {
		got := extractDCPID(msgs[i+1].StringContent())
		if got != want {
			t.Fatalf("message %d id = %q, want %q", i+1, got, want)
		}
	}
}

func TestAssignMessageIDs_Idempotent(t *testing.T) {
	msgs := []provider.Message{makeUserMsg("u1"), makeAssistantMsg("a1")}
	state := NewState("s1")

	AssignMessageIDs(msgs, state, DefaultConfig())
	first := msgs[0].StringContent()
	AssignMessageIDs(msgs, state, DefaultConfig())
	if msgs[0].StringContent() != first {
		t.Fatalf("message retagged: %q != %q", msgs[0].StringContent(), first)
	}
}

func TestAssignMessageIDs_ContentBlocks(t *testing.T) {
	msgs := []provider.Message{{
		Role: "assistant",
		Content: []provider.ContentBlock{
			{Type: "text", Text: "hello"},
		},
	}}
	state := NewState("s1")

	AssignMessageIDs(msgs, state, DefaultConfig())
	blocks, ok := msgs[0].Content.([]provider.ContentBlock)
	if !ok || !strings.Contains(blocks[0].Text, "<dcp-id>m0001</dcp-id>") {
		t.Fatalf("content blocks not tagged: %#v", msgs[0].Content)
	}
}

func TestAssignMessageIDs_ProtectedMsgs(t *testing.T) {
	msgs := []provider.Message{
		makeAssistantWithToolCall("", "write_file", `{"path":"a.txt"}`, "tc1"),
	}
	state := NewState("s1")

	AssignMessageIDs(msgs, state, DefaultConfig())
	if !strings.Contains(msgs[0].StringContent(), "PROTECTED") {
		t.Fatalf("protected message missing PROTECTED tag: %q", msgs[0].StringContent())
	}
}

func TestStripDCPTags(t *testing.T) {
	content := "hello <dcp-id>m0001</dcp-id> world <dcp-reminder>warn</dcp-reminder>"
	if got := StripDCPTags(content); got != "hello  world " {
		t.Fatalf("StripDCPTags() = %q", got)
	}
}
