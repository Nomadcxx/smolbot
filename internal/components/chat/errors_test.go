package chat

import (
	"strings"
	"testing"
)

func TestTruncateError(t *testing.T) {
	short := "short error"
	if got := TruncateError(short, 100); got != short {
		t.Errorf("expected unchanged short error, got %q", got)
	}

	long := strings.Repeat("x", 200)
	got := TruncateError(long, 100)
	if !strings.Contains(got, "[truncated]") {
		t.Errorf("expected truncation marker in %q", got)
	}
	// Result is 2*half + len(marker); allow generous bound.
	if len(got) > 150 {
		t.Errorf("expected shorter result, got length %d", len(got))
	}
}

func TestTruncateErrorLines(t *testing.T) {
	// 15 "line\n" → Split produces 16 tokens (trailing empty after last \n)
	lines := strings.Repeat("line\n", 15)
	got := TruncateErrorLines(lines, 10)
	gotLines := strings.Split(got, "\n")
	last := gotLines[len(gotLines)-1]
	if last != "... and 6 more lines" {
		t.Errorf("expected 'and 6 more lines' suffix, got %q", last)
	}

	short := "a\nb\nc"
	if TruncateErrorLines(short, 10) != short {
		t.Errorf("expected unchanged short output")
	}
}

func TestCategorizeError(t *testing.T) {
	cases := []struct {
		input    string
		wantCat  ErrorCategory
		wantHint bool
	}{
		{"CERT_HAS_EXPIRED", ErrorCategorySSL, true},
		{"HTTP 429 rate limit exceeded", ErrorCategoryRateLimit, true},
		{"401 unauthorized", ErrorCategoryAuth, true},
		{"connection refused", ErrorCategoryNetwork, true},
		{"some random error", ErrorCategoryUnknown, false},
	}
	for _, tc := range cases {
		cat, hint := CategorizeError(tc.input)
		if cat != tc.wantCat {
			t.Errorf("CategorizeError(%q) category = %q, want %q", tc.input, cat, tc.wantCat)
		}
		if tc.wantHint && hint == "" {
			t.Errorf("CategorizeError(%q) expected non-empty hint", tc.input)
		}
		if !tc.wantHint && hint != "" {
			t.Errorf("CategorizeError(%q) expected no hint, got %q", tc.input, hint)
		}
	}
}

func TestFormatValidationErrors(t *testing.T) {
	errs := []ValidationError{
		{Type: ValidationMissing, Field: "path"},
		{Type: ValidationUnexpected, Field: "debug"},
		{Type: ValidationTypeMismatch, Field: "limit", Message: "expected int"},
	}
	got := FormatValidationErrors(errs)
	if !strings.Contains(got, "Missing required:") {
		t.Error("expected 'Missing required:' section")
	}
	if !strings.Contains(got, "• path") {
		t.Error("expected path in missing section")
	}
	if !strings.Contains(got, "Unexpected:") {
		t.Error("expected 'Unexpected:' section")
	}
	if !strings.Contains(got, "Type errors:") {
		t.Error("expected 'Type errors:' section")
	}
	if !strings.Contains(got, "expected int") {
		t.Error("expected type mismatch message")
	}
}

func TestRetryStateFormat(t *testing.T) {
	r := RetryState{Attempt: 2, MaxAttempt: 3}
	got := r.Format()
	if got != "attempt 2/3" {
		t.Errorf("got %q, want 'attempt 2/3'", got)
	}
}
