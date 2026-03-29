# Provider Quota Setup And Usage UI Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Refactor quota setup so it is provider-scoped instead of Ollama-specific, expose setup through config plus guided onboarding/installer flows, and improve the `USAGE` sidebar so quota is easier to read at a glance while remaining dynamic.

**Architecture:** Config remains the source of truth. Runtime chooses a quota runner by provider using normalized provider-scoped quota config. The UI keeps `Observed` and `Quota` separate, hides `Quota` when no quota summary exists, and applies stronger visual hierarchy plus percentage severity styling without changing the gateway payload shape again.

**Tech Stack:** Go, Bubble Tea, Lip Gloss, existing `pkg/config`, `pkg/usage`, `cmd/smolbot`, `cmd/installer`, gateway status payloads, sidebar rendering in `internal/components/sidebar`.

---

## Purpose Of This Plan

This plan is intentionally written for an agent with weak project context and weaker judgment than GPT-5.4 High. It assumes the implementer does not know the codebase well and needs:

- exact files to inspect
- exact tests to add and run
- gate ordering
- code review/spec review checkpoints
- clear “done means” criteria
- guardrails about what not to break

Do not skip gates. Do not bundle unrelated refactors into these changes.

## Current Project Context

### Current working assumptions

- Current worktree: `feat/usage-tracking-v2`
- Current useful baseline commit for quota auto-discovery fixes: `be11e40`
- Current Ollama quota fetching works
- Current Zen/Firefox-family and Chromium-family browser discovery works for Ollama quota auto-discovery
- Current config is still Ollama-specific even though runtime fetch itself now works

### Important existing behavior that must not regress

1. `Observed` usage is smolbot-recorded and must remain separate from account-backed `Quota`.
2. `Quota` should only render when a quota summary exists.
3. Existing Ollama quota fetching and caching must keep working.
4. Existing context usage behavior must not change.
5. Existing gateway/client/TUI contract for persisted usage summaries must remain compatible unless tests are updated deliberately.

### Key files to understand before changing anything

Config and runtime:
- `pkg/config/config.go`
- `pkg/config/config_test.go`
- `pkg/config/paths.go`
- `cmd/smolbot/runtime.go`
- `cmd/smolbot/runtime_services_test.go`

Quota backend:
- `pkg/usage/models.go`
- `pkg/usage/quota.go`
- `pkg/usage/summary.go`
- `pkg/usage/ollama_quota_fetcher.go`
- `pkg/usage/browser_cookies_linux.go`

Installer and onboarding:
- `cmd/smolbot/runtime.go`
- `cmd/installer/tasks.go`
- `cmd/installer/views.go`
- any installer model/update tests already present under `cmd/installer/`

UI:
- `internal/components/sidebar/usage_section.go`
- `internal/components/sidebar/sidebar_test.go`
- `internal/tui/tui.go`
- `internal/tui/tui_test.go`

Gateway/client contract:
- `pkg/gateway/server.go`
- `pkg/gateway/server_test.go`
- `internal/client/types.go`
- `internal/client/client_test.go`

Docs and plans:
- `docs/plans/2026-03-29-provider-quota-setup-and-usage-ui-design.md`
- `docs/plans/2026-03-28-ollama-account-quota-design.md`
- `docs/plans/2026-03-28-ollama-account-quota-ui.md`

## Non-Goals

These are not part of this plan:

- implementing non-Ollama quota fetchers
- adding dashboard graphs to the sidebar
- changing observed usage aggregation semantics
- redesigning the entire sidebar or TUI theme system
- introducing a plugin architecture for quota providers

## Required Product Decisions Already Made

These are fixed inputs. Do not re-open them during implementation.

1. Config must support manual editing.
2. CLI onboarding and installer TUI must support quota setup.
3. `Quota` is dynamic:
   - if not configured / no summary exists, do not show it
4. Config must become provider-scoped, not Ollama-only at top level.
5. UI should improve hierarchy:
   - provider/model label distinct from `Observed` and `Quota`
   - glanceable quota severity styling
6. Sidebar should stay compact.

## Execution Strategy

Implement in gates. Each gate has:

- purpose
- exact files
- explicit test-first workflow
- code review checklist
- spec review checklist
- done criteria

Do not start the next gate until the current gate passes both review checklists.

## Gate 0: Baseline And Safety Checks

### Purpose

Confirm the baseline before touching code and make sure the implementer understands the current quota path.

### Files to inspect

- `pkg/config/config.go`
- `cmd/smolbot/runtime.go`
- `internal/components/sidebar/usage_section.go`
- `pkg/usage/browser_cookies_linux.go`

### Steps

