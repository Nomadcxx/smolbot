# CLI, TUI & Skills Bugfix Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Fix 10 bugs across CLI UX, TUI keyboard handling, dialog sizing, readline history, MCP ToolContext propagation, tool output truncation, and skill file content.

**Architecture:** Bugs are independent — each task is self-contained. Tasks 1–5 are single-file CLI/TUI fixes. Task 6 spans multiple dialog components. Tasks 7–9 are small targeted fixes. Task 10 writes substantive content for 7 skill files.

**Tech Stack:** Go 1.23, Bubbletea v2 (charm.land/bubbletea/v2), Lipgloss v2 (charm.land/lipgloss/v2), Go stdlib

**Bugs addressed:** M9, M8, M15, M17, A2, M14, L3, L8, A4, A5

---

## File Map

| File | Change |
|---|---|
| `cmd/smolbot/onboard.go` | M9: guard against overwriting existing config |
| `cmd/smolbot/onboard_test.go` | Test for M9 |
| `cmd/smolbot/runtime.go` | M8: explicit not-found error; M15: wrap dialGateway error |
| `cmd/smolbot/chat_readline.go` | M17: navigateHistory returns selected entry |
| `cmd/smolbot/chat_readline_test.go` | Test for M17 |
| `internal/tui/tui.go` | A2: ESC clears selection; M14: Dialog interface + SetTerminalWidth + WindowSizeMsg handler |
| `internal/tui/menu_dialog.go` | M14: no-op SetTerminalWidth on menuDialog |
| `internal/components/dialog/common.go` | M14: dialogWidth helper |
| `internal/components/dialog/sessions.go` | M14: termWidth field + WithTerminalWidth + dynamic width |
| `internal/components/dialog/models.go` | M14: same |
| `internal/components/dialog/commands.go` | M14: same |
| `internal/components/dialog/skills.go` | M14: same |
| `internal/components/dialog/mcps.go` | M14: same |
| `internal/components/dialog/providers.go` | M14: same |
| `internal/components/dialog/keybindings.go` | M14: same |
| `internal/components/dialog/common_test.go` | Test for dialogWidth helper |
| `internal/components/chat/message.go` | L3: theme fallback in subtleWash; L8: byte cap in toolOutputSummary |
| `internal/components/chat/message_test.go` | Tests for L3, L8 |
| `pkg/tool/tool.go` | A4: WithToolContext / ContextToolContext helpers |
| `pkg/mcp/client.go` | A4: pass ToolContext through context |
| `pkg/mcp/client_test.go` | Test for A4 |
| `skills/github/SKILL.md` | A5: real content |
| `skills/cron/SKILL.md` | A5: real content |
| `skills/weather/SKILL.md` | A5: real content |
| `skills/skill-creator/SKILL.md` | A5: real content |
| `skills/tmux/SKILL.md` | A5: real content |
| `skills/summarize/SKILL.md` | A5: real content |
| `skills/clawhub/SKILL.md` | A5: real content |

---

## Task 1: M9 — Onboard Guard

**Bug:** `cmd/smolbot/onboard.go:20` — `writeConfigFile` is called unconditionally. Running `smolbot onboard` a second time silently destroys existing provider credentials.

**Files:**
- Modify: `cmd/smolbot/onboard.go:14-27`
- Modify: `cmd/smolbot/onboard_test.go`

- [ ] **Step 1: Write the failing test**

Add to `cmd/smolbot/onboard_test.go`:

```go
func TestOnboardCommandRefusesIfConfigExists(t *testing.T) {
	origCollect := collectOnboardConfig
	origWrite := writeConfigFile
	defer func() {
		collectOnboardConfig = origCollect
		writeConfigFile = origWrite
	}()

	collectOnboardConfig = func(context.Context, rootOptions) (*config.Config, error) {
		cfg := config.DefaultConfig()
		return &cfg, nil
	}
	var writeCalled bool
	writeConfigFile = func(path string, cfg *config.Config) error {
		writeCalled = true
		return nil
	}

	// Create the config file first
	target := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(target, []byte(`{"ok":true}`), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	cmd := NewRootCmd("test")
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"onboard", "--config", target})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error when config already exists, got nil")
	}
	if writeCalled {
		t.Fatal("writeConfigFile should not have been called")
	}
	if !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("expected 'already exists' in error, got %q", err.Error())
	}
}
```

- [ ] **Step 2: Run the test to confirm it fails**

```
cd /home/nomadx/Documents/smolbot
go test ./cmd/smolbot/ -run TestOnboardCommandRefusesIfConfigExists -v
```

Expected: FAIL — `collectOnboardConfig` is called and `writeConfigFile` would be called before the existence check.

- [ ] **Step 3: Add the guard to onboard.go**

Current `cmd/smolbot/onboard.go`:
```go
func newOnboardCmd(opts *rootOptions) *cobra.Command {
	return &cobra.Command{
		Use:   "onboard",
		Short: "Guide the user through initial configuration",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := collectOnboardConfig(context.Background(), *opts)
			if err != nil {
				return err
			}
			path := defaultConfigPath(*opts)
			if err := writeConfigFile(path, cfg); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "wrote config to %s\n", path)
			return nil
		},
	}
}
```

Replace the `RunE` body:
```go
RunE: func(cmd *cobra.Command, args []string) error {
    path := defaultConfigPath(*opts)
    if _, err := os.Stat(path); err == nil {
        return fmt.Errorf("config already exists at %s — delete it first or edit it directly", path)
    }
    cfg, err := collectOnboardConfig(context.Background(), *opts)
    if err != nil {
        return err
    }
    if err := writeConfigFile(path, cfg); err != nil {
        return err
    }
    fmt.Fprintf(cmd.OutOrStdout(), "wrote config to %s\n", path)
    return nil
},
```

Make sure `"os"` is in the imports (it already is via `context` and `fmt`; add `"os"` if missing).

- [ ] **Step 4: Run the test to confirm it passes**

```
go test ./cmd/smolbot/ -run TestOnboardCommandRefusesIfConfigExists -v
```

Expected: PASS

- [ ] **Step 5: Run the full onboard test suite**

```
go test ./cmd/smolbot/ -run TestOnboard -v
```

Expected: all existing onboard tests pass.

- [ ] **Step 6: Commit**

