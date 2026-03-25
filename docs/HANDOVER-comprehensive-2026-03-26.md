# Comprehensive Handover Document

**Date:** 2026-03-26
**Session:** smolbot backend improvements and WhatsApp installer integration

---

## 1. Backend Improvements (Completed Earlier Session)

### Commits on `main`:
- `820f0f7`: populate Data map in EventContextCompressed
- `bdc1516`: handle EventToolHint in gateway
- `e1fd869`: include tool output in EventToolDone  
- `3342a9b`: log writeEvent errors via emitEvent
- `b667766`: include compression mode and timestamp
- `ab673ed`: log memory consolidation errors
- `d89389a`: cron loop continues after errors
- `75b7deb`: heartbeat loop continues after errors
- `aa3c92d`: validate heartbeat interval > 0
- `3872da6`: WebSocket ping/pong in gateway
- `2581e2e`: WebSocket ping in client
- `e5f8b8c`: log memory consolidation errors in /new handler
- `5482233`: merge: backend gap2 fixes
- `1c55af3`: fix: log errors in heartbeat, whatsapp, gateway event writes

### What These Fix:
- EventContextCompressed Data map now populated with `originalTokens`, `compressedTokens`, `reductionPercent`, `mode`
- EventToolHint handled in gateway → `chat.tool.hint` event
- EventToolDone includes truncated tool output
- All `writeEvent` errors logged via `emitEvent` helper
- Cron/heartbeat loops log errors and continue (not `return`)
- Heartbeat interval validation (errors if enabled but ≤0)
- WebSocket ping/pong keepalive (30s interval)
- Memory consolidation errors logged

---

## 2. WhatsApp Improvements Worktree (Merged)

### Commits on `whatsapp-improvements` (merged to main):
- `71e9319`: feat(whatsapp): add status tracking (lastConnectedAt, lastMessageAt, reconnectCount)
- `a1c7ac7`: feat(whatsapp): add message deduplication with 5-minute sliding window
- `c28683b`: feat(whatsapp): add error categorization and structured logging
- `5802cb9`: feat(whatsapp): add interactive TUI login with ASCII QR code

### Files Created:
- `pkg/channel/whatsapp/terminal.go` - ASCII QR code renderer
- `cmd/smolbot/channels_whatsapp_login.go` - bubblesTea TUI model for WhatsApp login
- `cmd/smolbot/channels_login.go` - wired up WhatsApp TUI login

### Gap Analysis Findings (smolbot vs OpenClaw):
| Feature | OpenClaw | Smolbot |
|---------|----------|---------|
| Media handling | ✓ | ✗ |
| Configurable reconnect | ✓ | ✗ |
| Error extraction | ✓ | ✗ |
| Deduplication | ✓ | ✗ |
| Debouncing | ✓ | ✗ |
| Access control | ✓ | ✗ |
| Group support | ✓ | ✗ |

---

## 3. Installer TUI Improvements

### Bug Fixes:
- **Channels step navigation** - Added up/down keys to navigate between Signal and WhatsApp options
- **Model sync** - `signalEnabled`/`whatsappEnabled` now properly sync between model and globals
- **Space toggle** - Changed toggle from left/right arrows to Space key

### Commits:
- `5af7afe`: fix(installer): add up/down navigation to channels step
- `e400097`: fix(installer): sync channel toggles to/from model properly
- `81fe1af`: feat(installer): add WhatsApp setup step, space to toggle channels
- `a088565`: feat(installer): integrate WhatsApp QR login via subprocess
- `99cc757`: fix(installer): show WhatsApp login errors, capture stderr
- `0342dd0`: fix(installer): improve smolbot binary lookup with PATH search
- `cbcce7e`: fix(installer): debug to log file, not stdout

### Files Modified:
- `cmd/installer/main.go` - Key handlers, WhatsApp setup flow
- `cmd/installer/views.go` - Render functions, help text
- `cmd/installer/types.go` - Model struct
- `cmd/installer/tasks.go` - Task list
- `cmd/installer/utils.go` - Global state

---

## 4. WhatsApp Installer Direct Linking (BROKEN)

### Goal
Allow installer to link WhatsApp directly using Go/Whatsmeow (no smolbot binary needed). Creates handshake file at `~/.smatsapp/whatsapp.db`.