1. Read the files listed above.
2. Run:
   - `go test ./pkg/config ./pkg/usage ./cmd/smolbot ./internal/components/sidebar -run 'Quota|Usage|Config' -v`
3. Confirm the worktree is clean before starting implementation.

### Done means

- baseline tests pass
- no unrelated local changes are mixed into the implementation branch

## Gate 1: Provider-Scoped Quota Config

### Purpose

Replace the Ollama-specific top-level quota config shape with provider-scoped configuration while preserving backward compatibility.

### Files

- Modify: `pkg/config/config.go`
- Modify: `pkg/config/config_test.go`
- Possibly modify: `pkg/config/paths.go` only if path helpers truly need generalization
- Possibly modify: `cmd/smolbot/runtime.go` only for config access changes, not runtime behavior changes yet

### Target outcome

The config layer can answer:
- is quota enabled for provider `X`?
- what auth/setup settings exist for provider `X`?

Existing Ollama-specific config keys must continue to load for compatibility.

### Step 1: Write failing tests

Add tests for:
- decoding provider-scoped quota config
- compatibility loading for existing Ollama-specific keys
- normalization result for Ollama provider
- defaults that do not accidentally enable quota for unrelated providers

Run:
- `go test ./pkg/config -run 'Quota|quota' -v`

Expected:
- FAIL because provider-scoped config does not exist yet

### Step 2: Implement minimal config model

Introduce a provider-scoped structure. Keep it simple.

Recommended direction:
- `QuotaConfig`
  - `RefreshIntervalMinutes`
  - `Providers map[string]ProviderQuotaConfig`
  - compatibility fields for old Ollama config, if needed during transition
- `ProviderQuotaConfig`
  - `Enabled bool`
  - `BrowserCookieDiscoveryEnabled bool`
  - `CookieHeader string`

Add normalization helpers, for example:
- `func (q QuotaConfig) Provider(name string) ProviderQuotaConfig`
- `func (q QuotaConfig) HasEnabledProvider(name string) bool`

Do not add speculative fields for providers that do not exist yet.

### Step 3: Re-run tests

Run:
- `go test ./pkg/config -run 'Quota|quota' -v`

Expected:
- PASS

### Code review checklist

- no hardcoded `ollama` assumptions remain in config shape itself
- backward compatibility path is explicit and tested
- no YAGNI fields were added

### Spec review checklist

- config is still the source of truth
- manual config editing is supported
- provider-scoped structure exists

### Commit

```bash
git add pkg/config/config.go pkg/config/config_test.go cmd/smolbot/runtime.go
git commit -m "refactor(config): make quota setup provider-scoped"
```

### Done means

- provider-scoped config exists
- old Ollama config still loads
- tests pass

## Gate 2: Provider-Aware Runtime Selection

### Purpose

Make runtime choose quota behavior by provider-scoped config instead of implicit Ollama-specific checks.

### Files

- Modify: `cmd/smolbot/runtime.go`
- Modify: `cmd/smolbot/runtime_services_test.go`
- Possibly modify: `pkg/usage/models.go` only if needed for clearer provider-neutral semantics

### Target outcome

- runtime only wires a quota runner if the active/default provider has quota enabled/configured
- Ollama remains the only concrete quota runner implementation for now

### Step 1: Write failing tests

Add tests for:
- Ollama quota runner is selected when Ollama provider quota is enabled
- quota runner is not selected when provider quota is absent/disabled
- backward-compatible old config still results in Ollama quota runner selection

Run:
- `go test ./cmd/smolbot -run 'Quota|Runtime|BuildRuntime' -v`

Expected:
- FAIL

### Step 2: Implement minimal runtime selection logic

Refactor:
- `shouldEnableOllamaQuota`
- any runtime config lookups that assume top-level Ollama quota fields

Recommended direction:
- runtime asks config for provider quota settings for `ollama`
- do not add generic non-Ollama runner creation yet
- do add seams so another provider runner can be added later

### Step 3: Re-run tests

Run:
- `go test ./cmd/smolbot -run 'Quota|Runtime|BuildRuntime' -v`

Expected:
- PASS

### Code review checklist

- runtime selection is provider-aware
- no runtime behavior changed for non-quota paths
- Ollama runner internals were not unnecessarily rewritten

### Spec review checklist

- quota setup is no longer hardcoded at the top-level config shape
- runtime reflects provider-scoped config design

### Commit

```bash
git add cmd/smolbot/runtime.go cmd/smolbot/runtime_services_test.go
git commit -m "refactor(runtime): select quota runner by provider config"
```

### Done means

- runtime wires quota by provider config
- tests pass

## Gate 3: CLI Onboarding Quota Setup

