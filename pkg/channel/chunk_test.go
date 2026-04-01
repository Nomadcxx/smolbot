package channel

import "testing"

func TestChunkMessageShortPassthrough(t *testing.T) {
	content := "Short message"
	chunks := ChunkMessage(content, 4096)
	if len(chunks) != 1 || chunks[0] != content {
		t.Errorf("expected single chunk %q, got %v", content, chunks)
	}
}

func TestChunkMessageExactLimit(t *testing.T) {
	content := "Exactly 16" // 16 bytes
	chunks := ChunkMessage(content, 16)
	if len(chunks) != 1 || chunks[0] != content {
		t.Errorf("expected single chunk %q, got %v", content, chunks)
	}
}

func TestChunkMessageBreaksAtParagraph(t *testing.T) {
	content := "First paragraph.\n\nSecond paragraph.\n\nThird paragraph."
	chunks := ChunkMessage(content, 20)
	if len(chunks) != 3 {
		t.Errorf("expected 3 chunks, got %d: %v", len(chunks), chunks)
	}
	if chunks[0] != "First paragraph." {
		t.Errorf("expected first chunk %q, got %q", "First paragraph.", chunks[0])
	}
}

func TestChunkMessageBreaksAtNewline(t *testing.T) {
	content := "Line one\nLine two\nLine three"
	chunks := ChunkMessage(content, 15)
	if len(chunks) != 3 {
		t.Errorf("expected 3 chunks, got %d: %v", len(chunks), chunks)
	}
}

func TestChunkMessageBreaksAtSpace(t *testing.T) {
	content := "Word word word word word"
	chunks := ChunkMessage(content, 15)
	// "Word word word wor" (15 chars) + "d word word word" (14 chars)
	if len(chunks) != 2 {
		t.Errorf("expected 2 chunks, got %d: %v", len(chunks), chunks)
	}
}

func TestChunkMessageHardBreak(t *testing.T) {
	// 47 chars total, with maxLen=15 should give 3 chunks
	content := "ThisIsALongWordWithoutAnyBreaksThatExceeds"
	chunks := ChunkMessage(content, 15)
	if len(chunks) != 3 {
		t.Errorf("expected 3 chunks for hard break, got %d: %v", len(chunks), chunks)
	}
}

func TestChunkMessageEmptyString(t *testing.T) {
	chunks := ChunkMessage("", 4096)
	if len(chunks) != 1 || chunks[0] != "" {
		t.Errorf("expected single empty chunk, got %v", chunks)
	}
}

func TestChunkMessageZeroMaxLen(t *testing.T) {
	content := "Test"
	chunks := ChunkMessage(content, 0)
	if len(chunks) != 1 || chunks[0] != content {
		t.Errorf("expected single chunk for zero maxLen, got %v", chunks)
	}
}

func TestChunkMessageNegativeMaxLen(t *testing.T) {
	content := "Test"
	chunks := ChunkMessage(content, -1)
	if len(chunks) != 1 || chunks[0] != content {
		t.Errorf("expected single chunk for negative maxLen, got %v", chunks)
	}
}

func TestChunkMessageSingleLongWord(t *testing.T) {
	content := "supercalifragilisticexpialidocious" // 34 chars
	chunks := ChunkMessage(content, 10)
	if len(chunks) != 4 {
		t.Errorf("expected 4 chunks, got %d: %v", len(chunks), chunks)
	}
}

func TestChunkMessagePreservesContent(t *testing.T) {
	content := "Hello world this is a test message.\n\nWith a paragraph break."
	joined := ""
	for _, chunk := range ChunkMessage(content, 30) {
		joined += chunk
	}
	normalized := normalizeForComparison(content)
	normalizedJoined := normalizeForComparison(joined)
	if normalized != normalizedJoined {
		t.Errorf("content mismatch after chunk/rejoin\ngot:      %q\nwanted: %q", normalizedJoined, normalized)
	}
}

func normalizeForComparison(s string) string {
	result := ""
	for _, r := range s {
		if r != ' ' && r != '\n' {
			result += string(r)
		}
	}
	return result
}

func TestChunkMessageDiscordLimit(t *testing.T) {
	content := "Lorem ipsum dolor sit amet, consectetur adipiscing elit. Sed do eiusmod tempor incididunt ut labore."
	chunks := ChunkMessage(content, 2000)
	if len(chunks) != 1 {
		t.Errorf("expected 1 chunk for content under 2000, got %d", len(chunks))
	}
}

func TestChunkMessageTelegramLimit(t *testing.T) {
	content := "Lorem ipsum dolor sit amet, consectetur adipiscing elit. Sed do eiusmod tempor incididunt ut labore."
	chunks := ChunkMessage(content, 4096)
	if len(chunks) != 1 {
		t.Errorf("expected 1 chunk for content under 4096, got %d", len(chunks))
	}
}
