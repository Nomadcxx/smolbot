# Plan 4: Concurrent Tool Execution + Tool Deferment

**Created**: 2026-04-02  
**Status**: Draft  
**Goal**: Faster execution via parallelism + smaller prompts via deferment

---

## Part 1: Concurrent Tool Execution

### Evidence: Current Ollama Models (from `ollama list`)

```
minimax-m2.7:cloud, nemotron-3-super:cloud, glm-5:cloud, qwen3.5:397b-cloud,
kimi-k2.5:cloud, deepseek-v3.2:cloud, gemini-3-flash-preview:cloud, etc.
```

These `:cloud` models are cloud-proxied and support parallel tool calls.

### Model Support Matrix (Evidence-Based)

| Provider | Parallel Tool Calls | How We Know | Config Source |
|----------|---------------------|-------------|---------------|
| **OpenAI** | ✅ Yes | API docs: `parallel_tool_calls: true` default | Provider default |
| **Anthropic** | ✅ Yes | Claude 3+ returns multiple `tool_use` blocks | Provider default |
| **Azure OpenAI** | ✅ Yes | Same as OpenAI | Provider default |
| **Ollama Local** | ⚠️ Model-specific | Depends on underlying model | Model metadata |
| **Ollama Cloud** | ✅ Yes (cloud models) | GPT/Claude/Gemini proxied | Check `:cloud` suffix |
| **DeepSeek** | ✅ Yes | OpenAI-compatible | Provider default |
| **Groq** | ✅ Yes | OpenAI-compatible | Provider default |
| **vLLM** | ⚠️ Model-specific | Depends on model loaded | Query /v1/models |

### Key Insight: Detection Strategy

Instead of hardcoding, **detect capability dynamically**:

1. **Provider-level defaults** (most providers support it)
2. **Model-level override** (Ollama local models may not)
3. **Runtime detection** (check model metadata if available)

### Design: Model Capability Flag

```go
// pkg/provider/types.go

type ModelInfo struct {
    ID                    string
    Name                  string
    Provider              string
    ContextWindow         int
    MaxOutputTokens       int
    // NEW: Capability flags
    SupportsParallelTools bool   // Can return multiple tool calls
    SupportsVision        bool   // Can process images
    SupportsStreaming     bool   // Supports streaming responses
}

// pkg/provider/registry.go

func (r *Registry) GetModelCapabilities(modelID string) ModelCapabilities {
    // Check provider-level defaults
    provider := extractProvider(modelID)
    caps := defaultCapabilities[provider]
    
    // Override with model-specific settings if known
    if modelCaps, ok := modelCapabilityOverrides[modelID]; ok {
        return modelCaps
    }
    return caps
}

var defaultCapabilities = map[string]ModelCapabilities{
    "openai":    {SupportsParallelTools: true},
    "anthropic": {SupportsParallelTools: true},
    "azure":     {SupportsParallelTools: true},
    "deepseek":  {SupportsParallelTools: true},
    "groq":      {SupportsParallelTools: true},
    "ollama":    {SupportsParallelTools: false}, // Conservative default
    "vllm":      {SupportsParallelTools: false}, // Conservative default
}
```

### Design: Tool Concurrency Safety

Not all tools are safe to run in parallel:

| Tool | Concurrent Safe? | Reason |
|------|------------------|--------|
| `read` | ✅ Yes | Read-only, no side effects |
| `list_dir` | ✅ Yes | Read-only |
| `web_search` | ✅ Yes | Independent requests |
| `web_fetch` | ✅ Yes | Independent requests |
| `edit` | ❌ No | Modifies files, race conditions |
| `write` | ❌ No | Modifies files |
| `exec` | ⚠️ Depends | Could have side effects |
| `spawn` | ✅ Yes | Independent agents |
| `message` | ✅ Yes | Independent channel sends |

```go
// pkg/tool/tool.go

type ToolDef struct {
    Name        string
    Description string
    Parameters  any
    Execute     func(...) (*Result, error)
    
    // NEW: Concurrency control
    ConcurrencySafe bool  // true = can run in parallel with other safe tools
}

// Example registrations
var ReadTool = ToolDef{
    Name:            "read",
    ConcurrencySafe: true,  // Safe to parallelize
    // ...
}

var EditTool = ToolDef{
    Name:            "edit",
    ConcurrencySafe: false, // Must run exclusively
    // ...
}
```

### Design: Parallel Executor

