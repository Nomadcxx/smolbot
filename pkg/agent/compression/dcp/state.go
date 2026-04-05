package dcp

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"sync"
	"time"
)

const stateSchema = `
CREATE TABLE IF NOT EXISTS dcp_state (
    session_key TEXT PRIMARY KEY,
    state_json  TEXT NOT NULL,
    updated_at  TEXT DEFAULT (datetime('now'))
);
`

type State struct {
	SessionKey       string                    `json:"sessionKey"`
	Blocks           map[int]*CompressionBlock `json:"blocks"`
	NextBlockID      int                       `json:"nextBlockId"`
	MessageIDs       MessageIDState            `json:"messageIds"`
	ToolPairs        []ToolPairState           `json:"toolPairs,omitempty"`
	ProtectedIndexes map[int]bool              `json:"protectedIndexes,omitempty"`
	CurrentTurn      int                       `json:"currentTurn"`
	RequestCount     int                       `json:"requestCount"`
	NudgeAnchors     map[string][]string       `json:"nudgeAnchors"`
	Stats            DCPStats                  `json:"stats"`
}

type DCPStats struct {
	TotalPrunedTokens int `json:"totalPrunedTokens"`
	TotalCompressions int `json:"totalCompressions"`
	TotalDedups       int `json:"totalDedups"`
	TotalErrorPurges  int `json:"totalErrorPurges"`
}

type MessageIDState struct {
	ByMsgIndex map[int]string `json:"byMsgIndex"`
	ByRef      map[string]int `json:"byRef"`
	NextRef    int            `json:"nextRef"`
}

type ToolPairState struct {
	ToolCallID  string `json:"toolCallId"`
	CallIndex   int    `json:"callIndex"`
	ResultIndex int    `json:"resultIndex"`
}

type StateManager struct {
	db    *sql.DB
	cache map[string]*State
	mu    sync.RWMutex
}

func NewState(sessionKey string) *State {
	return &State{
		SessionKey:  sessionKey,
		Blocks:      make(map[int]*CompressionBlock),
		NextBlockID: 1,
		MessageIDs: MessageIDState{
			ByMsgIndex: make(map[int]string),
			ByRef:      make(map[string]int),
			NextRef:    1,
		},
		ProtectedIndexes: make(map[int]bool),
		NudgeAnchors: make(map[string][]string),
	}
}

func NewStateManager(db *sql.DB) (*StateManager, error) {
	if db == nil {
		return nil, fmt.Errorf("nil db")
	}
	if _, err := db.Exec(stateSchema); err != nil {
		return nil, fmt.Errorf("create dcp_state schema: %w", err)
	}
	return &StateManager{
		db:    db,
		cache: make(map[string]*State),
	}, nil
}

func (sm *StateManager) Load(sessionKey string) (*State, error) {
	sm.mu.RLock()
	if cached, ok := sm.cache[sessionKey]; ok {
		defer sm.mu.RUnlock()
		return cloneState(cached)
	}
	sm.mu.RUnlock()

	var raw string
	err := sm.db.QueryRow(`SELECT state_json FROM dcp_state WHERE session_key = ?`, sessionKey).Scan(&raw)
	switch {
	case err == sql.ErrNoRows:
		state := NewState(sessionKey)
		sm.mu.Lock()
		sm.cache[sessionKey] = cloneStateOrFallback(state)
		sm.mu.Unlock()
		return state, nil
	case err != nil:
		return nil, fmt.Errorf("load dcp state: %w", err)
	}

	var state State
	if err := json.Unmarshal([]byte(raw), &state); err != nil {
		return nil, fmt.Errorf("decode dcp state: %w", err)
	}
	normalizeState(&state, sessionKey)

	sm.mu.Lock()
	sm.cache[sessionKey] = cloneStateOrFallback(&state)
	sm.mu.Unlock()
	return &state, nil
}

func (sm *StateManager) Save(state *State) error {
	if state == nil {
		return fmt.Errorf("nil state")
	}
	normalizeState(state, state.SessionKey)

	payload, err := json.Marshal(state)
	if err != nil {
		return fmt.Errorf("encode dcp state: %w", err)
	}
	if _, err := sm.db.Exec(`
		INSERT INTO dcp_state (session_key, state_json, updated_at)
		VALUES (?, ?, datetime('now'))
		ON CONFLICT(session_key) DO UPDATE SET
			state_json = excluded.state_json,
			updated_at = datetime('now')
	`, state.SessionKey, string(payload)); err != nil {
		return fmt.Errorf("save dcp state: %w", err)
	}

	sm.mu.Lock()
	sm.cache[state.SessionKey] = cloneStateOrFallback(state)
	sm.mu.Unlock()
	return nil
}

func (sm *StateManager) Delete(sessionKey string) error {
	if _, err := sm.db.Exec(`DELETE FROM dcp_state WHERE session_key = ?`, sessionKey); err != nil {
		return fmt.Errorf("delete dcp state: %w", err)
	}
	sm.mu.Lock()
	delete(sm.cache, sessionKey)
	sm.mu.Unlock()
	return nil
}

func normalizeState(state *State, sessionKey string) {
	if state.SessionKey == "" {
		state.SessionKey = sessionKey
	}
	if state.Blocks == nil {
		state.Blocks = make(map[int]*CompressionBlock)
	}
	if state.NextBlockID <= 0 {
		state.NextBlockID = 1
	}
	if state.MessageIDs.ByMsgIndex == nil {
		state.MessageIDs.ByMsgIndex = make(map[int]string)
	}
	if state.MessageIDs.ByRef == nil {
		state.MessageIDs.ByRef = make(map[string]int)
	}
	if state.MessageIDs.NextRef <= 0 {
		state.MessageIDs.NextRef = 1
	}
	if state.NudgeAnchors == nil {
		state.NudgeAnchors = make(map[string][]string)
	}
	if state.ProtectedIndexes == nil {
		state.ProtectedIndexes = make(map[int]bool)
	}
	if state.ToolPairs == nil {
		state.ToolPairs = []ToolPairState{}
	}
}

// cloneStateOrFallback deep-copies state, falling back to the original on marshal error.
// This avoids a panic in production while preserving cache isolation as best-effort.
func cloneStateOrFallback(state *State) *State {
	cloned, err := cloneState(state)
	if err != nil {
		// Return a normalized copy of the original rather than panicking.
		normalizeState(state, state.SessionKey)
		return state
	}
	return cloned
}

func cloneState(state *State) (*State, error) {
	if state == nil {
		return nil, nil
	}
	payload, err := json.Marshal(state)
	if err != nil {
		return nil, err
	}
	var cloned State
	if err := json.Unmarshal(payload, &cloned); err != nil {
		return nil, err
	}
	normalizeState(&cloned, cloned.SessionKey)
	return &cloned, nil
}

type CompressionBlock struct {
	ID             int       `json:"id"`
	Active         bool      `json:"active"`
	Topic          string    `json:"topic"`
	Summary        string    `json:"summary"`
	StartRef       string    `json:"startRef"`
	EndRef         string    `json:"endRef"`
	AnchorMsgIndex int       `json:"anchorMsgIndex"`
	MessageIDs     []string  `json:"messageIds"`
	ToolIDs        []string  `json:"toolIds"`
	ConsumedBlocks []int     `json:"consumedBlocks"`
	TokensSaved    int       `json:"tokensSaved"`
	SummaryTokens  int       `json:"summaryTokens"`
	ConsumedBy     int       `json:"consumedBy,omitempty"`
	CreatedAt      time.Time `json:"createdAt"`
}
