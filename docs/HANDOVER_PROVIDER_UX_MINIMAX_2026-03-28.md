# Comprehensive Handover: Provider UX And MiniMax

This document is intended to let a new agent continue the `provider-ux-minimax` project with no prior knowledge of this repo and still execute reliably, even if that next model is weaker than GPT-5.4 High.

Treat this document as the operational source of truth for this feature branch.

## Primary Objective

Continue implementing the plan in:

- [2026-03-28-provider-ux-and-minimax.md](/home/nomadx/Documents/smolbot/docs/plans/2026-03-28-provider-ux-and-minimax.md)

Feature goal:

- fix broken model/provider switching
- make the F1 provider/model surfaces genuinely usable
- add first-class MiniMax support
- leave OAuth/OpenAI-via-Codex as a design-only follow-up, not an opportunistic implementation

## Current Status

As of this handover:

- Task 1: complete
- Task 2: complete
- Task 3: complete
- Task 4: not started
- Task 5: not started
- Task 6: not started
- Task 7: not started

Current worktree status:

- clean

Current branch:

- `feat/provider-ux-minimax`

Current worktree:

- [`.worktrees/provider-ux-minimax`](/home/nomadx/Documents/smolbot/.worktrees/provider-ux-minimax)

Do not continue this work in the root repo checkout unless explicitly told to.

## First Commands The Next Agent Should Run

Run these immediately before changing anything:

```bash
git -C /home/nomadx/Documents/smolbot/.worktrees/provider-ux-minimax status --short
git -C /home/nomadx/Documents/smolbot/.worktrees/provider-ux-minimax log --oneline -10
sed -n '1,260p' /home/nomadx/Documents/smolbot/docs/plans/2026-03-28-provider-ux-and-minimax.md
```

If the worktree is no longer clean, inspect before proceeding.

## Current Git History

Most relevant commits in the worktree:

- `24a571e feat(tui): upgrade model picker workflow`
- `a8f6937 feat(provider): expand model discovery across providers`
- `6a988f6 fix(provider): tighten discovery fallback rows`
- `acaecce feat(provider): expand model discovery across providers`
- `67a333c fix(provider): stabilize model switching contract`
- `e874150 fix(provider): unify models.set request contract`

Interpretation:

- `67a333c` is the real Task 1 checkpoint
- `a8f6937` is the finalized Task 2 checkpoint after multiple review-fix passes
- `24a571e` is the Task 3 checkpoint from the last worker

## Critical Repo Hygiene Notes

1. Use the worktree, not root
   - Use [/home/nomadx/Documents/smolbot/.worktrees/provider-ux-minimax](/home/nomadx/Documents/smolbot/.worktrees/provider-ux-minimax)
   - Root repo has unrelated history and prior mistakes

2. Do not “clean up” root `main`
   - An earlier worker accidentally committed related work to root `main`
   - Ignore that for this feature
   - The feature branch/worktree is the source of truth

3. Review against the worktree path only
   - A prior review agent accidentally reviewed `/home/nomadx/Documents/smolbot` instead of the worktree
   - That produced stale findings

## Execution Workflow To Continue

Follow the plan’s workflow exactly:

1. Implement one task slice only.
2. Run the focused verification for that task.
3. Run spec/completeness review for that task.
4. Run code-quality review for that task.
5. Fix findings.
6. Create a checkpoint commit.
7. Only then move to the next task.

Do not push or merge until the full final gate is green.

## What Is Already Done

## Task 1 Complete: Fix The Existing Model-Switching Contract

Task 1 status:

- complete
- verified
- spec review green
- code-quality review green
- checkpoint commit: `67a333c`

What Task 1 fixed:

- `models.set` uses one canonical request field:
  - `model`
- gateway response is canonical:
  - `current`
  - `previous`
- legacy `id` request payload is rejected
- gateway trims model IDs before callback/persistence
- gateway does not mutate config before `SetModelCallback` succeeds
- runtime provider switching now actually changes the provider backend, not just the visible model string
- heartbeat model propagation is fixed
- usage and sanitize logic snapshot provider identity per run/model

Important files:

