# Transcript Selection And Clipboard Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add Crush-style transcript mouse selection and reliable clipboard copying so users can drag-select transcript text and copy it on mouse release.

**Architecture:** Keep alt-screen and mouse capture enabled, but move transcript selection into the chat/messages layer. The TUI translates mouse events into transcript-local coordinates, while clipboard writes switch from OSC 52-only to Bubble Tea clipboard plus native fallback.

**Tech Stack:** Go, Bubble Tea v2, Lip Gloss, existing transcript viewport model, `github.com/atotto/clipboard`

---

### Task 1: Inspect Current Transcript And Clipboard Paths

**Files:**
- Read: `internal/tui/tui.go`
- Read: `internal/components/chat/messages.go`
- Read: `internal/components/chat/message.go`
- Read: `internal/components/common/clipboard.go`
- Reference: `/home/nomadx/crush/internal/ui/model/chat.go`
- Reference: `/home/nomadx/crush/internal/ui/model/ui.go`
- Reference: `/home/nomadx/crush/internal/ui/common/common.go`

**Step 1: Capture the current focused baseline**

Run:

```bash
go test ./internal/components/chat ./internal/tui -run 'Test(EventMsgUpdatesToolLifecycle|MouseWheelScrollsTranscript|SidebarAreaClicksAreConsumed|CopyShortcutWritesOSC52AndFlashesStatus)'
```

Expected: current focused tests pass.

**Step 2: Record exact ownership boundaries before edits**

Confirm:

- where transcript rendering is built
- where viewport offsets are stored
- where mouse events are consumed
- where copy feedback is surfaced

**Step 3: Commit baseline note**

```bash
git status --short
```

Expected: no new implementation edits yet.

---

### Task 2: Replace Clipboard Helper With Crush-Style Write Path

**Files:**
- Modify: `internal/components/common/clipboard.go`
- Test: `internal/components/common/clipboard_test.go`
- Modify if needed: `internal/tui/tui.go`
- Test if needed: `internal/tui/tui_test.go`
- Modify only if needed: `go.mod`

**Step 1: Write failing clipboard helper tests**

Add tests that verify:

- the helper emits a Bubble Tea clipboard command path
- native fallback write is attempted via an injected function
- the TUI copy path can still be tested without relying on raw OSC 52 output
- errors are handled cleanly

**Step 2: Run the focused test to confirm failure**

Run:

```bash
go test ./internal/components/common -run 'Test(WriteClipboard|ClipboardHelper)'
```

Expected: FAIL because the helper still only writes OSC 52 and the current TUI test harness is bound to raw OSC 52 output.

**Step 3: Implement a layered clipboard helper**

Replace the raw OSC 52 helper with a small API that:

- returns a `tea.Cmd` using `tea.SetClipboard(text)`
- optionally invokes injected native clipboard write logic
- keeps OSC 52 only as a fallback utility if still useful

Prefer dependency injection for tests instead of direct global writes.

Update `Model.copyLastAssistantCmd()` and its tests to consume the new helper instead of writing directly to `clipboardOut`.

**Step 4: Run the focused test to verify pass**

Run:

```bash
go test ./internal/components/common -run 'Test(WriteClipboard|ClipboardHelper)'
```

Expected: PASS.

**Step 5: Commit**

```bash
git add internal/components/common/clipboard.go internal/components/common/clipboard_test.go internal/tui/tui.go internal/tui/tui_test.go go.mod go.sum
git commit -m "feat(tui): improve clipboard write path"
```

---

### Task 3: Add Transcript Selection State To Messages

**Files:**
- Modify: `internal/components/chat/messages.go`
- Modify if needed: `internal/components/chat/message.go`
- Test: `internal/components/chat/messages_test.go`
- Add if useful: `internal/components/chat/selection_test.go`

**Step 1: Write failing transcript selection tests**

Add tests covering:

- selection start and drag inside transcript content
- single-line selection
- multi-line selection
- selection after viewport scrolling
- selection across wrapped transcript lines
- clearing selection

**Step 2: Run the focused test to confirm failure**

Run:

```bash
go test ./internal/components/chat -run 'Test(MessagesSelection|HighlightContent|ClearSelection)'
```

Expected: FAIL because selection APIs do not exist yet.

**Step 3: Implement minimal selection state**

Add state to `MessagesModel` for:

- selection anchor
- current drag position
- whether a selection is active
- a stable plain-text line map for transcript selection

Add methods such as:

- `HandleMouseDown(x, y int) bool`
- `HandleMouseDrag(x, y int) bool`
- `HandleMouseUp(x, y int) bool`
- `HasSelection() bool`
- `SelectedText() string`
- `ClearSelection()`

