# Harvest And Store

## Principle

Store only durable, distilled findings that would help a future session act better.

## Harvest Gate

Consider harvest only if the session produced one or more of:

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

If the environment supports delegated summarization, prefer a cheaper/lower-tier agent for the first-pass harvest distillation rather than using a premium model for routine memory cleanup.

## Good Candidates

- validated root causes
- decision rationales
- rejected alternatives that should not be retried blindly
- durable conventions, constraints, or preferences
- benchmark results that should change future choices

## Bad Candidates

- raw command output
- transcript-like notes
- "worked on X" summaries
- temporary blockers that do not generalize
- speculative fixes not yet validated