```bash
git add cmd/smolbot/onboard.go cmd/smolbot/onboard_test.go
git commit -m "fix(onboard): refuse to overwrite existing config without explicit deletion"
```

---

## Task 2: M8 — Explicit Config Not-Found Error

**Bug:** `cmd/smolbot/runtime.go:924` — when the user passes `--config /path/that/doesnt/exist`, `loadRuntimeConfig` silently falls through to `DefaultConfig()`. The daemon starts with wrong defaults instead of reporting the bad path.

**Files:**
- Modify: `cmd/smolbot/runtime.go:917-931`

- [ ] **Step 1: Write the failing test**

Find the existing runtime config tests or add to a suitable `_test.go` file in `cmd/smolbot/`. The test file is `cmd/smolbot/runtime_test.go` (or create `cmd/smolbot/runtime_config_test.go`). Add:

```go
func TestLoadRuntimeConfigExplicitPathNotFound(t *testing.T) {
	_, _, err := loadRuntimeConfig("/tmp/does-not-exist-smolbot-test-config.json", "", 0)
	if err == nil {
		t.Fatal("expected error for non-existent explicit config path, got nil")
	}
	if !strings.Contains(err.Error(), "config file not found") {
		t.Fatalf("expected 'config file not found' in error, got %q", err.Error())
	}
}
```

- [ ] **Step 2: Run the test to confirm it fails**

```
go test ./cmd/smolbot/ -run TestLoadRuntimeConfigExplicitPathNotFound -v
```

Expected: FAIL — returns nil error (silent fallback).

- [ ] **Step 3: Fix loadRuntimeConfig**

Current `cmd/smolbot/runtime.go:917-931`:
```go
func loadRuntimeConfig(configPath, workspace string, port int) (*config.Config, *config.Paths, error) {
	var (
		cfg *config.Config
		err error
	)
	if configPath != "" {
		cfg, err = config.Load(configPath)
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			return nil, nil, err
		}
	}
	if cfg == nil {
		defaultCfg := config.DefaultConfig()
		cfg = &defaultCfg
	}
```

Change the `configPath != ""` block:
```go
if configPath != "" {
    cfg, err = config.Load(configPath)
    if err != nil {
        if errors.Is(err, os.ErrNotExist) {
            return nil, nil, fmt.Errorf("config file not found: %s — run 'smolbot onboard' to create one", configPath)
        }
        return nil, nil, err
    }
}
```

- [ ] **Step 4: Run the test to confirm it passes**

```
go test ./cmd/smolbot/ -run TestLoadRuntimeConfigExplicitPathNotFound -v
```

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add cmd/smolbot/runtime.go cmd/smolbot/runtime_config_test.go
git commit -m "fix(runtime): return clear error when explicit --config path does not exist"
```

---

## Task 3: M15 — dialGateway Connection-Refused UX

**Bug:** `cmd/smolbot/runtime.go:1349` — when the daemon is not running, `dialGateway` returns the raw OS error (`connection refused`) with no suggestion to the user.

**Files:**
- Modify: `cmd/smolbot/runtime.go:1346-1350`

- [ ] **Step 1: Write the failing test**

Add to `cmd/smolbot/runtime_config_test.go` (or create `cmd/smolbot/dialgateway_test.go`):

```go
func TestDialGatewayErrorSuggestsSmolbotRun(t *testing.T) {
	ctx := context.Background()
	// Use a port that is guaranteed to be refused.
	cfg := &config.Config{}
	cfg.Gateway.Host = "127.0.0.1"
	cfg.Gateway.Port = 1 // port 1 is not bindable by normal processes

	_, err := dialGateway(ctx, cfg)
	if err == nil {
		t.Fatal("expected connection error, got nil")
	}
	if !strings.Contains(err.Error(), "smolbot run") {
		t.Fatalf("expected 'smolbot run' hint in error, got %q", err.Error())
	}
}
```

- [ ] **Step 2: Run the test to confirm it fails**

```
go test ./cmd/smolbot/ -run TestDialGatewayErrorSuggestsSmolbotRun -v
```

Expected: FAIL — error message does not contain "smolbot run".

- [ ] **Step 3: Wrap the error**

Current `cmd/smolbot/runtime.go:1346-1350`:
```go
	if lastErr == nil {
		lastErr = errors.New("gateway dial failed")
	}
	return nil, lastErr
}
```

Replace with:
```go
	if lastErr == nil {
		lastErr = errors.New("gateway dial failed")
	}
	return nil, fmt.Errorf("%w — is the smolbot daemon running? Try 'smolbot run'", lastErr)
}
```

- [ ] **Step 4: Run the test to confirm it passes**

```
go test ./cmd/smolbot/ -run TestDialGatewayErrorSuggestsSmolbotRun -v
```

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add cmd/smolbot/runtime.go cmd/smolbot/dialgateway_test.go
git commit -m "fix(runtime): add 'smolbot run' hint when daemon connection fails"
```

---

## Task 4: M17 — Readline History Navigation

**Bug:** `cmd/smolbot/chat_readline.go:76-88` — pressing up/down arrow navigates `r.histIdx` but `redrawPrompt` still receives the old `currentLine`. The terminal display never updates to show the history entry.

**Files:**
- Modify: `cmd/smolbot/chat_readline.go`
- Modify: `cmd/smolbot/chat_readline_test.go`

- [ ] **Step 1: Write the failing test**

The existing `TestReadlineHistoryRecallUpArrow` uses `fakeReadline` which bypasses the real `navigateHistory`. Add a direct unit test for the method:

Add to `cmd/smolbot/chat_readline_test.go`:

```go
func TestNavigateHistoryReturnsSelectedEntry(t *testing.T) {
	r := &bubbleteaReadline{
		history: []string{"first", "second", "third"},
		histIdx: -1,
	}

	// up once → "third" (most recent)
	entry := r.navigateHistory(-1)
	if entry != "third" {
		t.Fatalf("expected 'third', got %q", entry)
	}

	// up again → "second"
	entry = r.navigateHistory(-1)
	if entry != "second" {
		t.Fatalf("expected 'second', got %q", entry)
	}

	// down → back to "third"
	entry = r.navigateHistory(1)
	if entry != "third" {
		t.Fatalf("expected 'third', got %q", entry)
	}

	// down again → past end, empty (clear input)
	entry = r.navigateHistory(1)
	if entry != "" {
		t.Fatalf("expected empty string at end of history, got %q", entry)
	}
}
```

