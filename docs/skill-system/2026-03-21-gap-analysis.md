# Skill System Gap Analysis

**Date:** 2026-03-21
**Project:** smolbot
**Comparisons:** Python nanobot, OpenClaw

---

## Executive Summary

Our Go implementation has a functional but minimal skill system. While it successfully loads and uses skills, it lacks the sophisticated features that make OpenClaw and Python nanobot's skill systems powerful: progressive disclosure, skill commands, bundled resources, and rich metadata. This analysis identifies these gaps and prioritizes them for implementation.

---

## Current State (smolbot)

### What's Implemented

1. **Basic Skill Loading**
   - Embedded builtin skills via `//go:embed`
   - Workspace skill override (workspace/skills/<name>/SKILL.md)
   - Two-level precedence: builtin → workspace

2. **Simple Frontmatter Parsing**
   - Fields: `name`, `description`, `requires.bins`, `requires.env`, `always`
   - String splitting on `---\n` and `\n---\n`
   - Basic YAML unmarshaling

3. **Registry Pattern**
   - `skill.Registry` with name-based lookup
   - `AlwaysOn()` for skills marked `always: true`
   - `SummaryXML()` for system prompt inclusion

4. **Availability Checking**
   - Binary presence on PATH
   - Environment variable presence

### Current Architecture

```
skills/
├── clawhub/SKILL.md
├── cron/SKILL.md
├── github/SKILL.md
├── memory/SKILL.md
├── skill-creator/SKILL.md
├── summarize/SKILL.md
├── tmux/SKILL.md
└── weather/SKILL.md
```

**Loading Flow:**
1. `NewBuiltinRegistry()` → load from embedded FS
2. `NewRegistry(workspace)` → load workspace skills, merge with builtin
3. `BuildSystemPrompt()` → inject `SummaryXML()` + `AlwaysOn()` content

---

## Gap 1: Progressive Disclosure

### Current Behavior
All skill metadata AND full content are loaded into the LLM context immediately via `SummaryXML()` and `AlwaysOn()`.

### Target Behavior (OpenClaw/nanobot)
Three-level system:
1. **Metadata only** (always in context): name, description, availability
2. **SKILL.md body** (loaded on-demand): Full instructions when agent decides to use skill
3. **Bundled resources** (as-needed): Scripts, references, assets used directly

### Impact
- **Token efficiency**: Skills only consume context when actually used
- **Scalability**: Can have 50+ skills without context bloat
- **Clarity**: Agent sees clean summary, loads full details when relevant

### Implementation Complexity
**Medium** - Requires:
- Modify `SummaryXML()` to exclude full content
- Add `LoadSkillContent(name)` method
- Modify agent loop to handle skill loading decision
- Potentially add tool for agent to read skill files

---

## Gap 2: Skill Commands / Trigger System

### Current Behavior
Skills are passive content injected into system prompt. No explicit invocation mechanism.

### Target Behavior (OpenClaw)
- Auto-generated slash commands: `/skill <name>` or `/<command>`
- Can dispatch to tools via frontmatter: `command-dispatch: tool`
- Reserved name collision detection

### Target Behavior (nanobot Python)
- Agent uses `read_file` tool to load skill when LLM decides it's relevant
- Skill descriptions guide LLM triggering
- No explicit commands, just intelligent loading

### Recommendation
Adopt **nanobot Python's approach** - simpler, no command infrastructure needed. The agent already has file reading capability.

### Implementation Complexity
**Low** - Requires:
- Remove full content from system prompt
- Agent naturally learns to read skills based on descriptions
- May need prompt engineering to encourage skill usage

---

## Gap 3: Robust Frontmatter Parsing

### Current Behavior
Simple string splitting that fails on:
- Windows line endings (\r\n)
- Extra whitespace
- Malformed frontmatter

### Target Behavior
Proper YAML frontmatter extraction with:
- Line ending normalization
- Graceful degradation
- Clear error messages

### Current Error
```
Error: unterminated YAML frontmatter
```

### Root Cause
`splitFrontmatter()` expects exactly `---\n` prefix and `\n---\n` delimiter.

### Implementation Complexity
**Low** - Requires:
- Normalize line endings before parsing
- More flexible delimiter detection
- Better error context (which file failed)

---

## Gap 4: Bundled Resources

### Current Behavior
Only SKILL.md is loaded. No support for supplementary files.

### Target Behavior (OpenClaw/nanobot)
```
skill-name/
├── SKILL.md              (required)
├── scripts/              (executable code)
│   ├── setup.sh
│   └── deploy.py
├── references/           (documentation)
│   └── api-docs.md
└── assets/               (templates, images)
    └── template.html
```

