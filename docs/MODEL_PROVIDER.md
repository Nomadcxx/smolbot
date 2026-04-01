# Model & Provider UX — Gap Analysis

> **Purpose**: Compare smolbot's current model/provider selection UX against opencode's
> implementation, identify all gaps, and provide a concrete baseline for an implementation plan.
>
> **Baseline assumption**: smolbot is attempting parity with opencode's model/provider UX.
> All gaps are measured against opencode as the reference implementation.

---

## 1. Executive Summary

smolbot has a functional but minimal model/provider UX. It supports filtering, provider-grouped
display, and separate provider configuration. opencode's implementation is significantly richer,
with favourites, recents, inline provider addition, OAuth, richer model metadata, free-tier badges,
and sorting by recency/release date. The table below summarises coverage at a feature level.

| Feature | smolbot | opencode | Gap |
|---|---|---|---|
| Model list dialog | ✅ | ✅ | Partial (see sections below) |
| Provider-grouped display | ✅ | ✅ | Minor differences |
| Active model indicator | ⚠️ text " current" | ✅ `●` prefix + bold | Visual polish |
| Filter/search | ⚠️ token prefix match | ✅ fuzzy search | Quality |
| Visible item window | ⚠️ 7 items | ✅ 10 items | Minor |
| Favourites system | ❌ | ✅ Ctrl+F, persisted | **Missing** |
| Recent models | ❌ | ✅ top 10, F2 cycle | **Missing** |
| Inline provider addition | ❌ | ✅ Ctrl+A → full flow | **Missing** |
| OAuth provider auth | ❌ | ✅ browser + code flows | **Missing** |
| Remove model from recents | ❌ | ✅ Ctrl+X | **Missing** |
| Model cost/pricing display | ❌ | ⚠️ data present, not shown | Partial |
| Context window display | ❌ | ⚠️ data present, not shown | Partial |
| Capability display | ⚠️ "reasoning" only | ✅ vision, tools, etc. | Partial |
| Free model badge | ❌ | ✅ "Free" label | **Missing** |
| Release date display | ❌ | ✅ shown in description | **Missing** |
| Sorting (release date) | ❌ | ✅ release date desc | **Missing** |
| Provider categories | ❌ flat list | ✅ Popular / Other | **Missing** |
| Post-add model selection | ❌ back to main | ✅ auto-opens for new provider | **Missing** |
| Live model catalogue | ❌ ~27 hardcoded | ✅ models.dev live feed | Architectural |

---

## 2. Model Selection Dialog

### 2.1 Current state (smolbot)

```
╭──────────────────────────────────────────╮
│  Switch model                            │
│                                          │
│  Filter: gpt_                            │
│                                          │
│  Provider: openai (current)              │
│    GPT-4o  gpt-4o                current │
│    GPT-4o mini  gpt-4o-mini              │
│    GPT-4 Turbo  gpt-4-turbo              │
│  ▼ more below                            │
│                                          │
│  Type filter • Space mark • Enter save   │
╰──────────────────────────────────────────╯
```

- Flat filter bar at top; all tokens must appear as prefixes of words in the haystack.
- Models grouped by provider with a header row showing `Provider: <id> (current)`.
- Active model appends the text `" current"` to its label (no prefix indicator).
- 7 visible items (constant `maxVisibleItems = 7`).
- `Space` marks a pending change; `Enter` commits.

### 2.2 opencode reference

```
╭──────────────────────────────────────────╮
│  Select a model                          │
│  > gpt_                                  │
│                                          │
│  Favorites                               │
│    ● claude-sonnet-4-5   Anthropic       │
│      gpt-4o              OpenAI          │
│                                          │
│  Recent                                  │
│    gemini-2.0-flash      Google          │
│                                          │
│  OpenAI                                  │
│    GPT-4o                May 2024  Free  │
│    GPT-4o mini           Jul 2024  Free  │
│    o3                    Apr 2025        │
│  ▼ more below                            │
│                                          │
│  Ctrl+F favorite • Ctrl+A add provider  │
│  Ctrl+X remove from recent • Esc close  │
╰──────────────────────────────────────────╯
```

