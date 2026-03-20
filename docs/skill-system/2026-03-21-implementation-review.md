# Implementation Plan Review Report

**Date:** 2026-03-21
**Plan:** Progressive Disclosure Skill System
**Reviewer:** Claude
**Status:** CRITICAL ISSUES FOUND - DO NOT EXECUTE YET

---

## Executive Summary

The implementation plan contains **CRITICAL gaps and inconsistencies** that must be addressed before execution. While the overall architecture is sound, several implementation details will cause immediate failures.

**Key Findings:**
1. **Registry interface mismatch** - Plan assumes methods that don't exist
2. **Missing error handling** - Several failure modes not covered
3. **Concurrency issues** - Registry is not thread-safe
4. **Test gaps** - Critical paths untested
5. **Breaking changes** - Will break existing functionality

**Recommendation:** Fix critical issues before execution. Estimated additional work: 2-3 hours.

---

## CRITICAL Issues (Must Fix Before Execution)

### CRITICAL-1: Registry Interface Mismatch

**Problem:** The plan assumes `Registry` has methods that don't exist.

**Current `Registry` struct:**
```go
type Registry struct {
    skills map[string]*Skill
}
```

**Plan assumes:**
- `LoadContent(name string) (string, error)` - Doesn't exist
- `HasResource(skill, resource string) bool` - Doesn't exist  
- `GetResourcePath(skill, resource string) string` - Doesn't exist

**Impact:** Code won't compile. Tasks 3, 5, 6 will fail.

**Fix:**
1. Define interface first:
```go
type SkillLoader interface {
    LoadContent(name string) (string, error)
    HasResource(name, resource string) bool
    GetResourcePath(name, resource string) (string, error)
}
```

2. Implement methods on Registry

**Effort:** 30 minutes

---

### CRITICAL-2: NewRegistry Signature Mismatch

**Problem:** Current `NewRegistry` takes `workspace string`, plan assumes `paths *config.Paths`

**Current code:**
```go
func NewRegistry(workspace string) (*Registry, error)
```

**Plan assumes:**
```go
func NewRegistry(paths *config.Paths) (*Registry, error)
```

**Impact:** Tasks 5 and 6 won't compile.

**Fix:** Update signature OR update plan to use current signature

**Decision needed:** Should we inject Paths struct or keep simple string?

**Recommendation:** Keep simple string for now, add Paths in Phase 2. Update plan accordingly.

**Effort:** 15 minutes

---

### CRITICAL-3: No Thread Safety

**Problem:** Registry has zero concurrency protection.

**Current Registry:**
```go
type Registry struct {
    skills map[string]*Skill  // No mutex!
}
```

**Plan adds:** `LoadContent()` which reads files - potentially slow I/O

**Race conditions:**
- `Names()` reads map while `NewRegistry` writes
- `LoadContent` may be called concurrently
- SummaryXML reads skills during potential modification

**Impact:** Data races, undefined behavior in concurrent access.

**Fix:** Add RWMutex:
```go
type Registry struct {
    skills map[string]*Skill
    mu     sync.RWMutex
}
```

**Effort:** 30 minutes + testing

---

### CRITICAL-4: LoadContent Implementation Flaw

**Problem:** Plan's `LoadContent` has bugs.

**Plan implementation:**
```go
func (r *Registry) LoadContent(name string) (string, error) {
    skill, ok := r.skills[name]
    if !ok {
        return "", fmt.Errorf("skill not found: %s", name)
    }
    // If content is already loaded, return it
    if skill.Content != "" {
        return skill.Content, nil
    }
    // Otherwise read from disk
    data, err := os.ReadFile(skill.Path)
    // ...
}
```

**Bug 1:** For builtin skills (embed.FS), `skill.Path` is a virtual path like "skills/weather/SKILL.md" - can't `os.ReadFile` this!

**Bug 2:** No distinction between "empty content" and "not loaded yet". A skill with no body after frontmatter would have `Content == ""` but should still be loadable.