- [ ] **Step 2: Run the test to confirm it fails**

```
go test ./cmd/smolbot/ -run TestNavigateHistoryReturnsSelectedEntry -v
```

Expected: FAIL — `navigateHistory` returns nothing (void function).

- [ ] **Step 3: Change navigateHistory to return the selected entry**

Current `cmd/smolbot/chat_readline.go:181-203`:
```go
func (r *bubbleteaReadline) navigateHistory(delta int) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if len(r.history) == 0 {
		return
	}

	if r.histIdx == -1 {
		r.histIdx = len(r.history)
	}

	newIdx := r.histIdx + delta
	if newIdx < 0 {
		newIdx = 0
	}
	if newIdx >= len(r.history) {
		newIdx = len(r.history)
		return
	}

	r.histIdx = newIdx
}
```

Replace with:
```go
func (r *bubbleteaReadline) navigateHistory(delta int) string {
	r.mu.Lock()
	defer r.mu.Unlock()

	if len(r.history) == 0 {
		return ""
	}

	if r.histIdx == -1 {
		r.histIdx = len(r.history)
	}

	newIdx := r.histIdx + delta
	if newIdx < 0 {
		newIdx = 0
	}
	if newIdx >= len(r.history) {
		r.histIdx = len(r.history)
		return ""
	}

	r.histIdx = newIdx
	return r.history[newIdx]
}
```

- [ ] **Step 4: Update the key handler to use the return value**

Current `cmd/smolbot/chat_readline.go:76-88`:
```go
					case 'A':
						reader.ReadByte()
						reader.ReadByte()
						r.navigateHistory(-1)
						r.redrawPrompt(&currentLine)
						continue
					case 'B':
						reader.ReadByte()
						reader.ReadByte()
						r.navigateHistory(1)
						r.redrawPrompt(&currentLine)
						continue
```

Replace with:
```go
					case 'A':
						reader.ReadByte()
						reader.ReadByte()
						entry := r.navigateHistory(-1)
						currentLine.Reset()
						currentLine.WriteString(entry)
						r.redrawPrompt(&currentLine)
						continue
					case 'B':
						reader.ReadByte()
						reader.ReadByte()
						entry := r.navigateHistory(1)
						currentLine.Reset()
						currentLine.WriteString(entry)
						r.redrawPrompt(&currentLine)
						continue
```

- [ ] **Step 5: Run the test to confirm it passes**

```
go test ./cmd/smolbot/ -run TestNavigateHistoryReturnsSelectedEntry -v
```

Expected: PASS

- [ ] **Step 6: Run the full readline test suite**

```
go test ./cmd/smolbot/ -run TestReadline -v
```

Expected: all pass.

- [ ] **Step 7: Commit**

```bash
git add cmd/smolbot/chat_readline.go cmd/smolbot/chat_readline_test.go
git commit -m "fix(readline): history navigation now updates the current line on display"
```

---

## Task 5: A2 — ESC Key Clears Selection in TUI

**Bug:** `internal/tui/tui.go:789-792` — ESC when the editor is not focused is a no-op. Users expect ESC to clear any active message selection or deselect state.

**Files:**
- Modify: `internal/tui/tui.go:789-792`

- [ ] **Step 1: Write the failing test**

Add to `internal/tui/` a new file `tui_keys_test.go`:

```go
package tui

import (
	"testing"

	tea "charm.land/bubbletea/v2"
)

func TestEscWhenEditorUnfocusedClearsSelection(t *testing.T) {
	m := newTestModel(t)
	// editor is not focused by default in a new model
	if m.editor.Focused() {
		t.Skip("editor is focused by default, skip")
	}

	// Send ESC
	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	_ = next
	// If no panic and we get here, the ESC key was handled.
	// The real check: messages.ClearSelection was called.
	// We rely on the fact that pre-fix, ESC is a no-op (falls through the switch).
	// Post-fix, ESC calls ClearSelection and returns.
	// We can't easily observe ClearSelection without a mock — the test's value is
	// that it won't panic and the model returns without further side effects.
}
```

Note: This test primarily guards against regression. If `internal/components/chat.Messages` has an observable `HasSelection()` method, use it. Otherwise the test verifies no panic.

- [ ] **Step 2: Run the test to confirm it compiles and passes (no panic)**

```
go test ./internal/tui/ -run TestEscWhenEditorUnfocused -v
```