### Purpose

Add guided quota setup to CLI onboarding without creating hidden state outside config.

### Files

- Modify: onboarding logic in `cmd/smolbot/runtime.go`
- Modify: any onboarding tests in `cmd/smolbot/*test.go`

### Target outcome

When a user selects Ollama in onboarding:
- they can enable quota setup
- if enabled, onboarding writes provider-scoped config
- browser auto-discovery is the default path
- manual fallback remains possible in config

### Step 1: Write failing tests

Add tests for:
- Ollama provider selection prompts quota enablement
- enabling quota writes provider-scoped config
- disabling quota writes no provider quota config

Run:
- `go test ./cmd/smolbot -run 'Onboard|Quota|Config' -v`

Expected:
- FAIL

### Step 2: Implement minimal onboarding prompts

Add only the necessary prompts.

Recommended flow:
1. user chooses provider
2. if provider is `ollama`, prompt `Enable quota setup?`
3. if yes, default to browser auto-discovery enabled
4. do not force manual cookie entry in onboarding

Manual cookie header remains a config-edit fallback, not a required onboarding step.

### Step 3: Re-run tests

Run:
- `go test ./cmd/smolbot -run 'Onboard|Quota|Config' -v`

Expected:
- PASS

### Code review checklist

- onboarding remains simple
- no quota prompts appear for unsupported providers unless deliberately intended
- config writing remains single-source-of-truth

### Spec review checklist

- CLI onboarding now supports quota setup
- config is still the only persisted source of truth

### Commit

```bash
git add cmd/smolbot/runtime.go cmd/smolbot/*test.go
git commit -m "feat(onboarding): add provider quota setup prompts"
```

### Done means

- onboarding writes correct quota config for Ollama
- tests pass

## Gate 4: Installer TUI Quota Setup

### Purpose

Add the same quota setup capability to the installer TUI.

### Files

- Modify: `cmd/installer/tasks.go`
- Modify: `cmd/installer/views.go`
- Modify: installer model/update logic under `cmd/installer/`
- Add/modify tests under `cmd/installer/`

### Target outcome

Installer can:
- enable or skip provider quota setup
- persist provider-scoped quota config
- default Ollama quota to browser auto-discovery when enabled

### Step 1: Write failing tests

Add tests for:
- installer can persist enabled Ollama quota config
- installer can persist disabled/no quota state
- installer does not write legacy-only fields as the primary representation

Run:
- `go test ./cmd/installer/... -run 'Quota|Config|Install' -v`

Expected:
- FAIL

### Step 2: Implement minimal installer UI changes

Do not redesign the whole installer. Add only:
- quota enable/disable choice
- Ollama-specific setup path for now
- writing normalized provider-scoped config

### Step 3: Re-run tests

Run:
- `go test ./cmd/installer/... -run 'Quota|Config|Install' -v`

Expected:
- PASS

### Code review checklist

- installer changes are localized
- no hidden state outside config
- no unnecessary multi-provider UI scaffolding was added

### Spec review checklist

- installer now supports quota setup
- guided setup writes config, not separate state

### Commit

```bash
git add cmd/installer
git commit -m "feat(installer): add provider quota setup flow"
```

### Done means

- installer supports quota setup for Ollama
- tests pass

## Gate 5: Sidebar Usage Hierarchy And Quota Severity

### Purpose

Improve `USAGE` presentation without changing the compact layout or conflating `Observed` with `Quota`.

### Files

- Modify: `internal/components/sidebar/usage_section.go`
- Modify: `internal/components/sidebar/sidebar_test.go`
- Possibly modify: theme helpers only if current theme API is insufficient
- Possibly modify: `internal/tui/tui_test.go` if sidebar rendering expectations change

### Target outcome

- provider/model label color is distinct from subsection headers
- `Observed` and `Quota` remain subsection titles
- quota percentage values are severity-colored
- `Quota` remains hidden when absent

### Step 1: Write failing tests

Add tests that prove:
- `Quota` block is absent when `summary.Quota == nil`
- severity treatment is applied to quota percentage values
- provider/model label styling differs from subsection header styling

If styling is hard to assert directly, assert through rendered markers or helper behavior rather than brittle full-string snapshots.

Run:
- `go test ./internal/components/sidebar -run 'Usage|Quota' -v`

Expected:
- FAIL

### Step 2: Implement minimal sidebar refinements

Recommended UI rules:
- model/provider label: secondary highlight, not accent
- subsection headers `Observed` and `Quota`: accent
- severity thresholds:
  - `<60%` normal/safe
  - `60-80%` warning
  - `>80%` danger

