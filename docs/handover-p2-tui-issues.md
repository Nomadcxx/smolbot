# TUI P2 Implementation Handover

## Executive Summary

Previous agent implemented P0/P1/P2 TUI features according to spec. Code IS present in repo (9 commits), but **features not visible** when tested. User sees:
- ✅ Thinking blocks now working (fixed: Ollama uses "reasoning" not "reasoning_content")
- ❌ Context % not visible in header  
- ❌ Tool truncation not visible
- ❌ Compact header may not be applying
- ❌ STREAM element appears above THINKING, unreadable (single line streaming)
- ❌ Message width cap not verified

## Implementation Status by Task

### P0 - Critical Fixes (MUST BE WORKING)
| Task | Status | Evidence Needed |
|------|--------|----------------|
| 1. CoerceArgs bool parse | ⚠️ UNVERIFIED | Test with tool that has bool param (e.g., `recursive: true`) |
| 2. Streaming thinking events | ✅ WORKING | Thinking blocks now visible |
| 3. Tool duplicate rendering | ⚠️ UNVERIFIED | Run two tools with same name, verify no duplicates |
| 4. Usage/context tracker | ❌ NOT WORKING | Context % not visible in header |

### P1 - Major Features
| Task | Status | Evidence Needed |
|------|--------|----------------|
| Tool call IDs | ⚠️ UNVERIFIED | Check if multiple same-name tools work correctly |

### P2 - UI Polish
| Task | Status | Evidence Needed |
|------|--------|----------------|
| Compact header | ❌ NOT WORKING | Header shows full banner, not compact "model • context% • path" |
| Context % display | ❌ NOT WORKING | No percentage shown in header |
| Tool truncation | ❌ NOT WORKING | Tool output shows all lines, not "… (N lines hidden)" |
| Thinking duration | ⚠️ UNVERIFIED | Check if "Thought for Xs" footer appears |
| Message width cap | ❌ NOT WORKING | Messages may exceed 120 char width |
| STREAM rendering | ❌ BROKEN | STREAM appears as single line above thinking, unreadable |

## What Was Implemented

### Commits in main branch:
1. `2e9947b` - fix(tool): CoerceArgs bool parse
2. `cc0f543` - feat(gateway): stream thinking chunks
3. `6b1ad94` - fix(tui): match tool calls by unique ID
4. `c8c50bf` - feat(gateway): emit token usage events
5. `f0bd4e6` - feat(tui): wire header to live app state, compact layout
6. `07096fa` - feat(tui): cap message content width at 120
7. `626e989` - feat(tui): 'Thought for Xs' footer to thinking blocks
8. `f97f599` - feat(tui): truncate tool output to 10 lines
9. `036f34a` - test(tui): update test heights

### Code Locations (VERIFY THESE EXIST):
- `pkg/provider/openai.go:335` - Ollama thinking support with Reasoning field
- `internal/components/chat/message.go:221` - renderThinkingBlock with duration
- `internal/components/chat/message.go:35` - maxToolOutputLines = 10
- `internal/components/header/header.go:47` - SetContextPercent
- `internal/tui/tui.go:407` - chat.usage handler with SetContextPercent
- `internal/tui/tui.go:387` - chat.thinking handler

## Critical Issue: The "Ghost Implementation"

**The code exists but doesn't appear.** This is the central mystery. Possible causes:

1. **Event Flow Break** - Events emitted but not reaching TUI
2. **Rendering Bug** - Content rendered but invisible (viewport, colors, overflow)
3. **Condition Not Met** - Features gated by some condition not triggered
4. **Layout Issue** - Elements rendered outside visible area or overlapped
5. **Viewport/Scroll Issue** - Content exists but scrolled out of view

## Files to Investigate

### Event Flow (verify events actually arrive):
- `pkg/gateway/server.go:507` - Event emission points
- `pkg/agent/loop.go:409-416` - Event generation from stream
- `internal/client/client.go:170-200` - WebSocket event dispatch
- `internal/tui/tui.go:360-430` - Event handlers in TUI

### Rendering (verify content is actually rendered):
- `internal/tui/tui.go:575-595` - View() composition, canvas rendering
- `internal/components/chat/messages.go:423-435` - View() with sync
- `internal/components/chat/messages.go:198-242` - renderContent() - CHECK THIS
- `internal/components/chat/message.go:221-270` - renderThinkingBlock
- `internal/components/header/header.go:70-110` - renderCompact

### Layout/Sizing (verify elements fit):
- `internal/tui/tui.go:220-245` - WindowSizeMsg handling, compact threshold
- Check: `m.height <= 30` triggers compact mode
- Check: chatH calculation for messages viewport

## The STREAM Issue

**Current behavior:** Stream appears as single line above thinking block, unreadable
**Expected:** Stream should appear in transcript like assistant message

Investigate:
- `internal/components/chat/messages.go:235` - STREAM block rendering
- Why is stream not in same flow as messages?
- Is viewport scrolling causing stream to appear at top?

## Debugging Strategy

1. **Verify events arrive:** Add logging in `internal/tui/tui.go` EventMsg handler
2. **Verify renderContent output:** Log the `lines` slice before join
3. **Check viewport state:** Log `m.viewport` dimensions and offset
4. **Force feature visibility:** Temporarily remove conditionals

## Test Commands

```bash
# Check current binary
ls -la /tmp/smolbot-tui
md5sum /tmp/smolbot-tui

# Check gateway events
sudo journalctl -u smolbot-go.service -f | grep -E "chat\.(thinking|usage|tool)"

# Test with fresh build
cd /home/nomadx/Documents/smolbot
go build -o /tmp/smolbot-tui-test ./cmd/smolbot-tui
/tmp/smolbot-tui-test
```

## Context from Hybrid Memory

Retrieve these entries:
- `tui-p2-implementation-status` - Current state
- `tui-critical-files` - File locations
- `tui-implementation-mystery` - The core mystery

## Spec References

- `docs/superpowers/plans/2026-03-24-p0-p1-tui-fixes.md`
- `docs/superpowers/plans/2026-03-24-p2-ui-polish.md`
- `docs/superpowers/plans/2026-03-24-review-handover.md`

## Success Criteria

When fixed, user should see:
- [ ] Context % in header (e.g., "context 15%")
- [ ] Tool output truncated with "… (N lines hidden)" hint
- [ ] Thinking blocks with "Thought for Xs" footer
- [ ] Compact header when terminal height ≤30 rows
- [ ] Stream appearing normally in transcript flow (not as separate line)