```go
// pkg/agent/executor.go (NEW FILE)

type ToolExecutor struct {
    tools           *tool.Registry
    parallelEnabled bool
}

type toolExecution struct {
    call     provider.ToolCall
    result   *tool.Result
    err      error
    safe     bool
}

func (e *ToolExecutor) ExecuteAll(
    ctx context.Context,
    calls []provider.ToolCall,
    toolCtx tool.ToolContext,
    emit func(Event),
) []provider.Message {
    
    if !e.parallelEnabled || len(calls) == 1 {
        // Sequential fallback
        return e.executeSequential(ctx, calls, toolCtx, emit)
    }
    
    // Classify tools by safety
    var safeCalls, unsafeCalls []provider.ToolCall
    for _, call := range calls {
        if e.tools.IsConcurrencySafe(call.Function.Name) {
            safeCalls = append(safeCalls, call)
        } else {
            unsafeCalls = append(unsafeCalls, call)
        }
    }
    
    var results []provider.Message
    
    // Execute safe tools in parallel
    if len(safeCalls) > 0 {
        results = append(results, e.executeParallel(ctx, safeCalls, toolCtx, emit)...)
    }
    
    // Execute unsafe tools sequentially AFTER safe ones complete
    if len(unsafeCalls) > 0 {
        results = append(results, e.executeSequential(ctx, unsafeCalls, toolCtx, emit)...)
    }
    
    return results
}

func (e *ToolExecutor) executeParallel(
    ctx context.Context,
    calls []provider.ToolCall,
    toolCtx tool.ToolContext,
    emit func(Event),
) []provider.Message {
    
    var wg sync.WaitGroup
    results := make([]toolExecution, len(calls))
    
    for i, call := range calls {
        wg.Add(1)
        go func(idx int, tc provider.ToolCall) {
            defer wg.Done()
            
            emit(Event{Type: EventToolStart, Content: tc.Function.Name})
            
            result, err := e.tools.Execute(ctx, tc.Function.Name, 
                json.RawMessage(tc.Function.Arguments), toolCtx)
            
            results[idx] = toolExecution{
                call:   tc,
                result: result,
                err:    err,
            }
            
            emit(Event{Type: EventToolDone, Content: tc.Function.Name})
        }(i, call)
    }
    
    wg.Wait()
    
    // Convert to messages in original order
    return e.toMessages(results)
}
```

### Integration in Agent Loop

```go
// pkg/agent/loop.go - Modify ProcessDirect

func (a *AgentLoop) ProcessDirect(...) {
    // Determine if parallel execution is enabled for this model
    modelCaps := a.registry.GetModelCapabilities(req.Model)
    
    executor := &ToolExecutor{
        tools:           a.tools,
        parallelEnabled: modelCaps.SupportsParallelTools,
    }
    
    // ... existing code ...
    
    if len(resp.ToolCalls) > 0 {
        // NEW: Use executor instead of sequential loop
        toolMessages := executor.ExecuteAll(runCtx, resp.ToolCalls, toolCtx, cb)
        conversation = append(conversation, toolMessages...)
        newMessages = append(newMessages, toolMessages...)
    }
}
```

---

## Part 2: Tool Deferment

### Evidence: Claude Code Implementation

**Source**: `/home/nomadx/claude-code/src/tools/ToolSearchTool/`

#### Tools Marked as `shouldDefer: true` in Claude Code:

| Tool | Why Deferred |
|------|--------------|
| `TaskCreateTool` | Advanced workflow |
| `TodoWriteTool` | Specialized |
| `LSPTool` | Language-specific |
| `CronCreateTool`, `CronListTool`, `CronDeleteTool` | Scheduling |
| `ListMcpResourcesTool` | MCP-specific |
| `WebSearchTool` | External API |
| `WebFetchTool` | External API |
| `SendMessageTool` | Channel-specific |
| `AskUserQuestionTool` | Interactive |
| `ConfigTool` | Admin |
| `TeamDeleteTool` | Team mode |
| `TaskUpdateTool`, `TaskGetTool`, `TaskOutputTool`, `TaskListTool`, `TaskStopTool` | Task management |
| `RemoteTriggerTool` | Remote |
| `EnterWorktreeTool`, `ExitWorktreeTool` | Git worktree |
| `ExitPlanModeTool` | Mode switching |
| **All MCP tools** | External/workflow-specific |

#### Deferment Logic (`prompt.ts:62-108`):

```typescript
export function isDeferredTool(tool: Tool): boolean {
  // 1. Explicit opt-out via alwaysLoad
  if (tool.alwaysLoad === true) return false

  // 2. MCP tools are ALWAYS deferred
  if (tool.isMcp === true) return true

  // 3. Never defer ToolSearch itself (meta-tool)
  if (tool.name === TOOL_SEARCH_TOOL_NAME) return false

  // 4. Some tools must be available turn 1 (feature flags)
  if (feature('FORK_SUBAGENT') && tool.name === AGENT_TOOL_NAME) return false

  // 5. Check explicit shouldDefer flag
  return tool.shouldDefer === true
}
```

