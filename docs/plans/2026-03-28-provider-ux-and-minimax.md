# Provider UX And MiniMax Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Fix the broken provider/model selection flow, make the F1 provider surfaces actually useful, add first-class MiniMax support, and leave a clean design seam for future OpenAI/Codex-style OAuth.

**Architecture:** Treat provider work as three layers: protocol correctness, discovery/config plumbing, and TUI surfaces. First make the existing `models.set` path work end-to-end, then upgrade model/provider discovery so the UI has real data, then add a stronger picker and provider detail panel modeled after OpenCode’s grouped/recent model selection and Crush’s provider hygiene.

**Tech Stack:** Go, Bubble Tea, Lip Gloss, smolbot gateway protocol, provider registry, installer/config JSON.

---

## Execution Workflow

Use a dedicated worktree. Implement one task at a time. After each task:

1. Run the focused verification listed for that task.
2. Run a spec/completeness review for the task slice.
3. Run a code-quality review for the task slice.
4. Fix findings before moving on.
5. Create a checkpoint commit.

Do not push or merge until the final gate at the end of this document is green.

## Reference Notes

- Current smolbot findings:
  - `internal/client/messages.go` sends `models.set` as `{id: ...}`.
  - `pkg/gateway/server.go` expects `models.set` as `{model: ...}`.
  - `pkg/provider/ollama_discovery.go` only discovers real models for Ollama; other providers return just the current model.
  - `internal/components/dialog/providers.go` is read-only.
  - `cmd/installer/types.go` and `cmd/installer/tasks.go` only expose a narrow provider set and currently use `azure` instead of `azure_openai`.
- OpenCode reference:
  - `/home/nomadx/opencode/packages/tui/internal/components/dialog/models.go`
  - grouped provider sections
  - recent models
  - fuzzy search mode
- Crush reference:
  - `/home/nomadx/crush/README.md`
  - `/home/nomadx/crush/internal/config/config.go`
  - clear `openai` vs `openai-compat` distinction
  - richer provider metadata/config handling
  - MiniMax presence as a first-class provider
- External references:
  - MiniMax API overview: https://platform.minimax.io/docs/api-reference/api-overview
  - MiniMax token plan quickstart: https://platform.minimax.io/docs/token-plan/quickstart
  - OpenAI Codex/ChatGPT sign-in overview: https://help.openai.com/en/articles/11369540/

## Non-Negotiable Invariants

- `models.set` must have one wire contract across client, gateway, tests, and runtime.
- Provider/model selection must be testable end-to-end through the real gateway, not only fake clients.
- The model picker must work for non-Ollama providers in a meaningful way.
- MiniMax must be configurable and testable without weakening existing providers.
- OAuth is design-only in this plan unless a clearly supported and safe implementation seam emerges.

## Task 1: Fix The Existing Model-Switching Contract

**Files:**
- Modify: `internal/client/messages.go`
- Modify: `internal/client/protocol.go`
- Modify: `pkg/gateway/server.go`
- Modify: `pkg/gateway/server_test.go`
- Modify: `internal/tui/tui_test.go`
- Modify: `cmd/smolbot/runtime_model_test.go`

**Intent:**
Close the `id` vs `model` mismatch and add a real end-to-end regression for model switching.

**Steps:**
1. Decide on one canonical payload shape for `models.set`.
2. Make client and gateway accept that canonical shape.
3. Add compatibility handling only if needed for existing callers.
4. Add an integration-style test that starts the real gateway, drives the client-side `ModelsSet`, and asserts the configured model actually changes.
5. Tighten TUI tests so the fake client no longer hides protocol mismatches.

**Verification:**
- `go test ./internal/client ./pkg/gateway ./cmd/smolbot -run 'Test.*Model'`

**Gate:**
- Spec review: model switching works from the real client path.
- Code-quality review: no duplicate protocol structs or silent fallback ambiguity.

**Checkpoint Commit:**
- `fix(provider): unify models.set request contract`

