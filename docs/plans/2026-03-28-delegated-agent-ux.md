# Delegated Agent UX Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Give smolbot a Codex-style delegated-agent transcript experience with spawned-agent cards, explicit waiting, and finished-waiting summaries, without child-session navigation or live child internals.

**Architecture:** Add a first-class delegated-agent orchestration path in the backend, emit dedicated gateway events for child-agent lifecycle, and teach the TUI transcript to render delegated-agent artifacts separately from generic tool blocks. Keep Phase 1 intentionally narrow: background child runs, explicit wait, compact summaries, and deterministic transcript state.

**Tech Stack:** Go, Bubble Tea, Lip Gloss, smolbot gateway websocket events, existing agent/tool runtime

---

## Reference Notes

This plan is intentionally informed by three sources:

1. **Codex UX reference**
   - target behavior is compact transcript artifacts for spawned agents, explicit waiting, and explicit finished-waiting summaries
   - the user-visible feel we want is “background delegation without transcript freeze”
   - OpenAI’s public Codex overview explicitly frames delegated work as background execution in isolated environments rather than as blocking inline work:
     - https://help.openai.com/en/articles/11369540/

2. **OpenCode backend reference**
   - OpenCode’s `task` tool is the better backend model than smolbot’s current `spawn`
   - key ideas to borrow:
     - structured delegated input (`description`, `prompt`, `subagent_type`)
     - child work tracked as an independent long-running artifact
     - compact metadata summaries available to the parent artifact
   - key idea to reject for Phase 1:
     - child-session navigation

3. **External async-task UX guidance**
   - long-running delegated work must not feel like a frozen UI
   - explicit state transitions matter more than decorative progress
   - indeterminate waiting is acceptable if the state is honest, visible, and stable
   - useful references:
     - https://www.nngroup.com/articles/progress-indicators/
     - https://m3.material.io/components/progress-indicators/overview

## Non-Negotiable Phase 1 Invariants

The implementation is out of scope if it introduces any of the following:

- child-session switching
- live nested child tool reporting in the parent transcript
- streamed child reasoning/progress text in the parent transcript
- deep expansion into child internals
- heuristic UI inference based only on tool names like `spawn`

The implementation is incomplete unless it satisfies all of the following:

- child delegation appears immediately in the transcript as a dedicated artifact
- the parent no longer appears frozen while children run
- explicit waiting is shown only when the parent is actually blocked on children
- completion summaries appear only after the child is complete
- generic tool rendering remains intact for non-agent tools

## Execution Workflow

Use the agreed subagent-driven workflow for this plan:

1. implement one task slice at a time in an isolated worktree
2. run the task’s focused verification before claiming completion
3. run a spec-compliance review gate for that task
4. run a code-quality review gate for that task
5. only checkpoint/commit the task after both review gates are clear
6. after all tasks are complete, run the final verification matrix
7. run a final whole-change spec review
8. run a final whole-change code-quality review
9. only then push and/or merge

### Per-Task Gate Template

Every task below should be treated as incomplete until all of the following are true:

- focused tests/build for that task are green
- spec review returns no blocking findings
- code-quality review returns no blocking findings
- the resolved slice is checkpointed in a commit

### Final Closure Gate

The implementation loop closes only when all of the following are true:

- every task in this plan is implemented
- the final verification matrix is green
- final spec review returns no blocking findings
- final code-quality review returns no blocking findings
- the branch is pushed

---

## Event Contract And Ordering Rules

These ordering guarantees should be treated as design requirements, not implementation suggestions.

### Spawn ordering

When a delegated child is created:

1. backend registers child-run state
2. gateway emits `agent.spawned`
3. child run begins in background

The parent transcript must never observe a child completion for an unknown child id.

### Wait ordering

When the parent blocks on children:

1. backend determines the exact outstanding child set for the wait
2. gateway emits `agent.wait.started`
3. backend waits for those children
4. gateway emits `agent.wait.completed`

Do not emit `agent.wait.started` if there are zero outstanding children.

### Completion ordering

For each child:

1. child finishes
2. backend stores final normalized summary
3. gateway emits `agent.completed`

Phase 1 UI rule:

- `agent.completed` updates state but does not create noisy standalone transcript spam
- final transcript-visible completion summaries are rendered by `agent.wait.completed`

## Delegated-Agent Display Rules

These rules exist to avoid drifting into OpenCode’s heavier session/task display.

### Spawn card

Must include:

