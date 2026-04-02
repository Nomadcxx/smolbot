package format

import (
	"fmt"
	"strings"
	"time"
)

// FormatDuration converts a duration to a compact human-readable string.
// Sub-second: "0.3s". Seconds: "45s". Longer: "2m 30s", "1h 5m", "2d 5h".
func FormatDuration(d time.Duration) string {
	ms := d.Milliseconds()
	if ms < 0 {
		ms = 0
	}

	if ms < 1000 {
		return fmt.Sprintf("%.1fs", float64(ms)/1000)
	}

	totalSec := ms / 1000
	days := totalSec / 86400
	remaining := totalSec % 86400
	hours := remaining / 3600
	remaining = remaining % 3600
	minutes := remaining / 60
	seconds := remaining % 60

	var parts []string
	if days > 0 {
		parts = append(parts, fmt.Sprintf("%dd", days))
	}
	if hours > 0 {
		parts = append(parts, fmt.Sprintf("%dh", hours))
	}
	if minutes > 0 {
		parts = append(parts, fmt.Sprintf("%dm", minutes))
	}
	if seconds > 0 || len(parts) == 0 {
		parts = append(parts, fmt.Sprintf("%ds", seconds))
	}

	return strings.Join(parts, " ")
}

// FormatFileSize converts byte count to a compact human-readable size string.
// Examples: "512 bytes", "1.5KB", "2MB", "1.2GB".
func FormatFileSize(bytes int64) string {
	const (
		kb = 1024
		mb = kb * 1024
		gb = mb * 1024
	)

	if bytes < kb {
		return fmt.Sprintf("%d bytes", bytes)
	}

	var value float64
	var unit string
	switch {
	case bytes >= gb:
		value = float64(bytes) / float64(gb)
		unit = "GB"
	case bytes >= mb:
		value = float64(bytes) / float64(mb)
		unit = "MB"
	default:
		value = float64(bytes) / float64(kb)
		unit = "KB"
	}

	formatted := fmt.Sprintf("%.1f", value)
	formatted = strings.TrimSuffix(formatted, ".0")
	return formatted + unit
}

// FormatTokens formats a token count with K/M suffix for compact display.
// Examples: "42", "1.2K", "3.5M".
func FormatTokens(count int) string {
	switch {
	case count >= 1_000_000:
		return trimTrailingZero(fmt.Sprintf("%.1fM", float64(count)/1_000_000))
	case count >= 1_000:
		return trimTrailingZero(fmt.Sprintf("%.1fK", float64(count)/1_000))
	default:
		return fmt.Sprintf("%d", count)
	}
}

func trimTrailingZero(s string) string {
	// "1.0K" → "1K", "10.0K" → "10K", "1.5K" → "1.5K"
	if len(s) >= 4 && s[len(s)-3] == '.' && s[len(s)-2] == '0' {
		return s[:len(s)-3] + s[len(s)-1:]
	}
	return s
}
