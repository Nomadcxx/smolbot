package dcp

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"

	"github.com/Nomadcxx/smolbot/pkg/provider"
	"github.com/Nomadcxx/smolbot/pkg/tokenizer"
	"github.com/Nomadcxx/smolbot/pkg/tool"
)

type CompressTool struct {
	stateManager *StateManager
	config       Config
	tokenizer    *tokenizer.Tokenizer
	messages     []provider.Message
	messageFn    func() []provider.Message
}

type compressToolArgs struct {
	Topic  string               `json:"topic"`
	Ranges []compressToolRange  `json:"ranges"`
}

type compressToolRange struct {
	StartID string `json:"start_id"`
	EndID   string `json:"end_id"`
	Summary string `json:"summary"`
}

func NewCompressTool(sm *StateManager, cfg Config, tok *tokenizer.Tokenizer) *CompressTool {
	return &CompressTool{stateManager: sm, config: cfg, tokenizer: tok}
}

func (t *CompressTool) WithMessageProvider(fn func() []provider.Message) *CompressTool {
	t.messageFn = fn
	return t
}

func (t *CompressTool) Name() string { return "compress" }

func (t *CompressTool) Description() string {
	return "Summarize old conversation ranges into reusable compression blocks."
}

func (t *CompressTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"topic": map[string]any{"type": "string"},
			"ranges": map[string]any{
				"type": "array",
				"items": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"start_id": map[string]any{"type": "string"},
						"end_id":   map[string]any{"type": "string"},
						"summary":  map[string]any{"type": "string"},
					},
					"required": []string{"start_id", "end_id", "summary"},
				},
			},
		},
		"required": []string{"topic", "ranges"},
	}
}

func (t *CompressTool) Execute(ctx context.Context, args json.RawMessage, tctx tool.ToolContext) (*tool.Result, error) {
	_ = ctx
	var payload compressToolArgs
	if err := json.Unmarshal(args, &payload); err != nil {
		return &tool.Result{Error: fmt.Sprintf("invalid arguments: %v", err)}, nil
	}
	if t.stateManager == nil {
		return &tool.Result{Error: "compress tool is unavailable"}, nil
	}
	state, err := t.stateManager.Load(tctx.SessionKey)
	if err != nil {
		return nil, err
	}
	visibleMessages := t.visibleMessages()
	if len(state.ToolPairs) == 0 && len(visibleMessages) > 0 {
		state.ToolPairs = buildToolPairs(visibleMessages)
	}

	type plannedRange struct {
		start   int
		end     int
		payload compressToolRange
	}
	planned := make([]plannedRange, 0, len(payload.Ranges))
	for _, r := range payload.Ranges {
		start, ok := resolveBoundaryIndex(r.StartID, state, true)
		if !ok {
			return &tool.Result{Error: "invalid message id: " + r.StartID}, nil
		}
		end, ok := resolveBoundaryIndex(r.EndID, state, false)
		if !ok {
			return &tool.Result{Error: "invalid message id: " + r.EndID}, nil
		}
		if start > end {
			return &tool.Result{Error: "range start must be before end"}, nil
		}
		for idx := start; idx <= end; idx++ {
			if state.ProtectedIndexes[idx] {
				return &tool.Result{Error: fmt.Sprintf("range includes protected message at index %d", idx)}, nil
			}
		}
		planned = append(planned, plannedRange{start: start, end: end, payload: r})
	}
	sort.Slice(planned, func(i, j int) bool { return planned[i].start < planned[j].start })
	for i := 1; i < len(planned); i++ {
		if planned[i].start <= planned[i-1].end {
			return &tool.Result{Error: "ranges overlap"}, nil
		}
	}

	totalSaved := 0
	for _, r := range planned {
		id := state.AllocateBlockID()
		block := CompressionBlock{
			ID:             id,
			Topic:          payload.Topic,
			Summary:        r.payload.Summary,
			StartRef:       r.payload.StartID,
			EndRef:         r.payload.EndID,
			AnchorMsgIndex: r.start,
			SummaryTokens:  t.tokenizer.EstimateTokens(r.payload.Summary),
		}
		for _, active := range state.ActiveBlocks() {
			if active.AnchorMsgIndex >= r.start && active.AnchorMsgIndex <= r.end {
				block.ConsumedBlocks = append(block.ConsumedBlocks, active.ID)
			}
		}
		if len(visibleMessages) > r.end && t.tokenizer != nil {
			block.TokensSaved = t.tokenizer.EstimatePromptTokens(visibleMessages[r.start:r.end+1]) - block.SummaryTokens
		}
		totalSaved += block.TokensSaved
		if err := state.CreateBlock(block); err != nil {
			return &tool.Result{Error: err.Error()}, nil
		}
		for _, consumedID := range block.ConsumedBlocks {
			_ = state.DeactivateBlock(consumedID, block.ID)
		}
	}
	state.Stats.TotalCompressions += len(planned)
	state.Stats.TotalPrunedTokens += totalSaved
	if err := t.stateManager.Save(state); err != nil {
		return nil, err
	}
	msg := fmt.Sprintf("Compressed %d ranges into %d blocks. Tokens saved: %d", len(planned), len(planned), totalSaved)
	return &tool.Result{Output: msg, Content: msg}, nil
}

func (t *CompressTool) IsDeferred() bool           { return true }
func (t *CompressTool) IsAlwaysLoad() bool         { return true }
func (t *CompressTool) DeferredKeywords() []string { return []string{"compress", "context", "summarize", "prune"} }
func (t *CompressTool) IsConcurrencySafe() bool    { return false }

func (t *CompressTool) visibleMessages() []provider.Message {
	if t.messageFn != nil {
		return t.messageFn()
	}
	return t.messages
}
