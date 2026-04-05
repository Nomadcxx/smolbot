package dcp

import (
	"sync"
	"testing"
)

func TestStateManagerCRUD(t *testing.T) {
	sm, err := NewStateManager(makeInMemoryDB(t))
	if err != nil {
		t.Fatalf("NewStateManager: %v", err)
	}

	state, err := sm.Load("s1")
	if err != nil {
		t.Fatalf("Load(empty): %v", err)
	}
	if state.SessionKey != "s1" {
		t.Fatalf("SessionKey = %q, want s1", state.SessionKey)
	}
	if state.NextBlockID != 1 {
		t.Fatalf("NextBlockID = %d, want 1", state.NextBlockID)
	}

	state.RequestCount = 2
	state.CurrentTurn = 3
	state.MessageIDs.NextRef = 9
	state.Stats.TotalDedups = 4
	if err := sm.Save(state); err != nil {
		t.Fatalf("Save: %v", err)
	}

	reloaded, err := sm.Load("s1")
	if err != nil {
		t.Fatalf("Load(saved): %v", err)
	}
	if reloaded.RequestCount != 2 || reloaded.CurrentTurn != 3 {
		t.Fatalf("reloaded state = %+v, want persisted values", reloaded)
	}
	if reloaded.MessageIDs.NextRef != 9 {
		t.Fatalf("MessageIDs.NextRef = %d, want 9", reloaded.MessageIDs.NextRef)
	}
	if reloaded.Stats.TotalDedups != 4 {
		t.Fatalf("Stats.TotalDedups = %d, want 4", reloaded.Stats.TotalDedups)
	}

	if err := sm.Delete("s1"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	afterDelete, err := sm.Load("s1")
	if err != nil {
		t.Fatalf("Load(after delete): %v", err)
	}
	if afterDelete.RequestCount != 0 {
		t.Fatalf("RequestCount = %d, want 0 after delete", afterDelete.RequestCount)
	}
}

func TestStateManagerConcurrency(t *testing.T) {
	sm, err := NewStateManager(makeInMemoryDB(t))
	if err != nil {
		t.Fatalf("NewStateManager: %v", err)
	}

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			state, err := sm.Load("shared")
			if err != nil {
				t.Errorf("Load: %v", err)
				return
			}
			state.RequestCount = i + 1
			state.CurrentTurn = i
			if err := sm.Save(state); err != nil {
				t.Errorf("Save: %v", err)
			}
		}(i)
	}
	wg.Wait()

	state, err := sm.Load("shared")
	if err != nil {
		t.Fatalf("Load(final): %v", err)
	}
	if state.RequestCount == 0 {
		t.Fatalf("RequestCount = 0, want persisted value")
	}
}
