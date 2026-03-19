package session

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/Nomadcxx/nanobot-go/pkg/provider"
)

type StoredMessage struct {
	ID           int64
	SessionKey   string
	Message      provider.Message
	Consolidated bool
	CreatedAt    time.Time
}

func (s *Store) GetHistory(sessionKey string, maxMessages int) ([]provider.Message, error) {
	records, err := s.GetUnconsolidatedMessages(sessionKey, maxMessages)
	if err != nil {
		return nil, err
	}

	msgs := make([]provider.Message, 0, len(records))
	for _, record := range records {
		msgs = append(msgs, record.Message)
	}

	msgs = dropLeadingNonUser(msgs)
	msgs = enforceLegalBoundary(msgs)
	msgs = stripMessages(msgs)

	return msgs, nil
}

func (s *Store) GetUnconsolidatedMessages(sessionKey string, maxMessages int) ([]StoredMessage, error) {
	if maxMessages <= 0 {
		maxMessages = 500
	}

	rows, err := s.db.Query(`
		SELECT id, session_key, role, content, tool_calls, tool_call_id, name, reasoning_content, thinking_blocks, consolidated, created_at
		FROM (
			SELECT id, session_key, role, content, tool_calls, tool_call_id, name, reasoning_content, thinking_blocks, consolidated, created_at
			FROM messages
			WHERE session_key = ? AND consolidated = FALSE
			ORDER BY id DESC
			LIMIT ?
		)
		ORDER BY id ASC
	`, sessionKey, maxMessages)
	if err != nil {
		return nil, fmt.Errorf("query history: %w", err)
	}
	defer rows.Close()

	var stored []storedMessage
	for rows.Next() {
		var msg storedMessage
		if err := rows.Scan(
			&msg.ID,
			&msg.SessionKey,
			&msg.Role,
			&msg.Content,
			&msg.ToolCalls,
			&msg.ToolCallID,
			&msg.Name,
			&msg.ReasoningContent,
			&msg.ThinkingBlocks,
			&msg.Consolidated,
			&msg.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan history: %w", err)
		}
		stored = append(stored, msg)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate history: %w", err)
	}

	msgs, err := decodeStoredMessages(stored)
	if err != nil {
		return nil, err
	}

	return msgs, nil
}

func (s *Store) CountUnconsolidated(sessionKey string) (int, error) {
	var count int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM messages WHERE session_key = ? AND consolidated = FALSE`, sessionKey).Scan(&count); err != nil {
		return 0, fmt.Errorf("count unconsolidated: %w", err)
	}
	return count, nil
}

func decodeStoredMessages(stored []storedMessage) ([]StoredMessage, error) {
	messages := make([]StoredMessage, 0, len(stored))
	for _, row := range stored {
		msg := provider.Message{
			Role:             row.Role,
			ToolCallID:       row.ToolCallID.String,
			Name:             row.Name.String,
			ReasoningContent: row.ReasoningContent.String,
		}

		if row.Content.Valid {
			content, err := decodeContent(row.Content.String)
			if err != nil {
				return nil, fmt.Errorf("decode content for message %d: %w", row.ID, err)
			}
			msg.Content = content
		}

		if row.ToolCalls.Valid && row.ToolCalls.String != "" {
			if err := json.Unmarshal([]byte(row.ToolCalls.String), &msg.ToolCalls); err != nil {
				return nil, fmt.Errorf("decode tool calls for message %d: %w", row.ID, err)
			}
		}

		if row.ThinkingBlocks.Valid && row.ThinkingBlocks.String != "" {
			if err := json.Unmarshal([]byte(row.ThinkingBlocks.String), &msg.ThinkingBlocks); err != nil {
				return nil, fmt.Errorf("decode thinking blocks for message %d: %w", row.ID, err)
			}
		}

		messages = append(messages, StoredMessage{
			ID:           row.ID,
			SessionKey:   row.SessionKey,
			Message:      msg,
			Consolidated: row.Consolidated,
			CreatedAt:    row.CreatedAt,
		})
	}

	return messages, nil
}

func decodeContent(raw string) (any, error) {
	var decoded any
	if err := json.Unmarshal([]byte(raw), &decoded); err != nil {
		return nil, err
	}

	if list, ok := decoded.([]any); ok && looksLikeContentBlocks(list) {
		blocks := make([]provider.ContentBlock, 0, len(list))
		for _, item := range list {
			blockMap, ok := item.(map[string]any)
			if !ok {
				return decoded, nil
			}
			block, err := mapToContentBlock(blockMap)
			if err != nil {
				return nil, err
			}
			blocks = append(blocks, block)
		}
		return blocks, nil
	}

	return decoded, nil
}

func looksLikeContentBlocks(items []any) bool {
	for _, item := range items {
		blockMap, ok := item.(map[string]any)
		if !ok {
			return false
		}
		if _, ok := blockMap["type"]; !ok {
			return false
		}
	}
	return true
}

func mapToContentBlock(raw map[string]any) (provider.ContentBlock, error) {
	block := provider.ContentBlock{}
	if rawType, ok := raw["type"].(string); ok {
		block.Type = rawType
	}
	if rawText, ok := raw["text"].(string); ok {
		block.Text = rawText
	}
	if rawImage, ok := raw["image_url"].(map[string]any); ok {
		block.ImageURL = &provider.ImageURL{}
		if value, ok := rawImage["url"].(string); ok {
			block.ImageURL.URL = value
		}
		if value, ok := rawImage["detail"].(string); ok {
			block.ImageURL.Detail = value
		}
	}
	return block, nil
}

func dropLeadingNonUser(msgs []provider.Message) []provider.Message {
	for i, msg := range msgs {
		if msg.Role == "user" {
			return msgs[i:]
		}
	}
	return nil
}

func enforceLegalBoundary(msgs []provider.Message) []provider.Message {
	declaredToolCalls := make(map[string]struct{})
	legalStart := 0

	for i, msg := range msgs {
		if msg.Role == "assistant" {
			for _, call := range msg.ToolCalls {
				if call.ID != "" {
					declaredToolCalls[call.ID] = struct{}{}
				}
			}
			continue
		}

		if msg.Role == "tool" && msg.ToolCallID != "" {
			if _, ok := declaredToolCalls[msg.ToolCallID]; !ok {
				legalStart = i + 1
				declaredToolCalls = make(map[string]struct{})
			}
		}
	}

	if legalStart >= len(msgs) {
		return nil
	}

	return dropLeadingNonUser(msgs[legalStart:])
}

func stripMessages(msgs []provider.Message) []provider.Message {
	stripped := make([]provider.Message, 0, len(msgs))
	for _, msg := range msgs {
		stripped = append(stripped, provider.Message{
			Role:             msg.Role,
			Content:          msg.Content,
			ToolCalls:        msg.ToolCalls,
			ToolCallID:       msg.ToolCallID,
			Name:             msg.Name,
			ReasoningContent: msg.ReasoningContent,
			ThinkingBlocks:   msg.ThinkingBlocks,
		})
	}
	return stripped
}
