# Handover: Remaining Audit Bugfix Plan (Plan 6)

**Assigned issues:** M1, M3, M4, M12, A6, M13, M16, M18, L1, L2, L5, L6, L7, A1, A7 (15 bugs across 14 tasks)
**Plan file:** `docs/superpowers/plans/2026-03-31-remaining-audit-bugfix.md`
**Branch:** Create a new worktree branch (e.g., `fix/remaining-audit-bugfix`)
**Worktree location:** `.worktrees/remaining-audit-bugfix`

---

## What You Are Taking Over

This is the final plan completing 100% coverage of the AUDIT_2026-03-30.md findings. The issues you are responsible for are:

| Bug ID | Issue | Task |
|--------|-------|------|
| M1 | ThinkingBlocks/ReasoningContent not cleared on session save | Task 1 |
| M3 | repairJSON doesn't handle unclosed braces/brackets | Task 2 |
| M4 | Unrecognized model silently falls back to OpenAI | Task 3 |
| M12 | MCPServerInfo.Tools count always 0 | Task 4 |
| A6 | MCP server status always "configured", not "connected" | Task 4 |
| M13 | Session preview never populated in sessions.list | Task 5 |
| M16 | Signal adapter doesn't validate signal-cli binary at Start | Task 6 |
| M18 | Exec timeout discards partial output | Task 7 |
| L1 | sessionLocks concurrency needs race-detector test | Task 8 |
| L2 | TUI discards JSON unmarshal errors silently | Task 9 |
| L5 | Signal login report left as nil causing potential panic | Task 10 |
| L6 | tx.Rollback needs nil-safe guard | Task 11 |
| L7 | Heartbeat decider error silently returned as nil | Task 12 |
| A1 | MiniMax OAuth token can expire mid-stream | Task 13 |
| A7 | run subcommand has duplicate persistent flags | Task 14 |

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
   git worktree add .worktrees/remaining-audit-bugfix -b fix/remaining-audit-bugfix
   cd .worktrees/remaining-audit-bugfix
   go mod download
   go test ./... -count=1 2>&1 | tee /tmp/baseline-tests.txt
   ```
   Confirm all tests pass on the baseline before making any changes.

2. **Read the plan** — `docs/superpowers/plans/2026-03-31-remaining-audit-bugfix.md` in full. Understand the root cause of each issue, not just the fix.

3. **Verify line references** — the plan was written before the current state of the code. Lines may have shifted. Verify each referenced location before starting a task.

4. **Note the complexity distribution** — Tasks 1, 2, 3, 5, 6, 7, 9, 10, 11, 12, 13, 14 are single-file targeted fixes. Task 4 (M12/A6) spans 4 files. Task 8 (L1) is a test-only addition. Task 14 (A7) requires creating a new test file.

### Per-Task Workflow

For each task, follow this sequence:

1. **Validate root cause** — Run the verification command from the Root Cause Validation section of the plan before writing any test code. Confirm the bug actually exists as described.

2. **Write the failing test first** — as specified in the plan. Do not implement the fix until the test fails with the expected error.

3. **Run the test** — confirm it fails with the expected symptom.

4. **Implement the fix** — exactly as described in the plan. If the plan's approach doesn't fit the actual code, stop and report the discrepancy.

5. **Run the test again** — confirm it passes.

6. **Run the full test suite for the affected package(s)** — confirm no regressions.

7. **Commit** — with a conventional commit message.

### Issue-Specific Notes

**Task 1 (M1 — Clear ThinkingBlocks):** Simple fix — two lines added to the `assistant` case in `normalizeMessagesForSave`. The test verifies both `ReasoningContent` and `ThinkingBlocks` are cleared.

**Task 2 (M3 — repairJSON unclosed braces):** The `closeUnclosed` helper uses a stack-based approach to track open `{` and `[`. It's inserted before the second `json.Valid` check so it has a chance to repair before giving up.

**Task 3 (M4 — Unrecognized model error):** This removes silent fallbacks in two places: `resolveProvider` and `ForModelWithCtx`. If any existing test relies on the silent OpenAI fallback, it will fail — update it to explicitly configure a provider or expect an error.

**Task 4 (M12 + A6 — MCP Tool Counts):** This is the most complex task. It adds `toolCounts map[string]int` to `Manager`, exposes `ToolCounts() map[string]int`, adds `MCPToolCounter` interface to `ServerDeps`, updates `mcps.list` to use real counts, and wires `MCPTools: mcpManager` in `runtime.go`. Take care to use `sync.RWMutex` for the counts map.

**Task 5 (M13 — Session Preview):** Adds `Preview` field to `Session` struct and a correlated SQL subquery in `ListSessions`. The subquery fetches the last non-empty user/assistant message truncated to 80 chars.

**Task 6 (M16 — Signal binary check):** Uses `exec.LookPath` before starting goroutines. The test sets `CLIPath` to a path that definitely doesn't exist. Note: if existing tests use a `commandRunner` stub, they may need adjustment if `cliPath()` resolves to a non-existent path.

**Task 7 (M18 — Exec partial output):** The fix appends partial output to the timeout error message. The test for this is a unit test of the message construction function, since timing-dependent tests are flaky.

**Task 8 (L1 — sessionLocks race test):** This is a test-only task — no production code changes. The existing `sessionLock` implementation in `memory.go` is already correct; this adds a `-race` test to document and guard the guarantee.

**Task 9 (L2 — JSON unmarshal errors):** There are ~10 `_ = json.Unmarshal` calls in the `EventMsg` handler. Replace each with `if err := json.Unmarshal(...); err != nil { slog.Debug(...) }`. Verify `log/slog` is imported.

**Task 10 (L5 — Signal login report nil):** Initialize `report` as a no-op lambda `func(channel.Status) error { return nil }` before checking `if out != nil`. This prevents panic if `LoginWithUpdates` calls `report` with a nil check bypass.

**Task 11 (L6 — Nil-safe Rollback):** Replace all `defer tx.Rollback()` with a nil-safe anonymous function. There may be multiple occurrences — check with `grep -n "defer tx.Rollback"`.

**Task 12 (L7 — Heartbeat decider error):** Changes `return nil` to `return fmt.Errorf("heartbeat decider: %w", err)`. Verify `fmt` is in imports.

**Task 13 (A1 — MiniMax proactive refresh):** Adds a pre-stream token expiry check. If `time.Until(tok.ExpiresAt) < 5*time.Minute`, call `RefreshToken` before starting. This is a compile-and-review task — full integration testing requires an HTTP mock server.

**Task 14 (A7 — Duplicate flags):** Creates a new test file `run_test.go`. The test uses `runCmd.Flags().Lookup(name)` to verify `--config`, `--workspace`, `--verbose` are NOT locally defined on the run subcommand (they're inherited from root).

---

## What To Report When You're Done

When you complete your assigned tasks, report:

- Which tasks you completed and their commit SHAs
- Any tasks you could not complete and why
- Any discrepancies you found between the plan and the actual code
- Any new issues you discovered that are related but outside scope

---

## Contacts and Context

This project is `smolbot` at `/home/nomadx/Documents/smolbot`. The codebase is Go with:
- Agent: `pkg/agent/` (loop.go, memory.go)
- Provider: `pkg/provider/` (sanitize.go, registry.go, minimax_oauth.go)
- MCP: `pkg/mcp/` (client.go)
- Gateway: `pkg/gateway/` (server.go)
- Session: `pkg/session/` (store.go)
- Channel/Signal: `pkg/channel/signal/` (adapter.go)
- Tool: `pkg/tool/` (exec.go, tool.go)
- Heartbeat: `pkg/heartbeat/` (service.go)
- TUI: `internal/tui/` (tui.go)
- CLI: `cmd/smolbot/` (run.go, channels_signal_login.go)

Tests use the standard `testing` package and `go test`. Run the full suite with `go test ./...`.

---

## A Note From Me

This is the final audit plan — completing it closes 100% of the AUDIT_2026-03-30.md findings. A few notes:

1. **Task 4 (M12/A6)** is the most complex — it spans 4 files and adds interface+implementation for tool counting. Use the compile check at Step 4.3 to catch any missing interface implementations before moving to wiring.

2. **Task 3 (M4)** may cause test failures if existing tests relied on the silent OpenAI fallback. Search the test files for any test that passes an unrecognized model and expects it to work.

3. **Task 9 (L2)** requires careful find-and-replace across ~10 occurrences — don't miss any.

4. **Task 13 (A1)** is marked compile-and-review since full integration testing of the proactive refresh requires a mock HTTP server. The code change is straightforward.

The worktree `.worktrees/remaining-audit-bugfix` is available for your use.

Good luck.
