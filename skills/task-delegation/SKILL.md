---
name: task-delegation
description: When and how to use the task and wait tools to delegate sub-tasks. Use when a problem has independent sub-tasks that can be worked concurrently.
always: false
---

## Task Delegation

Delegate independent sub-tasks to background child agents using the `task` tool, then collect results with `wait`.

### The `task` Tool

**Parameters:**
```
task(description="label", prompt="instructions", agent_type="explorer", model="optional", reasoning_effort="optional")
```

| Parameter | Required | Description |
|-----------|----------|-------------|
| `description` | Yes | Short label shown in UI and logs |
| `prompt` | Yes | Full instructions for the child agent |
| `agent_type` | Yes | Type hint; defaults to "explorer" if empty |
| `model` | No | Override model for this task |
| `reasoning_effort` | No | "low", "medium", or "high" |

**Returns:** A result with `agentID` in metadata:
```json
{
  "Output": "delegated X to Y",
  "Metadata": {
    "agentID": "task:parent-session:uuid",
    "agentName": "Curie",
    ...
  }
}
```

### The `wait` Tool

**Parameters:**
```
wait(agent_ids=["task:parent-session:uuid"])  // optional
```

If `agent_ids` is omitted, waits for **all** outstanding child agents.

**Returns:**
```json
{
  "Output": "finished waiting for N agent(s)",
  "Metadata": {
    "count": 2,
    "results": [
      {
        "id": "task:...",
        "name": "Curie",
        "status": "completed",
        "summary": "child agent output..."
      }
    ]
  }
}
```

### Identifying Parallelizable Sub-tasks

Sub-tasks are parallelizable when:
1. They do not depend on each other's output
2. They can start with information available now
3. Results can be merged afterwards

Good candidates:
- "fetch these 5 URLs independently"
- "run these 3 searches in parallel"
- "analyze each of these 4 log files"

Bad candidates (sequential required):
- "debug X, then write a fix" — fix depends on debug result
- "check auth, then access data" — second depends on first

### Writing Clear Task Prompts

The child agent receives **only** what is in `prompt`. Write complete, self-contained instructions:

**Bad:**
```
check the logs
```

**Good:**
```
Read /var/log/app.log. Find all ERROR-level lines from the past 24 hours.
Return a JSON array with fields: timestamp, message, stack_trace (if present).
```

Include:
- Exact file paths or URLs
- Expected output format
- What to return (the `summary` field in wait results)

### Child Agent Capabilities

Child agents have **limited tools**:
- Disabled: `message`, `spawn`, `task`
- Enabled: file operations, grep, read, web_search, web_fetch, etc.

Children **cannot send messages** to users or spawn further agents.

### Iteration Limit

Children are limited to **15 iterations** max. Keep tasks focused enough to complete within this limit.

### Handling Results

Check the `count` field in wait results. Iterate over `results`:

```go
for _, r := range results {
    switch r.Status {
    case "completed":
        // use r.Summary
    case "error":
        // handle r.Error
    case "running":
        // should not happen after wait returns
    }
}
```

If 3 of 4 tasks succeeded, report the 3 successes and surface the failure for the 4th.

### Error Handling

If `task` returns `"spawner unavailable"` or `"session key required"` — these are infrastructure errors. Report to the user rather than retrying.

If a child times out or errors, `wait` still returns partial results. Surface the failure in your response.

### Worked Example — Good Decomposition

**Problem:** "Summarize what changed in these 4 GitHub repos"

**Good:**
```python
# Spawn 4 tasks in parallel
task(description="summarize repo A", prompt="...", agent_type="explorer")
task(description="summarize repo B", prompt="...", agent_type="explorer")
task(description="summarize repo C", prompt="...", agent_type="explorer")
task(description="summarize repo D", prompt="...", agent_type="explorer")

# Wait for all and merge
results = wait()
```

**Bad:** One task fetching all 4 repos sequentially (no parallelism benefit).

### Worked Example — Bad Decomposition

**Problem:** "Debug why the API is slow and then write a fix"

**Bad:**
```python
task(description="debug API", prompt="...", agent_type="explorer")
task(description="write fix", prompt="...", agent_type="explorer")  # Can't start until debug done!
```

**Correct:** Debug sequentially, then delegate fix writing only after root cause is known.
