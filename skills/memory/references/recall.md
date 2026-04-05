# Startup Recall

## Goal

Recall only the smallest amount of memory that can materially improve the current session.

## Default Recall Policy

At session start:

1. determine the active project or repo
2. assume project-scoped recall first
3. recall only a small shortlist
4. summarize the shortlist instead of dumping raw memory rows

## Preferred Retrieval Order

1. targeted `mcp_hybrid-memory_memory_search`
2. `mcp_hybrid-memory_memory_semantic` only if keyword search is weak or ambiguous
3. `mcp_hybrid-memory_memory_get` only for shortlisted items that need detail

## Good Startup Recall Cases

- continuing work in a known repo with recent RCA or design history
- resuming an interrupted debugging thread
- returning to a project with conventions that affect implementation
- working on a system where prior benchmarks or failure boundaries matter

## Bad Startup Recall Cases

- casual one-off shell tasks
- generic questions with no repo or project context
- broad exploratory work where no stable project is in play

## Output Shape

Startup recall should produce:

- a compact shortlist
- 2-5 actionable bullets at most
- emphasis on decisions, errors, patterns, and conventions

Not:

- raw rows
- long memory dumps
- every matching result
