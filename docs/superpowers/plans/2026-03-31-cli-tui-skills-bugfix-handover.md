# Handover: CLI, TUI & Skills Bugfix Plan

**Assigned issues:** M9, M8, M15, M17, A2, M14, L3, L8, A4, A5 (10 bugs across 10 tasks)
**Plan file:** `docs/superpowers/plans/2026-03-31-cli-tui-skills-bugfix.md`
**Branch:** Create a new worktree branch (e.g., `fix/cli-tui-skills-bugfix`)
**Worktree location:** `.worktrees/cli-tui-skills-bugfix`

---

## What You Are Taking Over

This is a bugfix implementation plan covering CLI UX improvements, TUI keyboard handling, dialog responsiveness, tool context propagation, and skill content. The issues you are responsible for are:

| Bug ID | Issue | Task |
|--------|-------|------|
| M9 | Onboard guard — `smolbot onboard` silently overwrites existing config | Task 1 |
| M8 | Explicit not-found error — bad `--config` path silently falls back to defaults | Task 2 |
| M15 | dialGateway UX — connection refused gives no hint to run `smolbot run` | Task 3 |
| M17 | Readline history nav — up/down arrows don't update terminal display | Task 4 |
| A2 | ESC key — no-op when editor is not focused, should clear selection | Task 5 |
| M14 | Dialog widths — hardcoded pixel widths break narrow terminals | Task 6 |
| L3 | Theme fallback — `subtleWash` uses hardcoded `#111111` on light themes | Task 7 |
| L8 | Tool output cap — single long lines bypass 10-line truncation | Task 8 |
| A4 | MCP ToolContext — session/channel info discarded, MCP tools can't access it | Task 9 |
| A5 | Skill content — 7 of 8 skill files are stubs with no real content | Task 10 |

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
   git worktree add .worktrees/cli-tui-skills-bugfix -b fix/cli-tui-skills-bugfix
   cd .worktrees/cli-tui-skills-bugfix
   go mod download
   go test ./... -count=1 2>&1 | tee /tmp/baseline-tests.txt
   ```
   Confirm all tests pass on the baseline before making any changes.

2. **Read the plan** — `docs/superpowers/plans/2026-03-31-cli-tui-skills-bugfix.md` in full. Understand the root cause of each issue, not just the fix.

3. **Verify line references** — the plan was written before the current state of the code. Lines may have shifted. Verify each referenced location before starting a task.

4. **Note the scope** — Task 6 (M14 — Dialog Widths) is the largest task, touching 9 files across `internal/components/dialog/` and `internal/tui/`. Tasks 1–5 are single-file fixes. Tasks 7–9 are targeted one-file changes. Task 10 writes 7 skill markdown files.

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

**Task 1 (M9 — Onboard Guard):** Simple one-file fix. The test captures `writeConfigFile` via a package-level variable, so verify the variable is still assignable in `onboard.go`. The test also creates the config file first, then runs `NewRootCmd` — confirm `NewRootCmd` exists and the test pattern matches the existing `onboard_test.go` style.

**Task 2 (M8 — Not-Found Error):** The fix removes the `errors.Is(err, os.ErrNotExist)` guard so ALL errors from `config.Load` on an explicit path are returned. This is correct behavior — if the user specifies a path, any error (not found, permission, corruption) should be surfaced.

**Task 3 (M15 — dialGateway UX):** The fix wraps `lastErr` with `fmt.Errorf` using `%w`. Verify that `lastErr` is indeed the connection error and not a different variable. The test uses port 1 which is guaranteed refused on normal systems.

**Task 4 (M17 — History Navigation):** The fix changes `navigateHistory` from void to returning `string`. This is a behavioral change that requires updating the key handler (which currently ignores the return). The test directly tests `navigateHistory` on a `bubbleteaReadline` struct — verify the struct fields (`history`, `histIdx`) are accessible and the method signature matches.

**Task 5 (A2 — ESC Key):** This is a TUI key handling fix. The test in `tui_keys_test.go` is a guard against regression — it won't fully verify the behavior without a mock for `ClearSelection`. Verify `ClearSelection` exists on `m.messages` before implementing.

**Task 6 (M14 — Dialog Widths):** This is the largest task — 9 files modified. The pattern is identical for all 7 dialog models: add `termWidth int` field, add `WithTerminalWidth(w int)` method, replace `Width(N)` with `Width(dialogWidth(m.termWidth, N))`. Apply methodically. The `SetTerminalWidth` interface method and wrapper implementations are straightforward but verbose.

**Task 7 (L3 — Theme Fallback):** The fix checks `theme.Current()` before falling back to `#111111`. Verify the `theme` package has a `Current()` method that returns a non-nil theme when one is active.

**Task 8 (L8 — Tool Output Byte Cap):** The byte cap check is added BEFORE the line count check, since a single very long line would bypass the line count. The test creates an 8KB string to exceed the 4KB cap.

**Task 9 (A4 — MCP ToolContext):** This adds two context helpers (`WithToolContext`, `ContextToolContext`) to `pkg/tool/tool.go`. The test uses a `contextCaptureClient` that captures the ctx passed to `Invoke`. The test also verifies that the captured ctx contains the original `ToolContext` values.

**Task 10 (A5 — Skill Content):** No test — skill content is verified by reading the files. Simply create/update the 7 `SKILL.md` files with the provided content.

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
- CLI commands: `cmd/smolbot/` (onboard.go, runtime.go, chat_readline.go)
- TUI: `internal/tui/` (tui.go, menu_dialog.go)
- Dialog components: `internal/components/dialog/` (sessions.go, models.go, commands.go, skills.go, mcps.go, providers.go, keybindings.go, common.go)
- Chat components: `internal/components/chat/` (message.go)
- MCP: `pkg/mcp/` (client.go)
- Tool core: `pkg/tool/` (tool.go)
- Skills: `skills/` directories

Tests use the standard `testing` package and `go test`. Run the full suite with `go test ./...`.

---

## A Note From Me

The plan is well-structured. The two most likely sources of friction are:

1. **Task 6 (M14 — Dialog Widths)** — This is the largest task with 9 files. Take your time and apply the pattern systematically. Don't skip the compile check at Step 6.3d before moving to Step 6.4.

2. **Task 9 (A4 — MCP ToolContext)** — The context key type needs to be unexported (`toolContextKey`) to avoid collisions. Make sure the `toolContextKey` struct is defined in `pkg/tool/tool.go` and used consistently.

The worktree `.worktrees/cli-tui-skills-bugfix` is available for your use.

Good luck.