- Fuzzy search with character-sequence matching (not just prefix tokens).
- Three sections: **Favorites → Recent → Provider Groups** (provider name as header, not `Provider: <id>`).
- Active model is marked with `●` prefix (accent-bold).
- 10 visible items.
- Release date and "Free" badge shown inline per model row.
- `Ctrl+F` toggles favourite; `Ctrl+A` opens provider addition inline; `Ctrl+X` removes from recents.

### 2.3 Gaps

| # | Gap | Detail |
|---|---|---|
| G1 | No favourites section | No Ctrl+F, no `Favorites` bucket, no persistence |
| G2 | No recents section | No per-session or cross-session recent tracking |
| G3 | No Ctrl+A inline provider add | Must leave dialog to F1 providers panel |
| G4 | No Ctrl+X remove from recents | N/A (recents don't exist) |
| G5 | Active indicator is trailing text | `" current"` vs `●` prefix — hard to scan |
| G6 | Token-prefix search only | Multi-token prefix match; opencode uses true fuzzy (char sequence) |
| G7 | 7 visible items | opencode shows 10 |
| G8 | Provider header says `Provider: <id>` | Should be display name e.g. "OpenAI", "Anthropic" |
| G9 | No release date per row | opencode shows e.g. "May 2024" as secondary text |
| G10 | No "Free" badge | opencode shows "Free" for zero-cost models |
| G11 | No F2 cycle-recent keybind | opencode allows cycling through recents without opening dialog |

---

## 3. Favourites System

### 3.1 Current state (smolbot)

Does not exist. There is no concept of marking a model as a favourite, no persistent storage for
favourites, and no visual section for them in the model list.

### 3.2 opencode reference

- **Toggle**: `Ctrl+F` while a model is focused in the model dialog.
- **Visual**: `●` bullet prefix + `(Favorite)` in the description column.
- **Section ordering**: Favorites block always at top of the list, above Recents and provider groups.
- **Persistence**: Saved to `~/.local/share/opencode/model.json` (or equivalent XDG data dir).
- **Deduplication**: A model appearing in Favorites is not repeated in its provider group.
- **Unlimited**: No cap on number of favourites.

### 3.3 Gaps

| # | Gap | Detail |
|---|---|---|
| F1 | No favourites concept | Ctrl+F keybind absent; no visual indicator |
| F2 | No persisted favourites list | `model.json` or equivalent not implemented |
| F3 | No Favorites section at top | Section header + deduplication logic absent |
| F4 | No `(Favorite)` description label | Cannot distinguish favourites from ordinary models visually |

---

## 4. Recent Models

### 4.1 Current state (smolbot)

Does not exist. The currently-active model is tracked in config but there is no history of
previously used models displayed in the dialog or accessible via a keybind.

### 4.2 opencode reference

- **Tracking**: Last 10 distinct models used are kept in the recents list.
- **Section**: "Recent" section below Favorites, above provider groups.
- **Keybind**: `F2` cycles through recent models without opening the dialog (quick swap).
- **Removal**: `Ctrl+X` removes the focused model from the recents list while in the dialog.
- **Persistence**: Stored in same `model.json` as favourites.

### 4.3 Gaps

| # | Gap | Detail |
|---|---|---|
| R1 | No recent models tracking | Model switches not recorded |
| R2 | No Recent section in dialog | Section header, deduplication, and display absent |
| R3 | No F2 quick-cycle keybind | No way to swap to last-used model outside dialog |
| R4 | No Ctrl+X remove from recents | N/A |
| R5 | No cap / eviction policy | Needs max-10 sliding window |

---

## 5. Provider Addition UX

### 5.1 Current state (smolbot)

Provider configuration is accessed via `F1` (providers panel) which is separate from model
selection. The flow is: browse providers → select one → enter API key → save. After adding a
provider, the user must manually open the model dialog to select a model. There is no inline
flow from model selection → add provider → select model.

### 5.2 opencode reference

- **Entry point**: `Ctrl+A` while in the model selection dialog opens "Connect a provider".
- **Categories**: Providers listed as **Popular** (opencode/anthropic/github-copilot/openai/google)
  and **Other** (all remaining).
- **Auth method selection**: After choosing a provider, the user selects between:
  - `OAuth (browser)` — opens a browser to the provider's OAuth page
  - `OAuth (code)` — displays a device code
  - `API Key` — inline text input (existing smolbot flow)
- **Post-add transition**: After a provider is successfully configured, the dialog transitions
  directly into model selection **filtered to the newly added provider**, so the user immediately
  picks a model without extra navigation.

### 5.3 Gaps

| # | Gap | Detail |
|---|---|---|
| P1 | No Ctrl+A from model dialog | Provider addition only accessible via F1 panel |
| P2 | No OAuth flows | Only API key entry; browser OAuth and device-code OAuth absent |
| P3 | No Popular/Other categorisation | Provider list is a flat sorted list |
| P4 | No post-add auto-transition | After adding provider, user must manually open model dialog |
| P5 | No provider connection feedback | No in-progress or success state during OAuth redirect |

---

## 6. Model Metadata & Display Quality

### 6.1 Current state (smolbot)

Each model row shows: `<Name>  <ID>` (secondary). The `CatalogueEntry` struct captures only
`ID`, `Name`, and `Capability`. No cost, context window, release date, or free/paid tier is
stored or displayed.

`client.ModelInfo` (the API response type) includes `ContextWindow int`, but this is not
populated by the catalogue and not rendered in the dialog.

### 6.2 opencode reference

`ModelInfo` carries:
- `name`, `id` — display and API identifier
- `releaseDate` — ISO date string, used for sorting and display
- `isFree` — boolean, renders "Free" badge
- `inputCostPer1MTokens` / `outputCostPer1MTokens` — pricing
- `contextLength` — integer token count
- `supportsAttachments`, `supportedParameters` — capability flags

In the dialog, the primary text is the model name; secondary text shows provider name +
release date + "Free" badge. Cost and context are available but not yet surfaced in the list
(considered a future enhancement in opencode too, but the data is present).

### 6.3 Gaps

| # | Gap | Detail |
|---|---|---|
| M1 | No release date in catalogue | `CatalogueEntry` has no `ReleaseDate` field |
| M2 | No release date display | No secondary-text slot for date in model row |
| M3 | No pricing data | `InputCostPer1MTokens` / `OutputCostPer1MTokens` absent |
| M4 | No free-tier flag | `IsFree bool` absent from `CatalogueEntry` and `ModelInfo` |
| M5 | No "Free" badge rendering | Badge rendering logic absent from row renderer |
| M6 | ContextWindow not populated | Field exists on `ModelInfo` but catalogue never fills it |
| M7 | Capability limited to "chat"/"reasoning" | No vision, tools, function-calling, image-gen flags |
| M8 | No release-date–based sorting | Models returned in catalogue-insertion order |

---

## 7. Sorting & Ordering

### 7.1 Current state (smolbot)

Models within each provider group appear in catalogue-insertion order (i.e., the order they are
written in `catalogue.go`). There is no dynamic sorting. No recents or favourites affect order.

### 7.2 opencode reference

Sort priority (descending):
1. Favourites (explicit user preference)
2. Recents (ordered by last-used timestamp)
3. Within provider groups: `isFree DESC, releaseDate DESC, name ASC`

This means the newest, free models surface first within each provider.

### 7.3 Gaps

| # | Gap | Detail |
|---|---|---|
| S1 | No favourites-first ordering | No favourites concept yet |
| S2 | No recents-first ordering | No recents concept yet |
| S3 | No release-date sort within groups | Insertion order only |
| S4 | No free-first sort within groups | No `IsFree` flag |

---

## 8. Model Catalogue Coverage & Freshness

### 8.1 Current state (smolbot)

27 models hardcoded across 6 providers in `pkg/provider/catalogue.go`:

| Provider | Count | Notes |
|---|---|---|
| anthropic | 7 | Current as of Sonnet/Opus 4-series |
| openai | 7 | Current as of o3 |
| gemini | 5 | Current as of 2.5 Pro |
| groq | 4 | Llama + Mixtral + Gemma |
| deepseek | 2 | Chat + Reasoner |
| minimax | 2 | Text-01 + ABAB 6.5s |

Catalogue must be manually updated with each code release. Custom/compatible providers (OpenRouter,
Kilo, self-hosted) return a single stub row and are not enumerable.

### 8.2 opencode reference

Uses the `models.dev` API to fetch a live JSON catalogue. This means:
- New models appear automatically without a code release.
- Full metadata (release date, pricing, context window, capabilities) is available.
- 300+ models across dozens of providers.
- Providers added via API key can immediately surface their model list.

### 8.3 Gaps

| # | Gap | Detail |
|---|---|---|
| C1 | Static hardcoded catalogue | Models require a code release to be added |
| C2 | Only 6 providers in catalogue | opencode supports 30+ |
| C3 | No live fetch from models.dev | No HTTP catalogue refresh mechanism |
| C4 | No metadata richness | Cost, context, release date all absent |
| C5 | Custom providers not enumerable | OpenRouter/Kilo users see a single stub |

> **Note**: C3 is an architectural gap. Implementing it would require HTTP at startup (or cached
> fetch) and a richer data model. It may be out of scope for a first iteration; improving the
> static catalogue (C1–C2) is lower risk.

---

## 9. Keybind & Navigation Summary

| Action | smolbot | opencode |
|---|---|---|
| Open model dialog | `F2` / header click | model icon / keybind |
| Filter models | type in filter bar | type in search bar |
| Confirm selection | `Enter` | `Enter` |
| Close dialog | `Esc` | `Esc` |
| Toggle favourite | ❌ | `Ctrl+F` |
| Add provider inline | ❌ | `Ctrl+A` |
| Remove from recents | ❌ | `Ctrl+X` |
| Cycle recent models | ❌ | `F2` (outside dialog) |
| Mark pending | `Space` | immediate focus |

---

## 10. Recommended Implementation Priority

Based on user impact, the following order is recommended for closing these gaps:

### High priority (core UX parity)
1. **G5 / Active indicator** — Replace `" current"` with `●` prefix; small change, big visual win.
2. **G8 / Provider header names** — Show display names ("OpenAI") not provider IDs ("openai").
3. **G1–G4, F1–F4 / Favourites** — Ctrl+F toggle, Favorites section, persistence to `~/.smolbot/state.json`.
4. **R1–R5 / Recents** — Track last-10 model switches, Recent section, F2 quick-cycle.
5. **G7 / Visible window** — Increase `maxVisibleItems` from 7 to 10.

### Medium priority (provider flow)
6. **P1, P4 / Inline provider add** — Ctrl+A from model dialog → existing providers dialog → auto-transition back.
7. **M1–M2, M8 / Release date** — Add `ReleaseDate` to `CatalogueEntry`, display in row, sort within group.
8. **M4–M5 / Free badge** — Add `IsFree` to catalogue, render badge.

### Lower priority (data richness)
9. **M3, M6, M7 / Pricing + context + capabilities** — Enrich `CatalogueEntry` and display in a detail pane or secondary text.
10. **P2 / OAuth flows** — Browser and device-code OAuth; requires provider-specific integration.
11. **C1–C5 / Live catalogue** — models.dev integration or significantly expanded static catalogue.

---

## Appendix A: Key Files

| File | Role |
|---|---|
| `internal/components/dialog/models.go` | Model selection dialog — all rendering + input |
| `internal/components/dialog/common.go` | `matchesQuery`, `visibleBounds`, `maxVisibleItems` |
| `internal/components/dialog/providers.go` | Provider browse/configure/confirm dialog |
| `pkg/provider/catalogue.go` | Static model catalogue — 27 models, 6 providers |
| `pkg/config/config.go` | `ProviderConfig`, `ModelConfig` — persisted state |
| `pkg/config/writeback.go` | Atomic config write-back |
| `internal/tui/tui.go` | `ModelSetMsg` — wires model selection to app state |

## Appendix B: opencode Reference Files

| File | Role |
|---|---|
| `packages/opencode/src/cli/cmd/tui/component/dialog-model.tsx` | Model selection with favourites/recents/Ctrl+A |
| `packages/opencode/src/cli/cmd/tui/component/dialog-provider.tsx` | Provider addition with OAuth |
| `packages/opencode/src/provider/models.ts` | Live models.dev integration |
| `packages/opencode/src/provider/model.json` | Favourites + recents persistence schema |
