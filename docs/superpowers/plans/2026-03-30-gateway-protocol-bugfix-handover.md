# Handover: Gateway & Protocol Bugfixes

**Assigned issues:** C10, C12, H3, K2, K4 (5 issues)
**Plan file:** `docs/superpowers/plans/2026-03-30-gateway-protocol-bugfix.md`
**Branch:** Create a new worktree branch (e.g., `fix/gateway-protocol-bugfix`)
**Worktree location:** `.worktrees/gateway-protocol-bugfix`

---

## What You Are Taking Over

This is a continuation of bugfix work. The plan at `docs/superpowers/plans/2026-03-30-gateway-protocol-bugfix.md` has already been reviewed and verified as accurate. The issues you are responsible for are:

1. **C10** — `sessions.reset` sends `"key"` param instead of `"session"` (messages.go:57)
2. **C12** — Already fixed in a prior session; `BroadcastEvent` calls exist at runtime.go:1133-1195
3. **H3** — `ToolDonePayload` missing `DeliveredToRequestTarget` bool field (protocol.go:96-101)
4. **K2** — `Skills` and `Cron` not wired to `gateway.NewServer()` (runtime.go:746)
5. **K4** — `statusReport` struct has wrong JSON tags and wrong types (runtime.go:47)

**My note on C12:** The self-review in the plan says C12 is already fixed. Verify this independently before treating it as done.

---

## Professional Standards I Hold Myself To

**Be thorough, not fast.** Speed without accuracy creates rework. If something in the plan seems wrong or the codebase doesn't match the plan's line references, investigate before implementing. It is always better to report a discrepancy and ask than to silently do the wrong thing.

**Verify before claiming.** Every test you write should fail before the fix and pass after. If a test doesn't fail, the bug it was meant to catch doesn't exist — figure out why before proceeding. Every commit should have passing tests.

**Keep scope tight.** The plan describes specific bugs with specific fixes. Do not refactor adjacent code, rename things for style, or add functionality not in the plan. If you find a related bug that isn't in the plan, mention it in your report but don't fix it unless explicitly asked.

**Communicate clearly.** If you encounter something unexpected — a referenced line doesn't exist, a function signature is different, a test helper doesn't exist — stop and report it. You will never be penalized for asking. You will be questioned if you silently do something different from what was planned.

**Own your commits.** Each task should be its own commit with a clear message following conventional commit format (`fix(scope): description`). Review what you've changed before committing.

---

## How To Approach This Work

### Before You Start

1. **Set up a fresh worktree** — do not work on `main` or an existing branch:
   ```bash
   cd /home/nomadx/Documents/smolbot
   git worktree add .worktrees/gateway-protocol-bugfix -b fix/gateway-protocol-bugfix
   cd .worktrees/gateway-protocol-bugfix
   go mod download
   go test ./internal/client/ ./cmd/smolbot/ ./pkg/gateway/ -count=1 2>&1 | tail -10
   ```
   Confirm all tests pass on the baseline before making any changes.

2. **Read the plan** — `docs/superpowers/plans/2026-03-30-gateway-protocol-bugfix.md` in full. Understand the root cause of each issue, not just the fix.

3. **Verify line references** — the plan was written before the current state of the code. Lines may have shifted. Verify each referenced location before starting a task.

### Per-Task Workflow

For each of your 5 issues, follow this sequence:

1. **Write the failing test first** — as specified in the plan. Do not implement the fix until the test fails with the expected error.

2. **Run the test** — confirm it fails with the expected symptom (wrong param name, missing struct field, empty list, wrong type, etc.).

3. **Implement the fix** — exactly as described in the plan. If the plan's approach doesn't fit the actual code, stop and report the discrepancy.

4. **Run the test again** — confirm it passes.

5. **Run the full test suite for the affected package(s)** — confirm no regressions.

6. **Commit** — with a conventional commit message.

### Issue-Specific Notes

**C10 (sessions.reset param):** Already confirmed at `internal/client/messages.go:57`. The bug is `map[string]string{"key": key}` should be `map[string]string{"session": key}`. The plan provides a complete integration test that reads from a WebSocket — follow it exactly.

**H3 (ToolDonePayload):** The plan specifies adding `DeliveredToRequestTarget bool `json:"deliveredToRequestTarget,omitempty"`` to the struct at `internal/client/protocol.go:96-101`. Note the `omitempty` tag — when false, the field should not appear in JSON output. The plan includes two tests: one to verify decoding works, one to verify `omitempty` behavior.

**K2 (Skills/Cron wiring):** At `cmd/smolbot/runtime.go:746`, `gateway.NewServer()` is called without `Skills:` or `Cron:` fields. Both `skills` and `cronService` are already in scope (variables at lines 618 and 740). The test in the plan uses `connectGatewayClient` from `runtime_model_test.go` — verify this helper exists before writing the test.

**K4 (statusReport):** This is the most complex fix — three related bugs in the `statusReport` struct and its consumers. The plan tells you exactly what to change and provides updated test assertions. The struct and two functions (`formatStatus`, `fetchChannelStatusesImpl`) all need changes. Read carefully and implement in order.

---

## What To Report When You're Done

When you complete your assigned issues, report:

- Which issues you completed and their commit SHAs
- Any issues you could not complete and why
- Any discrepancies you found between the plan and the actual code
- Any new issues you discovered that are related but outside scope

---

## Contacts and Context

This project is `smolbot` at `/home/nomadx/Documents/smolbot`. The codebase is Go with:
- Client protocol: `internal/client/`
- Gateway: `pkg/gateway/`
- CLI/TUI: `cmd/smolbot/`
- Agent loop: `pkg/agent/`

Tests use the standard `testing` package and `go test`. Run the full suite with `go test ./...`.

---

## A Note From Me

The plan for these bugs was verified accurate at time of review — the bugs are real and the fixes are well-specified. The issues are isolated and surgical. The main risk is line-number drift between when the plan was written and when you read the files. Always verify references before implementing.

The worktree `.worktrees/gateway-protocol-bugfix` is available for your use.

Good luck.