- visible verb `Spawned`
- child display name
- bracketed role, e.g. `[explorer]`
- model/reasoning suffix when known
- one nested truncated task summary line

Must not include:

- raw JSON
- child session ids
- child tool details

### Waiting card

Must include:

- visible verb `Waiting`
- exact child count
- indented list of outstanding child names and roles

Must not include:

- fake percentages
- animated pseudo-progress disconnected from real state

### Finished-waiting card

Must include:

- visible verb `Finished waiting`
- one nested line per completed child in that wait set
- compact success/error summary per child

Must not include:

- raw child transcript
- child tool-by-tool replay

## Summary Generation Rules

Phase 1 should use a deliberately simple summary contract to avoid overbuilding.

### Delegated task preview

Preferred source priority:

1. `description`
2. truncated `prompt`

Rules:

- normalize whitespace
- single logical preview line
- keep useful path/model/task detail if present
- truncate with ellipsis at the renderer boundary, not inside backend storage

### Child completion summary

Preferred source priority:

1. explicit normalized summary returned by supervisor/child result
2. fallback truncated final child text
3. fallback normalized error text

Rules:

- backend stores normalized plain text summary
- renderer truncates for width
- do not persist ANSI or markdown-heavy formatting in summaries

## Recommended File Decomposition

To keep the implementation reviewable and reduce conflict risk, prefer the following structure:

- `pkg/tool/task.go`
  - delegated-task tool contract
- `pkg/tool/wait.go`
  - wait tool contract
- `cmd/smolbot/runtime_agents.go`
  - child-run supervisor, waiting state, summary normalization
- `pkg/gateway/server.go`
  - websocket emission only
- `internal/components/chat/agentblock.go`
  - delegated-agent artifact rendering

Avoid concentrating all new logic into:

- `cmd/smolbot/runtime.go`
- `internal/tui/tui.go`

If a new helper file is unnecessary in practice, keep the split logical anyway: backend orchestration and UI rendering should remain easy to review independently.

### Task 1: Define delegated-agent runtime types and event payloads

**Files:**
- Modify: `pkg/agent/types.go`
- Modify: `internal/client/types.go`
- Test: `pkg/gateway/protocol_test.go`
- Test: `internal/tui/tui_test.go`

**Step 1: Write failing protocol tests**

Add tests that expect new payload decoding/encoding support for:

- `agent.spawned`
- `agent.completed`
- `agent.wait.started`
- `agent.wait.completed`

Include fields for:

- id
- name
- agentType
- model
- reasoningEffort
- description
- summary

**Step 2: Run tests to verify failure**

Run:

```bash
go test ./pkg/gateway ./internal/tui -run 'Test.*Agent'
```

Expected:

- FAIL because the new payload structs and/or event handling do not exist yet

**Step 3: Add minimal shared payload types**

Add client-visible payload structs in `internal/client/types.go` for:

- spawned agent payload
- completed agent payload
- wait started payload
- wait completed payload

Add agent event type constants in `pkg/agent/types.go` only if the agent loop needs them directly; otherwise keep them gateway-local.

Recommended payloads:

- `AgentSpawnedPayload`
  - `ID`
  - `Name`
  - `AgentType`
  - `Model`
  - `ReasoningEffort`
  - `Description`
  - `PromptPreview`
- `AgentCompletedPayload`
  - `ID`
  - `Name`
  - `AgentType`
  - `Status`
  - `Summary`
  - `Error`
- `AgentWaitStartedPayload`
  - `Count`
  - `Agents []AgentListItem`
- `AgentWaitCompletedPayload`
  - `Count`
  - `Results []AgentResultItem`

**Step 4: Re-run tests**

Run:

```bash
go test ./pkg/gateway ./internal/tui -run 'Test.*Agent'
```

Expected:

- still failing on missing behavior, but payload decoding compiles

**Step 5: Commit**

```bash
git add pkg/agent/types.go internal/client/types.go pkg/gateway/protocol_test.go internal/tui/tui_test.go
git commit -m "feat(agent): add delegated-agent event payloads"
```

**Gate:**
- Run spec review for delegated-agent payload shape and event naming before moving to Task 2.
- Run code-quality review for payload minimality and compatibility before moving to Task 2.

**Implementation Notes:**
- Keep payloads UI-oriented and free of runtime-only internals.
- Do not expose session ids in Phase 1 payloads unless a test proves they are required.

### Task 2: Replace synchronous spawn behavior with structured delegated-task orchestration

