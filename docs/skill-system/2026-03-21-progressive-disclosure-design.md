# Progressive Disclosure Skill Loading - Design Document

**Date:** 2026-03-21
**Status:** Design Phase
**Priority:** High

---

## Problem Statement

Currently, all skill content is loaded into the LLM context immediately. With 8+ skills, this consumes significant tokens even when most skills are irrelevant to the current task. We need a system where:

1. Only skill **metadata** is always in context (names, descriptions, availability)
2. Full skill **content** is loaded **on-demand** when the agent decides it's relevant
3. **Bundled resources** are accessible but not loaded into context

---

## Design Goals

1. **Token Efficiency**: Reduce baseline context usage by 80%+
2. **Scalability**: Support 50+ skills without context bloat
3. **Intelligence**: Agent naturally discovers and loads relevant skills
4. **Simplicity**: No new commands or infrastructure
5. **Compatibility**: Works with existing file reading tools

---

## Architecture Overview

### Three-Level Disclosure Model

```
┌─────────────────────────────────────────────────────────────┐
│ Level 1: Metadata (Always in Context)                       │
├─────────────────────────────────────────────────────────────┤
│ - Skill names                                              │
│ - Descriptions (when to use)                               │
│ - Availability status                                        │
│ - Always-on flag                                          │
│                                                             │
│ Size: ~50 tokens per skill                                 │
└─────────────────────────────────────────────────────────────┘
                              │
                              │ Agent decides skill is relevant
                              ▼
┌─────────────────────────────────────────────────────────────┐
│ Level 2: SKILL.md Body (Loaded on-demand)                   │
├─────────────────────────────────────────────────────────────┤
│ - Full instructions                                        │
│ - Workflows                                                │
│ - Examples                                                 │
│ - Tool usage patterns                                       │
│                                                             │
│ Size: 200-1000 tokens per skill                            │
│ Access: Via file reading tool                              │
└─────────────────────────────────────────────────────────────┘
                              │
                              │ Skill references resources
                              ▼
┌─────────────────────────────────────────────────────────────┐
│ Level 3: Bundled Resources (Used as-needed)                 │
├─────────────────────────────────────────────────────────────┤
│ - scripts/ - Executable helpers                             │
│ - references/ - Documentation                              │
│ - assets/ - Templates, images                              │
│                                                             │
│ Size: Variable (not loaded into context)                     │
│ Access: Via execution or file reading                      │
└─────────────────────────────────────────────────────────────┘
```

---

## Detailed Design

### 1. Modified SummaryXML Output

**Current:**
```xml
<available_skills>
  <skill name="weather" status="available">
    Check weather forecasts and conditions
    <!-- Full skill content here -->
  </skill>
</available_skills>
```

**New (Metadata Only):**
```xml
<available_skills>
  <skill name="weather" status="available">
    Check weather forecasts and conditions
  </skill>
  <skill name="github" status="unavailable" reason="missing bin: gh">
    Work with GitHub repositories and pull requests
  </skill>
  <skill name="memory" status="available" always="true">
    Maintain durable memory and history summaries
  </skill>
</available_skills>
```

### 2. Always-On Skills

Skills marked `always: true` are still loaded fully into context (current behavior).

**Rationale:**
- These are core capabilities the agent always needs
- Examples: memory, heartbeat, session management
- Keeps them out of the "load on demand" flow

### 3. Skill Loading Instruction

Add to system prompt:

```markdown
## Available Skills

You have access to specialized skills that can help with specific tasks.
Skills are listed above in <available_skills>.

**When to use a skill:**
- When a task matches the skill's description
- When you need specialized knowledge or workflows
- When the user explicitly asks for skill-related help

**How to use a skill:**
1. Read the skill file: `read_file(workspace/skills/{skill-name}/SKILL.md)`
   (or read_file(templates/SKILL.md) for builtin skills)
2. Follow the instructions in the skill
3. The skill may reference scripts in `skills/{skill-name}/scripts/`

**Always-on skills** (marked with always="true") are already loaded and ready to use.

**Example:**
If the user asks about weather, and you see the "weather" skill is available:
- Read: `read_file(workspace/skills/weather/SKILL.md)`
- Follow the instructions to check weather
```

### 4. Registry Interface Changes

**Current `Registry` interface:**
```go
type Registry interface {
    Names() []string
    Get(name string) (*Skill, bool)
    AlwaysOn() []*Skill
    SummaryXML() string
}
```

**New interface:**
```go
type Registry interface {
    Names() []string
    Get(name string) (*Skill, bool)
    AlwaysOn() []*Skill
    SummaryXML() string              // Metadata only
    LoadContent(name string) (string, error)  // Load full content
    HasResource(skill, resource string) bool
    GetResourcePath(skill, resource string) string
}
```