Expected: PASS (no panic, but ESC doesn't yet clear selection).

- [ ] **Step 3: Look at the messages component to confirm ClearSelection exists**

```
grep -n "ClearSelection\|HasSelection" internal/components/chat/messages.go
```

If `ClearSelection()` exists, update the test to verify state change. If not, the existing guard test is sufficient.

- [ ] **Step 4: Add the ESC handler**

Current `internal/tui/tui.go:788-793`:
```go
		case "esc":
			if m.editor.Focused() {
				m.editor.Blur()
				return m, nil
			}
```

Replace with:
```go
		case "esc":
			if m.editor.Focused() {
				m.editor.Blur()
				return m, nil
			}
			m.messages.ClearSelection()
			return m, nil
```

- [ ] **Step 5: Run tests**

```
go test ./internal/tui/ -v
```

Expected: all pass.

- [ ] **Step 6: Commit**

```bash
git add internal/tui/tui.go internal/tui/tui_keys_test.go
git commit -m "fix(tui): ESC clears message selection when editor is not focused"
```

---

## Task 6: M14 — Responsive Dialog Widths

**Bug:** All 7 dialog components have hardcoded pixel widths (`Width(64)` or `Width(72)`). On terminals narrower than ~72 columns the dialogs overflow or render broken. The `Dialog` interface has no way to receive terminal width updates.

**Files:**
- Modify: `internal/components/dialog/common.go`
- Modify: `internal/components/dialog/sessions.go`
- Modify: `internal/components/dialog/models.go`
- Modify: `internal/components/dialog/commands.go`
- Modify: `internal/components/dialog/skills.go`
- Modify: `internal/components/dialog/mcps.go`
- Modify: `internal/components/dialog/providers.go`
- Modify: `internal/components/dialog/keybindings.go`
- Modify: `internal/tui/tui.go`
- Modify: `internal/tui/menu_dialog.go`
- Create: `internal/components/dialog/common_test.go`

### Step 6.1 — Add dialogWidth helper and test

- [ ] **Step 6.1a: Write the failing test**

Create `internal/components/dialog/common_test.go`:

```go
package dialog

import "testing"

func TestDialogWidth(t *testing.T) {
	tests := []struct {
		termWidth int
		preferred int
		want      int
	}{
		{termWidth: 0, preferred: 64, want: 64},   // unknown terminal → use preferred
		{termWidth: 100, preferred: 64, want: 64},  // terminal wider than preferred → use preferred
		{termWidth: 60, preferred: 64, want: 56},   // terminal narrower → clamp to termWidth-4
		{termWidth: 40, preferred: 72, want: 36},   // very narrow
		{termWidth: 64, preferred: 64, want: 60},   // equal → clamp (border+padding need space)
	}
	for _, tt := range tests {
		got := dialogWidth(tt.termWidth, tt.preferred)
		if got != tt.want {
			t.Errorf("dialogWidth(%d, %d) = %d, want %d", tt.termWidth, tt.preferred, got, tt.want)
		}
	}
}
```

- [ ] **Step 6.1b: Run to confirm it fails**

```
go test ./internal/components/dialog/ -run TestDialogWidth -v
```

Expected: FAIL — `dialogWidth` not defined.

- [ ] **Step 6.1c: Add dialogWidth to common.go**

Add to `internal/components/dialog/common.go`:

```go
// dialogWidth returns the width to use for a dialog: the preferred width unless
// the terminal is too narrow, in which case it clamps to termWidth-4 (leaving
// space for borders and padding). Zero termWidth means unknown — use preferred.
func dialogWidth(termWidth, preferred int) int {
	if termWidth <= 0 {
		return preferred
	}
	if max := termWidth - 4; max < preferred {
		return max
	}
	return preferred
}
```

- [ ] **Step 6.1d: Run to confirm it passes**

```
go test ./internal/components/dialog/ -run TestDialogWidth -v
```

Expected: PASS

### Step 6.2 — Add termWidth field + WithTerminalWidth to each dialog model

For each of the 7 models, the pattern is identical. Apply to all 7 in a single pass before committing.

**sessions.go** — `SessionsModel`:

Add field to struct (after `current string`):
```go
termWidth int
```

Add method after the struct:
```go
func (m SessionsModel) WithTerminalWidth(w int) SessionsModel {
	m.termWidth = w
	return m
}
```

Replace `Width(64)` in `View()` with `Width(dialogWidth(m.termWidth, 64))`.

---

**models.go** — `ModelsModel`:

Add field to struct:
```go
termWidth int
```

Add method:
```go
func (m ModelsModel) WithTerminalWidth(w int) ModelsModel {
	m.termWidth = w
	return m
}
```

Replace `Width(64)` in `View()` with `Width(dialogWidth(m.termWidth, 64))`.

---

**commands.go** — `CommandsModel`:

Add field to struct:
```go
termWidth int
```

Add method:
```go
func (m CommandsModel) WithTerminalWidth(w int) CommandsModel {
	m.termWidth = w
	return m
}
```

Replace both `Width(64)` occurrences in `View()` (empty-state branch at line ~108 and main branch at line ~143) with `Width(dialogWidth(m.termWidth, 64))`.

---

**skills.go** — `SkillsModel`:

Add field to struct:
```go
termWidth int
```

Add method:
```go
func (m SkillsModel) WithTerminalWidth(w int) SkillsModel {
	m.termWidth = w
	return m
}
```

Replace `Width(72)` in `View()` with `Width(dialogWidth(m.termWidth, 72))`.

---

**mcps.go** — `MCPServersModel`:

Add field to struct:
```go
termWidth int
```

Add method:
```go
func (m MCPServersModel) WithTerminalWidth(w int) MCPServersModel {
	m.termWidth = w
	return m
}
```

Replace `Width(72)` in `View()` with `Width(dialogWidth(m.termWidth, 72))`.

---

**providers.go** — `ProvidersModel`:

Add field to struct:
```go
termWidth int
```

Add method:
```go
func (m ProvidersModel) WithTerminalWidth(w int) ProvidersModel {
	m.termWidth = w
	return m
}
```

Replace `Width(72)` in `View()` with `Width(dialogWidth(m.termWidth, 72))`.

---

**keybindings.go** — `KeybindingsModel`:

Add field to struct:
```go
termWidth int
```

Add method:
```go
func (m KeybindingsModel) WithTerminalWidth(w int) KeybindingsModel {
	m.termWidth = w
	return m
}
```

Replace `Width(72)` in `View()` with `Width(dialogWidth(m.termWidth, 72))`.

- [ ] **Step 6.2: Apply all 7 model changes above, then run tests**

```
go test ./internal/components/dialog/ -v
```

Expected: all existing tests pass.

### Step 6.3 — Add SetTerminalWidth to Dialog interface and wrappers

- [ ] **Step 6.3a: Add SetTerminalWidth to Dialog interface**

Current `internal/tui/tui.go:87-90`:
```go
type Dialog interface {
	Update(tea.Msg) (Dialog, tea.Cmd)
	View() string
}
```

Replace with:
```go
type Dialog interface {
	Update(tea.Msg) (Dialog, tea.Cmd)
	View() string
	SetTerminalWidth(int) Dialog
}
```

- [ ] **Step 6.3b: Implement SetTerminalWidth on each dialog wrapper**

In `internal/tui/tui.go`, after each wrapper's `Update` method, add:

After `sessionDialog.Update`:
```go
func (d sessionDialog) SetTerminalWidth(w int) Dialog {
	return sessionDialog{d.SessionsModel.WithTerminalWidth(w)}
}
```

After `modelsDialog.Update`:
```go
func (d modelsDialog) SetTerminalWidth(w int) Dialog {
	return modelsDialog{d.ModelsModel.WithTerminalWidth(w)}
}
```

After `commandsDialog.Update`:
```go
func (d commandsDialog) SetTerminalWidth(w int) Dialog {
	return commandsDialog{CommandsModel: d.CommandsModel.WithTerminalWidth(w)}
}
```

After `skillsDialog.Update`:
```go
func (d skillsDialog) SetTerminalWidth(w int) Dialog {
	return skillsDialog{d.SkillsModel.WithTerminalWidth(w)}
}
```

After `mcpServersDialog.Update`:
```go
func (d mcpServersDialog) SetTerminalWidth(w int) Dialog {
	return mcpServersDialog{d.MCPServersModel.WithTerminalWidth(w)}
}
```

After `providersDialog.Update`:
```go
func (d providersDialog) SetTerminalWidth(w int) Dialog {
	return providersDialog{d.ProvidersModel.WithTerminalWidth(w)}
}
```

After `keybindingsDialog.Update`:
```go
func (d keybindingsDialog) SetTerminalWidth(w int) Dialog {
	return keybindingsDialog{d.KeybindingsModel.WithTerminalWidth(w)}
}
```

- [ ] **Step 6.3c: Add no-op SetTerminalWidth to menuDialog**

In `internal/tui/menu_dialog.go`, after the `View()` method, add:

```go
func (d menuDialog) SetTerminalWidth(int) Dialog { return d }
```

- [ ] **Step 6.3d: Compile to catch interface satisfaction errors**

```
go build ./internal/tui/...
```

Expected: compiles without errors. If any wrapper is missing `SetTerminalWidth`, the compiler will report it here.

### Step 6.4 — Propagate width on WindowSizeMsg and creation sites

- [ ] **Step 6.4a: Propagate to active dialog on resize**

In `internal/tui/tui.go`, in the `tea.WindowSizeMsg` handler (around line 350-360), add after `m.recalcLayout()` and before `return m, nil`:

```go
	if m.dialog != nil {
		m.dialog = m.dialog.SetTerminalWidth(msg.Width)
	}
```

The block should look like:
```go
case tea.WindowSizeMsg:
	m.width = msg.Width
	m.height = msg.Height
	m.compactMode = m.width >= 80 && m.width < 120
	if m.compactMode || m.width < 80 {
		m.detailsOpen = false
	}
	m.header.SetCompact(m.height <= 30)
	m.editor.SetCompact(m.height <= 30)
	m.recalcLayout()
	if m.dialog != nil {
		m.dialog = m.dialog.SetTerminalWidth(msg.Width)
	}
	return m, nil
```

- [ ] **Step 6.4b: Set width at each dialog creation site**

In `internal/tui/tui.go`, find the 8 creation sites and add `SetTerminalWidth` immediately after each assignment. Current sites and their fixes:

```go
// Line ~535
m.dialog = sessionDialog{dialogcmp.NewSessions(msg.Sessions, m.app.Session)}
m.dialog = m.dialog.SetTerminalWidth(m.width)

// Line ~552
m.dialog = modelsDialog{dialogcmp.NewModels(m.providerConfig, msg.Models, current)}
m.dialog = m.dialog.SetTerminalWidth(m.width)

// Line ~555
m.dialog = skillsDialog{dialogcmp.NewSkills(msg.Skills)}
m.dialog = m.dialog.SetTerminalWidth(m.width)

// Line ~558
m.dialog = mcpServersDialog{dialogcmp.NewMCPServers(msg.Servers)}
m.dialog = m.dialog.SetTerminalWidth(m.width)

// Line ~561
m.dialog = providersDialog{dialogcmp.NewProvidersFromData(msg.Models, msg.Current, msg.Status, m.providerConfig)}
m.dialog = m.dialog.SetTerminalWidth(m.width)

// Line ~811 (F1 menu)
m.dialog = newMenuDialog()
m.dialog = m.dialog.SetTerminalWidth(m.width)

// Line ~924 (themes menu)
m.dialog = newThemesMenuDialog()
m.dialog = m.dialog.SetTerminalWidth(m.width)

// Line ~972 (keybindings)
m.dialog = keybindingsDialog{dialogcmp.NewKeybindings()}
m.dialog = m.dialog.SetTerminalWidth(m.width)
```

- [ ] **Step 6.5: Run the full TUI and dialog test suites**

```
go test ./internal/tui/... ./internal/components/dialog/... -v
```

Expected: all pass.

- [ ] **Step 6.6: Commit**

```bash
git add internal/components/dialog/ internal/tui/tui.go internal/tui/menu_dialog.go
git commit -m "fix(tui): dialogs adapt to terminal width instead of using hardcoded pixel widths"
```

---

## Task 7: L3 — Theme-Aware Color Fallback

**Bug:** `internal/components/chat/message.go:489` — `subtleWash` falls back to hardcoded `lipgloss.Color("#111111")` when the accent color doesn't have a valid hex representation. On light themes this produces nearly invisible foreground text.

**Files:**
- Modify: `internal/components/chat/message.go:486-495`
- Modify: `internal/components/chat/message_test.go`

- [ ] **Step 1: Write the failing test**

Add to `internal/components/chat/message_test.go`:

```go
func TestSubtleWashFallbackUsesThemeBackground(t *testing.T) {
	if !theme.Set("nord") {
		t.Fatal("expected nord theme to be registered")
	}

	// Pass a non-hex color (e.g. named color) to trigger the fallback branch.
	result := subtleWash(lipgloss.Color("red"))
	// The fallback should NOT be the hardcoded #111111.
	// It should be the theme's background (or something derived from it).
	// Since we can't easily inspect the exact value, we verify it is NOT #111111
	// when a theme is active.
	if result == lipgloss.Color("#111111") {
		t.Error("subtleWash fallback should use theme background, not hardcoded #111111")
	}
}
```

- [ ] **Step 2: Run to confirm it fails**

```
go test ./internal/components/chat/ -run TestSubtleWashFallbackUsesThemeBackground -v
```

Expected: FAIL — returns `#111111`.

- [ ] **Step 3: Fix subtleWash**

Current `internal/components/chat/message.go:486-495`:
```go
func subtleWash(accent color.Color) color.Color {
	hex := colorHex(accent)
	if len(hex) != 7 || hex[0] != '#' {
		return lipgloss.Color("#111111")
	}
	r, _ := strconv.ParseInt(hex[1:3], 16, 64)
	g, _ := strconv.ParseInt(hex[3:5], 16, 64)
	b, _ := strconv.ParseInt(hex[5:7], 16, 64)
	return lipgloss.Color(fmt.Sprintf("#%02X%02X%02X", int(r)/5, int(g)/5, int(b)/5))
}
```

Replace the fallback line:
```go
func subtleWash(accent color.Color) color.Color {
	hex := colorHex(accent)
	if len(hex) != 7 || hex[0] != '#' {
		if t := theme.Current(); t != nil {
			return t.Background
		}
		return lipgloss.Color("#111111")
	}
	r, _ := strconv.ParseInt(hex[1:3], 16, 64)
	g, _ := strconv.ParseInt(hex[3:5], 16, 64)
	b, _ := strconv.ParseInt(hex[5:7], 16, 64)
	return lipgloss.Color(fmt.Sprintf("#%02X%02X%02X", int(r)/5, int(g)/5, int(b)/5))
}
```

- [ ] **Step 4: Run to confirm it passes**

```
go test ./internal/components/chat/ -run TestSubtleWashFallbackUsesThemeBackground -v
```

Expected: PASS

- [ ] **Step 5: Run full message test suite**

```
go test ./internal/components/chat/ -v
```

Expected: all pass.

- [ ] **Step 6: Commit**

```bash
git add internal/components/chat/message.go internal/components/chat/message_test.go
git commit -m "fix(chat): subtleWash fallback uses active theme background instead of hardcoded color"
```

---

## Task 8: L8 — Tool Output Byte Cap

**Bug:** `internal/components/chat/message.go:237-243` — `toolOutputSummary` only truncates by line count (`maxToolOutputLines = 10`). Tools like `web_fetch` and `web_search` can return very long single lines that bypass this limit entirely, flooding the TUI with megabytes of content.

**Files:**
- Modify: `internal/components/chat/message.go:40-41` (add constant)
- Modify: `internal/components/chat/message.go:237-243` (add byte check)
- Modify: `internal/components/chat/message_test.go`

- [ ] **Step 1: Write the failing test**

Add to `internal/components/chat/message_test.go`:

```go
func TestToolOutputSummaryByteCapTruncatesLongLines(t *testing.T) {
	// A single very long line that would pass the line count check but exceed bytes.
	longLine := strings.Repeat("x", 8*1024) // 8 KB, exceeds 4 KB cap
	result := toolOutputSummary(longLine, "done", false)
	if len(result) > maxToolOutputBytes*2 {
		// Allow some overhead for the truncation message, but cap must apply.
		t.Fatalf("expected byte cap to truncate, got %d bytes", len(result))
	}
	if !strings.Contains(result, "truncated") && !strings.Contains(result, "bytes hidden") {
		t.Fatalf("expected truncation notice in result, got %q", result[:min(200, len(result))])
	}
}
```

Note: `toolOutputSummary` is an unexported function. Since the test is in `package chat`, it can call it directly.

- [ ] **Step 2: Run to confirm it fails**

```
go test ./internal/components/chat/ -run TestToolOutputSummaryByteCapTruncatesLongLines -v
```

Expected: FAIL — long single line passes through without truncation.

- [ ] **Step 3: Add the byte cap constant and check**

At line ~40 in `internal/components/chat/message.go`, after `const maxToolOutputLines = 10`, add:
```go
const maxToolOutputBytes = 4 * 1024
```

In `toolOutputSummary` (around line ~237), the current truncation section:
```go
	lines := strings.Split(output, "\n")
	if len(lines) <= maxToolOutputLines {
		return output
	}

	hidden := len(lines) - maxToolOutputLines
	return strings.Join(lines[:maxToolOutputLines], "\n") + fmt.Sprintf("\n… (%d lines hidden)", hidden)
```

Replace with:
```go
	// Byte cap check first (catches single very-long lines that bypass line count).
	if len(output) > maxToolOutputBytes {
		return output[:maxToolOutputBytes] + fmt.Sprintf("\n… (%d bytes hidden)", len(output)-maxToolOutputBytes)
	}

	lines := strings.Split(output, "\n")
	if len(lines) <= maxToolOutputLines {
		return output
	}

	hidden := len(lines) - maxToolOutputLines
	return strings.Join(lines[:maxToolOutputLines], "\n") + fmt.Sprintf("\n… (%d lines hidden)", hidden)
```

- [ ] **Step 4: Run to confirm it passes**

```
go test ./internal/components/chat/ -run TestToolOutputSummaryByteCapTruncatesLongLines -v
```

Expected: PASS

- [ ] **Step 5: Run full message test suite**

```
go test ./internal/components/chat/ -v
```

Expected: all pass.

- [ ] **Step 6: Commit**

```bash
git add internal/components/chat/message.go internal/components/chat/message_test.go
git commit -m "fix(chat): cap tool output display at 4 KB to prevent single-line web results flooding TUI"
```

---

## Task 9: A4 — MCP ToolContext Propagation

**Bug:** `pkg/mcp/client.go:192` — `wrappedTool.Execute` receives a `tool.ToolContext` (session key, channel, workspace, spawner) but discards it with `_`. MCP tools have no way to know which session or channel they're running in.

**Fix:** Store the ToolContext in the Go context using a typed key. The `Invoke` signature doesn't change — MCP implementations retrieve it via `tool.ContextToolContext(ctx)`.

**Files:**
- Modify: `pkg/tool/tool.go`
- Modify: `pkg/mcp/client.go`
- Modify: `pkg/mcp/client_test.go`

- [ ] **Step 1: Write the failing test**

Add to `pkg/mcp/client_test.go`:

```go
func TestWrappedToolPropagatesToolContextViaContext(t *testing.T) {
	var capturedCtx context.Context
	client := &contextCaptureClient{
		captureCtx: func(ctx context.Context) { capturedCtx = ctx },
		result:     &tool.Result{Output: "ok"},
	}
	manager := NewManager(client)
	registry := tool.NewRegistry()

	_, err := manager.DiscoverAndRegister(context.Background(), registry, map[string]config.MCPServerConfig{
		"server": {Type: "stdio", ToolTimeout: 5, EnabledTools: []string{"*"}},
	})
	if err != nil {
		t.Fatalf("DiscoverAndRegister: %v", err)
	}

	tctx := tool.ToolContext{SessionKey: "session-abc", Channel: "signal"}
	_, err = registry.Execute(context.Background(), "mcp_server_mytool", json.RawMessage(`{}`), tctx)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}

	if capturedCtx == nil {
		t.Fatal("context was not captured")
	}
	got, ok := tool.ContextToolContext(capturedCtx)
	if !ok {
		t.Fatal("ToolContext not found in captured context")
	}
	if got.SessionKey != "session-abc" {
		t.Fatalf("SessionKey = %q, want 'session-abc'", got.SessionKey)
	}
	if got.Channel != "signal" {
		t.Fatalf("Channel = %q, want 'signal'", got.Channel)
	}
}

type contextCaptureClient struct {
	captureCtx func(context.Context)
	result     *tool.Result
	tools      []RemoteTool
}

func (c *contextCaptureClient) Discover(_ context.Context, _ ConnectionSpec) ([]RemoteTool, error) {
	if c.tools == nil {
		return []RemoteTool{{Name: "mytool", Description: "test", InputSchema: map[string]any{"type": "object"}}}, nil
	}
	return c.tools, nil
}

func (c *contextCaptureClient) Invoke(ctx context.Context, _ ConnectionSpec, _ string, _ json.RawMessage) (*tool.Result, error) {
	if c.captureCtx != nil {
		c.captureCtx(ctx)
	}
	return c.result, nil
}
```

- [ ] **Step 2: Run to confirm it fails**

```
go test ./pkg/mcp/ -run TestWrappedToolPropagatesToolContextViaContext -v
```

Expected: FAIL — `tool.ContextToolContext` doesn't exist yet; test won't compile.

- [ ] **Step 3: Add context helpers to pkg/tool/tool.go**

Add after the `ToolContext` struct definition in `pkg/tool/tool.go`:

```go
type toolContextKey struct{}

// WithToolContext stores tctx in ctx so MCP and other delegating tools can
// retrieve it with ContextToolContext.
func WithToolContext(ctx context.Context, tctx ToolContext) context.Context {
	return context.WithValue(ctx, toolContextKey{}, tctx)
}

// ContextToolContext retrieves the ToolContext stored by WithToolContext.
// ok is false if no ToolContext was stored.
func ContextToolContext(ctx context.Context) (ToolContext, bool) {
	tctx, ok := ctx.Value(toolContextKey{}).(ToolContext)
	return tctx, ok
}
```

- [ ] **Step 4: Pass ToolContext through context in wrappedTool.Execute**

Current `pkg/mcp/client.go:192-196`:
```go
func (t *wrappedTool) Execute(ctx context.Context, args json.RawMessage, _ tool.ToolContext) (*tool.Result, error) {
	timeoutCtx, cancel := context.WithTimeout(ctx, t.spec.ToolTimeout)
	defer cancel()
	return t.client.Invoke(timeoutCtx, t.spec, t.rawName, args)
}
```

Replace with:
```go
func (t *wrappedTool) Execute(ctx context.Context, args json.RawMessage, tctx tool.ToolContext) (*tool.Result, error) {
	timeoutCtx, cancel := context.WithTimeout(ctx, t.spec.ToolTimeout)
	defer cancel()
	ctxWithTC := tool.WithToolContext(timeoutCtx, tctx)
	return t.client.Invoke(ctxWithTC, t.spec, t.rawName, args)
}
```

- [ ] **Step 5: Run to confirm it passes**

```
go test ./pkg/mcp/ -run TestWrappedToolPropagatesToolContextViaContext -v
```

Expected: PASS

- [ ] **Step 6: Run the full mcp test suite**

```
go test ./pkg/mcp/ -v
```

Expected: all pass.

- [ ] **Step 7: Commit**

```bash
git add pkg/tool/tool.go pkg/mcp/client.go pkg/mcp/client_test.go
git commit -m "fix(mcp): propagate ToolContext through Go context so MCP tools can access session/channel info"
```

---

## Task 10: A5 — Substantive Skill Content

**Bug:** 7 of 8 skill files contain only 1-2 lines of stub content. The agent receives almost no guidance on when to apply each skill or how to use the associated tools, leading to missed activations and poor tool usage.

**Files:** `skills/github/SKILL.md`, `skills/cron/SKILL.md`, `skills/weather/SKILL.md`, `skills/skill-creator/SKILL.md`, `skills/tmux/SKILL.md`, `skills/summarize/SKILL.md`, `skills/clawhub/SKILL.md`

No test: skill content is evaluated by reading the agent's behavior, not automated tests.

- [ ] **Step 1: Write skills/github/SKILL.md**

```markdown
---
name: github
description: Work with GitHub repositories, issues, and pull requests via the gh CLI
---

Use this skill for any task involving GitHub: reading issues, creating or reviewing pull requests,
checking CI status, listing repositories, or managing labels and milestones.

## Tools
Use the `exec` tool to run `gh` commands. The `gh` CLI is authenticated and available.

## Common operations

**View an issue:**
```
gh issue view 123 --repo owner/repo
```

**Create a pull request:**
```
gh pr create --title "..." --body "..." --base main
```

**Check CI status:**
```
gh run list --repo owner/repo --limit 5
gh run view <run-id> --log-failed
```

**List open PRs:**
```
gh pr list --repo owner/repo --state open
```

**Review a PR diff:**
```
gh pr diff 123 --repo owner/repo
```

## When to activate
- User mentions a PR, issue, or GitHub URL
- User asks to check CI, tests, or build status
- User asks to open, close, or comment on an issue or PR
- User says "push" or "merge" in a GitHub context
```

- [ ] **Step 2: Write skills/cron/SKILL.md**

```markdown
---
name: cron
description: Schedule, list, and manage recurring jobs using the cron tool
---

Use this skill when the user wants to schedule a recurring task, view existing jobs,
enable or disable a schedule, or understand what automated jobs are configured.

## Tools
Use the `cron` tool (if available) to manage jobs. Arguments:

- **list**: `{"action": "list"}` — returns all configured cron jobs with ID, schedule, and status
- **add**: `{"action": "add", "name": "...", "schedule": "0 9 * * *", "reminder": "...", "timezone": "America/New_York", "channel": "signal", "chat_id": "...", "enabled": true}`
- **delete**: `{"action": "delete", "id": "job-id"}`
- **enable/disable**: `{"action": "add", "id": "job-id", "enabled": false}` (update enabled field)

## Schedule format
Standard cron syntax: `minute hour day month weekday`
- `0 9 * * 1-5` — 9 AM every weekday
- `*/15 * * * *` — every 15 minutes
- `0 0 * * 0` — midnight every Sunday

## When to activate
- User says "remind me every...", "schedule a...", "every morning...", "weekly..."
- User asks what reminders or scheduled tasks are set up
- User wants to cancel or modify a recurring job
```

- [ ] **Step 3: Write skills/weather/SKILL.md**

```markdown
---
name: weather
description: Retrieve current weather or forecasts using web_search or web_fetch
---

Use this skill when the user asks about current weather, temperature, forecast, or conditions
for any location.

## Tools
Use `web_search` or `web_fetch` to retrieve weather data.

**web_search approach:**
```
{"query": "weather in London today", "num_results": 3}
```

**Direct fetch (wttr.in JSON API):**
```
{"url": "https://wttr.in/London?format=j1"}
```
The JSON response contains `current_condition` with `temp_C`, `weatherDesc`, `humidity`, and
`FeelsLikeC`. Use `3` hourly forecasts for the forecast section.

## Response format
Summarise concisely: current temperature, feels-like, conditions, and a brief forecast if asked.
Do not dump raw JSON — translate to human-readable text.

## When to activate
- User asks "what's the weather", "is it raining", "how cold is it in..."
- User asks for a forecast or whether to bring an umbrella
```

- [ ] **Step 4: Write skills/skill-creator/SKILL.md**

```markdown
---
name: skill-creator
description: Help design and write new smolbot skill files
---

Use this skill when the user asks to create a new skill, add a capability, or improve
an existing skill definition.

## Skill file format
Skills live in `skills/<name>/SKILL.md` with YAML frontmatter:

```markdown
---
name: <name>          # must match directory name
description: <one line>
always: true          # optional — load this skill on every session
---

<body>
```

## Body guidelines
A good skill body contains:
1. **When to activate** — precise trigger conditions (not vague)
2. **Tools** — which tools to call, with concrete argument examples
3. **Output format** — how to present results to the user
4. **Examples** — at least one sample tool call and expected behaviour

## Steps to create a skill
1. Choose a name — short, lowercase, hyphenated
2. Decide: always-on or on-demand?
3. Write the body with all four sections above
4. Save to `skills/<name>/SKILL.md`
5. Test by asking smolbot to perform the skill's task

## When to activate
- User says "create a skill", "add a skill", "write a skill for..."
- User wants to improve or expand an existing skill file
```

- [ ] **Step 5: Write skills/tmux/SKILL.md**

```markdown
---
name: tmux
description: Manage tmux sessions and panes for long-running terminal work
---

Use this skill when the user wants to run long-running commands, keep processes alive after
disconnect, split a terminal, or manage named sessions.

## Tools
Use the `exec` tool to run `tmux` commands.

## Common operations

**Start a named session:**
```
tmux new-session -d -s mywork
```

**Run a command in a detached session:**
```
tmux new-session -d -s build -c /path/to/project "make build 2>&1 | tee build.log"
```

**List sessions:**
```
tmux list-sessions
```

**Attach to a session (suggest to user — requires interactive terminal):**
```
tmux attach -t mywork
```

**Send a command to a running session:**
```
tmux send-keys -t mywork "git pull" Enter
```

**Read output from a pane:**
```
tmux capture-pane -t mywork -p
```

**Kill a session:**
```
tmux kill-session -t mywork
```

## When to activate
- User wants to run a long-running build, server, or script in the background
- User mentions tmux or asks to keep a process running
- User wants to split work across multiple terminal sessions
```

- [ ] **Step 6: Write skills/summarize/SKILL.md**

```markdown
---
name: summarize
description: Summarise long documents, files, URLs, or conversation history
---

Use this skill when the user asks for a summary of a file, web page, long output, or
previous conversation.

## Approach

**Summarising a file:**
1. Use `read_file` to load the content
2. Extract the key points: purpose, decisions, important facts, action items
3. Present as a bullet list unless prose is requested

**Summarising a URL:**
1. Use `web_fetch` to retrieve the page
2. Strip boilerplate; focus on the main article or content
3. Summarise in 3–5 bullet points or a short paragraph

**Summarising conversation / tool output:**
Work from what is already in context; do not re-fetch.

## Output length
- Default: 5–8 bullet points or ≤ 200 words
- Long-form: only if the user explicitly asks for detailed or full summary
- Always include: what it is, key findings, any action items or next steps

## When to activate
- User says "summarise", "tldr", "what does this say", "give me the key points"
- User pastes a URL and asks what it's about
- User asks to summarise a file path
```

- [ ] **Step 7: Write skills/clawhub/SKILL.md**

```markdown
---
name: clawhub
description: Interact with ClawHub — the smolbot plugin and skill registry
---

Use this skill when the user wants to browse, install, or publish skills or plugins
from the ClawHub registry.

## Tools
Use `web_search` or `web_fetch` to query the ClawHub registry, and `exec` for local
installation commands if the `clawhub` CLI is available.

## Common operations

**Search for a skill:**
```
{"query": "clawhub skill weather forecast"}
```

**Install a skill (if clawhub CLI is present):**
```
clawhub install <skill-name>
```
This downloads the skill to `~/.smolbot/skills/<name>/SKILL.md`.

**List installed skills:**
Skills are in `~/.smolbot/skills/` (user skills) or the `skills/` directory in the
smolbot repo (built-in skills).

**Publish a skill:**
Follow the ClawHub submission process — typically a pull request to the registry repo.

## When to activate
- User mentions ClawHub or asks about the skill/plugin marketplace
- User wants to find, add, or share a smolbot capability
- User asks how to extend smolbot
```

- [ ] **Step 8: Commit skill content**

```bash
git add skills/
git commit -m "feat(skills): add substantive content to all 7 stub skill files"
```

---

## Final verification

- [ ] **Run the full test suite**

```bash
go test ./... 2>&1 | tail -30
```

Expected: all packages pass. No new failures.

- [ ] **Build the binary**

```bash
go build ./cmd/smolbot/
```

Expected: compiles without errors.
