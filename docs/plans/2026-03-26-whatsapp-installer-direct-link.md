# WhatsApp Direct Linking in Installer - Implementation Plan

> **For Claude:** Use executing-plans to implement this plan.

**Goal:** Allow installer to link WhatsApp directly using Go (whatsmeow library) without requiring smolbot binary to exist. Creates handshake file at `~/.smolbot/whatsapp.db`.

**Architecture:** The installer already has access to `github.com/Nomadcxx/smolbot/pkg/channel/whatsapp` via QR renderer import. We will use the same package to perform WhatsApp linking directly in the installer process. No subprocess needed.

**Tech Stack:** Go, whatsmeow, bubblesTea (already in installer), lipgloss (already in installer)

---

## Problem Statement

Current flow has a fatal flaw:
1. Channels step → enable WhatsApp → Enter → WhatsApp Setup step
2. User presses Enter → `startWhatsAppLogin()` tries to run `smolbot channels login whatsapp --json`
3. But `~/.local/bin/smolbot` doesn't exist yet - we haven't built/installed it!

**Correct flow:** Installer links WhatsApp directly using Go/Whatsmeow - no smolbot binary required.

---

## Task 1: Create WhatsApp Linker Package

**Files:**
- Create: `cmd/installer/whatsapp_link.go`

**Step 1: Create the file with direct WhatsApp linking logic**

```go
package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"go.mau.fi/whatsmeow"
	waLog "go.mau.fi/whatsmeow/util/log"
	"go.mau.fi/whatsmeow/store/sqlstore"
)

const linkTimeout = 5 * time.Minute

type WhatsAppLinker struct {
	storePath string
	client    *whatsmeow.Client
}

func NewWhatsAppLinker(storePath string) (*WhatsAppLinker, error) {
	if err := os.MkdirAll(filepath.Dir(storePath), 0755); err != nil {
		return nil, fmt.Errorf("create store dir: %w", err)
	}

	container, err := sqlstore.New(context.Background(), "sqlite3", sqliteStoreDSN(storePath), waLog.Noop)
	if err != nil {
		return nil, fmt.Errorf("create store: %w", err)
	}
	
	device, err := container.GetFirstDevice(context.Background())
	if err != nil {
		return nil, fmt.Errorf("get device: %w", err)
	}

	client := whatsmeow.NewClient(device, waLog.Noop)
	
	return &WhatsAppLinker{
		storePath: storePath,
		client:    client,
	}, nil
}

func (l *WhatsAppLinker) IsLinked() bool {
	return l.client.Store.ID != nil
}

func (l *WhatsAppLinker) StartLinking(onQR func(string), onStatus func(string)) error {
	ctx, cancel := context.WithTimeout(context.Background(), linkTimeout)
	defer cancel()

	// Already linked?
	if l.client.Store.ID != nil {
		onStatus("Already linked")
		return nil
	}

	qrChan, err := l.client.GetQRChannel(ctx)
	if err != nil {
		return fmt.Errorf("get qr channel: %w", err)
	}

	if err := l.client.Connect(); err != nil {
		return fmt.Errorf("connect: %w", err)
	}
	defer l.client.Disconnect()

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("timeout")
		case item, ok := <-qrChan:
			if !ok {
				if l.client.IsLoggedIn() {
					onStatus("Linked successfully!")
					return nil
				}
				return fmt.Errorf("qr channel closed")
			}
			
			switch item.Event {
			case whatsmeow.QRChannelEventCode:
				onQR(item.Code)
				onStatus("Scan QR with WhatsApp")
			case whatsmeow.QRChannelSuccess.Event:
				onStatus("Linked successfully!")
				return nil
			case whatsmeow.QRChannelTimeout.Event:
				return fmt.Errorf("QR timed out")
			case whatsmeow.QRChannelEventError:
				if item.Error != nil {
					return item.Error
				}
				return fmt.Errorf("link error")
			}
		}
	}
}

func sqliteStoreDSN(path string) string {
	return "file:" + path + "?_foreign_keys=on"
}
```

**Step 2: Add whatsmeow import to go.mod**

Check if installer already has whatsmeow in go.mod...

Actually, we need to ADD the import to the installer's dependencies. Since the installer is a separate Go module, it needs its own dependencies.

**Step 3: Add dependency to installer**

Run: `cd /home/nomadx/Documents/smolbot/cmd/installer && go get go.mau.fi/whatsmeow`

---

## Task 2: Update Installer Model and Handlers

**Files:**
- Modify: `cmd/installer/types.go`
- Modify: `cmd/installer/main.go`
- Modify: `cmd/installer/views.go`

**Step 1: Add WhatsAppLinker to model in types.go**

```go
// WhatsApp setup state
whatsappQRCode  string
whatsappStatus  string
whatsappDone    bool
whatsappError   string
whatsappLinker  *WhatsAppLinker  // Add this
```

**Step 2: Initialize WhatsAppLinker when entering WhatsApp setup step**

In `handleWhatsAppSetupKeys`, when Enter is pressed and we're not done yet:

