# Consolidated Skill System Design

**Date:** 2026-03-21
**Sources:** OpenClaw, Python nanobot, opencode/codex
**Status:** Final Design

---

## Executive Summary

After analyzing three mature skill systems (OpenClaw, Python nanobot, opencode/codex), clear patterns emerge:

1. **Progressive Disclosure** is universal - all systems use metadata → instructions → resources
2. **Filesystem is the Registry** - no central database, skills self-register by presence
3. **Agent-Driven Loading** - agent decides relevance based on description matching
4. **Context is a Public Good** - token efficiency is paramount

This design synthesizes the best patterns into a cohesive Go implementation.

---

## Common Patterns Across All Systems

### Pattern 1: Three-Level Progressive Disclosure

| Level | OpenClaw | nanobot Python | opencode | Content |
|-------|----------|---------------|----------|---------|
| **1. Metadata** | Always loaded | Always loaded | Always loaded | name, description, availability |
| **2. Instructions** | Agent reads SKILL.md | Agent uses read_file | Agent reads SKILL.md | Full skill content |
| **3. Resources** | As needed | As needed | As needed | scripts/, references/, assets/ |

**Key Insight:** All systems converge on this pattern. It's the optimal balance of discoverability vs token efficiency.

### Pattern 2: Description-Driven Discovery

**OpenClaw:**
```yaml
description: "Concise description used for skill triggering by the LLM"
```

**opencode (strict rule):**
```yaml
description: "Use when [trigger conditions and symptoms]"
# MUST describe WHEN to use, never WHAT it does
```

**Critical Finding from opencode:**
> "Testing showed descriptions summarizing workflows cause agents to skip reading the full skill."

**Recommendation:** Adopt opencode's strict description format: **"Use when [conditions]"**

### Pattern 3: Skill Invocation Rule

**opencode's "Iron Rule":**
> "If you think there is even a 1% chance a skill might apply to what you are doing, you ABSOLUTELY MUST invoke the skill."

**This rule ensures:**
- Agent doesn't try to "optimize" by skipping potentially relevant skills
- Better user experience (skill applied when appropriate)
- Less cognitive load on agent (just load it)

---

## Synthesis: Our Design

### Core Principles

1. **Token-First**: Context window is precious. Load only what's needed.
2. **Description-Driven**: Agent decides based on metadata descriptions only.
3. **Progressive Loading**: Metadata → Skill Content → Resources (as needed)
4. **Filesystem-Native**: Skills are files. No abstraction overhead.
5. **Simple to Complex**: Start with SKILL.md, add resources if needed.

### Skill Format

```markdown
---
name: skill-name-with-hyphens
description: Use when [specific trigger conditions and symptoms]
requires:
  bins: ["optional-binary"]
  env: ["OPTIONAL_ENV"]
always: false
---

# Skill Title

## Overview
1-2 sentence description of what this skill provides.

## When to Use
- Specific condition 1
- Specific condition 2
- When NOT to use: specific exclusion

## How to Use
[Instructions, workflows, examples]

## References (optional)
- `references/commands.md` - Read for detailed command reference
- `scripts/helper.sh` - Execute for complex operations
```

**Field Constraints:**
- `name`: lowercase letters, numbers, hyphens only; max 64 chars
- `description`: max 500 chars; MUST start with "Use when"
- `requires.bins`: array of required CLI binaries
- `requires.env`: array of required environment variables
- `always`: boolean - load this skill's content into every context

### Directory Structure

**Simple Skill:**
```
skills/
└── skill-name/
    └── SKILL.md
```

**Complex Skill with Resources:**
```
skills/
└── skill-name/
    ├── SKILL.md
    ├── scripts/
    │   └── helper.sh          # Executables (not loaded to context)
    ├── references/
    │   └── commands.md        # Docs loaded on-demand
    └── assets/
        └── template.txt       # Templates, images, etc.
```

### Discovery Hierarchy

```
Priority (high to low):

1. ~/.nanobot-go/skills/          # User-managed skills
2. <workspace>/skills/            # Project-specific skills
3. (embedded) skills/             # Built-in skills

Later overrides earlier (same as nanobot Python)
```

### Loading Flow

