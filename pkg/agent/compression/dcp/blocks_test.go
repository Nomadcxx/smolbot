package dcp

import "testing"

func TestAllocateBlockID(t *testing.T) {
	state := NewState("s1")
	if got := state.AllocateBlockID(); got != 1 {
		t.Fatalf("AllocateBlockID() = %d, want 1", got)
	}
	if got := state.AllocateBlockID(); got != 2 {
		t.Fatalf("AllocateBlockID() = %d, want 2", got)
	}
}

func TestCreateBlock_Valid(t *testing.T) {
	state := NewState("s1")
	state.MessageIDs.ByRef["m0001"] = 0
	state.MessageIDs.ByRef["m0003"] = 2
	id := state.AllocateBlockID()
	err := state.CreateBlock(CompressionBlock{
		ID:             id,
		Topic:          "topic",
		Summary:        "summary",
		StartRef:       "m0001",
		EndRef:         "m0003",
		AnchorMsgIndex: 1,
	})
	if err != nil {
		t.Fatalf("CreateBlock: %v", err)
	}
	if !state.Blocks[id].Active {
		t.Fatalf("block active = false, want true")
	}
}

func TestCreateBlock_InvalidRange(t *testing.T) {
	state := NewState("s1")
	state.MessageIDs.ByRef["m0001"] = 0
	state.MessageIDs.ByRef["m0003"] = 2
	id := state.AllocateBlockID()
	if err := state.CreateBlock(CompressionBlock{
		ID:       id,
		Topic:    "topic",
		Summary:  "summary",
		StartRef: "m0003",
		EndRef:   "m0001",
	}); err == nil {
		t.Fatal("CreateBlock() error = nil, want error")
	}
}

func TestDeactivateBlock(t *testing.T) {
	state := NewState("s1")
	state.MessageIDs.ByRef["m0001"] = 0
	state.MessageIDs.ByRef["m0002"] = 1
	id := state.AllocateBlockID()
	if err := state.CreateBlock(CompressionBlock{
		ID:       id,
		Topic:    "topic",
		Summary:  "summary",
		StartRef: "m0001",
		EndRef:   "m0002",
	}); err != nil {
		t.Fatalf("CreateBlock: %v", err)
	}
	if err := state.DeactivateBlock(id, 2); err != nil {
		t.Fatalf("DeactivateBlock: %v", err)
	}
	if state.Blocks[id].Active {
		t.Fatal("block should be inactive")
	}
	if state.Blocks[id].ConsumedBy != 2 {
		t.Fatalf("ConsumedBy = %d, want 2", state.Blocks[id].ConsumedBy)
	}
}
