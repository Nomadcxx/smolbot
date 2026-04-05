---
name: memory
description: Use when starting, resuming, or ending non-trivial coding, debugging, RCA, incident analysis, or architecture/design work where prior failures, decisions, conventions, constraints, or durable user or team preferences may matter across sessions and should be recalled from or stored to the MCP-backed hybrid-memory system
requires:
  bins: [node]
---

# Hybrid Memory Ops

## Overview

Use this skill to operate smolbot's MCP-backed `hybrid-memory` integration well.

The core rule is simple: recall small, retrieve in stages, and store only distilled durable leverage.

smolbot exposes the memory server through wrapped MCP tools:

- `mcp_hybrid-memory_memory_search`
- `mcp_hybrid-memory_memory_semantic`
- `mcp_hybrid-memory_memory_store`
- `mcp_hybrid-memory_memory_get`

Additional maintenance tools may also be available:

- `mcp_hybrid-memory_memory_delete`
- `mcp_hybrid-memory_memory_stats`
- `mcp_hybrid-memory_memory_cleanup`

## When To Use

Use this skill when:

- starting or resuming non-trivial coding, debugging, RCA, incident triage, or architecture/design work
- recurring bugs, repeated dead ends, or "we've seen this before" signals suggest prior debugging may matter
- a root cause, failed experiment, benchmark, mitigation, or rejected approach should be checked before repeating work
- making, revisiting, documenting, or explaining an architecture/design decision, tradeoff, or prior rationale
- stable user or team preferences, conventions, constraints, or operating habits are shaping implementation choices
- ending a session that produced durable causes, decisions, benchmarks, conventions, constraints, or preferences worth preserving

Do not use this skill to:

- treat small mechanical tasks as memory-worthy by default
- dump raw session notes into long-term memory
- treat `hybrid-memory` like a transcript archive
- run broad memory searches continuously

## Required Workflow

Always follow this 5-stage workflow:

1. decide whether startup recall is warranted
2. if warranted, do one small startup recall
3. do targeted mid-session lookup when recurring failures appear, an RCA question arises, an architecture/design choice is being revisited, or a durable preference/constraint may change the approach
4. run a brief harvest gate; a candidate passes only if all are true: it will likely help in a future session, it is specific enough to change behavior, it remains true beyond the current task or run, and it can be distilled into a short reusable statement
5. store only candidates that passed the gate and are new, corrective, or materially better than existing memory

Invoking this skill does not automatically require touching the memory backend.
If the startup gate is not met, skip recall.
If no durable outcome emerged, skip store.

## Startup Gate

At the start of a session, ask:

- is this a known repo or recurring system?
- is there likely prior debugging, architecture, or convention context that would materially change the next action?
- can I formulate a narrow project-scoped query instead of a broad search?

If the answer is no, skip recall and work normally.

If the answer is yes, prefer one small shortlist over a memory dump.

## Retrieval Order

Prefer this order:

1. `mcp_hybrid-memory_memory_search` for precise keyword lookups
2. `mcp_hybrid-memory_memory_semantic` only if keyword search is weak or ambiguous
3. `mcp_hybrid-memory_memory_get` only for shortlisted items that need detail

Good startup recall output:

- a compact shortlist
- 2-5 actionable bullets at most
- emphasis on decisions, errors, patterns, and conventions

Bad startup recall output:

- raw rows
- long dumps
- every matching result

## Mid-Session Triggers

Run targeted lookup when:

- a bug or failure pattern repeats
- you suspect "we solved this before"
- a project-specific convention likely exists
- a user preference materially affects execution
- an RCA or architecture decision needs prior context
- a benchmark or prior result may change next steps

Avoid lookup when:

- the task is routine and local
- there is no sign that prior context matters
- the query would be broad and noisy
- you are just filling silence with extra actions

## Harvest Gate

Before storing anything, ask whether the session produced one or more of:

- a validated root cause
- a durable decision with rationale
- a reusable pattern
- a durable project convention
- a user preference that changes future execution
- a benchmark or comparison result that should influence future work

If none of these happened, store nothing.

## Distill Before Store

Before calling `mcp_hybrid-memory_memory_store`:

1. produce compact candidate items
2. remove noise and transient detail
3. store only the final distilled candidates

Do not store:

- raw command output
- transcript-like notes
- "worked on X" summaries
- temporary blockers that do not generalize
- speculative fixes not yet validated
- task status, to-dos, or implementation inventories

## Critical Rules

- Never confuse `hybrid-memory` with local markdown memory files
- Never store raw command output or transcript-like notes
- Never write memory just because a session happened
- Never do broad recall when a small shortlist will do
- Never skip the distillation step before `mcp_hybrid-memory_memory_store`
- Default to zero memory writes unless a concrete durable item is identified
- Prefer a single focused recall or lookup burst; do not keep reformulating searches unless new evidence appears
- Most sessions should produce zero memory writes
- Do not store a candidate that overlaps existing memory unless it materially updates, corrects, or replaces it
- Prefer one precise memory over several overlapping ones
- Before storing, ask: will this still help after the current ticket is forgotten? If not, skip it

## References

Read the reference files as needed:

- `references/recall.md`
- `references/triggers.md`
- `references/harvest.md`

## Bottom Line

`hybrid-memory` should hold distilled future leverage, not session exhaust.