Keep the implementation transcript-focused only. Do not add keyboard selection in this pass.

Do not attempt selection directly against the final ANSI-rendered viewport string. Build selection against plain-text transcript lines and only then apply visual highlighting.

**Step 4: Apply selection rendering**

Highlight the selected transcript region in rendered output, using existing semantic theme colors or a minimal selection tint derived from them.

**Step 5: Run focused tests**

Run:

```bash
go test ./internal/components/chat -run 'Test(MessagesSelection|HighlightContent|ClearSelection)'
```

Expected: PASS.

**Step 6: Commit**

```bash
git add internal/components/chat/messages.go internal/components/chat/message.go internal/components/chat/messages_test.go internal/components/chat/selection_test.go
git commit -m "feat(tui): add transcript text selection"
```

---

### Task 4: Route Mouse Down/Drag/Release Through The Transcript Pane

**Files:**
- Modify: `internal/tui/tui.go`
- Test: `internal/tui/tui_test.go`

**Step 1: Write failing TUI mouse routing tests**

Add tests covering:

- mouse down in transcript starts selection
- mouse motion updates selection
- mouse release copies selection
- sidebar clicks are still consumed by the sidebar
- dialog mouse handling still takes priority

**Step 2: Run the focused tests to confirm failure**

Run:

```bash
go test ./internal/tui -run 'Test(MouseSelectionRoutesToTranscript|MouseReleaseCopiesSelection|SidebarAreaClicksAreConsumed|DialogMousePriority)'
```

Expected: FAIL because transcript selection routing does not exist.

**Step 3: Implement transcript-local mouse routing**

Update `Model.Update` to:

- handle `tea.MouseClickMsg`, `tea.MouseMotionMsg`, and `tea.MouseReleaseMsg` explicitly for transcript selection
- detect transcript-pane mouse down
- forward drag/release events to `MessagesModel`
- keep existing sidebar/dialog mouse ownership intact

Translate from global coordinates into transcript-local coordinates before forwarding.

**Step 4: Trigger copy-on-release**

On mouse release:

- if `messages.HasSelection()` is true, trigger the new clipboard helper
- clear the selection after dispatching the copy command sequence
- show the existing flash/status feedback

**Step 5: Run focused tests**

Run:

```bash
go test ./internal/tui -run 'Test(MouseSelectionRoutesToTranscript|MouseReleaseCopiesSelection|SidebarAreaClicksAreConsumed|DialogMousePriority)'
```

Expected: PASS.

**Step 6: Commit**

```bash
git add internal/tui/tui.go internal/tui/tui_test.go
git commit -m "feat(tui): copy transcript selections on mouse release"
```

---

### Task 5: Preserve Existing Copy/Paste Behavior

**Files:**
- Modify if needed: `internal/tui/tui.go`
- Modify if needed: `internal/components/chat/editor.go`
- Test: `internal/tui/tui_test.go`
- Test: `internal/components/chat/editor_test.go`

**Step 1: Add regression tests**

Cover:

- existing keyboard copy shortcut still works
- bracketed paste still inserts editor text
- transcript selection does not interfere with editor paste
- the updated copy tests no longer depend on raw OSC 52 bytes

**Step 2: Run focused tests to confirm any regressions**

Run:

```bash
go test ./internal/tui ./internal/components/chat -run 'Test(CopyShortcut|PasteMsg|TranscriptSelectionDoesNotBreakEditorPaste)'
```

Expected: PASS after implementation.

**Step 3: Make minimal fixes only if needed**

Do not redesign editor input. Keep this as regression protection.

**Step 4: Commit**

```bash
git add internal/tui/tui.go internal/tui/tui_test.go internal/components/chat/editor.go internal/components/chat/editor_test.go
git commit -m "test(tui): protect copy and paste regressions"
```

---

### Task 6: Run Full Feature Verification

**Files:**
- No code changes expected

**Step 1: Run feature-owned package tests**

```bash
go test ./internal/components/common ./internal/components/chat ./internal/tui
```

Expected: PASS.

**Step 2: Run any additional targeted package tests touched by clipboard dependencies**

```bash
go test ./...
```

Expected: if unrelated baseline failures remain, record them explicitly instead of claiming full-suite clean. Do not block feature completion on already-known repo-wide baseline failures.

**Step 3: Review against design**

Confirm:

- drag-select transcript text works
- release copies immediately
- selection clears after copy
- sidebar and dialog mouse behavior still work
- editor paste still works

**Step 4: Final commit**

```bash
git status --short
git add -A
git commit -m "feat(tui): add crush-style transcript copy behavior"
```
