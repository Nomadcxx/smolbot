package dialog

import "strings"

func dialogWidth(termWidth, preferred int) int {
	if termWidth <= 0 {
		return preferred
	}
	if max := termWidth - 4; max < preferred {
		return max
	}
	return preferred
}

const maxVisibleItems = 7

func matchesQuery(query string, fields ...string) bool {
	tokens := strings.Fields(strings.ToLower(query))
	if len(tokens) == 0 {
		return true
	}
	haystack := strings.ToLower(strings.Join(fields, " "))
	words := strings.FieldsFunc(haystack, func(r rune) bool {
		return (r < 'a' || r > 'z') && (r < '0' || r > '9')
	})
	for _, token := range tokens {
		if len(token) == 1 {
			if !hasWordPrefix(words, token) {
				return false
			}
			continue
		}
		if !strings.Contains(haystack, token) && !hasWordPrefix(words, token) {
			return false
		}
	}
	return true
}

func visibleBounds(total, cursor int) (int, int) {
	if total <= maxVisibleItems {
		return 0, total
	}
	start := cursor - maxVisibleItems/2
	if start < 0 {
		start = 0
	}
	end := start + maxVisibleItems
	if end > total {
		end = total
		start = max(0, end-maxVisibleItems)
	}
	return start, end
}

func hasWordPrefix(words []string, token string) bool {
	for _, word := range words {
		if strings.HasPrefix(word, token) {
			return true
		}
	}
	return false
}
