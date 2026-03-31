---
---
name: code-review
description: "Review changed code for correctness, logic bugs, edge cases, security issues, and clarity. Use when asked to review a diff, PR, or specific file."
always: false
---
---

## Code Review

Review changed code systematically. Always read the PR description first.

### Review Order

Review in this order: correctness first, then logic bugs and edge cases, then security, then naming and clarity, then style. Never lead with style.

### Correctness

Does the code do what the PR description says? Are error return values checked? Are resources (files, connections, goroutines) cleaned up? Are there off-by-one errors?

### Logic Bugs and Edge Cases

Test these cases mentally:
- Empty input, nil/null, zero values
- Negative numbers, very large inputs
- Concurrent access, timeout expiry, unexpected ordering

Ask: what happens if this function is called twice? What happens if the network drops here?

### Security Issues

Flag these immediately:
- Injection (SQL, shell, HTML)
- Secrets hardcoded or logged
- Authentication bypass, missing authorisation check
- Unbounded resource consumption (no limit on loop or allocation)
- Path traversal

### Naming and Clarity

Can you understand the intent from the name alone? Are variable names a single letter where a word would help? Are function names verbs? Are boolean names positive (avoid `notDisabled`)?

### Substance vs Style

Style (flag only if project convention violated): formatting, whitespace, import ordering.

Substance (always flag): logic, correctness, security.

### Flag vs Suggest vs Praise

- **Flag (must fix):** correctness bugs, security issues, crashes, data loss
- **Suggest (nice to fix):** clarity improvements, alternative approaches
- **Praise (no action needed):** clever solutions, good test coverage, well-chosen names

### Matching Intent

The most important question is: does this change do what was intended? If no, flag the mismatch first.

### Go-Specific Points

- Error wrapping: `fmt.Errorf("...: %w", err)`
- Deferred close on error path
- Unexported fields in exported structs
- Goroutine cleanup on context cancellation