- [internal/client/messages.go](/home/nomadx/Documents/smolbot/.worktrees/provider-ux-minimax/internal/client/messages.go)
- [internal/client/messages_test.go](/home/nomadx/Documents/smolbot/.worktrees/provider-ux-minimax/internal/client/messages_test.go)
- [internal/client/protocol.go](/home/nomadx/Documents/smolbot/.worktrees/provider-ux-minimax/internal/client/protocol.go)
- [pkg/gateway/server.go](/home/nomadx/Documents/smolbot/.worktrees/provider-ux-minimax/pkg/gateway/server.go)
- [pkg/gateway/server_test.go](/home/nomadx/Documents/smolbot/.worktrees/provider-ux-minimax/pkg/gateway/server_test.go)
- [cmd/smolbot/runtime.go](/home/nomadx/Documents/smolbot/.worktrees/provider-ux-minimax/cmd/smolbot/runtime.go)
- [cmd/smolbot/runtime_model_test.go](/home/nomadx/Documents/smolbot/.worktrees/provider-ux-minimax/cmd/smolbot/runtime_model_test.go)
- [pkg/provider/registry.go](/home/nomadx/Documents/smolbot/.worktrees/provider-ux-minimax/pkg/provider/registry.go)
- [pkg/provider/registry_test.go](/home/nomadx/Documents/smolbot/.worktrees/provider-ux-minimax/pkg/provider/registry_test.go)
- [pkg/agent/loop.go](/home/nomadx/Documents/smolbot/.worktrees/provider-ux-minimax/pkg/agent/loop.go)
- [pkg/agent/evaluator.go](/home/nomadx/Documents/smolbot/.worktrees/provider-ux-minimax/pkg/agent/evaluator.go)
- [pkg/agent/evaluator_test.go](/home/nomadx/Documents/smolbot/.worktrees/provider-ux-minimax/pkg/agent/evaluator_test.go)

Task 1 verification commands that were used:

```bash
go test ./internal/client ./pkg/gateway ./cmd/smolbot ./internal/tui -run 'Test.*Model'
go test ./pkg/agent ./pkg/provider ./pkg/gateway ./cmd/smolbot -run 'Test.*Model|TestRegistry.*'
```

## Task 2 Complete: Expand Provider And Model Discovery

Task 2 status:

- complete
- verified
- spec review green
- code-quality review green
- checkpoint commit: `a8f6937`

What Task 2 added:

- extracted generic provider/model discovery to:
  - [pkg/provider/discovery.go](/home/nomadx/Documents/smolbot/.worktrees/provider-ux-minimax/pkg/provider/discovery.go)
- kept Ollama live discovery in:
  - [pkg/provider/ollama_discovery.go](/home/nomadx/Documents/smolbot/.worktrees/provider-ux-minimax/pkg/provider/ollama_discovery.go)
- `models.list` now returns richer rows instead of “current model only”
- model rows now carry:
  - `id`
  - `name`
  - `provider`
  - `description`
  - `source`
  - `capability`
  - `selectable`

Task 2 contract decisions that must be preserved:

1. Config-backed provider rows are informational rows
   - `Source == "config"`
   - `Selectable == false`

2. Live and fallback/current rows are selectable
   - live Ollama rows: `Selectable == true`
   - fallback/current rows: `Selectable == true`

3. Compatibility rule in the dialog:
   - rows are treated as info-only only when `Source == "config"`, unless `Selectable == true`
   - this preserves compatibility with older payloads that omit `selectable`

4. Ollama fallback behavior:
   - do not fabricate bogus IDs like `ollama/gpt-5`
   - if Ollama is configured but unavailable while another provider is active, fallback row stays provider-backed, not cross-provider-fabricated

5. Azure note:
   - legacy `providers.azure` handling was adjusted to stay consistent with runtime resolution
   - be careful not to reintroduce mismatched discovery IDs vs runtime provider resolution

Important files:

- [pkg/provider/discovery.go](/home/nomadx/Documents/smolbot/.worktrees/provider-ux-minimax/pkg/provider/discovery.go)
- [pkg/provider/discovery_test.go](/home/nomadx/Documents/smolbot/.worktrees/provider-ux-minimax/pkg/provider/discovery_test.go)
- [pkg/provider/ollama_discovery.go](/home/nomadx/Documents/smolbot/.worktrees/provider-ux-minimax/pkg/provider/ollama_discovery.go)
- [pkg/gateway/server.go](/home/nomadx/Documents/smolbot/.worktrees/provider-ux-minimax/pkg/gateway/server.go)
- [pkg/gateway/server_test.go](/home/nomadx/Documents/smolbot/.worktrees/provider-ux-minimax/pkg/gateway/server_test.go)
- [internal/client/protocol.go](/home/nomadx/Documents/smolbot/.worktrees/provider-ux-minimax/internal/client/protocol.go)

Task 2 verification commands that were used:

```bash
go test ./pkg/provider ./pkg/gateway -run 'Test.*Model|Test.*Discovery'
go test ./internal/components/dialog ./internal/tui -run 'Test.*Model'
```

## Task 3 Complete: Rebuild The F1 Model Picker

Task 3 status:

- complete
- implementation commit present
- checkpoint commit: `24a571e`
- review gates have not been re-run by me after the worker completion in this handover session, so the next agent should still run Task 3 spec and quality review if they want perfect continuity before moving on