**Files:**
- Modify: `pkg/tool/spawn.go`
- Create: `pkg/tool/task.go`
- Modify: `pkg/tool/tool.go` or equivalent registry wiring point
- Modify: `cmd/smolbot/runtime.go`
- Create: `cmd/smolbot/runtime_agents.go` if the supervisor logic becomes non-trivial
- Modify: `pkg/agent/loop.go`
- Test: `pkg/tool/routing_test.go`
- Test: `pkg/agent/loop_test.go`
- Test: `cmd/smolbot/runtime_test.go`

**Step 1: Write failing backend tests**

Cover:

- delegated task accepts `description`, `prompt`, `agent_type`
- child work is launched in background instead of returning final child output synchronously
- parent receives a stable child id and display metadata

**Step 2: Run focused tests**

Run:

```bash
go test ./pkg/tool ./pkg/agent ./cmd/smolbot -run 'Test.*(Task|Spawn|Delegated|Child)'
```

Expected:

- FAIL because the runtime still uses synchronous `spawn`

**Step 3: Implement a dedicated delegated-task tool**

Recommended shape:

- keep `spawn` for compatibility or route it internally to the new path
- add a new `task` tool with structured params:
  - `description`
  - `prompt`
  - `agent_type`
  - optional `model`
  - optional `reasoning_effort`

Tool execution should:

- allocate child id/name
- register child with runtime supervisor
- launch child in background
- return metadata immediately

Recommended child naming strategy:

- use a deterministic display-name allocator backed by a fixed curated list
- names should be user-friendly and stable for a single parent run
- uniqueness only needs to hold within a parent run

Recommended compatibility strategy:

- keep `spawn` available
- either:
  - map `spawn.message` to `task.prompt` with generated description and default `agent_type`, or
  - leave `spawn` functional but do not use it for the new UX path

Avoid deleting `spawn` in Phase 1.

**Step 4: Add backend child-run supervisor in runtime**

In `cmd/smolbot/runtime.go`, add parent-owned child-run tracking that can:

- register child runs
- launch child runs
- mark child completion
- expose outstanding children for wait behavior

Do not add user-navigable child sessions.

Supervisor should also own:

- blocked/unblocked wait state
- completion summary normalization
- thread-safe access to child-run maps
- cleanup after parent completion/cancellation

**Step 5: Re-run focused tests**

Run:

```bash
go test ./pkg/tool ./pkg/agent ./cmd/smolbot -run 'Test.*(Task|Spawn|Delegated|Child)'
```

Expected:

- PASS

**Step 6: Commit**

```bash
git add pkg/tool/spawn.go pkg/tool/task.go cmd/smolbot/runtime.go pkg/agent/loop.go pkg/tool/routing_test.go pkg/agent/loop_test.go cmd/smolbot/runtime_test.go
git commit -m "feat(agent): add structured delegated task orchestration"
```

**Gate:**
- Run spec review for Phase 1 scope control: no child-session navigation and no live child internals.
- Run code-quality review focused on child-run lifecycle, synchronization, and backward compatibility of `spawn`.

**Implementation Notes:**
- Use explicit synchronization around child-run state; do not depend on websocket ordering alone.
- Child completion should be recorded before the completion event is emitted.

### Task 3: Add explicit wait orchestration and backend summaries

**Files:**
- Create: `pkg/tool/wait.go`
- Modify: `cmd/smolbot/runtime.go`
- Modify: `pkg/tool/tool.go` or equivalent registry wiring point
- Test: `pkg/tool/integration_test.go`
- Test: `cmd/smolbot/runtime_test.go`

**Step 1: Write failing wait tests**

Cover:

- waiting for all outstanding child agents
- waiting for a subset if supported
- wait returns final compact per-agent summaries
- parent blocks only during wait, not during spawn

**Step 2: Run focused tests**

Run:

```bash
go test ./pkg/tool ./cmd/smolbot -run 'Test.*Wait'
```

Expected:

- FAIL because the wait tool/path does not exist

**Step 3: Implement wait tool**

The wait tool should:

- inspect outstanding child runs owned by the parent run
- emit wait-started lifecycle state to the supervisor
- block until outstanding children complete
- aggregate final summaries
- return deterministic summary data

Keep it narrow:

- no child transcript replay
- no deep child logs

Recommended contract:

