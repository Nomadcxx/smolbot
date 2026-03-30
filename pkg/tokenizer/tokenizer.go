package tokenizer

import (
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	"github.com/Nomadcxx/smolbot/pkg/provider"
	tiktoken "github.com/pkoukk/tiktoken-go"
)

const (
	messageOverhead = 4
	promptOverhead  = 3
	imageOverhead   = 256
	toolOverhead    = 8
)

type Tokenizer struct {
	model string
	enc   *tiktoken.Tiktoken
	once  sync.Once
	err   error
}

func New() *Tokenizer {
	return &Tokenizer{}
}

func NewForModel(model string) *Tokenizer {
	return &Tokenizer{model: model}
}

func encodingForModel(model string) string {
	lower := strings.ToLower(model)
	if idx := strings.LastIndex(lower, "/"); idx >= 0 {
		lower = lower[idx+1:]
	}
	cl100kFamilies := []string{"gpt-", "claude", "o1", "o3", "o4", "chatgpt", "text-embedding"}
	for _, prefix := range cl100kFamilies {
		if strings.HasPrefix(lower, prefix) {
			return "cl100k_base"
		}
	}
	return ""
}

func (t *Tokenizer) EstimateTokens(text string) int {
	if text == "" {
		return 0
	}

	t.init()
	if t.err != nil || t.enc == nil {
		return fallbackEstimate(text)
	}

	return len(t.enc.Encode(text, nil, nil))
}

func (t *Tokenizer) EstimateMessageTokens(msg provider.Message) int {
	total := messageOverhead + t.EstimateTokens(msg.Role)
	total += t.estimateContent(msg.Content)
	total += t.EstimateTokens(msg.ToolCallID)
	total += t.EstimateTokens(msg.Name)
	total += t.EstimateTokens(msg.ReasoningContent)

	for _, toolCall := range msg.ToolCalls {
		total += toolOverhead
		total += t.EstimateTokens(toolCall.ID)
		total += t.EstimateTokens(toolCall.Function.Name)
		total += t.EstimateTokens(toolCall.Function.Arguments)
	}

	for _, block := range msg.ThinkingBlocks {
		total += t.EstimateTokens(block.Type)
		total += t.EstimateTokens(block.Content)
	}

	return total
}

func (t *Tokenizer) EstimatePromptTokens(messages []provider.Message) int {
	total := promptOverhead
	for _, msg := range messages {
		total += t.EstimateMessageTokens(msg)
	}
	return total
}

func (t *Tokenizer) init() {
	t.once.Do(func() {
		enc := encodingForModel(t.model)
		if enc == "" {
			if t.model == "" {
				enc = "cl100k_base"
			} else {
				t.err = fmt.Errorf("no cl100k encoding for model %q", t.model)
				return
			}
		}
		t.enc, t.err = tiktoken.GetEncoding(enc)
	})
}

func (t *Tokenizer) estimateContent(content any) int {
	switch value := content.(type) {
	case nil:
		return 0
	case string:
		return t.EstimateTokens(value)
	case []provider.ContentBlock:
		total := 0
		for _, block := range value {
			total += t.EstimateTokens(block.Type)
			total += t.EstimateTokens(block.Text)
			if block.ImageURL != nil {
				total += imageOverhead
				total += t.EstimateTokens(block.ImageURL.URL)
				total += t.EstimateTokens(block.ImageURL.Detail)
			}
		}
		return total
	case []any:
		total := 0
		for _, item := range value {
			total += t.estimateContent(item)
		}
		return total
	default:
		data, err := json.Marshal(value)
		if err != nil {
			return fallbackEstimate(fmt.Sprint(value))
		}
		return t.EstimateTokens(string(data))
	}
}

func fallbackEstimate(text string) int {
	if text == "" {
		return 0
	}
	return max(1, (len(text)+3)/4)
}