What Task 3 appears to have added:

- grouped provider-aware model picker
- provider/model filtering
- pending-selection flow:
  - `Space` marks pending
  - `Enter` saves pending
- clear separation between info-only rows and selectable rows
- current model highlighting
- TUI fallback to current app model when `ModelsLoadedMsg.Current` is omitted

Worker-reported changed files:

- [internal/components/dialog/models.go](/home/nomadx/Documents/smolbot/.worktrees/provider-ux-minimax/internal/components/dialog/models.go)
- [internal/components/dialog/models_test.go](/home/nomadx/Documents/smolbot/.worktrees/provider-ux-minimax/internal/components/dialog/models_test.go)
- [internal/tui/tui.go](/home/nomadx/Documents/smolbot/.worktrees/provider-ux-minimax/internal/tui/tui.go)
- [internal/tui/tui_test.go](/home/nomadx/Documents/smolbot/.worktrees/provider-ux-minimax/internal/tui/tui_test.go)

Worker-reported verification:

```bash
go test ./internal/components/dialog ./internal/tui -run 'Test.*Model|Test.*Providers'
```

Reported result:

- passed

Current recommendation:

- Before starting Task 4, run the Task 3 reviews explicitly in the new session:
  - spec review
  - code-quality review
- If both are green, then treat Task 3 as fully closed

## What Remains

## Task 4: Rebuild The Provider Detail Surface

This is the next implementation task.

Primary files:

- [internal/components/dialog/providers.go](/home/nomadx/Documents/smolbot/.worktrees/provider-ux-minimax/internal/components/dialog/providers.go)
- [internal/components/dialog/providers_test.go](/home/nomadx/Documents/smolbot/.worktrees/provider-ux-minimax/internal/components/dialog/providers_test.go)
- [internal/tui/tui.go](/home/nomadx/Documents/smolbot/.worktrees/provider-ux-minimax/internal/tui/tui.go)
- [internal/tui/tui_test.go](/home/nomadx/Documents/smolbot/.worktrees/provider-ux-minimax/internal/tui/tui_test.go)

Task 4 intent:

- make `/providers` and the F1 provider surface genuinely useful
- replace static string rows with structured provider sections
- show:
  - current provider
  - current model
  - API base
  - provider type/classification
  - available configured providers
  - whether auth is configured
- clearly distinguish:
  - active
  - configured but inactive
  - incomplete/misconfigured

Task 4 verification target:

```bash
go test ./internal/components/dialog ./internal/tui -run 'Test.*Provider'
```

## Task 5: Add First-Class MiniMax Support

Not started.

Key likely files:

- [pkg/provider/registry.go](/home/nomadx/Documents/smolbot/.worktrees/provider-ux-minimax/pkg/provider/registry.go)
- [pkg/provider/openai.go](/home/nomadx/Documents/smolbot/.worktrees/provider-ux-minimax/pkg/provider/openai.go)
- [pkg/provider/registry_test.go](/home/nomadx/Documents/smolbot/.worktrees/provider-ux-minimax/pkg/provider/registry_test.go)
- [pkg/config/config.go](/home/nomadx/Documents/smolbot/.worktrees/provider-ux-minimax/pkg/config/config.go)
- [cmd/installer/types.go](/home/nomadx/Documents/smolbot/.worktrees/provider-ux-minimax/cmd/installer/types.go)
- [cmd/installer/tasks.go](/home/nomadx/Documents/smolbot/.worktrees/provider-ux-minimax/cmd/installer/tasks.go)
- installer tests
- [cmd/smolbot/runtime.go](/home/nomadx/Documents/smolbot/.worktrees/provider-ux-minimax/cmd/smolbot/runtime.go)
- [internal/tui/tui_test.go](/home/nomadx/Documents/smolbot/.worktrees/provider-ux-minimax/internal/tui/tui_test.go)

Important Task 5 intent:

- make `minimax` a user-facing provider, not just a registry alias
- add installer/provider config surface
- use a sane official API base default
- clean up obvious provider naming issues like `azure` vs `azure_openai`

## Task 6: Provider-Focused End-To-End Coverage

Not started.

This should consolidate real end-to-end regressions that prove:

- model switching works through the real gateway path
- grouped picker behavior works
- current/pending/provider grouping works
- provider config display works

## Task 7: OAuth Design Handoff Only

Not started.

Important:

- design only
- do not quietly start implementing OAuth unless explicitly redirected

## Known Failure Modes And Pitfalls

These are real issues already encountered during this project. Do not regress them.

