package chat

import (
	"fmt"
	"strings"
)

// --- Phase 11: Error Handling UI ---

const (
	// MaxAPIErrorChars is the maximum characters shown for API error messages.
	MaxAPIErrorChars = 1000

	// MaxToolErrorLines is the maximum lines shown for tool error output.
	MaxToolErrorLines = 10

	// MaxErrorTotalChars is the maximum characters before middle-truncation kicks in.
	MaxErrorTotalChars = 10000

	// MaxStackFrames is the maximum stack trace frames displayed.
	MaxStackFrames = 5
)

// TruncateError truncates long error text with a middle ellipsis.
// Text shorter than maxChars is returned unchanged.
func TruncateError(text string, maxChars int) string {
	if len(text) <= maxChars {
		return text
	}
	half := maxChars / 2
	return text[:half] + "\n... [truncated] ...\n" + text[len(text)-half:]
}

// TruncateErrorLines limits error output to maxLines lines.
// Excess lines are replaced with a count indicator.
func TruncateErrorLines(text string, maxLines int) string {
	lines := strings.Split(text, "\n")
	if len(lines) <= maxLines {
		return text
	}
	remaining := len(lines) - maxLines
	return strings.Join(lines[:maxLines], "\n") + fmt.Sprintf("\n... and %d more lines", remaining)
}

// ShortErrorStack returns only the top maxFrames frames of a stack trace.
func ShortErrorStack(stack string, maxFrames int) string {
	lines := strings.Split(stack, "\n")
	maxLines := maxFrames * 2
	if len(lines) <= maxLines {
		return stack
	}
	return strings.Join(lines[:maxLines], "\n") + "\n... (stack truncated)"
}

// RetryState tracks retry attempts for display in error messages.
type RetryState struct {
	Attempt    int
	MaxAttempt int
	Reason     string
}

// Format returns a compact retry description: "attempt 2/3" or "attempt 2/3 · retrying".
func (r RetryState) Format() string {
	if r.MaxAttempt <= 0 {
		return fmt.Sprintf("attempt %d", r.Attempt)
	}
	return fmt.Sprintf("attempt %d/%d", r.Attempt, r.MaxAttempt)
}

// ErrorCategory identifies the class of error for targeted hints.
type ErrorCategory string

const (
	ErrorCategoryNetwork    ErrorCategory = "network"
	ErrorCategoryAuth       ErrorCategory = "auth"
	ErrorCategoryRateLimit  ErrorCategory = "rate_limit"
	ErrorCategoryValidation ErrorCategory = "validation"
	ErrorCategorySSL        ErrorCategory = "ssl"
	ErrorCategoryUnknown    ErrorCategory = "unknown"
)

var sslErrorCodes = []string{
	"CERT_HAS_EXPIRED", "UNABLE_TO_VERIFY_LEAF_SIGNATURE",
	"SELF_SIGNED_CERT_IN_CHAIN", "DEPTH_ZERO_SELF_SIGNED_CERT",
	"UNABLE_TO_GET_ISSUER_CERT", "UNABLE_TO_GET_CRL",
	"CERT_SIGNATURE_FAILURE", "CRL_SIGNATURE_FAILURE",
	"CERT_NOT_YET_VALID", "CRL_NOT_YET_VALID", "CERT_HAS_EXPIRED",
	"ERROR_IN_CERT_NOT_BEFORE_FIELD", "ERROR_IN_CERT_NOT_AFTER_FIELD",
	"DEPTH_ZERO_SELF_SIGNED_CERT", "SELF_SIGNED_CERT_IN_CHAIN",
	"UNABLE_TO_GET_ISSUER_CERT_LOCALLY", "UNABLE_TO_VERIFY_LEAF_SIGNATURE",
	"CERT_CHAIN_TOO_LONG", "CERT_REVOKED", "INVALID_CA",
	"PATH_LENGTH_EXCEEDED", "INVALID_PURPOSE", "CERT_UNTRUSTED",
	"CERT_REJECTED", "HOSTNAME_MISMATCH", "certificate",
}

// CategorizeError determines the error type and returns a user-facing hint.
func CategorizeError(errStr string) (ErrorCategory, string) {
	lower := strings.ToLower(errStr)

	for _, code := range sslErrorCodes {
		if strings.Contains(errStr, code) || strings.Contains(lower, strings.ToLower(code)) {
			return ErrorCategorySSL, "Try setting SSL_CERT_FILE or check proxy settings"
		}
	}

	if strings.Contains(lower, "rate limit") || strings.Contains(errStr, "429") {
		return ErrorCategoryRateLimit, "API rate limit reached — will retry automatically"
	}

	if strings.Contains(errStr, "401") || strings.Contains(lower, "invalid_api_key") ||
		strings.Contains(lower, "unauthorized") {
		return ErrorCategoryAuth, "Check your API key configuration"
	}

	if strings.Contains(lower, "connection refused") || strings.Contains(lower, "no such host") ||
		strings.Contains(lower, "network") || strings.Contains(lower, "timeout") {
		return ErrorCategoryNetwork, "Check your network connection"
	}

	return ErrorCategoryUnknown, ""
}

// --- Validation Error Formatting ---

// ValidationErrorType identifies what kind of validation failed.
type ValidationErrorType string

const (
	ValidationMissing     ValidationErrorType = "missing"
	ValidationUnexpected  ValidationErrorType = "unexpected"
	ValidationTypeMismatch ValidationErrorType = "type_mismatch"
)

// ValidationError represents a single parameter validation issue.
type ValidationError struct {
	Type    ValidationErrorType
	Field   string
	Message string
}

// FormatValidationErrors renders a list of validation errors clearly grouped by type.
func FormatValidationErrors(errors []ValidationError) string {
	var b strings.Builder

	missing := filterValidationByType(errors, ValidationMissing)
	unexpected := filterValidationByType(errors, ValidationUnexpected)
	typeMismatch := filterValidationByType(errors, ValidationTypeMismatch)

	if len(missing) > 0 {
		b.WriteString("Missing required:\n")
		for _, e := range missing {
			b.WriteString(fmt.Sprintf("  • %s\n", e.Field))
		}
	}
	if len(unexpected) > 0 {
		b.WriteString("Unexpected:\n")
		for _, e := range unexpected {
			b.WriteString(fmt.Sprintf("  • %s\n", e.Field))
		}
	}
	if len(typeMismatch) > 0 {
		b.WriteString("Type errors:\n")
		for _, e := range typeMismatch {
			b.WriteString(fmt.Sprintf("  • %s: %s\n", e.Field, e.Message))
		}
	}

	return b.String()
}

func filterValidationByType(errors []ValidationError, t ValidationErrorType) []ValidationError {
	var out []ValidationError
	for _, e := range errors {
		if e.Type == t {
			out = append(out, e)
		}
	}
	return out
}

// applyErrorTruncation applies standard truncation rules to tool error output.
func applyErrorTruncation(output string) string {
	output = TruncateErrorLines(output, MaxToolErrorLines)
	if len(output) > MaxAPIErrorChars {
		output = TruncateError(output, MaxAPIErrorChars)
	}
	return output
}