### 5. Skill Struct Changes

**Current:**
```go
type Skill struct {
    Name              string
    Description       string
    Requires          Requires
    Always            bool
    Content           string    // Full content loaded
    Path              string
    Source            string
    Available         bool
    UnavailableReason string
}
```

**New:**
```go
type Skill struct {
    Name              string
    Description       string
    Requires          Requires
    Always            bool
    Content           string    // Loaded on demand, empty initially
    Path              string
    Source            string    // "builtin" or "workspace"
    Available         bool
    UnavailableReason string
    HasResources      bool      // True if scripts/ references/ assets/ exist
}
```

### 6. Lazy Content Loading

**Option A: Keep full content in memory (simple)**
- Registry loads all content at startup
- `SummaryXML()` only outputs metadata
- `LoadContent()` returns already-loaded content
- Trade-off: Memory usage vs simplicity

**Option B: Lazy file reading (memory efficient)**
- Registry only loads metadata at startup
- `LoadContent()` reads from disk/embed on demand
- Trade-off: I/O overhead vs memory

**Recommendation: Option A for now**
- Skills are small (few KB each)
- 50 skills × 5KB = 250KB memory
- Simpler implementation
- Can optimize to Option B later if needed

### 7. File Access Pattern

**For Builtin Skills:**
```go
// Path resolution
func (r *Registry) GetSkillPath(name string) (string, error) {
    skill, ok := r.Get(name)
    if !ok {
        return "", fmt.Errorf("skill not found: %s", name)
    }
    if skill.Source == "builtin" {
        return fmt.Sprintf("templates/skills/%s/SKILL.md", name), nil
    }
    return skill.Path, nil
}
```

**For Workspace Skills:**
```go
// Direct path from skill struct
path := skill.Path  // Already absolute
```

---

## Implementation Plan

### Phase 1: Modify Summary Output (1 day)
1. Update `SummaryXML()` to only output metadata
2. Remove content from XML output
3. Update tests

### Phase 2: Update System Prompt (1 day)
1. Add skill loading instructions to SOUL.md or AGENTS.md
2. Update `BuildSystemPrompt()` to include instructions
3. Test that agent understands how to load skills

### Phase 3: Add Content Loading Method (1 day)
1. Add `LoadContent(name string) (string, error)` to Registry
2. Implement for both builtin and workspace skills
3. Add tests

### Phase 4: Integration Testing (1 day)
1. Test end-to-end: agent reads skill → uses skill → completes task
2. Verify token savings
3. Test edge cases (unavailable skills, missing files)

---

## Success Metrics

1. **Token Reduction**: 80%+ reduction in baseline context size
2. **Skill Usage**: Agent can still effectively discover and use skills
3. **Performance**: No noticeable latency when loading skills
4. **Backward Compatibility**: Always-on skills work exactly as before

---

## Open Questions

1. **Should we preload any skills beyond always-on?**
   - Maybe "skill-creator" since it teaches how to use skills?
   - Probably not - let agent decide

2. **How do we handle skill dependencies?**
   - If skill A references skill B, should we auto-load B?
   - Probably not - skills should be self-contained

3. **What about skill updates during session?**
   - Current: fixed at startup
   - Future: could hot-reload

---

## Example Usage

**User:** "Check the weather in Tokyo"

**System prompt includes:**
```xml
<available_skills>
  <skill name="weather" status="available">Check weather forecasts and conditions</skill>
  ...
</available_skills>
```

**Agent thinks:** "User wants weather info. I see a 'weather' skill available. I should read it."

**Agent actions:**
1. `read_file(workspace/skills/weather/SKILL.md)`
2. Sees: "Use this skill when weather context matters..."
3. Sees: "You can use the `web_search` tool to find weather data..."
4. Uses web_search to get Tokyo weather
5. Responds with weather info

**Token usage:**
- Before: ~500 tokens for all skill content
- After: ~50 tokens for metadata + ~100 tokens for weather skill = ~150 tokens
- **Savings: 70%**

---

## Files to Modify

1. `pkg/skill/registry.go`
   - Update `SummaryXML()` to output metadata only
   - Add `LoadContent()` method

2. `pkg/skill/loader.go`
   - Update `Skill` struct to track `HasResources`

3. `pkg/agent/context.go`
   - Update `BuildSystemPrompt()` to include skill loading instructions

4. Templates
   - Update `templates/SOUL.md` or create `templates/AGENTS.md` with skill instructions

5. Tests
   - Update existing tests for new SummaryXML format
   - Add tests for LoadContent
   - Add integration test for skill loading flow

---

## Notes

This design intentionally mimics how OpenClaw and Python nanobot handle skills, but with Go idioms. The key insight is that the LLM is smart enough to decide when it needs a skill's full content - we just need to give it the metadata to make that decision.
