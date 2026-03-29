# Delegated Agent UX Design

**Date:** 2026-03-28

## Goal

Bring smolbot’s delegated-agent experience closer to Codex:

- child agents appear immediately as compact transcript artifacts
- waiting is explicit in the transcript instead of feeling like a frozen parent run
- completion is summarized cleanly when the parent resumes

This phase is intentionally limited. It does **not** add child-session navigation, live nested child tool streaming, per-agent streaming thought text, or deep expand/collapse for child internals.

## Current State

Smolbot currently treats child-agent execution as a synchronous `spawn` tool call:

- the tool accepts only a free-form `message`
- it calls the child synchronously and returns only after the child completes
- the child callback is `nil`, so child lifecycle is invisible to the parent UI
- the gateway only emits generic `chat.tool.start` / `chat.tool.done`
- the TUI only renders generic tool blocks

That architecture guarantees the current failure mode:

- the parent transcript shows a tool start
- the conversation appears to stall while the child runs
- the parent receives a large tool result dump at the end

## References

### Codex reference

Codex’s transcript UX is the target presentation:

- per-agent spawned rows with agent identity and a truncated delegated task
- explicit waiting rows
- explicit finished-waiting rows with compact completion summaries

### OpenCode reference

OpenCode provides the better backend reference than smolbot’s current `spawn` design:

- structured delegated-task input (`description`, `prompt`, `subagent_type`)
- child work tracked as a real long-running artifact
- metadata summary updated while the child is running

We want OpenCode’s backend direction, but **not** OpenCode’s child-session navigation model.

## Design Decision

### Recommended architecture

Introduce a first-class delegated-agent orchestration path made of:

1. a structured child-agent tool
2. an explicit wait primitive
3. gateway events for delegated-agent lifecycle
4. transcript artifacts dedicated to delegated agents

The UI should present waiting when the parent is blocked on children. Internally, that blocked state is driven by a wait primitive and lifecycle events rather than by heuristics over generic tool calls.

## Chosen Model

### 1. Structured task tool

Replace the current UX dependence on generic `spawn` with a structured delegated-agent tool, likely named `task`.

Required input:

- `description`: short user-visible summary, 3-8 words
- `prompt`: full delegated task text
- `agent_type`: role such as `explorer`

Optional input:

- `model`
- `reasoning_effort`

This tool should:

- allocate a stable child run id
- assign a user-visible child agent name
- emit a spawn lifecycle event immediately
- launch the child in the background
- return control to the parent quickly instead of blocking until the child finishes

Compatibility note:

- the existing `spawn` tool can remain as a compatibility alias or be rewritten internally to call the new delegated-agent path
- the new transcript UX should be keyed off delegated-agent events, not off a special-case tool title hack

### 2. Wait primitive

Add a wait primitive, likely `wait`, that blocks the parent only when the parent truly needs child results.

Expected behavior:

- wait for all outstanding delegated-agent runs in the current parent run, or for an explicit subset
- emit a wait-start event with the outstanding agent list
- block until those children complete
- emit a wait-finished event containing final per-agent summaries

The transcript should show waiting only when the parent is actually waiting, not merely because children exist.

### 3. Backend child-run model

Add a runtime structure for parent-owned child runs.

Each child run should track:

- `id`
- `name`
- `agent_type`
- `model`
- `reasoning_effort`
- `description`
- `prompt`
- `status`: spawned, running, completed, error
- `summary`: compact final result text
- `error`
- timestamps

This state belongs to the parent run lifecycle, not to a user-navigable session model.

### 4. Event model

Add first-class lifecycle events through the gateway. Recommended event surface:

- `agent.spawned`
- `agent.completed`
- `agent.wait.started`
- `agent.wait.completed`

Recommended payloads:

#### `agent.spawned`

- `id`
- `name`
- `agentType`
- `model`
- `reasoningEffort`
- `description`
- `promptPreview`

#### `agent.completed`

- `id`
- `name`
- `agentType`
- `status`
- `summary`
- `error`

This event is primarily for backend/TUI state synchronization. It does not need to render a standalone transcript block in Phase 1.

#### `agent.wait.started`

- `count`
- `agents`: list of `{id, name, agentType}`

#### `agent.wait.completed`

- `count`
- `results`: list of `{id, name, agentType, status, summary, error}`

### 5. TUI transcript artifacts

Add explicit delegated-agent artifacts separate from generic tool blocks.

Phase 1 artifact types:

- `SpawnedAgentArtifact`
- `WaitingAgentsArtifact`
- `FinishedWaitingArtifact`

Rendering goals:

#### Spawned

Example:

```text
• Spawned Bernoulli [explorer] (gpt-5.4 high)
  └ Spec review Gate 6 of usage-tracking-v2 ...
```

Rules:

- one line for identity
- one indented truncated line for delegated task description/prompt preview
- persistent in transcript after emission

#### Waiting

Example:

```text
• Waiting for 3 agents
  └ Bernoulli [explorer]
    Averroes [explorer]
    Curie [explorer]
```

Rules:

- only emitted when the parent explicitly waits
- should read as a distinct orchestration artifact, not as a generic tool block

#### Finished waiting

Example:

```text
• Finished waiting
  └ Bernoulli [explorer]: Completed - ✅ Spec compliant
    Averroes [explorer]: Completed - ✅ Approved
```

Rules:

- completion summary is shown here, not as a live child tool stream
- each child summary is compact and truncated if needed
- if a child failed, render a concise error variant

## UX Rules

### Out of scope for Phase 1

Do not implement:

- child-session switching
- nested live child tool updates in the parent transcript
- streamed child thought/progress content
- deep expansion into child internals
- a separate “agent management” panel

### Summary behavior

The only retained child detail in the transcript should be:

- what was delegated
- who it was delegated to
- the compact final completion summary

That is the Codex-like behavior we want for Phase 1.

## Backend/Frontend Boundary

### Backend responsibility

- own child run lifecycle
- assign display metadata
- know when the parent is waiting
- produce final compact summaries
- emit dedicated lifecycle events

### Frontend responsibility

- persist and render delegated-agent transcript artifacts
- keep them visually distinct from generic tool blocks
- update waiting/finished state deterministically from events

The frontend should not infer agent orchestration from tool names or raw JSON.

## Testing Strategy

### Backend

Add tests for:

- structured delegated task creation
- background child run registration
- wait behavior over multiple children
- final summary aggregation
- gateway event emission order and payloads

### TUI

Add tests for:

- spawned artifact rendering
- waiting artifact rendering
- finished-waiting artifact rendering
- truncation rules
- coexistence with existing chat/tool blocks

## Risks

### 1. Overloading generic tool UX

Risk:

- trying to fake Codex behavior by only retitling tool blocks

Mitigation:

- add first-class delegated-agent artifacts and event types

### 2. Parent/child state races

Risk:

- child completes before wait begins or before UI sees spawned state

Mitigation:

- keep parent-owned child-run state in the backend and emit deterministic payloads

### 3. Scope creep into session navigation

Risk:

- accidentally reproducing OpenCode’s child-session management complexity

Mitigation:

- keep child runs as parent-run artifacts only

## Recommendation

Implement a dedicated delegated-agent orchestration layer with:

- structured `task`
- explicit `wait`
- first-class delegated-agent events
- Codex-style transcript artifacts

This is the smallest design that can plausibly achieve the UX target without bolting hacks onto the existing synchronous `spawn` tool.
