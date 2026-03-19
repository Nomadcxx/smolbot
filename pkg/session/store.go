package session

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/Nomadcxx/nanobot-go/pkg/provider"
	_ "github.com/mattn/go-sqlite3"
)

const schema = `
CREATE TABLE IF NOT EXISTS sessions (
    key         TEXT PRIMARY KEY,
    metadata    TEXT DEFAULT '{}',
    created_at  DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at  DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS messages (
    id                INTEGER PRIMARY KEY AUTOINCREMENT,
    session_key       TEXT NOT NULL REFERENCES sessions(key),
    role              TEXT NOT NULL,
    content           TEXT,
    tool_calls        TEXT,
    tool_call_id      TEXT,
    name              TEXT,
    reasoning_content TEXT,
    thinking_blocks   TEXT,
    consolidated      BOOLEAN DEFAULT FALSE,
    created_at        DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_messages_session ON messages(session_key, consolidated, id);
`

type Session struct {
	Key       string
	Metadata  string
	CreatedAt time.Time
	UpdatedAt time.Time
}

type Store struct {
	db *sql.DB
}

type storedMessage struct {
	ID               int64
	SessionKey       string
	Role             string
	Content          sql.NullString
	ToolCalls        sql.NullString
	ToolCallID       sql.NullString
	Name             sql.NullString
	ReasoningContent sql.NullString
	ThinkingBlocks   sql.NullString
	Consolidated     bool
	CreatedAt        time.Time
}

func NewStore(dsn string) (*Store, error) {
	db, err := sql.Open("sqlite3", sqliteDSN(dsn))
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}

	if _, err := db.Exec(schema); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("create schema: %w", err)
	}

	return &Store{db: db}, nil
}

func (s *Store) Close() error {
	return s.db.Close()
}

func (s *Store) GetOrCreateSession(key string) (*Session, error) {
	if _, err := s.db.Exec(`INSERT OR IGNORE INTO sessions (key) VALUES (?)`, key); err != nil {
		return nil, fmt.Errorf("upsert session: %w", err)
	}

	var session Session
	if err := s.db.QueryRow(
		`SELECT key, metadata, created_at, updated_at FROM sessions WHERE key = ?`,
		key,
	).Scan(&session.Key, &session.Metadata, &session.CreatedAt, &session.UpdatedAt); err != nil {
		return nil, fmt.Errorf("query session: %w", err)
	}

	return &session, nil
}

func (s *Store) SaveMessages(sessionKey string, msgs []provider.Message) error {
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback()

	if _, err := tx.Exec(`INSERT OR IGNORE INTO sessions (key) VALUES (?)`, sessionKey); err != nil {
		return fmt.Errorf("upsert session: %w", err)
	}

	stmt, err := tx.Prepare(`
		INSERT INTO messages (
			session_key, role, content, tool_calls, tool_call_id, name, reasoning_content, thinking_blocks
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`)
	if err != nil {
		return fmt.Errorf("prepare insert: %w", err)
	}
	defer stmt.Close()

	for _, msg := range msgs {
		if shouldSkipAssistant(msg) {
			continue
		}

		contentJSON, err := marshalJSON(msg.Content)
		if err != nil {
			return fmt.Errorf("marshal content: %w", err)
		}
		toolCallsJSON, err := marshalJSON(msg.ToolCalls)
		if err != nil {
			return fmt.Errorf("marshal tool calls: %w", err)
		}
		thinkingJSON, err := marshalJSON(msg.ThinkingBlocks)
		if err != nil {
			return fmt.Errorf("marshal thinking blocks: %w", err)
		}

		if _, err := stmt.Exec(
			sessionKey,
			msg.Role,
			nullString(contentJSON),
			nullString(toolCallsJSON),
			nullString(msg.ToolCallID),
			nullString(msg.Name),
			nullString(msg.ReasoningContent),
			nullString(thinkingJSON),
		); err != nil {
			return fmt.Errorf("insert message: %w", err)
		}
	}

	if _, err := tx.Exec(`UPDATE sessions SET updated_at = CURRENT_TIMESTAMP WHERE key = ?`, sessionKey); err != nil {
		return fmt.Errorf("update session timestamp: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}

	return nil
}

func (s *Store) MarkConsolidated(sessionKey string, upToID int64) error {
	if _, err := s.db.Exec(
		`UPDATE messages SET consolidated = TRUE WHERE session_key = ? AND id <= ?`,
		sessionKey,
		upToID,
	); err != nil {
		return fmt.Errorf("mark consolidated: %w", err)
	}
	return nil
}

func (s *Store) ListSessions() ([]Session, error) {
	rows, err := s.db.Query(`SELECT key, metadata, created_at, updated_at FROM sessions ORDER BY updated_at DESC, key ASC`)
	if err != nil {
		return nil, fmt.Errorf("list sessions: %w", err)
	}
	defer rows.Close()

	var sessions []Session
	for rows.Next() {
		var session Session
		if err := rows.Scan(&session.Key, &session.Metadata, &session.CreatedAt, &session.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan session: %w", err)
		}
		sessions = append(sessions, session)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate sessions: %w", err)
	}

	return sessions, nil
}

func (s *Store) ClearSession(key string) error {
	if _, err := s.db.Exec(`DELETE FROM messages WHERE session_key = ?`, key); err != nil {
		return fmt.Errorf("clear session: %w", err)
	}
	return nil
}

func sqliteDSN(dsn string) string {
	if dsn == ":memory:" {
		return dsn
	}

	separator := "?"
	if strings.Contains(dsn, "?") {
		separator = "&"
	}
	return dsn + separator + "_journal_mode=WAL&_busy_timeout=5000&_foreign_keys=on"
}

func shouldSkipAssistant(msg provider.Message) bool {
	if msg.Role != "assistant" {
		return false
	}
	return contentIsEmpty(msg.Content) &&
		len(msg.ToolCalls) == 0 &&
		msg.ToolCallID == "" &&
		msg.Name == "" &&
		msg.ReasoningContent == "" &&
		len(msg.ThinkingBlocks) == 0
}

func contentIsEmpty(content any) bool {
	switch value := content.(type) {
	case nil:
		return true
	case string:
		return value == ""
	case []provider.ContentBlock:
		return len(value) == 0
	case []any:
		return len(value) == 0
	default:
		return false
	}
}

func marshalJSON(value any) (string, error) {
	if value == nil {
		return "", nil
	}

	data, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	if string(data) == "null" {
		return "", nil
	}
	return string(data), nil
}

func nullString(value string) any {
	if value == "" {
		return nil
	}
	return value
}
