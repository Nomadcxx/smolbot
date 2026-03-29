# Provider Quota Setup And Usage UI Design

## Goal

Make quota configuration provider-agnostic, keep Ollama as the first concrete implementation, and improve the `USAGE` sidebar so `Observed` and `Quota` are easier to parse at a glance.

## Current State

- Quota runtime fetching exists and works for Ollama.
- The live Ollama quota path now works via browser cookie auto-discovery for Chromium-family and Firefox-family browsers, including Zen.
- The current config surface is still Ollama-specific:
  - `quota.browserCookieDiscoveryEnabled`
  - `quota.ollamaCookieHeader`
- The current sidebar hides `Quota` when no quota summary exists, which is the right display behavior.
- The current sidebar hierarchy is weak:
  - provider/model label uses the same accent styling as subsection headers
  - quota percentages are plain text with low glanceability

## Product Decisions

### 1. Config source of truth

Quota setup should be editable manually in config and also writable through onboarding and installer flows.

### 2. Dynamic UI behavior

`Quota` should only appear when quota is configured and/or available for the active provider. If quota is not set up, it should not render.

### 3. Provider-agnostic architecture

The system should stop treating quota as an Ollama-only top-level concept. Ollama remains the first implementation, but config, runtime wiring, and UI contracts should be designed around provider-scoped quota integrations.

### 4. Sidebar presentation

The `USAGE` section should remain compact.

- provider/model label should be visually distinct from `Observed` and `Quota`
- `Observed` and `Quota` remain subsection headers
- quota percentages should communicate severity at a glance
- no heavy graphing in the sidebar

## Recommended Architecture

### Config

Replace the current Ollama-specific quota fields with a provider-scoped config shape.

Illustrative direction:

```json
{
  "quota": {
    "refreshIntervalMinutes": 60,
    "providers": {
      "ollama": {
        "enabled": true,
        "browserCookieDiscoveryEnabled": true,
        "cookieHeader": ""
      }
    }
  }
}
```

Notes:

- provider entries are optional
- quota is enabled per provider, not globally by assumption
- current Ollama fields should be migrated or preserved as backward-compatible inputs for one transition window

### Runtime

Introduce a provider-agnostic quota runner selection path:

- runtime asks whether the active/default provider has quota configured
- if yes, it selects the corresponding provider quota runner
- if no, no quota runner is wired and the UI sees no quota summary

For now:

- Ollama is the only concrete provider quota runner
- the contract should support future OpenAI/Anthropic integrations without reshaping the gateway payload again

### UI contract

Keep the existing `UsageSummary.Quota` pattern but treat it as provider-neutral account quota data.

- if quota summary is absent: hide `Quota`
- if quota summary exists and is live/stale/expired/unavailable: show `Quota` with that explicit state

### Sidebar presentation

Recommended visual structure:

- line 1: provider/model label in a neutral secondary highlight, not the same accent as subsection titles
- `Observed`
  - session
  - today
  - week
  - reqs
- `Quota`
  - plan
  - session %
  - week %

Recommended severity treatment:

- `0-60%`: normal/green
- `60-80%`: warning/yellow
- `80%+`: danger/red

Apply severity color to the percentage line values rather than adding extra words or icons. This keeps the sidebar compact.

## Setup Surface

### Manual config

Users should be able to:

- enable/disable provider quota manually
- choose cookie auto-discovery
- provide fallback cookie header manually

### CLI onboarding

When the selected/default provider supports quota integrations:

- offer an enable/disable prompt
- if enabled, offer preferred auth mode
- default to browser auto-discovery
- keep manual cookie header as fallback

### Installer TUI

Expose the same choices in a guided form:

- enable quota for Ollama
- browser auto-discovery on/off
- optional manual fallback

Installer and onboarding should write config only; they should not create separate hidden state.

## Documentation Gaps To Fix

We need explicit docs for:

- where quota is configured
- how auto-discovery works
- which browser families are supported
- manual fallback path
- the fact that `Quota` only appears when configured/available
- the distinction between `Observed` and `Quota`

## Risks

### 1. Accidental Ollama lock-in

If the config refactor is shallow, runtime and onboarding will still implicitly assume Ollama. Mitigation: introduce provider-keyed config and quota runner selection now.

### 2. Backward compatibility

Existing configs may already use the current Ollama-specific fields. Mitigation: support both during transition and normalize into the new shape internally.

### 3. Sidebar overload

Too much formatting or too many labels will hurt readability. Mitigation: use color and hierarchy, not extra text or graphics.

## Discrete Task Groups

1. Provider-agnostic quota config and normalization
2. Runtime/provider quota runner selection refactor
3. Onboarding and installer quota setup flows
4. Sidebar `USAGE` hierarchy and severity polish
5. Documentation updates for quota setup and behavior
