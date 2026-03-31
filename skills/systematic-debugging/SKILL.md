---
---
name: systematic-debugging
description: "Structured root cause analysis for bugs and unexpected behaviour. Use when encountering errors, test failures, or surprising results."
always: false
---
---

## Systematic Debugging

Work through bugs in numbered steps. Track each step explicitly so it is easy to revisit and revise.

### Step 1 — Reproduce the Failure

Get the exact error message, exact inputs, and exact environment. Never reproduce from memory. Copy the actual stack trace. Check whether the failure is deterministic or intermittent.

### Step 2 — Isolate the Failure

Binary-search through the call stack. Disable half the code; if the bug disappears, it was in the half you removed. Narrow the reproducer to the smallest possible input.

### Step 3 — Form and Test Hypotheses

State the hypothesis explicitly before testing it ("I think X is nil because Y"). Run exactly one experiment per hypothesis. Record the result even if it disproves you — disproof is progress.

### Step 4 — Read Error Messages Carefully

Go errors are wrapped; read from the innermost cause outward. JavaScript stack traces list callers top-to-bottom. Python tracebacks list callers bottom-to-top. Never skim the error message.

### Step 5 — Check Assumptions

List assumptions about the system before searching for a bug. Common wrong assumptions:
- "this function is always called"
- "this value is always set"
- "this goroutine is the only writer"
- "the test environment matches production"

### Step 6 — Check Recent Changes

When a bug is new, check `git log --oneline -20` and `git diff HEAD~1` before doing anything else. Most regressions live in the last commit.

### Step 7 — Do Not Jump to Solutions

Do not write code until the root cause is confirmed. Premature fixes mask the real problem and create two bugs.

### Step 8 — Verify the Fix

Re-run the exact reproducer. Also run the full test suite. Check that the fix does not break adjacent behaviour.

### Language-Specific Patterns

**Go:**
- nil pointer dereference — check every pointer before use
- goroutine leak — context not cancelled, channel never closed
- race condition — use `-race`
- silent error discard — `_ = err` is a red flag

**JS/TS:**
- `undefined is not a function` — method called on wrong type
- unhandled promise rejection
- mutation of shared state in async code
- stale closure in `useEffect`

**Python:**
- mutable default argument (`def f(x=[])`)
- import side-effects
- off-by-one in slice notation
- `==` vs `is` for None