Apply severity to the value line itself:
- `session 72.0%`
- `week 84.0%`

Do not add icons or graphs in this gate.

### Step 3: Re-run tests

Run:
- `go test ./internal/components/sidebar -run 'Usage|Quota' -v`
- `go test ./internal/tui -run 'Usage|Quota' -v`

Expected:
- PASS

### Code review checklist

- visual hierarchy improved without adding clutter
- no regression to hidden-when-absent `Quota` behavior
- observed usage remains semantically distinct from quota

### Spec review checklist

- `Quota` remains dynamic
- glanceable quota severity is improved
- provider/model label is visually distinct from subsection titles

### Commit

```bash
git add internal/components/sidebar/usage_section.go internal/components/sidebar/sidebar_test.go internal/tui/tui_test.go
git commit -m "feat(sidebar): refine usage hierarchy and quota severity"
```

### Done means

- sidebar hierarchy improved
- quota remains dynamic
- tests pass

## Gate 6: Documentation

### Purpose

Document how users configure quota and when they should expect `Quota` to appear.

### Files

- Modify: `README.md`
- Modify or create: relevant setup docs under `docs/`

### Required doc content

Document:
- quota config location
- provider-scoped quota config shape
- supported browser families for Ollama auto-discovery
  - Chromium-family
  - Firefox-family / Zen
- manual cookie-header fallback
- onboarding and installer quota setup availability
- `Observed` vs `Quota`
- `Quota` hidden when not configured/available

### Steps

1. Update README and any setup docs.
2. Run:
   - `rg -n "quota|Observed|Quota|browser|ollamaCookieHeader|providers" README.md docs pkg/config cmd/smolbot cmd/installer`
3. Manually compare docs to the implemented config/runtime behavior.

### Code review checklist

- docs match real behavior
- no stale Ollama-only top-level config examples remain

### Spec review checklist

- user setup story is documented for config, onboarding, and installer
- dynamic `Quota` display behavior is documented

### Commit

```bash
git add README.md docs
git commit -m "docs: document provider quota setup and usage ui behavior"
```

### Done means

- docs answer “how do I set this up?” clearly

## Gate 7: Final Verification And Review

### Purpose

Close the change safely.

### Focused verification

Run:
- `go test ./pkg/config ./pkg/usage ./cmd/smolbot ./cmd/installer ./pkg/gateway ./internal/components/sidebar ./internal/tui -run 'Quota|Usage|Onboard|Install|Config|Status' -v`

### Full verification

Run:
- `go test ./...`

### Manual runtime spot-check

If feasible after install/restart:
- verify active provider with quota configured shows `Quota`
- verify provider with no quota summary does not show `Quota`
- verify `Observed` still shows independently

### Mandatory review pass

Review all changed files with these questions:

1. Did we eliminate top-level Ollama-specific quota coupling?
2. Did we preserve backward compatibility for current configs?
3. Does runtime select quota by provider config rather than assumption?
4. Does onboarding/installer write config only?
5. Does sidebar hide `Quota` when absent?
6. Does UI improve glanceability without clutter?
7. Do docs accurately describe setup?

### Final commit if needed

```bash
git add .
git commit -m "test: harden provider quota setup and usage ui"
```

### Done means

- focused verification passes
- `go test ./...` passes
- gate reviews completed
- change is safe to merge

## Merge Order

If implemented in one branch, follow commit order:

1. config refactor
2. runtime selection
3. onboarding
4. installer
5. sidebar/UI
6. docs
7. hardening

If implemented by multiple agents, use these ownership boundaries:

- Agent A: `pkg/config`, `cmd/smolbot/runtime.go`, related runtime/onboarding tests
- Agent B: `cmd/installer/*`
- Agent C: `internal/components/sidebar/*`, `internal/tui/*`, only if UI depends on settled config/gateway semantics

Do not have multiple agents edit `cmd/smolbot/runtime.go` at the same time.

## Common Failure Modes

Watch for these specifically:

1. Replacing old Ollama config without compatibility handling.
2. Making `Quota` render with an empty placeholder when the requirement is to hide it.
3. Writing installer/onboarding-only state outside config.
4. Overengineering the provider abstraction before a second provider exists.
5. Breaking current gateway/client payload expectations while changing config internals.
6. Using the same accent styling for model label and subsection headings again.

## Execution Notes For A Weaker Model

- Read the cited files before editing.
- Write the failing test first for each gate.
- Make the smallest code change that satisfies the gate.
- Re-run only the targeted tests for that gate before moving on.
- Commit after each gate.
- Do not “clean up” unrelated code.
- Do not change provider fetcher behavior for non-Ollama providers.
- Do not touch unrelated UI sections.
