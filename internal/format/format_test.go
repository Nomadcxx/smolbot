package format

import (
	"testing"
	"time"
)

func TestFormatDuration(t *testing.T) {
	cases := []struct {
		d    time.Duration
		want string
	}{
		{0, "0.0s"},
		{300 * time.Millisecond, "0.3s"},
		{999 * time.Millisecond, "1.0s"},
		{1 * time.Second, "1s"},
		{45 * time.Second, "45s"},
		{90 * time.Second, "1m 30s"},
		{1*time.Hour + 5*time.Minute, "1h 5m"},
		{1*time.Hour + 5*time.Minute + 30*time.Second, "1h 5m 30s"},
		{25 * time.Hour, "1d 1h"},
		{48 * time.Hour, "2d"},
	}
	for _, tc := range cases {
		got := FormatDuration(tc.d)
		if got != tc.want {
			t.Errorf("FormatDuration(%v) = %q, want %q", tc.d, got, tc.want)
		}
	}
}

func TestFormatFileSize(t *testing.T) {
	cases := []struct {
		bytes int64
		want  string
	}{
		{0, "0 bytes"},
		{512, "512 bytes"},
		{1023, "1023 bytes"},
		{1024, "1KB"},
		{1536, "1.5KB"},
		{1024 * 1024, "1MB"},
		{int64(1.5 * 1024 * 1024), "1.5MB"},
		{1024 * 1024 * 1024, "1GB"},
	}
	for _, tc := range cases {
		got := FormatFileSize(tc.bytes)
		if got != tc.want {
			t.Errorf("FormatFileSize(%d) = %q, want %q", tc.bytes, got, tc.want)
		}
	}
}

func TestFormatTokens(t *testing.T) {
	cases := []struct {
		count int
		want  string
	}{
		{0, "0"},
		{42, "42"},
		{999, "999"},
		{1000, "1K"},
		{1200, "1.2K"},
		{1500, "1.5K"},
		{10000, "10K"},
		{1_000_000, "1M"},
		{3_500_000, "3.5M"},
	}
	for _, tc := range cases {
		got := FormatTokens(tc.count)
		if got != tc.want {
			t.Errorf("FormatTokens(%d) = %q, want %q", tc.count, got, tc.want)
		}
	}
}