1. Gateway mutating config before model switch callback success
2. Runtime changing visible model without changing backend provider
3. Heartbeat evaluator not receiving updated model
4. Whitespace model IDs surviving validation and entering runtime
5. Discovery fabricating cross-provider fallback IDs
6. Discovery/runtime mismatch for legacy Azure naming
7. Provider-backed config rows accidentally becoming selectable targets
8. `selectable` becoming a hard requirement and breaking old payloads
9. Review agents inspecting the wrong tree
10. Work on root checkout instead of the worktree

## Files Most Worth Reading Before Continuing

If the next agent is weaker or unfamiliar with the repo, read these first in this order:

1. [2026-03-28-provider-ux-and-minimax.md](/home/nomadx/Documents/smolbot/docs/plans/2026-03-28-provider-ux-and-minimax.md)
2. [internal/client/protocol.go](/home/nomadx/Documents/smolbot/.worktrees/provider-ux-minimax/internal/client/protocol.go)
3. [pkg/gateway/server.go](/home/nomadx/Documents/smolbot/.worktrees/provider-ux-minimax/pkg/gateway/server.go)
4. [pkg/provider/discovery.go](/home/nomadx/Documents/smolbot/.worktrees/provider-ux-minimax/pkg/provider/discovery.go)
5. [internal/components/dialog/models.go](/home/nomadx/Documents/smolbot/.worktrees/provider-ux-minimax/internal/components/dialog/models.go)
6. [internal/tui/tui.go](/home/nomadx/Documents/smolbot/.worktrees/provider-ux-minimax/internal/tui/tui.go)
7. [cmd/installer/tasks.go](/home/nomadx/Documents/smolbot/.worktrees/provider-ux-minimax/cmd/installer/tasks.go)

## Recommended Continuation Procedure

If you are the next agent, do this:

1. Confirm worktree state is clean.
2. Re-run Task 3 focused tests.
3. Run Task 3 spec review.
4. Run Task 3 code-quality review.
5. If both are green, checkpoint nothing further and move to Task 4.
6. Implement Task 4.
7. Run Task 4 focused tests.
8. Run Task 4 spec review.
9. Run Task 4 code-quality review.
10. Checkpoint Task 4.
11. Continue sequentially through Tasks 5, 6, and 7.

## Exact Verification Commands Known To Be Relevant

Task 1:

```bash
go test ./internal/client ./pkg/gateway ./cmd/smolbot ./internal/tui -run 'Test.*Model'
go test ./pkg/agent ./pkg/provider ./pkg/gateway ./cmd/smolbot -run 'Test.*Model|TestRegistry.*'
```

Task 2:

```bash
go test ./pkg/provider ./pkg/gateway -run 'Test.*Model|Test.*Discovery'
go test ./internal/components/dialog ./internal/tui -run 'Test.*Model'
```

Task 3:

```bash
go test ./internal/components/dialog ./internal/tui -run 'Test.*Model|Test.*Providers'
```

## How To Use This Handover Well

If the next model is weaker, use this handover as a structured operating prompt:

- do not summarize it away
- keep the task boundary tight
- always restate the exact current task before coding
- use the listed file scopes
- use the listed verification commands
- treat the “Known Failure Modes” list as regression traps
- only move to the next task after both review gates are green

This is consistent with OpenAI’s prompt-engineering guidance on giving clear instructions, decomposing tasks, and specifying validation explicitly:

- https://platform.openai.com/docs/guides/prompt-engineering

## Copy-Paste Prompt For The Next Agent

Use this prompt nearly verbatim:

> Continue the provider UX and MiniMax work in `/home/nomadx/Documents/smolbot/.worktrees/provider-ux-minimax` on branch `feat/provider-ux-minimax`.
> Read these two files first:
> - `/home/nomadx/Documents/smolbot/docs/HANDOVER_PROVIDER_UX_MINIMAX_2026-03-28.md`
> - `/home/nomadx/Documents/smolbot/docs/plans/2026-03-28-provider-ux-and-minimax.md`
> Treat the handover as authoritative for current state.
> Assume no prior context beyond those files.
> Work only in the worktree, not the root repo.
> Task 1 and Task 2 are complete and green.
> Task 3 has an implementation commit (`24a571e`) but should still get spec and code-quality review before continuing.
> If Task 3 reviews are green, move to Task 4.
> Follow the plan workflow exactly:
> 1. implement only one task
> 2. run focused verification
> 3. run spec review
> 4. run code-quality review
> 5. fix findings
> 6. checkpoint commit
> Do not push or merge.
> Preserve all known invariants and avoid the listed failure modes.

## Final State At This Handover

At the moment this handover was updated:

- worktree: clean
- current branch: `feat/provider-ux-minimax`
- latest worktree commit: `24a571e feat(tui): upgrade model picker workflow`
- next likely action: review Task 3, then proceed to Task 4

