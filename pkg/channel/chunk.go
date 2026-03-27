package channel

import (
	"strings"
	"unicode/utf8"
)

func ChunkMessage(content string, maxLen int) []string {
	if maxLen <= 0 {
		maxLen = 4096
	}
	if len(content) <= maxLen {
		return []string{content}
	}

	var chunks []string
	for len(content) > 0 {
		if len(content) <= maxLen {
			chunks = append(chunks, content)
			break
		}

		cut := runeSafePrefixLen(content, maxLen)
		prefix := content[:cut]
		if idx := strings.LastIndex(prefix, "\n\n"); idx > cut/2 {
			cut = idx + 2
		} else if idx := strings.LastIndex(prefix, "\n"); idx > cut/2 {
			cut = idx + 1
		} else if idx := strings.LastIndex(prefix, " "); idx > cut/2 {
			cut = idx + 1
		}

		chunks = append(chunks, content[:cut])
		content = content[cut:]
	}
	return chunks
}

func runeSafePrefixLen(content string, maxLen int) int {
	if len(content) <= maxLen {
		return len(content)
	}

	cut := 0
	for i := range content {
		if i > maxLen {
			break
		}
		cut = i
	}
	if cut > 0 {
		return cut
	}

	_, size := utf8.DecodeRuneInString(content)
	if size <= 0 {
		return len(content)
	}
	return size
}
