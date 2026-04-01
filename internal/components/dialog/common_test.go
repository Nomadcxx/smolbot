package dialog

import "testing"

func TestDialogWidth(t *testing.T) {
	tests := []struct {
		termWidth int
		preferred int
		want      int
	}{
		{termWidth: 0, preferred: 64, want: 64},
		{termWidth: 100, preferred: 64, want: 64},
		{termWidth: 60, preferred: 64, want: 56},
		{termWidth: 40, preferred: 72, want: 36},
		{termWidth: 64, preferred: 64, want: 60},
	}
	for _, tt := range tests {
		got := dialogWidth(tt.termWidth, tt.preferred)
		if got != tt.want {
			t.Errorf("dialogWidth(%d, %d) = %d, want %d", tt.termWidth, tt.preferred, got, tt.want)
		}
	}
}

func TestMatchesQuery(t *testing.T) {
	tests := []struct {
		query  string
		fields []string
		want   bool
	}{
		{"", []string{"anything"}, true},
		{"gpt", []string{"gpt-4o", "openai"}, true},
		{"gpt4", []string{"gpt-4o", "openai"}, true},      // fuzzy: g-p-t-4 present
		{"claude", []string{"claude-3-opus", "anthropic"}, true},
		{"xyz", []string{"gpt-4o", "openai"}, false},
		{"op", []string{"claude-3-opus", "anthropic"}, true}, // 'o','p' appear in haystack
		{"free", []string{"gemini-flash", "Google Gemini"}, false},
	}
	for _, tt := range tests {
		got := matchesQuery(tt.query, tt.fields...)
		if got != tt.want {
			t.Errorf("matchesQuery(%q, %v) = %v, want %v", tt.query, tt.fields, got, tt.want)
		}
	}
}

func TestFuzzyMatch(t *testing.T) {
	tests := []struct {
		needle, haystack string
		want             bool
	}{
		{"gpt", "gpt-4o openai", true},
		{"gpt4", "gpt-4o openai", true},
		{"abc", "aXbXc", true},
		{"abcd", "abc", false},
		{"", "anything", true},
		{"z", "abc", false},
	}
	for _, tt := range tests {
		got := fuzzyMatch(tt.needle, tt.haystack)
		if got != tt.want {
			t.Errorf("fuzzyMatch(%q, %q) = %v, want %v", tt.needle, tt.haystack, got, tt.want)
		}
	}
}
