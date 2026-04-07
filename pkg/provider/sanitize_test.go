package provider

import "testing"

func TestSanitizeEmptyContent(t *testing.T) {
	msgs := []Message{
		{Role: "user", Content: "Hello"},
		{Role: "tool", Content: "", ToolCallID: "tc1", Name: "exec"},
		{Role: "assistant", Content: ""},
	}

	sanitized := SanitizeMessages(msgs, "openai")

	if sanitized[1].StringContent() != " " {
		t.Fatalf("tool content = %q, want space", sanitized[1].StringContent())
	}
	if sanitized[2].StringContent() != " " {
		t.Fatalf("assistant content = %q, want space", sanitized[2].StringContent())
	}
}

func TestSanitizeToolCallIDNormalization(t *testing.T) {
	msgs := []Message{
		{
			Role: "assistant",
			ToolCalls: []ToolCall{
				{
					ID: "call_very_long_id_that_exceeds_nine_chars",
					Function: FunctionCall{
						Name:      "exec",
						Arguments: "{}",
					},
				},
			},
		},
		{Role: "tool", Content: "output", ToolCallID: "call_very_long_id_that_exceeds_nine_chars", Name: "exec"},
	}

	sanitized := SanitizeMessages(msgs, "openai")
	newID := sanitized[0].ToolCalls[0].ID
	want := "callverylongidthatexceedsninechars"
	if newID != want {
		t.Fatalf("normalized id = %q, want %q", newID, want)
	}
	if sanitized[1].ToolCallID != newID {
		t.Fatalf("tool_call_id = %q, want %q", sanitized[1].ToolCallID, newID)
	}
}

func TestSanitizeStripsThinkingForNonAnthropic(t *testing.T) {
	msgs := []Message{
		{
			Role:             "assistant",
			Content:          "response",
			ReasoningContent: "thinking",
			ThinkingBlocks:   []ThinkingBlock{{Type: "thinking", Content: "I think..."}},
		},
	}

	sanitized := SanitizeMessages(msgs, "openai")
	if len(sanitized[0].ThinkingBlocks) != 0 {
		t.Fatalf("thinking blocks should be stripped for openai-compatible providers")
	}
	if sanitized[0].ReasoningContent != "" {
		t.Fatalf("reasoning content should be stripped for openai-compatible providers")
	}

	sanitized = SanitizeMessages(msgs, "anthropic")
	if len(sanitized[0].ThinkingBlocks) != 1 {
		t.Fatalf("thinking blocks should be preserved for anthropic")
	}
	if sanitized[0].ReasoningContent != "thinking" {
		t.Fatalf("reasoning content = %q, want thinking", sanitized[0].ReasoningContent)
	}
}

func TestSanitizeNormalizesDictAndBlockContent(t *testing.T) {
	msgs := []Message{
		{
			Role: "user",
			Content: map[string]any{
				"text": "hello",
			},
		},
		{
			Role: "assistant",
			Content: []ContentBlock{
				{Type: "text", Text: ""},
				{Type: "text", Text: "kept"},
			},
		},
	}

	sanitized := SanitizeMessages(msgs, "openai")
	if sanitized[0].StringContent() == "" {
		t.Fatalf("dict content should be normalized to string content")
	}

	blocks, ok := sanitized[1].Content.([]ContentBlock)
	if !ok {
		t.Fatalf("assistant content type = %T, want []ContentBlock", sanitized[1].Content)
	}
	if len(blocks) != 1 || blocks[0].Text != "kept" {
		t.Fatalf("blocks = %#v, want single kept block", blocks)
	}
}

func TestNormalizeToolCallIDNoCollision(t *testing.T) {
	id1 := "callfunct0"
	id2 := "callfunct1"
	n1 := normalizeToolCallID(id1)
	n2 := normalizeToolCallID(id2)
	if n1 == n2 {
		t.Errorf("collision: both %q and %q normalize to %q", id1, id2, n1)
	}
}