**Fix:** Track load state separately:
```go
type Skill struct {
    // ... other fields ...
    Content   string
    Loaded    bool  // New field
}
```

**Alternative:** Don't lazy load - pre-load all content at startup. Simpler, acceptable for Phase 1.

**Recommendation:** Remove lazy loading from Phase 1. Just pre-load all content like current implementation. Token savings comes from SummaryXML change, not lazy loading.

**Effort:** 0 minutes (just remove lazy loading from plan)

---

### CRITICAL-5: No User Skills Directory Exists

**Problem:** Plan assumes `~/.nanobot-go/skills/` exists and is wired up.

**Current state:**
- No `SkillsDir()` method in config.Paths
- No creation of skills directory on startup
- Plan says "Add user skills directory" but doesn't say WHERE in codebase

**Impact:** Task 5 will fail - directory doesn't exist, no code to create it.

**Fix:**
1. Add `SkillsDir()` to `pkg/config/paths.go`
2. Add directory creation to `cmd/nanobot/main.go` or `pkg/agent/context.go:SyncWorkspaceTemplates()`
3. Update `NewRegistry` to take skills directory parameter OR look it up from config

**Effort:** 45 minutes

---

## HIGH Severity Issues

### HIGH-1: Test Isolation Problems

**Problem:** Integration test in Task 6 modifies real filesystem.

**Test code:**
```go
func TestAgentLoadsSkillOnDemand(t *testing.T) {
    workspace := t.TempDir()  // Good - temp dir
    // But then calls NewRegistry which may load from real paths!
    paths := &config.Paths{ /* setup with workspace */ }
    reg, err := skill.NewRegistry(paths)  // Which paths?
```

**Issue:** If `NewRegistry` loads builtin skills from embed.FS, that's fine. But if it tries to load from real `~/.nanobot-go/skills/`, test is not isolated.

**Fix:** Ensure test only tests the specific behavior, not full registry. Or use dependency injection for paths.

**Effort:** 30 minutes

---

### HIGH-2: No Rollback Strategy

**Problem:** If progressive disclosure breaks skill discovery, how do we revert?

**Current behavior:** All skills always available
**New behavior:** Agent must actively load skills

**Risk:** Agent might not load skills when needed, degrading experience.

**Mitigation:**
- Keep "always: true" skills fully loaded (plan does this) ✓
- Add feature flag to disable progressive disclosure
- Monitor skill load rates

**Fix:** Add config option `skills.progressiveDisclosure: true/false` (default: true after testing)

**Effort:** 30 minutes

---

### HIGH-3: Breaking Change to SummaryXML

**Problem:** Current tests likely expect content in SummaryXML.

**Current SummaryXML probably includes:** Full skill content
**New SummaryXML:** Metadata only

**Impact:** Tests will fail, possibly production code that parses SummaryXML.

**Fix:** 
1. Find all uses of SummaryXML
2. Update tests
3. Check if any other packages parse SummaryXML

**Effort:** 30 minutes

---

### HIGH-4: Missing Error Context

**Problem:** When `splitFrontmatter` fails, we don't know which file.

**Current:**
```go
return nil, fmt.Errorf("unterminated YAML frontmatter")
```

**Should be:**
```go
return nil, fmt.Errorf("unterminated YAML frontmatter in %s: %w", path, err)
```

**Impact:** Hard to debug which skill file is broken.

**Fix:** Add path to all errors in skill loading.

**Effort:** 15 minutes

---

## MEDIUM Severity Issues

### MEDIUM-1: Token Savings Calculation Unverified

**Claim:** "80%+ token savings"

**Calculation:**
- Current: ~500 tokens/skill × 8 skills = 4000 tokens
- New: ~20 tokens/skill × 8 skills = 160 tokens
- Savings: 96%

**Unverified assumptions:**
1. Is 500 tokens/skill accurate? Depends on skill size.
2. Does XML marshaling add overhead?
3. What about "always: true" skills still loaded?

