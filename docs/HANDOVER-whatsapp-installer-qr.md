# WhatsApp Installer QR Flow - Handover Document

## Date: 2026-03-26

## Summary

The session attempted to add WhatsApp QR code display to the smolbot installer TUI. The WhatsApp linking and QR rendering work correctly in standalone tests, but the installer TUI integration is broken.

---

## What Works

### 1. WhatsApp Linking (`cmd/installer/whatsapp_link.go`)

```go
type WhatsAppLinker struct {
    storePath string
    client   *whatsmeow.Client
}

func NewWhatsAppLinker(storePath string) (*WhatsAppLinker, error)
func (l *WhatsAppLinker) IsLinked() bool
func (l *WhatsAppLinker) StartLinking(onQR QRCallback, onStatus StatusCallback) error
```

This correctly connects to WhatsApp and receives QR codes.

### 2. QR Rendering (`pkg/channel/whatsapp/terminal.go`)

```go
type QRRenderer struct{ size int }
func NewQRRenderer(size int) *QRRenderer
func (r *QRRenderer) RenderToASCII(data string) (string, error)
```

This correctly renders QR codes as ASCII art.

### 3. Standalone Test (`cmd/testqr/main.go`)

Run `/tmp/testqr` - receives QR from WhatsApp and saves as PNG to `/tmp/whatsapp_qr.png`.

---

## What Doesn't Work

### Installer TUI QR Display

The QR code never appears in the installer TUI. User sees only a spinner.

---

## Root Cause

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

---

## Suggested Fix: Tick-Based Polling

Replace the goroutine approach with synchronous tick-based polling:

### 1. Modify WhatsAppLinker

Add a `Start()` method that initiates and returns immediately, and a `Poll()` method that returns current state:

```go
type WhatsAppLinker struct {
    storePath string
    client    *whatsmeow.Client
    qrCode    string
    status    string
    done      bool
    err       error
}

func (l *WhatsAppLinker) Start(ctx context.Context) // non-blocking
func (l *WhatsAppLinker) Poll() (qr string, status string, done bool, err error)
```

### 2. Modify handleWhatsAppSetupKeys

Return a tick command that polls the linker:

```go
case "enter":
    if m.whatsappQRCode == "" {
        m.linker = NewWhatsAppLinker(storePath)
        m.linker.Start(context.Background())
        return m, pollWhatsAppTick()
    }
```

### 3. Add pollWhatsAppTick

```go
func pollWhatsAppTick() tea.Cmd {
    return func() tea.Msg {
        time.Sleep(500 * time.Millisecond)
        if m.linker != nil {
            qr, status, done, err := m.linker.Poll()
            return whatsappPollMsg{qr: qr, status: status, done: done, err: err}
        }
        return nil
    }
}
```

### 4. Handle whatsappPollMsg in Update

```go
case whatsappPollMsg:
    m.whatsappQRCode = msg.qr
    m.whatsappStatus = msg.status
    if msg.done {
        return m, nil // stop ticking
    }
    return m, pollWhatsAppTick() // continue ticking
```

---

## Files to Modify

| File | Change |
|------|--------|
| `cmd/installer/whatsapp_link.go` | Add `Start()` and `Poll()` methods to WhatsAppLinker |
| `cmd/installer/main.go` | Rewrite `handleWhatsAppSetupKeys` to use tick-based polling |
| `cmd/installer/types.go` | Add `linker *WhatsAppLinker` and `whatsappPollMsg` type |

---

## Key Insight

The tea program runs in a single-threaded event loop. Calling `program.Send()` from a goroutine does NOT wake up the event loop - messages sent to a channel the loop isn't reading from are lost.

The correct pattern is: commands that need to produce values over time should return a `tea.Cmd` that calls `tea.Sequence()` or uses `tea.EventToFunc()` to schedule the next tick.

---

## Commands

```bash
# Test QR generation works
cd /home/nomadx/Documents/smolbot && go run ./cmd/testqr/

# Build installer
cd /home/nomadx/Documents/smolbot && go build -o /tmp/smolbot-installer ./cmd/installer
```