## Task 2: Expand Provider And Model Discovery Beyond “Current Model Only”

**Files:**
- Modify: `pkg/provider/ollama_discovery.go`
- Create: `pkg/provider/discovery.go`
- Create: `pkg/provider/discovery_test.go`
- Modify: `pkg/gateway/server.go`
- Modify: `pkg/gateway/server_test.go`
- Modify: `internal/client/protocol.go`

**Intent:**
Give the UI a provider-aware model catalog instead of the current “Ollama only, else current model” behavior.

**Steps:**
1. Extract generic provider/model discovery out of `ollama_discovery.go`.
2. Keep rich live discovery for Ollama.
3. For configured OpenAI-compatible providers, return provider-backed model entries from config rather than only the current model.
4. Include enough metadata for the UI to render sensible rows:
   - provider id
   - model id
   - display name
   - optional description
   - source/capability marker if available
5. Ensure the gateway returns the richer model payload.
6. Add tests for:
   - Ollama discovery
   - configured non-Ollama providers
   - fallback behavior when live discovery is unavailable

**Verification:**
- `go test ./pkg/provider ./pkg/gateway -run 'Test.*Model|Test.*Discovery'`

**Gate:**
- Spec review: model list is meaningful for Ollama and configured compatible providers.
- Code-quality review: discovery logic is not hardcoded into gateway handlers.

**Checkpoint Commit:**
- `feat(provider): expand model discovery across providers`

## Task 3: Rebuild The F1 Model Picker

**Files:**
- Modify: `internal/components/dialog/models.go`
- Modify: `internal/components/dialog/models_test.go`
- Modify: `internal/tui/tui.go`
- Modify: `internal/tui/tui_test.go`

**Intent:**
Replace the thin picker with a grouped, provider-aware picker that behaves predictably in the F1 flow.

**Steps:**
1. Add grouped display by provider.
2. Add a recent/current section if data is available; otherwise at minimum highlight the current model clearly.
3. Add search/filter behavior that narrows by provider and model.
4. Add explicit pending-selection behavior:
   - `Space` marks/selects the focused model.
   - `Enter` confirms and saves.
5. Keep `Enter` on the focused item working if no separate pending state is selected, so the flow is not cumbersome.
6. Render clear current vs pending vs provider-group states.
7. Add tests for:
   - group rendering
   - `Space` pending selection
   - `Enter` save behavior
   - keyboard-only flow
   - filter behavior

**Verification:**
- `go test ./internal/components/dialog ./internal/tui -run 'Test.*Model|Test.*Providers'`

**Gate:**
- Spec review: the picker now supports the expected `Space` then `Enter` workflow.
- Code-quality review: UI logic remains in the dialog/TUI layer and does not leak into protocol code.

**Checkpoint Commit:**
- `feat(tui): upgrade model picker workflow`

## Task 4: Rebuild The Provider Detail Surface

**Files:**
- Modify: `internal/components/dialog/providers.go`
- Modify: `internal/components/dialog/providers_test.go`
- Modify: `internal/tui/tui.go`
- Modify: `internal/tui/tui_test.go`

**Intent:**
Make `/providers` and the F1 provider surface informative enough to debug and use.

**Steps:**
1. Replace static freeform text rows with structured provider sections.
2. Show at minimum:
   - current provider
   - current model
   - API base
   - provider type/classification
   - available configured providers
   - whether API key/token is configured
3. Clearly distinguish:
   - active provider
   - configured but inactive providers
   - missing/incomplete provider configs
4. Add tests for mixed provider configurations and current-provider highlighting.

**Verification:**
- `go test ./internal/components/dialog ./internal/tui -run 'Test.*Provider'`

**Gate:**
- Spec review: provider surface is no longer a placeholder.
- Code-quality review: provider line construction is replaced by structured view data, not string concatenation glue.

**Checkpoint Commit:**
- `feat(tui): improve provider detail dialog`

## Task 5: Add First-Class MiniMax Support