### Use Cases
- **weather skill**: Script to fetch actual weather data
- **github skill**: Scripts for PR creation, branch management
- **deploy skill**: Templates for deployment manifests

### Implementation Complexity
**Medium-High** - Requires:
- Extend embed.FS to include scripts/references/assets
- Add `ExecuteScript(skill, script)` method
- Security review (arbitrary code execution)
- Path resolution for workspace vs builtin skills

---

## Gap 5: Rich Metadata & Installation

### Current Behavior
Minimal metadata: name, description, requires.bins, requires.env, always

### Target Behavior (OpenClaw)
```yaml
metadata:
  openclaw:
    emoji: "🐙"
    requires:
      bins: ["gh", "curl"]
      env: ["GITHUB_TOKEN"]
      config: ["github.enabled"]
    install:
      - type: brew
        package: gh
      - type: download
        url: https://...
        extract: true
```

### Benefits
- **Self-documenting**: Installation instructions built-in
- **Auto-setup**: Could install dependencies automatically
- **Better UX**: Emoji, rich descriptions

### Implementation Complexity
**Medium** - Requires:
- Extend frontmatter struct
- Add installation engine (brew, npm, go, download)
- Security considerations for automatic installs

---

## Gap 6: Hot Reload

### Current Behavior
Skills loaded at startup. Changes require restart.

### Target Behavior (OpenClaw)
- File watching on workspace/skills/
- Debounced reload (250ms default)
- Snapshot versioning for cache invalidation
- No restart needed for skill development

### Implementation Complexity
**Medium** - Requires:
- Add fsnotify or similar file watcher
- Debounce logic
- Thread-safe registry updates
- Cache invalidation

---

## Gap 7: Multi-Level Discovery Hierarchy

### Current Behavior
Two-level: builtin → workspace

### Target Behavior (OpenClaw)
Six-level precedence:
1. Extra directories (config.skills.load.extraDirs)
2. Bundled skills (distributed with app)
3. Managed skills (~/.config/openclaw/skills/)
4. Personal agent skills (~/.agents/skills/)
5. Project agent skills (<workspace>/.agents/skills/)
6. Workspace skills (<workspace>/skills/) ← highest

### Recommendation
Add **managed skills** level:
- `~/.config/smolbot/skills/` for user-installed skills
- Similar to ~/.nanobot's skill management

### Implementation Complexity
**Low** - Requires:
- Add path to config
- Load from additional directory in `NewRegistry()`

---

## Gap 8: OS-Based Availability

### Current Behavior
Only checks bins and env vars.

### Target Behavior (OpenClaw)
```yaml
requires:
  os: ["darwin", "linux"]  # Exclude windows
```

### Use Cases
- macOS-specific skills (brew-based)
- Linux-specific skills (systemd-based)
- Windows-specific skills

### Implementation Complexity
**Low** - Requires:
- Add `os` field to requires struct
- Check runtime.GOOS

---

## Priority Matrix

| Gap | Priority | Effort | Impact | Recommendation |
|-----|----------|--------|--------|----------------|
| Robust Frontmatter Parsing | **Critical** | Low | High | **Fix immediately** |
| Progressive Disclosure | **High** | Medium | High | Implement next |
| Skill Commands/Triggers | **High** | Low | High | Via read_file tool |
| Bundled Resources | **Medium** | High | Medium | Phase 2 |
| Rich Metadata | **Medium** | Medium | Medium | Phase 2 |
| Hot Reload | **Low** | Medium | Low | Nice to have |
| Multi-Level Discovery | **Low** | Low | Low | Add managed skills path |
| OS-Based Availability | **Low** | Low | Low | Nice to have |

---

## Immediate Actions Required

### Fix Critical Bug
The "unterminated YAML frontmatter" error is blocking CLI usage. Fix:
1. Normalize line endings in `splitFrontmatter()`
2. Add file path to error messages
3. Handle edge cases (empty files, missing delimiters)

### Next Sprint: Progressive Disclosure
1. Modify `SummaryXML()` to only output metadata
2. Add skill content loading method
3. Update system prompt to guide agent skill usage
4. Test that agent can still effectively use skills

---

## Long-Term Vision

A skill system that:
- Loads instantly with only metadata in context
- Allows agent to intelligently load skills when needed
- Supports rich supplementary resources
- Can self-install missing dependencies
- Works across all platforms
- Hot-reloads during development

This matches the sophistication of OpenClaw while maintaining the simplicity that makes nanobot approachable.

---

## References

- OpenClaw skill system: `~/openclaw/src/agents/skills/`
- Python nanobot skills: `~/.cache/uv/archive-*/nanobot/skills/`
- Our implementation: `~/smolbot/pkg/skill/`
