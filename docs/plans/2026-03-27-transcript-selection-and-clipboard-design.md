# Transcript Selection And Clipboard Design

**Problem:** smolbot runs in alt-screen with mouse capture enabled, so normal terminal text selection does not work. The current TUI only supports copying the last assistant message via OSC 52 and does not support selecting arbitrary transcript lines with the mouse.

**Goal:** make transcript copying behave like Crush: drag-select transcript text, copy on mouse release, and use a more reliable clipboard write path than OSC 52 alone.

## Decision

Adopt exact Crush-style behavior:

- transcript selection is owned by the app, not the terminal
- mouse down starts a transcript selection
- mouse drag extends the selection
- mouse release copies the selected text immediately
- selection clears after successful copy

Clipboard writes should use:

1. `tea.SetClipboard(...)` as the primary path
2. native OS clipboard fallback via `github.com/atotto/clipboard`
3. optional OSC 52 fallback only if needed for edge terminals

## Why This Approach

The root issue is not just clipboard encoding. It is that smolbot captures the mouse while in alt-screen, which prevents normal terminal selection from happening. Disabling mouse capture would regress sidebar, dialogs, and scroll behavior. The right fix is to add app-owned transcript selection, matching Crush.

Crush already demonstrates the correct pattern:

- it keeps alt-screen and mouse interaction
- it tracks transcript highlight state itself
- it copies selected text via Bubble Tea clipboard plus native fallback

This is a narrower and safer change than trying to partially restore terminal-native selection.

## Scope

Included:

- transcript mouse selection
- highlight extraction from rendered transcript content
- copy-on-release behavior
- improved clipboard write path
- status flash feedback

Not included:

- persistent selection after copy
- keyboard-driven transcript selection
- image clipboard reads
- broader editor paste changes beyond preserving current behavior

## Architecture

### 1. Transcript Selection Model

The chat/messages layer should own selection state because it already owns transcript rendering and viewport behavior. It needs:

- mouse anchor position
- current drag position
- helpers to determine whether a selection exists
- a function to extract selected text from transcript-local plain-text lines
- a function to clear selection

This should be modeled after Crush’s `Chat` highlight flow, but adapted to smolbot’s simpler transcript structure.

Important implementation note: `MessagesModel` currently stores only the final ANSI-rendered transcript string inside the viewport. That is not enough for robust selection. The implementation needs a stable line map for selection, ideally built from plain-text transcript rows before ANSI styling is applied, then kept in sync with viewport offsets and wrapping.

### 2. TUI Event Routing

The TUI should remain a thin coordinator:

- translate global mouse coordinates into transcript-local coordinates
- send mouse down/drag/up only when the event is inside the transcript pane
- leave sidebar and dialog mouse handling unchanged
- trigger copy on release if a selection exists

### 3. Clipboard Path

Replace the current OSC 52-only copy helper with a layered copy helper:

- `tea.SetClipboard(text)`
- native clipboard write fallback using `atotto/clipboard`
- preserve user feedback through the existing flash/status mechanism

The current `Model.copyLastAssistantCmd()` and its tests rely on an injected writer for OSC 52 output. That harness will need to change when the copy path is moved to Bubble Tea clipboard plus native fallback. The design should treat clipboard writes as commands, not direct writes to an `io.Writer`.

This matches Crush’s text clipboard behavior and is more robust across terminals.

## Risks

1. Transcript selection depends on stable mapping between viewport coordinates and rendered transcript rows. The implementation must test wrapped lines and scrolling explicitly.
2. Mouse event routing must not break sidebar interaction or dialogs.
3. Copy-on-release should only trigger for real transcript selections, not every click.
4. Bubble Tea’s clipboard API does not provide an explicit success acknowledgement, so “clear after successful copy” should really mean “clear after dispatching the copy command/fallback sequence.”

## Testing

We need focused tests for:

- transcript selection across single and multiple lines
- mouse drag selection with viewport offsets
- wrapped-line selection inside the transcript viewport
- copy-on-release behavior
- clipboard command sequencing
- no regression in sidebar/dialog mouse consumption

## Recommendation

Implement this as a dedicated TUI improvement pass, using Crush as the behavioral reference but keeping the clipboard dependency surface minimal: Bubble Tea clipboard plus `atotto/clipboard` for text writes only.