**Files:**
- Modify: `pkg/provider/registry.go`
- Modify: `pkg/provider/openai.go`
- Modify: `pkg/provider/registry_test.go`
- Modify: `pkg/config/config.go`
- Modify: `cmd/installer/types.go`
- Modify: `cmd/installer/tasks.go`
- Modify: `cmd/installer/*test.go`
- Modify: `cmd/smolbot/runtime.go`
- Modify: `internal/tui/tui_test.go`

**Intent:**
Turn the existing `minimax` alias into a complete, user-facing provider path.

**Steps:**
1. Normalize naming in config/UI around `minimax`.
2. Add installer support for MiniMax:
   - provider choice
   - API key
   - API base with sane default from official docs
3. Add config/runtime tests proving MiniMax resolves through the provider registry correctly.
4. Ensure model/provider surfaces recognize MiniMax as a first-class provider.
5. Add a real-world test checklist section to the plan execution notes for use with the user’s API key.
6. While in this area, fix any obvious provider naming mismatch like `azure` vs `azure_openai`.

**Verification:**
- `go test ./pkg/provider ./pkg/config ./cmd/installer ./cmd/smolbot -run 'Test.*MiniMax|Test.*Provider'`

**Gate:**
- Spec review: MiniMax can be configured end-to-end.
- Code-quality review: compatible-provider handling is clearer after the change, not more ad hoc.

**Checkpoint Commit:**
- `feat(provider): add minimax configuration support`

## Task 6: Add Provider-Focused End-To-End Coverage

**Files:**
- Modify: `internal/tui/tui_test.go`
- Modify: `pkg/gateway/server_test.go`
- Modify: `cmd/smolbot/runtime_model_test.go`
- Create: `internal/tui/provider_flow_test.go`

**Intent:**
Close the test gap that allowed provider/model work to regress unnoticed.

**Steps:**
1. Add a real client-to-gateway model-switch test.
2. Add a TUI dialog selection test that uses the real payload contract.
3. Add coverage for:
   - current-model highlight
   - pending `Space` selection
   - `Enter` save
   - provider grouping
   - non-Ollama populated model lists
4. Add a regression for provider config display.

**Verification:**
- `go test ./internal/tui ./pkg/gateway ./cmd/smolbot -run 'Test.*Provider|Test.*Model'`

**Gate:**
- Spec review: failure modes discovered in this research pass are covered.
- Code-quality review: tests do not rely on fake-client behavior that masks gateway bugs.

**Checkpoint Commit:**
- `test(provider): add end-to-end provider regressions`

## Task 7: OAuth Design Handoff Only

**Files:**
- Create: `docs/plans/2026-03-28-openai-codex-oauth-design.md`

**Intent:**
Capture the correct future direction for OpenAI/Codex-style sign-in without mixing it into the MiniMax/provider UX implementation.

**Steps:**
1. Document current provider auth limitations in smolbot.
2. Compare them with Crush’s OAuth token model.
3. Note that Codex/ChatGPT sign-in is not a generic provider toggle.
4. Define the minimum future auth subsystem shape:
   - token storage
   - refresh
   - interactive device/browser flow
   - provider capability flags

**Verification:**
- Manual review of the design doc against current provider structs and official docs.

**Gate:**
- Spec review: future OAuth work is scoped out cleanly.
- Code-quality review: no premature auth implementation leaks into this provider branch.

**Checkpoint Commit:**
- `docs(provider): capture oauth follow-up design`

## Final Gate

Before push or merge:

1. Run:
   - `go test ./internal/client ./pkg/provider ./pkg/gateway ./internal/components/dialog ./internal/tui ./cmd/installer ./cmd/smolbot`
   - `go build ./cmd/smolbot ./cmd/smolbot-tui ./cmd/installer`
2. Run a final spec/completeness review over the whole change.
3. Run a final code-quality review over the whole change.
4. Fix all findings.
5. Push only when:
   - model switching works through the real gateway path
   - provider/model dialogs are genuinely usable
   - MiniMax is configurable
   - provider regressions are covered by tests

