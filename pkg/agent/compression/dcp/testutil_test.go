package dcp

import (
	"database/sql"
	"testing"

	"github.com/Nomadcxx/smolbot/pkg/provider"
	_ "github.com/mattn/go-sqlite3"
)

func makeUserMsg(content string) provider.Message {
	return provider.Message{Role: "user", Content: content}
}

func makeAssistantMsg(content string) provider.Message {
	return provider.Message{Role: "assistant", Content: content}
}

func makeAssistantWithToolCall(content string, toolName string, args string, callID string) provider.Message {
	return provider.Message{
		Role:    "assistant",
		Content: content,
		ToolCalls: []provider.ToolCall{{
			ID: callID,
			Function: provider.FunctionCall{
				Name:      toolName,
				Arguments: args,
			},
		}},
	}
}

func makeToolResult(callID string, toolName string, content string) provider.Message {
	return provider.Message{Role: "tool", ToolCallID: callID, Name: toolName, Content: content}
}

func makeToolError(callID string, toolName string, errorMsg string) provider.Message {
	return makeToolResult(callID, toolName, errorMsg)
}

func makeInMemoryDB(t *testing.T) *sql.DB {
	t.Helper()

	db, err := sql.Open("sqlite3", "file::memory:?cache=shared")
	if err != nil {
		t.Fatalf("sql.Open: %v", err)
	}
	db.SetMaxOpenConns(1)
	t.Cleanup(func() {
		_ = db.Close()
	})
	return db
}