### What Works (Confirmed)

1. **WhatsAppLinker** (`cmd/installer/whatsapp_link.go`):
```go
type WhatsAppLinker struct {
    storePath string
    client   *whatsmeow.Client
}
func NewWhatsAppLinker(storePath string) (*WhatsAppLinker, error)
func (l *WhatsAppLinker) IsLinked() bool
func (l *WhatsAppLinker) StartLinking(onQR QRCallback, onStatus StatusCallback) error
```
- Correctly connects to WhatsApp and receives QR codes

2. **QRRenderer** (`pkg/channel/whatsapp/terminal.go`):
```go
type QRRenderer struct{ size int }
func NewQRRenderer(size int) *QRRenderer
func (r *QRRenderer) RenderToASCII(data string) (string, error)
```
- Correctly renders QR codes as ASCII art

3. **Standalone Test** (`cmd/testqr/main.go`):
- Run `go run ./cmd/testqr/` - receives QR from WhatsApp and saves as PNG
- Confirms both linking and rendering work

### What's Broken

**Installer TUI QR display** - The QR code never appears in the installer TUI. User sees only a spinner.

### Root Cause

The `startWhatsAppLink()` function uses a goroutine that sends messages via `program.Send()`:

```go
go func() {
    onQR := func(code string) {
        if prog != nil {
            prog.Send(whatsappQRMsg{code: code})
        }
    }
    // ...
}()
return whatsappLoginResult{success: true} // RETURNS IMMEDIATELY
```

The tea command returns `whatsappLoginResult{success: true}` BEFORE the goroutine receives the QR code. The goroutine is orphaned and its `program.Send()` messages are never processed by the tea event loop.

### Suggested Fix

Replace the goroutine approach with **tick-based polling**:

1. Modify `WhatsAppLinker` to have `Start(ctx)` (non-blocking) and `Poll()` methods
2. `handleWhatsAppSetupKeys` returns a tick command that polls the linker
3. Update loop handles poll results and re-renders view

### Files to Modify

| File | Change |
|------|--------|
| `cmd/installer/whatsapp_link.go` | Add `Start()` and `Poll()` methods |
| `cmd/installer/main.go` | Rewrite to use tick-based polling |
| `cmd/installer/types.go` | Add `linker *WhatsAppLinker` field |

---

## 5. Plans Created

- `docs/plans/2026-03-26-whatsapp-improvements.md` - Original WhatsApp improvements plan
- `docs/plans/2026-03-26-whatsapp-onboarding-cli.md` - Onboarding CLI plan
- `docs/plans/2026-03-26-whatsapp-installer-direct-link.md` - Direct linking implementation plan
- `docs/HANDOVER-whatsapp-installer-qr.md` - Quick handover for QR issue

---

## 6. Commands

```bash
# Test QR generation (standalone, works)
cd /home/nomadx/Documents/smolbot && go run ./cmd/testqr/

# Build installer
cd /home/nomadx/Documents/smolbot && go build -o /tmp/smolbot-installer ./cmd/installer

# Build smolbot
cd /home/nomadx/Documents/smolbot && go build -o /tmp/smolbot ./cmd/smolbot

# Run installer
/tmp/smolbot-installer

# Run smolbot
/tmp/smolbot run "your prompt"
```

---

## 7. Current Branch State

**Branch:** `main` (clean, pushed to origin)

**Uncommitted files:**
- `EOF` (artifact)
- `docs/handover-complete-p0-p1-p2.md`
- `docs/handover-p2-tui-issues.md`
- `docs/plans/2026-03-23-tui-transcript-ux.md`
- `docs/superpowers/`
- `internal/components/chat/render_test.go`
- `internal/components/chatlist/`
- `internal/tui/drawable.go`

---

## 8. What's Left

1. **Fix WhatsApp installer QR display** - Implement tick-based polling (see section 4)
2. **Post-install WhatsApp flow** - User wanted QR display to happen DURING Channels step, not after install
3. **Test the WhatsApp login TUI** - `smolbot channels login whatsapp` hasn't been tested yet
4. **Other TUI worktrees** - `tui-ui-overhaul`, `tui-transcript-ux`, `ultraviolet-lazy-viewport` not merged
