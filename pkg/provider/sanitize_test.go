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