#### ToolSearch Query Format:

```
- "select:Read,Edit,Grep"       → fetch exact tools by name
- "notebook jupyter"            → keyword search, max_results matches
- "+slack send"                 → require "slack" in name, rank by other terms
```

#### Search Scoring Algorithm (`ToolSearchTool.ts:259-290`):

```typescript
// Score calculation for each candidate tool:
let score = 0
for (const term of allScoringTerms) {
  // Exact part match (high weight)
  if (parsed.parts.includes(term)) {
    score += parsed.isMcp ? 12 : 10  // MCP tools weighted higher
  } else if (parsed.parts.some(part => part.includes(term))) {
    score += parsed.isMcp ? 6 : 5
  }
  
  // Description match
  if (pattern.test(descNormalized)) score += 2
  
  // Search hint match
  if (hintNormalized && pattern.test(hintNormalized)) score += 2
}
```

### OpenCode: No Tool Deferment

After searching `/home/nomadx/.cache/opencode/node_modules/oh-my-opencode/`, **opencode does NOT implement tool deferment**. All tools are sent to the model every request.

This confirms Claude Code's approach is more sophisticated.

```go
// pkg/tool/tool.go

type ToolDef struct {
    Name            string
    Description     string
    Parameters      any
    Execute         func(...) (*Result, error)
    ConcurrencySafe bool
    
    // NEW: Deferment control
    Deferred   bool     // If true, hidden until discovered
    AlwaysLoad bool     // If true, always show (bypass deferment)
    Keywords   []string // Search keywords for discovery
}
```

### Tool Classification

| Tool | Deferred? | Keywords | Rationale |
|------|-----------|----------|-----------|
| `read` | No | - | Core, always needed |
| `edit` | No | - | Core, always needed |
| `exec` | No | - | Core, always needed |
| `list_dir` | No | - | Core, always needed |
| `write` | Yes | file, create, new | Less common |
| `web_search` | Yes | search, web, internet | Specialized |
| `web_fetch` | Yes | fetch, url, http, web | Specialized |
| `spawn` | Yes | agent, parallel, delegate | Advanced |
| `task` | Yes | agent, task, delegate | Advanced |
| `wait` | Yes | agent, wait | Advanced |
| `cron` | Yes | schedule, cron, timer | Specialized |
| `message` | Yes | message, channel, send | Specialized |
| MCP tools | Yes | (from MCP server) | External |

### Design: ToolSearch Meta-Tool

```go
// pkg/tool/search.go (NEW FILE)

var ToolSearchTool = ToolDef{
    Name:        "tool_search",
    Description: "Search for additional tools by keyword. Use when you need capabilities not visible in current tools.",
    AlwaysLoad:  true,  // Always visible
    Parameters: map[string]any{
        "type": "object",
        "properties": map[string]any{
            "query": map[string]any{
                "type":        "string",
                "description": "Keywords to search for (e.g., 'web', 'file creation', 'scheduling')",
            },
        },
        "required": []string{"query"},
    },
    Execute: func(ctx context.Context, input json.RawMessage, toolCtx tool.ToolContext) (*Result, error) {
        var params struct {
            Query string `json:"query"`
        }
        json.Unmarshal(input, &params)
        
        // Find matching deferred tools
        matches := toolCtx.Registry.SearchDeferredTools(params.Query)
        
        if len(matches) == 0 {
            return &Result{
                Output: "No additional tools found matching query.",
            }, nil
        }
        
        // Add to session's discovered tools
        toolCtx.DiscoverTools(matches)
        
        // Format response
        var lines []string
        lines = append(lines, fmt.Sprintf("Found %d tools:", len(matches)))
        for _, t := range matches {
            lines = append(lines, fmt.Sprintf("- %s: %s", t.Name, t.Description))
        }
        lines = append(lines, "\nThese tools are now available for use.")
        
        return &Result{
            Output: strings.Join(lines, "\n"),
        }, nil
    },
}
```

### Design: Discovered Tools State

