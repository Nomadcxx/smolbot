package tokenizer

import (
	"encoding/json"
	"fmt"
	"sync"

	"github.com/Nomadcxx/nanobot-go/pkg/provider"
	tiktoken "github.com/pkoukk/tiktoken-go"
)

const (
	messageOverhead = 4
	promptOverhead  = 3
	imageOverhead   = 256
	toolOverhead    = 8
)

type Tokenizer struct {
	enc  *tiktoken.Tiktoken
	once sync.Once
	err  error
}

func New() *Tokenizer {
	return &Tokenizer{}
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
		t.enc, t.err = tiktoken.GetEncoding("cl100k_base")
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