```
┌─────────────────────────────────────────────────────────────┐
│ Startup                                                    │
└─────────────────────────────────────────────────────────────┘
    │
    ▼
┌─────────────────────────────────────────────────────────────┐
│ 1. Scan all skill directories                              │
│    - Find all SKILL.md files                               │
│    - Parse frontmatter only (name, description, requires)  │
│    - Check availability (bins, env vars)                   │
└─────────────────────────────────────────────────────────────┘
    │
    ▼
┌─────────────────────────────────────────────────────────────┐
│ 2. Build Metadata Registry                                   │
│    - Keep in memory: name, description, available, path    │
│    - DO NOT load full content yet                          │
└─────────────────────────────────────────────────────────────┘
    │
    ▼
┌─────────────────────────────────────────────────────────────┐
│ 3. Inject into System Prompt                                │
│    - <available_skills> XML with metadata only              │
│    - Include loading instructions                          │
│    - Include "always: true" skill content                   │
└─────────────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────────────┐
│ Runtime: User Request                                       │
└─────────────────────────────────────────────────────────────┘
    │
    ▼
┌─────────────────────────────────────────────────────────────┐
│ 4. Agent Decides                                             │
│    - Reads <available_skills> from context                 │
│    - Matches user request against descriptions              │
│    - If 1% chance of relevance: MUST load skill            │
└─────────────────────────────────────────────────────────────┘
    │
    ▼
┌─────────────────────────────────────────────────────────────┐
│ 5. Skill Loading                                             │
│    - Agent uses read_file tool: skills/{name}/SKILL.md     │
│    - Full content now available to agent                   │
│    - May reference: references/, scripts/, assets/         │
└─────────────────────────────────────────────────────────────┘
    │
    ▼
┌─────────────────────────────────────────────────────────────┐
│ 6. Resource Usage                                            │
│    - references/ → read_file (load to context)             │
│    - scripts/ → execute (run, output captured)             │
│    - assets/ → read_file (use content directly)            │
└─────────────────────────────────────────────────────────────┘
```

### System Prompt Additions

```markdown
## Available Skills

You have access to specialized skills. They are listed in <available_skills> above.

**When to use a skill:**
If there is even a 1% chance a skill might apply to what you're doing, 
ABSOLUTELY load it. Better to load an unnecessary skill than miss a relevant one.

**How to load a skill:**
Read the skill file: `read_file(skills/{skill-name}/SKILL.md)`

**Skill resources:**
- `references/{file}` - Additional documentation, load if needed
- `scripts/{file}` - Executable helpers, run if instructed
- `assets/{file}` - Templates and assets, read if needed

**Always-available skills** (marked always="true") are pre-loaded.
```

### SummaryXML Format (Metadata Only)

```xml
<available_skills>
  <skill name="weather" status="available">
    Use when checking weather forecasts or current conditions for a location
  </skill>
  <skill name="github" status="unavailable" reason="missing bin: gh">
    Use when working with GitHub repositories, PRs, or issues
  </skill>
  <skill name="memory" status="available" always="true">
    Use when maintaining durable memory and history summaries
  </skill>
</available_skills>
```

**Size:** ~50 tokens per skill (vs 200-1000 for full content)

---

## Comparison: Our Design vs Existing Systems

| Feature | OpenClaw | nanobot Python | opencode | Our Design |
|---------|----------|---------------|----------|------------|
| Progressive Disclosure | ✓ | ✓ | ✓ | ✓ (all converge here) |
| Filesystem Registry | ✓ | ✓ | ✓ | ✓ |
| Agent-Driven Loading | ✓ | ✓ | ✓ | ✓ |
| Description Format | Flexible | Flexible | Strict "Use when" | **Strict "Use when"** |
| Installation Specs | ✓ (rich) | ✗ | ✗ | **Optional** |
| Hot Reload | ✓ | ✗ | ✗ | **Phase 2** |
| MCP Integration | ✓ | ✗ | ✓ | **Via tools** |
| Multi-Level Discovery | 6 levels | 2 levels | 2 levels | **3 levels** |
| Bundled Resources | ✓ | Partial | ✓ | **✓** |
| OS Filtering | ✓ | ✗ | ✗ | **Phase 2** |

---

## Implementation Phases

### Phase 1: Foundation (This Sprint)

1. **Robust Frontmatter Parsing**
   - Fix "unterminated YAML" bug
   - Handle Windows line endings
   - Better error messages

2. **Progressive Disclosure Core**
   - Modify `SummaryXML()` → metadata only
   - Add `LoadContent()` method
   - Update system prompt with loading instructions
   - Add "1% rule" to agent instructions

