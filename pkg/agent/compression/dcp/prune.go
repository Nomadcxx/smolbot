package dcp

import (
	"fmt"
	"regexp"
	"strconv"

	"github.com/Nomadcxx/smolbot/pkg/provider"
)

var blockPlaceholderPattern = regexp.MustCompile(`\(b(\d+)\)`)

func ApplyCompressionBlocks(messages []provider.Message, state *State) []provider.Message {
	active := state.ActiveBlocks()
	if len(active) == 0 {
		return messages
	}

	blockByStart := make(map[int]*CompressionBlock)
	blockByEnd := make(map[int]int)
	for _, block := range active {
		start, okStart := resolveBoundaryIndex(block.StartRef, state, true)
		end, okEnd := resolveBoundaryIndex(block.EndRef, state, false)
		if !okStart || !okEnd || start > end || start < 0 || end >= len(messages) {
			continue
		}
		blockByStart[start] = block
		blockByEnd[start] = end
	}

	out := make([]provider.Message, 0, len(messages))
	for i := 0; i < len(messages); {
		block, ok := blockByStart[i]
		if !ok {
			out = append(out, messages[i])
			i++
			continue
		}
		summary := ExpandBlockPlaceholders(block.Summary, state.Blocks)
		out = append(out, provider.Message{
			Role:    "assistant",
			Content: WrapSummary(block.Topic, summary, block.ID),
		})
		i = blockByEnd[i] + 1
	}
	return out
}

func WrapSummary(topic string, summary string, blockID int) string {
	return fmt.Sprintf("[Compressed: %s]\n%s\n<dcp-id>b%d</dcp-id>", topic, summary, blockID)
}

func ExpandBlockPlaceholders(summary string, blocks map[int]*CompressionBlock) string {
	return expandBlockPlaceholders(summary, blocks, 0)
}

func expandBlockPlaceholders(summary string, blocks map[int]*CompressionBlock, depth int) string {
	if depth >= 10 {
		return summary
	}
	return blockPlaceholderPattern.ReplaceAllStringFunc(summary, func(match string) string {
		sub := blockPlaceholderPattern.FindStringSubmatch(match)
		if len(sub) != 2 {
			return match
		}
		id, _ := strconv.Atoi(sub[1])
		block, ok := blocks[id]
		if !ok || block == nil {
			return match
		}
		return expandBlockPlaceholders(block.Summary, blocks, depth+1)
	})
}
