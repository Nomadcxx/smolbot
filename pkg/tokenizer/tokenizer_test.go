package tokenizer

import (
	"errors"
	"testing"

	"github.com/Nomadcxx/smolbot/pkg/provider"
)

func TestEstimateTokens(t *testing.T) {
	tok := New()

	if n := tok.EstimateTokens(""); n != 0 {
		t.Errorf("EstimateTokens(empty) = %d, want 0", n)
	}

	n := tok.EstimateTokens("Hello, world!")
	if n < 2 || n > 10 {
		t.Errorf("EstimateTokens(Hello, world!) = %d, want 2..10", n)
	}

	long := "This is a somewhat longer piece of text that should tokenize to roughly the character count divided by four."
	n = tok.EstimateTokens(long)
	rough := len(long) / 4
	if n < rough/2 || n > rough*3 {
		t.Errorf("EstimateTokens(long) = %d, rough estimate %d", n, rough)
	}
}

func TestEstimateMessageTokens(t *testing.T) {
	tok := New()
	msg := provider.Message{
		Role:    "assistant",
		Content: "I'll call a tool.",
		ToolCalls: []provider.ToolCall{
			{
				ID: "call_123",
				Function: provider.FunctionCall{
					Name:      "read_file",
					Arguments: `{"path":"/tmp/test.txt"}`,
				},
			},
		},
		ReasoningContent: "Need to inspect the file before answering.",
	}

	n := tok.EstimateMessageTokens(msg)
	if n < 15 {
		t.Errorf("EstimateMessageTokens(message) = %d, want >= 15", n)
	}
}

func TestEstimatePromptTokens(t *testing.T) {
	tok := New()
	messages := []provider.Message{
		{Role: "system", Content: "You are helpful."},
		{
			Role: "user",
			Content: []provider.ContentBlock{
				{Type: "text", Text: "Describe this image."},
				{Type: "image_url", ImageURL: &provider.ImageURL{URL: "https://example.com/image.png", Detail: "auto"}},
			},
		},
	}

	total := tok.EstimatePromptTokens(messages)
	if total <= 8 {
		t.Errorf("EstimatePromptTokens(messages) = %d, want > 8", total)
	}
}

func TestEstimateTokensFallback(t *testing.T) {
	tok := &Tokenizer{err: errors.New("forced fallback")}
	tok.once.Do(func() {})

	n := tok.EstimateTokens("12345678")
	if n != 2 {
		t.Errorf("EstimateTokens fallback = %d, want 2", n)
	}
}
