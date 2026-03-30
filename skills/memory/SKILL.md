---
name: memory
description: Store and retrieve persistent memories using HybridMemory MCP. Use when starting a session, making decisions, completing tasks, learning something new, or any time context should persist across sessions.
---

Use this skill to store and retrieve persistent memories across sessions.

## When to Use

### Storing Memories
Use `memory_store` when:
- You learn something new about the project or codebase
- You make a significant decision (architectural, design, or implementation choice)
- You encounter and resolve an error (capture the problem and solution)
- You notice a pattern that would be useful to remember
- User expresses a preference for how things should work
- You want to maintain context across sessions

### Searching Memories
Use `memory_search` for full-text search of stored memories.
Use `memory_semantic` for semantic/meaning-based search.

### Retrieving Memories
Use `memory_get` to retrieve a specific memory by ID.
Use `memory_stats` to see memory statistics.

## Memory Categories

- **learning**: New information about the codebase, patterns, or domain
- **error**: Bugs encountered and their solutions
- **decision**: Architectural or design decisions made
- **pattern**: Useful code patterns or idioms discovered
- **preference**: User preferences for how things should work
- **context**: Session context that should persist

## Importance Levels

- **permanent**: Never expires, critical information
- **stable**: Valid until something changes (e.g., API version)
- **active**: Current session relevant
- **session**: Short-term, for current work
- **checkpoint**: Milestone markers

## Example Usage

Store a learning:
```
memory_store(text="Next.js App Router uses Server Components by default", category="learning", importance="stable")
```

Store a decision:
```
memory_store(text="Using PostgreSQL for all persistent data, Redis for caching", category="decision", importance="permanent")
```

Store an error solution:
```
memory_store(text="M1 Mac: brew install postgresql@14 instead of postgresql", category="error", importance="stable")
```