**Fix:** Measure actual token counts before claiming 80% savings.

**Effort:** 15 minutes to add measurement

---

### MEDIUM-2: No Hot Reload in Phase 1

**Plan says:** Hot reload in Phase 2

**User expectation:** May expect immediate skill updates without restart.

**Mitigation:** Document that restart is required for skill changes in Phase 1.

**Effort:** 0 minutes (documentation only)

---

### MEDIUM-3: Resource Path Resolution Ambiguity

**Problem:** How does agent reference skill resources?

**Options:**
1. `read_file(skills/{name}/references/commands.md)` - relative to workspace?
2. `read_file(~/.nanobot-go/skills/{name}/references/commands.md)` - absolute?
3. Some other path?

**Plan doesn't specify:** Clear path resolution strategy.

**Fix:** Define and document path format. Recommend: relative to workspace root.

**Effort:** 15 minutes

---

## LOW Severity Issues

### LOW-1: Plan Task Granularity Inconsistent

**Task 1:** Very detailed (6 steps, exact code)
**Task 7:** Less detailed (4 steps, more conceptual)

**Not a blocker** but makes execution harder.

**Fix:** Make Task 7 as detailed as Task 1.

---

### LOW-2: No Skill Validation on Creation

**Plan:** Shows how to create skill with heredoc
**Missing:** Validation that skill is parseable

**Fix:** Add `nanobot skill validate` command or similar.

**Effort:** Defer to Phase 2.

---

### LOW-3: Description Format Not Enforced

**Plan:** "MUST start with 'Use when'"
**Implementation:** No validation or enforcement

**Risk:** Users create skills with wrong format, agent doesn't trigger them.

**Fix:** Add validation in Phase 2.

---

## Recommended Fixes Summary

### Before Execution (Critical Path)

1. [ ] **CRITICAL-1:** Define Registry interface with new methods (30 min)
2. [ ] **CRITICAL-2:** Decide on NewRegistry signature (15 min)
3. [ ] **CRITICAL-3:** Add thread safety to Registry (30 min)
4. [ ] **CRITICAL-4:** Remove lazy loading, pre-load all content (0 min - just remove from plan)
5. [ ] **CRITICAL-5:** Add SkillsDir() and directory creation (45 min)
6. [ ] **HIGH-3:** Update all SummaryXML tests (30 min)

**Total:** ~2.5 hours

### During Execution (High Priority)

7. [ ] **HIGH-1:** Fix test isolation (30 min)
8. [ ] **HIGH-2:** Add feature flag for progressive disclosure (30 min)
9. [ ] **HIGH-4:** Add path context to errors (15 min)

### Nice to Have (Medium/Low)

10. [ ] **MEDIUM-1:** Verify token savings (15 min)
11. [ ] **MEDIUM-3:** Document resource path resolution (15 min)
12. [ ] **LOW-1:** Make Task 7 more detailed (15 min)

---

## Revised Execution Order

### Pre-Execution Fixes (Do These First)

**Fix A: Registry Interface**
- Define interface
- Implement LoadContent (simple version - just return skill.Content)
- Add mutex for thread safety

**Fix B: Paths Integration**
- Add SkillsDir() to config.Paths
- Add directory creation
- Update NewRegistry signature decision

**Fix C: Test Updates**
- Update existing SummaryXML tests
- Add frontmatter error context

### Then Execute Original Plan

Now Tasks 1-9 will actually work.

---

## Conclusion

The implementation plan is **architecturally sound** but **implementation-ready**. The critical issues are:

1. Registry interface doesn't match plan
2. No thread safety
3. No user skills directory infrastructure

**These are fixable in ~2.5 hours.** After fixes, the plan can be executed successfully.

**Do not execute the plan as written** - it will fail compilation and/or have race conditions.

**Recommendation:**
1. Spend 2-3 hours fixing critical issues
2. Update plan with fixed code
3. Then execute using executing-plans superpower

The design itself is good - progressive disclosure is the right approach. The implementation just needs these gaps filled.
