package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/Nomadcxx/nanobot-go/pkg/provider"
	"github.com/Nomadcxx/nanobot-go/pkg/session"
	"github.com/Nomadcxx/nanobot-go/pkg/tokenizer"
)

const maxMemoryFailuresBeforeRawArchive = 3

type MemoryConsolidator struct {
	provider            provider.Provider
	sessions            *session.Store
	tokenizer           *tokenizer.Tokenizer
	workspace           string
	contextWindowTokens int

	mu           sync.Mutex
	sessionLocks map[string]*sync.Mutex
	failures     map[string]int
}

func NewMemoryConsolidator(p provider.Provider, sessions *session.Store, tok *tokenizer.Tokenizer, workspace string, contextWindowTokens int) *MemoryConsolidator {
	return &MemoryConsolidator{
		provider:            p,
		sessions:            sessions,
		tokenizer:           tok,
		workspace:           workspace,
		contextWindowTokens: contextWindowTokens,
		sessionLocks:        make(map[string]*sync.Mutex),
		failures:            make(map[string]int),
	}
}

func (m *MemoryConsolidator) MaybeConsolidate(ctx context.Context, sessionKey string) error {
	lock := m.sessionLock(sessionKey)
	lock.Lock()
	defer lock.Unlock()

	for round := 0; round < 5; round++ {
		records, err := m.sessions.GetUnconsolidatedMessages(sessionKey, 500)
		if err != nil {
			return err
		}
		if len(records) == 0 {
			return nil
		}

		messages := make([]provider.Message, 0, len(records))
		for _, record := range records {
			messages = append(messages, record.Message)
		}
		if m.tokenizer.EstimatePromptTokens(messages) <= m.contextWindowTokens {
			return nil
		}

		boundaryIdx, upToID := findConsolidationBoundary(records)
		if boundaryIdx < 0 || upToID == 0 {
			return nil
		}

		toConsolidate := records[:boundaryIdx+1]
		if err := m.consolidateBatch(ctx, sessionKey, toConsolidate, upToID); err != nil {
			if m.incrementFailure(sessionKey) >= maxMemoryFailuresBeforeRawArchive {
				if err := m.rawArchive(sessionKey, toConsolidate, upToID); err != nil {
					return err
				}
				m.resetFailure(sessionKey)
				continue
			}
			return nil
		}

		m.resetFailure(sessionKey)
	}

	return nil
}

func (m *MemoryConsolidator) consolidateBatch(ctx context.Context, sessionKey string, records []session.StoredMessage, upToID int64) error {
	history := make([]provider.Message, 0, len(records))
	for _, record := range records {
		history = append(history, record.Message)
	}

	toolChoice := "save_memory"
	resp, err := m.provider.Chat(ctx, provider.ChatRequest{
		Model:      "save-memory",
		Messages:   history,
		Tools:      []provider.ToolDef{saveMemoryTool()},
		ToolChoice: toolChoice,
		MaxTokens:  1024,
	})
	if err != nil {
		toolChoice = "auto"
		resp, err = m.provider.Chat(ctx, provider.ChatRequest{
			Model:      "save-memory",
			Messages:   history,
			Tools:      []provider.ToolDef{saveMemoryTool()},
			ToolChoice: toolChoice,
			MaxTokens:  1024,
		})
		if err != nil {
			return err
		}
	}
	if len(resp.ToolCalls) == 0 {
		return fmt.Errorf("save_memory tool call missing")
	}

	var payload any
	if err := json.Unmarshal([]byte(resp.ToolCalls[0].Function.Arguments), &payload); err != nil {
		payload = resp.ToolCalls[0].Function.Arguments
	}
	historyEntry, memoryUpdate := normalizeSaveMemoryArgs(payload)
	if historyEntry == "" && memoryUpdate == "" {
		return fmt.Errorf("empty save_memory payload")
	}

	if err := appendHistory(filepath.Join(m.workspace, "memory", "HISTORY.md"), historyEntry); err != nil {
		return err
	}
	if memoryUpdate != "" {
		if err := os.WriteFile(filepath.Join(m.workspace, "memory", "MEMORY.md"), []byte(memoryUpdate), 0o644); err != nil {
			return fmt.Errorf("write MEMORY.md: %w", err)
		}
	}
	if err := m.sessions.MarkConsolidated(sessionKey, upToID); err != nil {
		return err
	}
	return nil
}