```go
func (m model) handleWhatsAppSetupKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "enter":
		if !m.whatsappDone && m.whatsappQRCode == "" {
			// Initialize linker
			storePath := filepath.Join(os.Getenv("HOME"), ".smolbot", "whatsapp.db")
			linker, err := NewWhatsAppLinker(storePath)
			if err != nil {
				m.whatsappDone = true
				m.whatsappError = err.Error()
				return m, nil
			}
			m.whatsappLinker = linker
			
			// Check if already linked
			if linker.IsLinked() {
				m.whatsappDone = true
				m.whatsappStatus = "Already linked!"
				m.whatsappQRCode = ""
				return m, nil
			}
			
			// Start linking
			return m, m.startWhatsAppLink()
		}
		m.step = stepService
		return m, nil
	case "esc":
		m.whatsappDone = false
		m.whatsappQRCode = ""
		m.whatsappStatus = ""
		m.whatsappError = ""
		m.whatsappLinker = nil
		m.step = stepService
		return m, nil
	}
	return m, nil
}
```

**Step 3: Replace startWhatsAppLogin with startWhatsAppLink**

Replace the subprocess-based `startWhatsAppLogin` with direct linking:

```go
func (m model) startWhatsAppLink() tea.Cmd {
	return func() tea.Msg {
		onQR := func(code string) {
			// This runs in goroutine, need to send to tea
		}
		onStatus := func(status string) {
			// This runs in goroutine, need to send to tea
		}
		
		err := m.whatsappLinker.StartLinking(onQR, onStatus)
		if err != nil {
			return whatsappLoginResult{success: false, message: err.Error()}
		}
		return whatsappLoginResult{success: true}
	}
}
```

**Note:** Since `StartLinking` blocks, we need to run it in a goroutine and use channels to send messages back to tea.

---

## Task 3: Debug Why QR Code Isn't Showing

**Files:**
- Modify: `cmd/installer/main.go`
- Modify: `cmd/installer/views.go`

**Step 1: Add debug logging to see what's happening**

Add fmt.Printf to `startWhatsAppLogin` to see if it's even being called:

```go
func (m model) startWhatsAppLogin() tea.Cmd {
	return func() tea.Msg {
		fmt.Printf("DEBUG: startWhatsAppLogin called\n")
		smolbotPath, err := findSmolbotBinary()
		fmt.Printf("DEBUG: findSmolbotBinary returned path=%s err=%v\n", smolbotPath, err)
```

**Step 2: Test manually**

Build installer: `go build -o installer .`
Run and check output.

---

## Task 4: Simplify Flow - Remove stepWhatsAppSetup

**Files:**
- Modify: `cmd/installer/types.go`
- Modify: `cmd/installer/main.go`
- Modify: `cmd/installer/views.go`
- Modify: `cmd/installer/tasks.go`

**Step 1: Revert to simple flow - WhatsApp handled post-install**

Remove `stepWhatsAppSetup` entirely. Flow becomes:
```
Channels → Service → Installing → Complete
              ↑            ↑
         If WhatsApp   WhatsApp link
         enabled,      happens in
         show info     Complete screen
```

**Step 2: At Complete screen, if WhatsApp was enabled, offer to link**

Add to `renderComplete()`:
```go
if whatsappEnabled && !isWhatsAppLinked() {
    b.WriteString("\n  Would you like to link WhatsApp now?")
    b.WriteString("\n  Run: smolbot channels login whatsapp")
}
```

Or better - show QR if they press a key.

**Actually, per user request: The TUI should allow WhatsApp linking DURING the Channels step, creating the handshake file directly.**

---

## Task 5: Direct WhatsApp Linking in Channels Step

**Files:**
- Modify: `cmd/installer/views.go` - modify renderChannels to show QR if linking
- Modify: `cmd/installer/main.go` - handle WhatsApp QR events in Channels step

**Step 1: During Channels step, if WhatsApp enabled and Enter pressed, show QR inline**

The Channels view already has the structure. We just need to intercept Enter on WhatsApp option and trigger linking instead of proceeding to Service.

```go
case "enter":
    if m.channelIndex == 1 && m.whatsappEnabled {
        // WhatsApp focused and enabled - start linking
        if m.whatsappQRCode == "" {
            return m, m.startWhatsAppLink()
        }
    }
    m.step = stepService
    return m, nil
```

---

## Verification

1. Build installer: `go build -o installer .`
2. Run installer: `./installer`
3. Go to Channels step
4. Enable WhatsApp (Space)
5. Press Enter - should see QR code appear
6. Scan with WhatsApp
7. Should show "Linked successfully!"

---

## Files Summary

| File | Action |
|------|--------|
| `cmd/installer/whatsapp_link.go` | Create - direct WhatsApp linking |
| `cmd/installer/types.go` | Modify - add linker field |
| `cmd/installer/main.go` | Modify - replace subprocess with direct linking, remove stepWhatsAppSetup |
| `cmd/installer/views.go` | Modify - inline QR display in Channels step |
| `cmd/installer/tasks.go` | Maybe modify - if post-install linking needed |