3. **User Skills Directory**
   - Add `~/.nanobot-go/skills/` to discovery
   - Create directory on first run
   - Document skill creation

### Phase 2: Enhanced Features (Next Sprint)

1. **Bundled Resources**
   - Support `references/`, `scripts/`, `assets/` subdirectories
   - Add resource path resolution
   - Security review for script execution

2. **Hot Reload**
   - File watching on user skills
   - Debounced reload
   - Cache invalidation

3. **Rich Metadata**
   - Emoji support
   - Installation hints (optional)
   - OS-based availability

### Phase 3: Advanced Features (Future)

1. **Skill Marketplace**
   - Git-based skill sharing
   - `nanobot skill install <repo>`

2. **Skill Dependencies**
   - Auto-load dependent skills
   - Dependency validation

3. **Conditional Loading**
   - Context-aware skill suggestions
   - Usage analytics

---

## Key Decisions

### Decision 1: Strict Description Format

**Choice:** Adopt opencode's strict "Use when [conditions]" format

**Rationale:**
- Testing shows agents skip skills when description summarizes instead of triggers
- Clear decision boundary for agent
- Matches successful opencode pattern

### Decision 2: Three-Level Discovery

**Choice:** `~/.nanobot-go/skills/` → `<workspace>/skills/` → embedded

**Rationale:**
- User-managed skills are most important (persistence across projects)
- Workspace skills for project-specific needs
- Embedded for defaults
- Simpler than OpenClaw's 6-level, sufficient for our use case

### Decision 3: No Auto-Installation

**Choice:** Skills check availability but don't auto-install dependencies

**Rationale:**
- Security (arbitrary code execution)
- Simplicity (no package manager integration)
- Can add later if needed
- Installation hints in metadata can guide users

### Decision 4: Agent Loads via read_file

**Choice:** Agent uses existing `read_file` tool to load skills

**Rationale:**
- No new infrastructure
- Agent already has file reading capability
- Consistent with nanobot Python approach
- No special "Skill" tool needed

---

## Success Metrics

1. **Token Efficiency**
   - 80%+ reduction in baseline context size
   - Measurable via token counting

2. **Skill Discovery**
   - Agent loads relevant skills 95%+ of the time
   - No false negatives (missing applicable skills)

3. **User Experience**
   - Skills "just work" without user knowing complexity
   - Easy to create new skills (copy template, edit)

4. **Performance**
   - Startup time < 1 second for 50 skills
   - Skill loading latency < 100ms

---

## Example: Weather Skill

**File:** `~/.nanobot-go/skills/weather/SKILL.md`

```markdown
---
name: weather
description: Use when checking weather forecasts, current conditions, or weather-related queries for specific locations
requires:
  bins: [curl]
---

# Weather Skill

## Overview
Check weather information using web APIs.

## When to Use
- User asks about weather in a location
- Current temperature, forecast, conditions
- Weather comparisons between locations

## When NOT to Use
- Climate change discussions (use web search)
- Historical weather data (use web search)

## How to Check Weather

1. Use the `web_search` tool with query: 
   "weather in {location}"

2. Parse results for:
   - Current temperature
   - Conditions (sunny, rainy, etc.)
   - Forecast

3. Present clearly with units

## Example

User: "What's the weather in Tokyo?"

Action: web_search("weather in Tokyo")
Response: "Currently 22°C and partly cloudy in Tokyo..."
```

**Usage:**

1. **Startup:** `weather` skill metadata in context (~20 tokens)
2. **User asks:** "What's the weather in Tokyo?"
3. **Agent matches:** Description matches query
4. **Agent loads:** `read_file(skills/weather/SKILL.md)`
5. **Agent follows:** Instructions to use web_search
6. **Agent responds:** With weather information

**Token savings:** ~500 tokens baseline → ~70 tokens (metadata + loaded skill)

---

## Conclusion

This design synthesizes proven patterns from three mature systems:

- **Progressive disclosure** from all three
- **Strict description format** from opencode (best tested)
- **Filesystem registry** from all three
- **Simple resources** from opencode (not over-engineered)
- **User-managed skills** from nanobot Python (practical)

The result is a skill system that is:
- Token-efficient (80%+ savings)
- Simple to understand and extend
- Proven by real-world usage
- Appropriate for Go implementation

Next step: Implementation plan.