```go
// pkg/tool/registry.go

type Registry struct {
    tools     map[string]ToolDef
    mu        sync.RWMutex
}

// GetVisibleTools returns tools for the current context
func (r *Registry) GetVisibleTools(discovered map[string]bool) []ToolDef {
    r.mu.RLock()
    defer r.mu.RUnlock()
    
    var visible []ToolDef
    for _, t := range r.tools {
        // Include if:
        // 1. Not deferred (core tools)
        // 2. AlwaysLoad (meta-tools like tool_search)
        // 3. Previously discovered in this session
        if !t.Deferred || t.AlwaysLoad || discovered[t.Name] {
            visible = append(visible, t)
        }
    }
    return visible
}

// SearchDeferredTools finds deferred tools matching query
func (r *Registry) SearchDeferredTools(query string) []ToolDef {
    r.mu.RLock()
    defer r.mu.RUnlock()
    
    query = strings.ToLower(query)
    words := strings.Fields(query)
    
    var matches []ToolDef
    for _, t := range r.tools {
        if !t.Deferred {
            continue // Skip non-deferred
        }
        
        // Check name, description, keywords
        searchText := strings.ToLower(t.Name + " " + t.Description + " " + strings.Join(t.Keywords, " "))
        
        for _, word := range words {
            if strings.Contains(searchText, word) {
                matches = append(matches, t)
                break
            }
        }
    }
    return matches
}
```

### Session-Level Discovery State

```go
// pkg/agent/loop.go

type AgentLoop struct {
    // ... existing fields ...
    discoveredTools map[string]bool  // Tools discovered via tool_search
}

func (a *AgentLoop) ProcessDirect(...) {
    // Build tool list for this request
    visibleTools := a.tools.GetVisibleTools(a.discoveredTools)
    
    // ... use visibleTools in ChatRequest ...
}

// Called by tool_search execution
func (a *AgentLoop) DiscoverTools(tools []ToolDef) {
    for _, t := range tools {
        a.discoveredTools[t.Name] = true
    }
}
```

---

## Implementation Phases

### Phase 37: Model Capability Flags
- Add `SupportsParallelTools` to `ModelInfo`
- Create `defaultCapabilities` map per provider
- Expose via `GetModelCapabilities()`

### Phase 38: Tool Concurrency Safety
- Add `ConcurrencySafe` field to `ToolDef`
- Mark each tool appropriately
- Add `IsConcurrencySafe()` to Registry

### Phase 39: Parallel Executor
- Create `pkg/agent/executor.go`
- Implement `ExecuteAll()` with parallel/sequential logic
- Add `sync.WaitGroup` based parallel execution

### Phase 40: Agent Loop Integration
- Modify `ProcessDirect` to use `ToolExecutor`
- Pass model capabilities from registry
- Preserve event emission order

### Phase 41: Tool Deferment Fields
- Add `Deferred`, `AlwaysLoad`, `Keywords` to `ToolDef`
- Update all tool registrations
- Implement `GetVisibleTools()`

### Phase 42: ToolSearch Tool
- Create `pkg/tool/search.go`
- Implement keyword matching
- Register as AlwaysLoad tool

### Phase 43: Discovery State
- Add `discoveredTools` map to `AgentLoop`
- Wire `DiscoverTools()` callback
- Persist discovered tools in session (optional)

---

## Files to Modify/Create

| File | Changes |
|------|---------|
| `pkg/provider/types.go` | Add `SupportsParallelTools` to `ModelInfo` |
| `pkg/provider/registry.go` | Add `GetModelCapabilities()`, `defaultCapabilities` |
| `pkg/tool/tool.go` | Add `ConcurrencySafe`, `Deferred`, `AlwaysLoad`, `Keywords` |
| `pkg/tool/registry.go` | Add `GetVisibleTools()`, `SearchDeferredTools()`, `IsConcurrencySafe()` |
| `pkg/agent/executor.go` | **NEW** - Parallel tool executor |
| `pkg/agent/loop.go` | Use `ToolExecutor`, add `discoveredTools` |
| `pkg/tool/search.go` | **NEW** - `tool_search` meta-tool |
| `pkg/tool/*.go` | Mark each tool's concurrency safety + deferment |

---

## Success Criteria

- [ ] 5 parallel `read` calls complete in ~1x time (not 5x)
- [ ] `edit` tools always run sequentially
- [ ] Model without parallel support falls back to sequential
- [ ] `tool_search` discovers hidden tools
- [ ] Discovered tools persist within session
- [ ] Prompt size reduced by ~50% for simple queries

---

## Example Interaction

```
User: Read these 5 files and summarize them

[Model returns 5 read tool calls]

[Parallel execution - all 5 run simultaneously]
  → read file1.go (50ms)
  → read file2.go (45ms)  
  → read file3.go (60ms)
  → read file4.go (40ms)
  → read file5.go (55ms)

Total: ~60ms (slowest) instead of ~250ms (sum)
```

```
User: I need to send a message to my WhatsApp

[Model sees: read, edit, exec, list_dir, tool_search]
[Model calls tool_search("message channel")]

tool_search result:
  Found 1 tool:
  - message: Send messages to configured channels (WhatsApp, Signal, etc.)
  
  This tool is now available for use.

[Model now has: read, edit, exec, list_dir, tool_search, message]
[Model calls message(channel="whatsapp", ...)]
```