- default behavior waits for all outstanding children owned by the current parent run
- if no children are outstanding, return immediately and emit no waiting artifact
- `agent.wait.completed` should contain a stable snapshot of result summaries in deterministic order

Deterministic order recommendation:

- preserve spawn order within the waited child set

**Step 4: Re-run focused tests**

Run:

```bash
go test ./pkg/tool ./cmd/smolbot -run 'Test.*Wait'
```

Expected:

- PASS

**Step 5: Commit**

```bash
git add pkg/tool/wait.go cmd/smolbot/runtime.go pkg/tool/integration_test.go cmd/smolbot/runtime_test.go
git commit -m "feat(agent): add delegated wait orchestration"
```

**Gate:**
- Run spec review to confirm waiting is explicit and only appears when the parent is actually blocked.
- Run code-quality review focused on wait semantics, deadlock risk, and deterministic summary ordering.

**Implementation Notes:**
- Wait should not rely on polling in the TUI.
- Wait semantics belong in backend orchestration, with the TUI acting as a pure event consumer.

### Task 4: Emit first-class delegated-agent gateway events

**Files:**
- Modify: `pkg/gateway/server.go`
- Test: `pkg/gateway/server_test.go`
- Test: `pkg/gateway/protocol_test.go`
- Test: `pkg/gateway/concurrency_test.go`

**Step 1: Write failing gateway tests**

Cover ordered emission of:

- `agent.spawned`
- `agent.completed`
- `agent.wait.started`
- `agent.wait.completed`

Verify payload contents for:

- identity metadata
- prompt/description preview
- waiting list
- completion summary list

**Step 2: Run focused tests**

Run:

```bash
go test ./pkg/gateway -run 'Test.*Agent'
```

Expected:

- FAIL because these events are not emitted today

**Step 3: Implement gateway mapping**

Teach the gateway to forward delegated-agent supervisor events as dedicated websocket events.

Do not overload `chat.tool.start` / `chat.tool.done` for this new UX.

Recommended event names:

- `agent.spawned`
- `agent.completed`
- `agent.wait.started`
- `agent.wait.completed`

Do not reuse:

- `chat.progress`
- `chat.tool.start`
- `chat.tool.done`

for delegated-agent transcript artifacts.

**Step 4: Re-run focused tests**

Run:

```bash
go test ./pkg/gateway -run 'Test.*Agent'
```

Expected:

- PASS

**Step 5: Commit**

```bash
git add pkg/gateway/server.go pkg/gateway/server_test.go pkg/gateway/protocol_test.go pkg/gateway/concurrency_test.go
git commit -m "feat(gateway): emit delegated-agent lifecycle events"
```

**Gate:**
- Run spec review for event naming and payload completeness versus the design doc.
- Run code-quality review for event ordering, duplicate emission, and race safety.

**Implementation Notes:**
- Gateway should preserve ordering as emitted by the runtime supervisor.
- Event emission should be idempotent enough that reconnect/status refresh does not duplicate transcript artifacts during a single run.

### Task 5: Add transcript artifact types for delegated agents

**Files:**
- Modify: `internal/components/chat/messages.go`
- Modify: `internal/components/chat/message.go`
- Create: `internal/components/chat/agentblock.go`
- Test: `internal/components/chat/messages_test.go`
- Test: `internal/components/chat/message_test.go`

**Step 1: Write failing rendering tests**

Cover:

- spawned-agent artifact render
- waiting artifact render
- finished-waiting artifact render
- truncation of delegated task preview
- truncation of completion summary
- coexistence with normal tool blocks

**Step 2: Run focused tests**

Run:

```bash
go test ./internal/components/chat -run 'Test.*Agent'
```

Expected:

- FAIL because delegated-agent artifact types do not exist

**Step 3: Add first-class transcript artifacts**

Extend the message model so it can store delegated-agent transcript items separately from generic tool calls.

Recommended artifact types:

- `SpawnedAgentArtifact`
- `WaitingAgentsArtifact`
- `FinishedWaitingArtifact`

Add a dedicated renderer in `agentblock.go`.

Recommended renderer rules:

- use a compact bullet-led layout closer to Codex than to the generic tool blocks
- do not wrap identity metadata into dense JSON-like lines
- nested lines should visually align and truncate cleanly on narrow terminals
- completed summaries should remain readable on 80-column terminals

**Step 4: Re-run focused tests**

Run:

```bash
go test ./internal/components/chat -run 'Test.*Agent'
```

Expected:

- PASS

**Step 5: Commit**

