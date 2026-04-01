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

const maxVisibleItems = 10

func matchesQuery(query string, fields ...string) bool {
	tokens := strings.Fields(strings.ToLower(query))
	if len(tokens) == 0 {
		return true
	}
	haystack := strings.ToLower(strings.Join(fields, " "))
	for _, token := range tokens {
		if !fuzzyMatch(token, haystack) {
			return false
		}
	}
	return true
}

func fuzzyMatch(needle, haystack string) bool {
	ni := 0
	for hi := 0; hi < len(haystack) && ni < len(needle); hi++ {
		if haystack[hi] == needle[ni] {
			ni++
		}
	}
	return ni == len(needle)
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