func TestNormalizeToolCallIDPreservesLongIDs(t *testing.T) {
	id := "call_very_long_id_that_exceeds_nine_chars"
	got := normalizeToolCallID(id)
	if len(got) <= 9 {
		t.Errorf("ID was truncated: got %q (len %d)", got, len(got))
	}
}

func TestStripProviderPrefix(t *testing.T) {
	cases := []struct {
		input string
		want  string
	}{
		{"anthropic/claude-3-opus", "claude-3-opus"},
		{"openai/gpt-4o", "gpt-4o"},
		{"azure/gpt-4", "gpt-4"},
		{"minimax-portal/MiniMax-M2.7", "MiniMax-M2.7"},
		{"gpt-4o", "gpt-4o"},
		{"claude-3-opus", "claude-3-opus"},
		{"", ""},
	}
	for _, tc := range cases {
		got := StripProviderPrefix(tc.input)
		if got != tc.want {
			t.Errorf("StripProviderPrefix(%q) = %q, want %q", tc.input, got, tc.want)
		}
	}
}

func TestSanitizeRepairsToolArguments(t *testing.T) {
	msgs := []Message{
		{
			Role: "assistant",
			ToolCalls: []ToolCall{
				{
					ID: "toolcall_1",
					Function: FunctionCall{
						Name:      "write_file",
						Arguments: `{path:"/tmp/x", trailing:true,}`,
					},
				},
			},
		},
	}

	sanitized := SanitizeMessages(msgs, "openai")
	got := sanitized[0].ToolCalls[0].Function.Arguments
	want := `{"path":"/tmp/x","trailing":true}`
	if got != want {
		t.Fatalf("arguments = %q, want %q", got, want)
	}
}

func TestRepairJSONClosesUnclosedBrace(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{`{"key": "val"`, `{"key":"val"}`},
		{`{"a":1,"b":[1,2,3`, `{"a":1,"b":[1,2,3]}`},
		{`[{"x":1}`, `[{"x":1}]`},
	}
	for _, tt := range tests {
		got := repairJSON(tt.input)
		if got != tt.want {
			t.Errorf("repairJSON(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestSanitizeGeneratesFallbackIDForEmptyToolCalls(t *testing.T) {
msgs := []Message{
{
Role: "assistant",
ToolCalls: []ToolCall{
{ID: "", Function: FunctionCall{Name: "exec", Arguments: `{"cmd":"ls"}`}},
{ID: "call_abc", Function: FunctionCall{Name: "read", Arguments: `{"path":"."}`}},
},
},
{
Role:       "tool",
ToolCallID: "",
Content:    "file1.txt",
},
{
Role:       "tool",
ToolCallID: "call_abc",
Content:    "contents",
},
}

sanitized := SanitizeMessages(msgs, "openai")

// The empty-ID tool call should get a generated fallback.
if len(sanitized[0].ToolCalls) < 1 {
t.Fatal("expected at least 1 tool call after sanitize")
}
for i, tc := range sanitized[0].ToolCalls {
if tc.ID == "" {
t.Errorf("tool call %d still has empty ID after sanitize", i)
}
}
}

func TestSanitizeDropsCorruptToolCallsWithNoName(t *testing.T) {
msgs := []Message{
{
Role: "assistant",
ToolCalls: []ToolCall{
{ID: "call_1", Function: FunctionCall{Name: "", Arguments: ""}},
{ID: "call_2", Function: FunctionCall{Name: "exec", Arguments: `{}`}},
},
},
}

sanitized := SanitizeMessages(msgs, "openai")

if len(sanitized[0].ToolCalls) != 1 {
t.Fatalf("expected 1 tool call (corrupt one dropped), got %d", len(sanitized[0].ToolCalls))
}
if sanitized[0].ToolCalls[0].Function.Name != "exec" {
t.Errorf("expected exec tool call, got %q", sanitized[0].ToolCalls[0].Function.Name)
}
}