func findConsolidationBoundary(records []session.StoredMessage) (int, int64) {
	if len(records) < 2 {
		return -1, 0
	}

	target := len(records) / 2
	if target == 0 {
		target = 1
	}

	boundaryIdx := -1
	for i := 0; i < len(records); i++ {
		msg := records[i].Message
		if msg.Role != "assistant" || (msg.StringContent() == "" && len(msg.ToolCalls) == 0) {
			continue
		}
		if i < target {
			continue
		}
		boundaryIdx = i
		break
	}
	if boundaryIdx == -1 {
		for i := target - 1; i >= 0; i-- {
			msg := records[i].Message
			if msg.Role == "assistant" && (msg.StringContent() != "" || len(msg.ToolCalls) > 0) {
				boundaryIdx = i
				break
			}
		}
	}
	if boundaryIdx == -1 {
		return len(records)/2 - 1, records[len(records)/2-1].ID
	}
	return boundaryIdx, records[boundaryIdx].ID
}

func normalizeSaveMemoryArgs(input any) (string, string) {
	switch value := input.(type) {
	case map[string]any:
		return stringify(value["history_entry"]), stringify(value["memory_update"])
	case string:
		var decoded map[string]any
		if json.Unmarshal([]byte(value), &decoded) == nil {
			return normalizeSaveMemoryArgs(decoded)
		}
		return value, ""
	case []any:
		var historyEntry string
		var memoryUpdate string
		for _, item := range value {
			switch typed := item.(type) {
			case string:
				if historyEntry == "" {
					historyEntry = typed
				}
			case map[string]any:
				if historyEntry == "" {
					historyEntry = stringify(typed["history_entry"])
				}
				if memoryUpdate == "" {
					memoryUpdate = stringify(typed["memory_update"])
				}
			}
		}
		return historyEntry, memoryUpdate
	default:
		return "", ""
	}
}

func saveMemoryTool() provider.ToolDef {
	return provider.ToolDef{
		Name:        "save_memory",
		Description: "Persist condensed memory state",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"history_entry": map[string]any{"type": "string"},
				"memory_update": map[string]any{"type": "string"},
			},
		},
	}
}

func appendHistory(path, entry string) error {
	if entry == "" {
		return nil
	}
	line := fmt.Sprintf("[%s] %s\n", time.Now().Format(time.RFC3339), entry)
	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return fmt.Errorf("open HISTORY.md: %w", err)
	}
	defer file.Close()
	if _, err := file.WriteString(line); err != nil {
		return fmt.Errorf("append HISTORY.md: %w", err)
	}
	return nil
}

func (m *MemoryConsolidator) rawArchive(sessionKey string, records []session.StoredMessage, upToID int64) error {
	var lines []string
	lines = append(lines, "RAW ARCHIVE")
	for _, record := range records {
		lines = append(lines, fmt.Sprintf("%s: %s", record.Message.Role, record.Message.StringContent()))
	}
	if err := appendHistory(filepath.Join(m.workspace, "memory", "HISTORY.md"), strings.Join(lines, "\n")); err != nil {
		return err
	}
	return m.sessions.MarkConsolidated(sessionKey, upToID)
}

func (m *MemoryConsolidator) sessionLock(sessionKey string) *sync.Mutex {
	m.mu.Lock()
	defer m.mu.Unlock()
	lock, ok := m.sessionLocks[sessionKey]
	if !ok {
		lock = &sync.Mutex{}
		m.sessionLocks[sessionKey] = lock
	}
	return lock
}

func (m *MemoryConsolidator) incrementFailure(sessionKey string) int {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.failures[sessionKey]++
	return m.failures[sessionKey]
}

func (m *MemoryConsolidator) resetFailure(sessionKey string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.failures, sessionKey)
}

func stringify(value any) string {
	switch typed := value.(type) {
	case nil:
		return ""
	case string:
		return typed
	default:
		data, err := json.Marshal(typed)
		if err != nil {
			return ""
		}
		return string(data)
	}
}
