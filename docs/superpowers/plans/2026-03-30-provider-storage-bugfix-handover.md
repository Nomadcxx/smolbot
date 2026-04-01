# Handover: Provider & Storage Bugfix Plan

**Assigned issues:** C8, C9, M2, C13, C16, H4, H5, K3, C14, M10 (10 bugs across 8 tasks)
**Plan file:** `docs/superpowers/plans/2026-03-30-provider-storage-bugfix.md`
**Branch:** Create a new worktree branch (e.g., `fix/provider-storage-bugfix`)
**Worktree location:** `.worktrees/provider-storage-bugfix`

---

## What You Are Taking Over

This is a continuation of bugfix work. The plan at `docs/superpowers/plans/2026-03-30-provider-storage-bugfix.md` has been reviewed and the key references verified. The issues you are responsible for are:

| Bug ID | Issue | Task |
|--------|-------|------|
| C8/C9 | Provider prefix stripping — private `stripProviderPrefix` only in minimax_oauth.go, not exported to other providers | Task 1 |
| M2 | Azure streaming usage loss — empty-choices chunk skipped without capturing usage | Task 2 |
| C13 | OAuth store silent failure — `NewOAuthTokenStore` returns without error even if directory creation fails | Task 3 |
| C16 | No model validation — daemon starts with empty model, first request fails cryptically | Task 4 |
| H4 | Cron no fsync — `persist()` uses `os.WriteFile` without sync, data loss on crash | Task 5 |
| H5 | web_search uncapped output — no size limit on search results | Task 6 |
| K3 | User skills wrong FS — user skills dir checked against embedded FS instead of real filesystem | Task 7 |
| C14 | HasResource nil panic — `fs.Stat(nil, path)` panics | Task 7 |
| M10 | Tokenizer ignores model — hardcoded `cl100k_base` used for all models | Task 8 |

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
   git worktree add .worktrees/provider-storage-bugfix -b fix/provider-storage-bugfix
   cd .worktrees/provider-storage-bugfix
   go mod download
   go test ./... -count=1 2>&1 | tee /tmp/baseline-tests.txt
   ```
   Confirm all tests pass on the baseline before making any changes.

2. **Read the plan** — `docs/superpowers/plans/2026-03-30-provider-storage-bugfix.md` in full. Understand the root cause of each issue, not just the fix.

3. **Verify line references** — the plan was written before the current state of the code. Lines may have shifted. Verify each referenced location before starting a task.

4. **Note the conflict risk** — Task 1 and Task 2 both modify `pkg/provider/azure.go`. If you're working sequentially (recommended), this is not an issue. If doing parallel subagents, coordinate carefully.

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

**Task 1 (C8/C9 — Provider prefix stripping):** The private `stripProviderPrefix` is confirmed at `minimax_oauth.go:383`. The plan adds an exported `StripProviderPrefix` to `sanitize.go`. The key files to modify are: `sanitize.go` (add export), `minimax_oauth.go` (remove private duplicate), `anthropic.go` (add call in `buildRequest`), `openai.go` (add call in `buildWireRequest`), `azure.go` (add call at start of `Chat` and `ChatStream`). The plan provides comprehensive test cases covering all provider prefix formats.

**Task 2 (M2 — Azure streaming usage loss):** This modifies `azure.go` (same file as Task 1). The two bugs are: (1) empty-choices chunk is skipped without capturing usage, (2) `azureRequest` lacks `StreamOptions` field. The plan provides streaming tests that require a real HTTP server. Follow the plan exactly for the test setup.

**Task 3 (C13 — OAuth store silent failure):** This changes the exported `NewOAuthTokenStore` signature from returning one value to returning `(OAuthTokenStore, error)`. This is a breaking API change — all call sites must be updated. The plan identifies 5 call sites in `oauth_store_test.go` plus one in `runtime.go`. The test creates a blocking file path to force `MkdirAll` to fail.

**Task 4 (C16 — Model validation):** Simple one-file fix in `runtime.go`. Adds a check in `buildRuntime` that returns an error if `cfg.Agents.Defaults.Model` is empty.

**Task 5 (H4 — Cron fsync):** Replaces `os.WriteFile` with explicit `os.OpenFile` + write + `fsync` + `Close` + `os.Rename`. The plan provides a test that verifies no `.tmp` file is left after successful persist.

**Task 6 (H5 — web_search output cap):** Adds `maxSearchOutputBytes = 32 * 1024` constant and checks in the result-building loop. The plan's test creates 10 results each with 10KB snippets to verify the cap works.

**Task 7 (K3, C14 — User skills and HasResource):** Both bugs are in `registry.go` and both are fixed with `os.Stat` instead of `fs.Stat`. K3 is at line 45 (user skills dir check), C14 is at line 135 (resource path check). The plan provides separate tests for each.

**Task 8 (M10 — Tokenizer model awareness):** Adds `NewForModel(model string)` constructor and model-aware encoding selection. GPT-family models use `cl100k_base`; others use a fallback estimator. The plan also updates `runtime.go` to use `NewForModel(cfg.Agents.Defaults.Model)`.

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
- Provider implementations: `pkg/provider/` (anthropic.go, azure.go, openai.go, minimax_oauth.go, etc.)
- Config and storage: `pkg/config/`
- Cron service: `pkg/cron/`
- Tool registry: `pkg/tool/`
- Skills: `pkg/skill/`
- Tokenizer: `pkg/tokenizer/`
- CLI runtime: `cmd/smolbot/`

Tests use the standard `testing` package and `go test`. Run the full suite with `go test ./...`.

---

## A Note From Me

The plan is well-structured and the root cause validation commands will help you verify each bug before implementing. The two most likely sources of friction are:

1. **Azure streaming tests (Task 2)** — the test setup uses `httptest.Server` with SSE streaming. The plan's test is comprehensive but complex. Take care to get the chunk ordering and `include_usage` flag correct.

2. **OAuth breaking change (Task 3)** — changing an exported function signature requires updating all call sites. The plan identifies them but verify you haven't missed any with a grep before committing.

The worktree `.worktrees/provider-storage-bugfix` is available for your use.

Good luck.
