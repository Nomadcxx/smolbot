package dcp

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"
)

func (s *State) AllocateBlockID() int {
	if s.NextBlockID <= 0 {
		s.NextBlockID = 1
	}
	id := s.NextBlockID
	s.NextBlockID++
	return id
}

func (s *State) CreateBlock(b CompressionBlock) error {
	if b.ID == 0 {
		b.ID = s.AllocateBlockID()
	}
	startIndex, ok := resolveBoundaryIndex(b.StartRef, s, true)
	if !ok {
		return fmt.Errorf("invalid block start ref: %s", b.StartRef)
	}
	endIndex, ok := resolveBoundaryIndex(b.EndRef, s, false)
	if !ok {
		return fmt.Errorf("invalid block end ref: %s", b.EndRef)
	}
	if startIndex > endIndex {
		return fmt.Errorf("invalid block range: %s > %s", b.StartRef, b.EndRef)
	}
	if broken := s.firstBrokenToolPair(startIndex, endIndex); broken != nil {
		return fmt.Errorf("block range splits tool pair %s", broken.ToolCallID)
	}
	if b.CreatedAt.IsZero() {
		b.CreatedAt = time.Now()
	}
	b.Active = true
	if s.Blocks == nil {
		s.Blocks = make(map[int]*CompressionBlock)
	}
	s.Blocks[b.ID] = &b
	return nil
}

func (s *State) DeactivateBlock(id int, consumedBy int) error {
	block, ok := s.Blocks[id]
	if !ok {
		return fmt.Errorf("block %d not found", id)
	}
	block.Active = false
	block.ConsumedBy = consumedBy
	return nil
}

func (s *State) ActiveBlocks() []*CompressionBlock {
	if s == nil || len(s.Blocks) == 0 {
		return nil
	}
	result := make([]*CompressionBlock, 0, len(s.Blocks))
	for _, block := range s.Blocks {
		if block != nil && block.Active {
			result = append(result, block)
		}
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].AnchorMsgIndex < result[j].AnchorMsgIndex
	})
	return result
}

func (s *State) firstBrokenToolPair(startIndex, endIndex int) *ToolPairState {
	for i := range s.ToolPairs {
		pair := &s.ToolPairs[i]
		callIn := startIndex <= pair.CallIndex && pair.CallIndex <= endIndex
		resultIn := startIndex <= pair.ResultIndex && pair.ResultIndex <= endIndex
		if callIn != resultIn {
			return pair
		}
	}
	return nil
}

func resolveBoundaryIndex(ref string, state *State, preferStart bool) (int, bool) {
	return resolveBoundaryIndexDepth(ref, state, preferStart, 0)
}

func resolveBoundaryIndexDepth(ref string, state *State, preferStart bool, depth int) (int, bool) {
	if depth > maxBlockNestingDepth {
		return 0, false
	}
	if idx, ok := state.MessageIDs.ByRef[ref]; ok {
		return idx, true
	}
	if !strings.HasPrefix(ref, "b") {
		return 0, false
	}
	id, err := strconv.Atoi(strings.TrimPrefix(ref, "b"))
	if err != nil {
		return 0, false
	}
	block, ok := state.Blocks[id]
	if !ok || block == nil {
		return 0, false
	}
	if preferStart {
		return resolveBoundaryIndexDepth(block.StartRef, state, true, depth+1)
	}
	return resolveBoundaryIndexDepth(block.EndRef, state, false, depth+1)
}