```bash
git add internal/components/chat/messages.go internal/components/chat/message.go internal/components/chat/agentblock.go internal/components/chat/messages_test.go internal/components/chat/message_test.go
git commit -m "feat(tui): render delegated-agent transcript artifacts"
```

**Gate:**
- Run spec review to confirm transcript output matches the Codex-style spawned/wait/finished-waiting artifacts agreed in the design.
- Run code-quality review focused on transcript stability, truncation behavior, and coexistence with normal tool blocks.

**Implementation Notes:**
- Keep delegated-agent artifacts separate from `ToolCall`.
- Do not try to fake them by special-casing `spawn` inside the generic tool renderer.

### Task 6: Wire delegated-agent events into the TUI

**Files:**
- Modify: `internal/tui/tui.go`
- Modify: `internal/tui/tui_test.go`

**Step 1: Write failing TUI event-flow tests**

Cover:

- `agent.spawned` appends spawned-agent artifact
- `agent.wait.started` appends waiting artifact
- `agent.wait.completed` appends finished-waiting artifact
- `agent.completed` updates internal state without rendering noisy duplicate artifacts

**Step 2: Run focused tests**

Run:

```bash
go test ./internal/tui -run 'Test.*Agent'
```

Expected:

- FAIL because the new events are not handled

**Step 3: Implement TUI event handling**

In `internal/tui/tui.go`:

- decode new payloads
- update chat message state
- keep delegated-agent transcript artifacts stable and ordered

Do not add a separate side panel or navigation flow.

Recommended handling rules:

- `agent.spawned` appends a persistent spawned artifact immediately
- `agent.completed` updates backend-facing child state only
- `agent.wait.started` appends a waiting artifact
- `agent.wait.completed` appends one finished-waiting artifact and marks the wait as resolved

Avoid:

- appending duplicate completion cards per child
- mutating older spawned artifacts into waiting artifacts
- relying on arrival order between unrelated generic tool events and agent events

**Step 4: Re-run focused tests**

Run:

```bash
go test ./internal/tui -run 'Test.*Agent'
```

Expected:

- PASS

**Step 5: Commit**

```bash
git add internal/tui/tui.go internal/tui/tui_test.go
git commit -m "feat(tui): handle delegated-agent lifecycle events"
```

**Gate:**
- Run spec review for end-to-end transcript behavior across spawned, waiting, and finished-waiting states.
- Run code-quality review for duplicate artifact creation, ordering bugs, and state-reset behavior across runs.

**Implementation Notes:**
- On reconnect or session reset, do not synthesize phantom waiting cards.
- Keep the delegated-agent transcript state scoped to the active run/session history model.

### Task 7: Final verification and review gates

**Files:**
- Review all files touched above

**Step 1: Run feature verification matrix**

Run:

```bash
go test ./pkg/tool ./pkg/gateway ./pkg/mcp ./pkg/channel/... ./pkg/config/... ./pkg/agent ./internal/components/chat ./internal/tui ./cmd/smolbot
```

Expected:

- PASS for feature scope

**Step 2: Run build verification**

Run:

```bash
go build ./cmd/smolbot ./cmd/smolbot-tui ./cmd/installer
```

Expected:

- PASS

**Step 3: Spec review gate**

Review against:

- `docs/plans/2026-03-28-delegated-agent-ux-design.md`
- this implementation plan

Expected:

- no missing spawned/wait/finished-waiting artifacts
- no accidental child-session navigation behavior
- no live child tool stream or deep child internals UI added in Phase 1

**Step 4: Code-quality gate**

Review for:

- race conditions in child-run tracking
- transcript ordering bugs
- duplicate or noisy completion artifacts
- summary truncation regressions

Expected:

- no blocking findings

**Step 5: Regression checklist**

Confirm specifically that:

- ordinary non-agent tools still render through the existing tool-display overhaul
- background child runs do not deadlock daemon shutdown
- parent completion still works when no child agents are spawned
- old `spawn` behavior does not crash older prompts/tool traces
- waiting artifacts do not appear spuriously when no wait occurs

**Step 6: Push/merge gate**

Do not push or merge until:

- Step 1 verification is green
- Step 2 build verification is green
- Step 3 spec review is green
- Step 4 code-quality review is green
- Step 5 regression checklist is manually confirmed

**Step 7: Final completion commit if needed**

```bash
git add <resolved-files>
git commit -m "feat(agent): add codex-style delegated agent ux"
```
