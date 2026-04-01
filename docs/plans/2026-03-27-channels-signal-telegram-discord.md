# Channels: Signal Onboarding + Telegram & Discord — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Complete Signal onboarding in the installer and CLI (matching the WhatsApp QR flow), then add native Telegram and Discord channel support.

**Architecture:** All channels follow the existing adapter/seam pattern established by WhatsApp. Each channel implements `Channel`, `StatusReporter`, and `InteractiveLoginHandler` interfaces. The `channel.Manager` handles lifecycle and routing — no changes needed to the manager, gateway, or agent loop.

**Tech Stack:** Go 1.26, `github.com/go-telegram/bot` (Telegram), `github.com/bwmarrin/discordgo` (Discord), `signal-cli` (Signal, existing)

---

## Current State Summary

| Channel | Adapter | Tests | Config | Runtime | Installer Toggle | Installer Setup | CLI Login |
|---------|---------|-------|--------|---------|-----------------|-----------------|-----------|
| WhatsApp | Complete | Complete | Complete | Complete | Yes | Yes (QR flow) | Yes (TUI + JSON) |
| Signal | **Complete** | **Complete (10 tests)** | Complete | Complete | Yes | **No setup step** | **Partial** (generic, no TUI) |
| Telegram | Missing | Missing | Missing | Missing | Missing | Missing | Missing |
| Discord | Missing | Missing | Missing | Missing | Missing | Missing | Missing |

**Key finding:** Signal's Go adapter (`pkg/channel/signal/adapter.go`, 281 lines) is fully implemented with send, receive, login (via `signal-cli link`), and status reporting. The gap is UX — the installer has no `stepSignalSetup` equivalent to WhatsApp's `stepWhatsAppSetup`, and the CLI login doesn't have a dedicated TUI with QR rendering.

---

## Channel Interface Reference

All channels must implement (`pkg/channel/channel.go`):

```go
type Channel interface {
    Name() string
    Start(ctx context.Context, handler Handler) error
    Stop(ctx context.Context) error
    Send(ctx context.Context, msg OutboundMessage) error
}

type StatusReporter interface {
    Status(ctx context.Context) (Status, error)
}

type InteractiveLoginHandler interface {
    LoginWithUpdates(ctx context.Context, report func(Status) error) error
}
```

The WhatsApp adapter pattern to follow:
- `clientSeam` interface wraps the protocol SDK for testability
- `Adapter` struct with seam, status tracking, allowlist, dedup
- Factory function: `NewProductionAdapter(cfg) (*Adapter, error)`
- Runtime factory: `var newXxxChannel = func(cfg config.XxxChannelConfig) (channel.Channel, error)`

---

## File Map

| File | Change |
|------|--------|
| **Signal Onboarding** | |
| `cmd/installer/signal_link.go` | **New** — SignalLinker for installer (mirrors WhatsAppLinker) |
| `cmd/installer/main.go` | Modify — Add `stepSignalSetup` step after channel selection |
| `cmd/installer/types.go` | Modify — Add Signal setup model fields |
| `cmd/installer/views.go` | Modify — Add Signal setup view with QR rendering |
| `cmd/installer/tasks.go` | Modify — Add signal-cli prerequisite check |
| `cmd/smolbot/channels_signal_login.go` | **New** — Dedicated Signal TUI login (like WhatsApp's) |
| **Telegram** | |
| `pkg/channel/telegram/adapter.go` | **New** — Telegram adapter + seam |
| `pkg/channel/telegram/adapter_test.go` | **New** — Tests with mock seam |
| `pkg/config/config.go` | Modify — Add `TelegramChannelConfig` |
| `cmd/smolbot/runtime.go` | Modify — Add factory + registration |
| `cmd/installer/views.go` | Modify — Add Telegram toggle + token input |
| `cmd/installer/types.go` | Modify — Add Telegram model fields |
| `cmd/installer/tasks.go` | Modify — Write Telegram config |
| `cmd/smolbot/channels_telegram_login.go` | **New** — Token validation login command |
| **Discord** | |
| `pkg/channel/discord/adapter.go` | **New** — Discord adapter + seam |
| `pkg/channel/discord/adapter_test.go` | **New** — Tests with mock seam |
| `pkg/config/config.go` | Modify — Add `DiscordChannelConfig` |
| `cmd/smolbot/runtime.go` | Modify — Add factory + registration |
| `cmd/installer/views.go` | Modify — Add Discord toggle + token input |
| `cmd/installer/types.go` | Modify — Add Discord model fields |
| `cmd/installer/tasks.go` | Modify — Write Discord config |
| `cmd/smolbot/channels_discord_login.go` | **New** — Token + intent validation login |

---

## Part 1: Signal Onboarding Completion

### Task 1: Signal Installer Setup Step

**Files:**
- New: `cmd/installer/signal_link.go`
- Modify: `cmd/installer/main.go`, `types.go`, `views.go`, `tasks.go`

The WhatsApp installer has `WhatsAppLinker` + `stepWhatsAppSetup`. Signal needs the same pattern but using `signal-cli link` instead of whatsmeow QR.

**How Signal linking works:**
1. Run `signal-cli link -n smolbot` → outputs a `tsdevice://` provisioning URI
2. Encode that URI as a QR code
3. User scans QR with Signal app on phone
4. signal-cli completes device linking, outputs account info
5. Store account phone number in config

- [ ] **Step 1: Create SignalLinker**

```go
// cmd/installer/signal_link.go
package main

import (
    "bufio"
    "context"
    "fmt"
    "os/exec"
    "strings"
    "sync"
)

type SignalLinker struct {
    cliPath  string
    dataDir  string

    mu       sync.Mutex
    qrURI    string      // tsdevice:// URI for QR
    status   string
    account  string      // phone number after successful link
    done     bool
    linkErr  error
    started  bool
    cmd      *exec.Cmd
    cancel   context.CancelFunc
}

func NewSignalLinker(cliPath, dataDir string) *SignalLinker {
    return &SignalLinker{
        cliPath: cliPath,
        dataDir: dataDir,
        status:  "Initializing...",
    }
}

func (l *SignalLinker) Start() error {
    l.mu.Lock()
    if l.started {
        l.mu.Unlock()
        return fmt.Errorf("already started")
    }
    l.started = true
    l.mu.Unlock()

    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
    l.cancel = cancel

    go l.run(ctx)
    return nil
}

func (l *SignalLinker) run(ctx context.Context) {
    defer func() {
        l.mu.Lock()
        l.done = true
        l.mu.Unlock()
    }()

    args := []string{"--config", l.dataDir, "link", "-n", "smolbot"}
    l.cmd = exec.CommandContext(ctx, l.cliPath, args...)

    stdout, err := l.cmd.StdoutPipe()
    if err != nil {
        l.setError(fmt.Errorf("stdout pipe: %w", err))
        return
    }
    stderr, err := l.cmd.StderrPipe()
    if err != nil {
        l.setError(fmt.Errorf("stderr pipe: %w", err))
        return
    }

    if err := l.cmd.Start(); err != nil {
        l.setError(fmt.Errorf("start signal-cli: %w", err))
        return
    }

    scanner := bufio.NewScanner(stdout)
    for scanner.Scan() {
        line := strings.TrimSpace(scanner.Text())

        // signal-cli link outputs the provisioning URI first
        if strings.HasPrefix(line, "tsdevice://") {
            l.mu.Lock()
            l.qrURI = line
            l.status = "Scan QR with Signal"
            l.mu.Unlock()
            continue
        }

        // After linking, it outputs the account phone number
        if strings.HasPrefix(line, "+") {
            l.mu.Lock()
            l.account = line
            l.status = "Linked successfully"
            l.mu.Unlock()
        }
    }

    // Read stderr for errors
    errScanner := bufio.NewScanner(stderr)
    var errLines []string
    for errScanner.Scan() {
        errLines = append(errLines, errScanner.Text())
    }

    if err := l.cmd.Wait(); err != nil {
        l.mu.Lock()
        if l.account == "" {
            l.linkErr = fmt.Errorf("signal-cli link failed: %s", strings.Join(errLines, "; "))
        }
        l.mu.Unlock()
    }
}

func (l *SignalLinker) Poll() (qrURI string, status string, account string, done bool, err error) {
    l.mu.Lock()
    defer l.mu.Unlock()
    return l.qrURI, l.status, l.account, l.done, l.linkErr
}

func (l *SignalLinker) Cleanup() {
    if l.cancel != nil {
        l.cancel()
    }
}

func (l *SignalLinker) setError(err error) {
    l.mu.Lock()
    l.linkErr = err
    l.mu.Unlock()
}
```

- [ ] **Step 2: Add stepSignalSetup to installer flow**

In `main.go`, after `stepChannels`, add:

```go
case stepChannels:
    // ... existing channel toggle logic
    // When user presses Enter and Signal is enabled:
    if m.signalEnabled {
        m.step = stepSignalSetup
        return m, startSignalLinkCmd(m.signalCLIPath, m.signalDataDir)
    }
    // Otherwise skip to WhatsApp or next step
```

- [ ] **Step 3: Add Signal setup view**

In `views.go`, add `renderSignalSetup()` mirroring `renderWhatsAppSetup()`:

```go
func renderSignalSetup(m model) string {
    var content strings.Builder
    content.WriteString("Signal Device Linking\n\n")

    if m.signalQR != "" {
        // Render QR from tsdevice:// URI
        qr, err := renderQRForTerminal(m.signalQR)
        if err == nil {
            content.WriteString(qr)
            content.WriteString("\n")
        }
    }

    content.WriteString(m.signalStatus + "\n")

    if m.signalAccount != "" {
        content.WriteString(fmt.Sprintf("\nLinked to: %s\n", m.signalAccount))
    }

    return content.String()
}
```

- [ ] **Step 4: Add signal-cli prerequisite check**

In the `stepPrerequisites` view and the prerequisite check logic, verify signal-cli is installed (if Signal is selected):

```go
func checkSignalCLI(cliPath string) (bool, string) {
    cmd := exec.Command(cliPath, "--version")
    out, err := cmd.Output()
    if err != nil {
        return false, "signal-cli not found"
    }
    return true, strings.TrimSpace(string(out))
}
```

Show as optional prerequisite (like Ollama), not blocking.

- [ ] **Step 5: Write Signal config with discovered account**

In `tasks.go`, when writing config, use the account discovered during linking:

```go
if signalEnabled {
    signalConfig := channels["signal"].(map[string]interface{})
    signalConfig["enabled"] = true
    if m.signalAccount != "" {
        signalConfig["account"] = m.signalAccount
    }
    if m.signalCLIPath != "" {
        signalConfig["cliPath"] = m.signalCLIPath
    }
}
```

- [ ] **Step 6: Add model fields to types.go**

```go
// Signal setup state
signalLinker   *SignalLinker
signalQR       string
signalStatus   string
signalAccount  string
signalDataDir  string
```

---

### Task 2: Signal CLI Login TUI

**Files:**
- New: `cmd/smolbot/channels_signal_login.go`

WhatsApp has a dedicated `channels_whatsapp_login.go` with a Bubble Tea TUI that shows the QR code. Signal needs the same.

- [ ] **Step 1: Create signalLoginModel**

```go
// cmd/smolbot/channels_signal_login.go
package main

import (
    "context"
    "fmt"

    tea "charm.land/bubbletea/v2"
    "github.com/Nomadcxx/smolbot/pkg/channel/signal"
)

type signalLoginModel struct {
    adapter   *signal.Adapter
    qrURI     string
    qrASCII   string
    status    string
    done      bool
    err       error
    width     int
    height    int
}

func runSignalLogin(ctx context.Context, opts rootOptions) error {
    cfg, err := loadConfig(opts)
    if err != nil {
        return err
    }

    adapter := signal.NewAdapter(cfg.Channels.Signal, nil)

    m := signalLoginModel{adapter: adapter}
    p := tea.NewProgram(m, tea.WithAltScreen())
    _, err = p.Run()
    return err
}
```

The model follows the same pattern as `whatsappLoginModel`:
- Calls `adapter.LoginWithUpdates()` in an async command
- Status callback reports QR URI → render as ASCII QR
- Shows "Scan QR with Signal app" prompt
- 5-minute timeout
- On success, shows linked account number

- [ ] **Step 2: Add QR rendering for signal URI**

Signal's provisioning URI (`tsdevice://...`) can be encoded directly as a QR code using the same `skip2/go-qrcode` library already in the project:

```go
func renderSignalQR(uri string) (string, error) {
    qr := whatsapp.NewQRRenderer(0)  // reuse existing renderer
    return qr.RenderToASCII(uri)
}
```

Or factor out the QR renderer into a shared location (e.g., `pkg/channel/qr/renderer.go`) since both WhatsApp and Signal need it.

- [ ] **Step 3: Add JSON mode**

Like WhatsApp, support `--json` flag for automation:

```bash
smolbot channels login signal          # TUI with QR
smolbot channels login signal --json   # JSON event stream
```

- [ ] **Step 4: Wire into channels_login.go**

In the existing `newChannelsLoginCmd()`, add Signal-specific routing:

```go
case "signal":
    if jsonOutput {
        return runSignalLoginJSON(ctx, opts)
    }
    return runSignalLogin(ctx, opts)
```

- [ ] **Step 5: Factor out QR renderer**

Move `QRRenderer` from `pkg/channel/whatsapp/terminal.go` to a shared location since Signal also needs it:

```go
// pkg/channel/qr/renderer.go
package qr

type Renderer struct { size int }
func New(size int) *Renderer
func (r *Renderer) RenderToASCII(data string) (string, error)
```

Update WhatsApp imports. Both channels use the same renderer.

- [ ] **Step 6: Write tests**

- Test SignalLinker with a mock signal-cli script that outputs a fake URI then a phone number
- Test signalLoginModel receives QR updates and renders them
- Test JSON mode outputs correct event types

---

## Part 2: Telegram Channel

### Task 3: Telegram Adapter

**Files:**
- New: `pkg/channel/telegram/adapter.go`
- New: `pkg/channel/telegram/adapter_test.go`
- New dependency: `github.com/go-telegram/bot`

Telegram is the simpler of the two new channels. Uses long polling (no WebSocket), token-based auth (no QR/OAuth), and the `go-telegram/bot` library has zero external dependencies.

**Key constraints:**
- Message limit: 4096 UTF-8 chars (need chunking for long responses)
- Rate limit: ~1 msg/sec per chat, 30 msg/sec global
- Bot privacy mode: by default bots in groups only see messages mentioning them or starting with `/`
- Bots cannot see messages from other bots

- [ ] **Step 1: Define the seam interface**

```go
// pkg/channel/telegram/adapter.go
package telegram

import "context"

type clientSeam interface {
    Start(ctx context.Context, handler func(chatID int64, text string)) error
    Stop(ctx context.Context) error
    Send(ctx context.Context, chatID int64, text string) error
    GetMe(ctx context.Context) (botName string, err error)
}
```

- [ ] **Step 2: Implement the adapter**

```go
type Adapter struct {
    seam           clientSeam
    mu             sync.RWMutex
    status         channel.Status
    allowedChatIDs map[string]struct{}
    enforceAllow   bool
    recentMsgs     map[string]time.Time
    recentMu       sync.Mutex
}

func NewProductionAdapter(cfg config.TelegramChannelConfig) (*Adapter, error) {
    token := cfg.BotToken
    if token == "" && cfg.TokenFile != "" {
        data, err := os.ReadFile(cfg.TokenFile)
        if err != nil {
            return nil, fmt.Errorf("read telegram token file: %w", err)
        }
        token = strings.TrimSpace(string(data))
    }
    if token == "" {
        return nil, fmt.Errorf("telegram bot token required")
    }

    seam, err := newTelegramSeam(token)
    if err != nil {
        return nil, err
    }

    allowed := make(map[string]struct{})
    for _, id := range cfg.AllowedChatIDs {
        allowed[id] = struct{}{}
    }

    return &Adapter{
        seam:           seam,
        allowedChatIDs: allowed,
        enforceAllow:   len(cfg.AllowedChatIDs) > 0,
        recentMsgs:     make(map[string]time.Time),
        status:         channel.Status{State: "disconnected"},
    }, nil
}

func (a *Adapter) Name() string { return "telegram" }

func (a *Adapter) Start(ctx context.Context, handler channel.Handler) error {
    return a.seam.Start(ctx, func(chatID int64, text string) {
        chatIDStr := strconv.FormatInt(chatID, 10)

        // Allowlist check
        if a.enforceAllow {
            if _, ok := a.allowedChatIDs[chatIDStr]; !ok {
                return
            }
        }

        // Deduplication (5-minute window)
        dedupKey := fmt.Sprintf("%s:%s", chatIDStr, text)
        a.recentMu.Lock()
        if _, dup := a.recentMsgs[dedupKey]; dup {
            a.recentMu.Unlock()
            return
        }
        a.recentMsgs[dedupKey] = time.Now()
        a.recentMu.Unlock()

        handler(ctx, channel.InboundMessage{
            Channel: "telegram",
            ChatID:  chatIDStr,
            Content: text,
        })
    })
}

func (a *Adapter) Stop(ctx context.Context) error {
    return a.seam.Stop(ctx)
}

func (a *Adapter) Send(ctx context.Context, msg channel.OutboundMessage) error {
    chatID, err := strconv.ParseInt(msg.ChatID, 10, 64)
    if err != nil {
        return fmt.Errorf("invalid telegram chat ID %q: %w", msg.ChatID, err)
    }

    // Chunk at 4096 chars on paragraph boundaries
    chunks := chunkMessage(msg.Content, 4096)
    for _, chunk := range chunks {
        if err := a.seam.Send(ctx, chatID, chunk); err != nil {
            return err
        }
    }
    return nil
}
```

- [ ] **Step 3: Implement the production seam**

```go
type telegramSeam struct {
    bot   *bot.Bot
    token string
}

func newTelegramSeam(token string) (*telegramSeam, error) {
    return &telegramSeam{token: token}, nil
}

func (s *telegramSeam) Start(ctx context.Context, handler func(chatID int64, text string)) error {
    opts := []bot.Option{
        bot.WithDefaultHandler(func(ctx context.Context, b *bot.Bot, update *models.Update) {
            if update.Message == nil || update.Message.Text == "" {
                return
            }
            handler(update.Message.Chat.ID, update.Message.Text)
        }),
    }

    b, err := bot.New(s.token, opts...)
    if err != nil {
        return fmt.Errorf("create telegram bot: %w", err)
    }
    s.bot = b

    // bot.Start blocks — run in goroutine
    go b.Start(ctx)
    return nil
}

func (s *telegramSeam) Stop(ctx context.Context) error {
    // bot.Start returns when ctx is cancelled — no explicit stop needed
    // The parent context cancellation handles this
    return nil
}

func (s *telegramSeam) Send(ctx context.Context, chatID int64, text string) error {
    _, err := s.bot.SendMessage(ctx, &bot.SendMessageParams{
        ChatID: chatID,
        Text:   text,
    })
    return err
}

func (s *telegramSeam) GetMe(ctx context.Context) (string, error) {
    me, err := s.bot.GetMe(ctx)
    if err != nil {
        return "", err
    }
    return me.Username, nil
}
```

- [ ] **Step 4: Implement message chunking**

```go
func chunkMessage(content string, maxLen int) []string {
    if len(content) <= maxLen {
        return []string{content}
    }

    var chunks []string
    for len(content) > 0 {
        if len(content) <= maxLen {
            chunks = append(chunks, content)
            break
        }

        // Try to break at paragraph boundary
        cut := maxLen
        if idx := strings.LastIndex(content[:maxLen], "\n\n"); idx > maxLen/2 {
            cut = idx + 2
        } else if idx := strings.LastIndex(content[:maxLen], "\n"); idx > maxLen/2 {
            cut = idx + 1
        } else if idx := strings.LastIndex(content[:maxLen], " "); idx > maxLen/2 {
            cut = idx + 1
        }

        chunks = append(chunks, content[:cut])
        content = content[cut:]
    }
    return chunks
}
```

- [ ] **Step 5: Implement Status and Login**

```go
func (a *Adapter) Status(ctx context.Context) (channel.Status, error) {
    a.mu.RLock()
    defer a.mu.RUnlock()
    return a.status, nil
}

func (a *Adapter) LoginWithUpdates(ctx context.Context, report func(channel.Status) error) error {
    // Telegram login is just token validation
    report(channel.Status{State: "connecting", Detail: "Validating bot token..."})

    name, err := a.seam.GetMe(ctx)
    if err != nil {
        report(channel.Status{State: "auth-required", Detail: fmt.Sprintf("Invalid token: %v", err)})
        return err
    }

    a.mu.Lock()
    a.status = channel.Status{State: "connected", Detail: fmt.Sprintf("Bot: @%s", name)}
    a.mu.Unlock()

    return report(a.status)
}
```

- [ ] **Step 6: Write tests**

```go
type fakeTelegramSeam struct {
    started     bool
    handler     func(int64, string)
    sentMsgs    []sentMsg
    getMeName   string
    getMeErr    error
}

func TestAdapterImplementsInterfaces(t *testing.T) {
    var _ channel.Channel = (*Adapter)(nil)
    var _ channel.StatusReporter = (*Adapter)(nil)
    var _ channel.InteractiveLoginHandler = (*Adapter)(nil)
}

func TestAdapterSendChunksLongMessages(t *testing.T) { ... }
func TestAdapterStartFiltersAllowedChatIDs(t *testing.T) { ... }
func TestAdapterStartDeduplicatesMessages(t *testing.T) { ... }
func TestAdapterLoginValidatesToken(t *testing.T) { ... }
```

---

### Task 4: Telegram Config, Runtime & Installer

**Files:**
- Modify: `pkg/config/config.go`
- Modify: `cmd/smolbot/runtime.go`
- Modify: `cmd/installer/types.go`, `views.go`, `tasks.go`
- New: `cmd/smolbot/channels_telegram_login.go`

- [ ] **Step 1: Add config struct**

```go
// pkg/config/config.go
type TelegramChannelConfig struct {
    Enabled        bool     `json:"enabled"`
    BotToken       string   `json:"botToken,omitempty"`
    TokenFile      string   `json:"tokenFile,omitempty"`
    AllowedChatIDs []string `json:"allowedChatIDs,omitempty"`
}

type ChannelsConfig struct {
    SendProgress  bool                      `json:"sendProgress"`
    SendToolHints bool                      `json:"sendToolHints"`
    Signal        SignalChannelConfig       `json:"signal"`
    WhatsApp      WhatsAppChannelConfig     `json:"whatsapp"`
    Telegram      TelegramChannelConfig     `json:"telegram"`
}
```

- [ ] **Step 2: Add runtime factory and registration**

```go
// cmd/smolbot/runtime.go
import telegramchannel "github.com/Nomadcxx/smolbot/pkg/channel/telegram"

var newTelegramChannel = func(cfg config.TelegramChannelConfig) (channel.Channel, error) {
    return telegramchannel.NewProductionAdapter(cfg)
}

// In configuredChannels():
if includeDisabled || cfg.Channels.Telegram.Enabled {
    tg, err := newTelegramChannel(cfg.Channels.Telegram)
    if err != nil {
        return nil, fmt.Errorf("telegram channel: %w", err)
    }
    out = append(out, tg)
}
```

- [ ] **Step 3: Add to installer**

In `views.go` channels screen, add Telegram toggle:

```
  Signal Integration      [disabled]  Requires signal-cli
  WhatsApp Integration    [enabled]   QR code pairing
  Telegram Integration    [disabled]  Requires BotFather token
```

When Telegram is enabled, prompt for bot token:

```go
case stepTelegramSetup:
    // Text input for bot token
    // Validate with GetMe() call
    // Show bot username on success
```

- [ ] **Step 4: Add CLI login command**

```go
// cmd/smolbot/channels_telegram_login.go
func runTelegramLogin(ctx context.Context, opts rootOptions) error {
    cfg, err := loadConfig(opts)
    if err != nil {
        return err
    }

    adapter, err := telegramchannel.NewProductionAdapter(cfg.Channels.Telegram)
    if err != nil {
        return err
    }

    return adapter.LoginWithUpdates(ctx, func(status channel.Status) error {
        fmt.Printf("[%s] %s\n", status.State, status.Detail)
        return nil
    })
}
```

Wire into `channels_login.go`:
```go
case "telegram":
    return runTelegramLogin(ctx, opts)
```

- [ ] **Step 5: Add onboard prompts**

In `collectOnboardConfig()`, add Telegram section:

```go
fmt.Print("Enable Telegram channel? (y/N): ")
if telegramEnabled {
    fmt.Print("  Bot token (from @BotFather): ")
    fmt.Print("  Allowed chat IDs (comma-separated, empty for all): ")
}
```

---

## Part 3: Discord Channel

### Task 5: Discord Adapter

**Files:**
- New: `pkg/channel/discord/adapter.go`
- New: `pkg/channel/discord/adapter_test.go`
- New dependency: `github.com/bwmarrin/discordgo`

Discord is more complex than Telegram: WebSocket gateway, OAuth2 bot invite flow, Message Content Intent requirement, 2000 char message limit, channel/guild hierarchy.

**Key constraints:**
- Message limit: 2000 chars (more aggressive chunking needed)
- Rate limit: 50 req/sec global, per-route varies
- **Message Content Intent** MUST be enabled in Discord Developer Portal or `m.Content` is always empty
- `discordgo.Open()` is non-blocking (unlike Telegram's blocking `Start()`)
- `gorilla/websocket` already in go.mod (discordgo dependency)

- [ ] **Step 1: Define the seam interface**

```go
// pkg/channel/discord/adapter.go
package discord

import "context"

type clientSeam interface {
    Open(ctx context.Context, handler func(channelID, authorID, content string)) error
    Close(ctx context.Context) error
    Send(ctx context.Context, channelID, content string) error
    GetMe(ctx context.Context) (botID string, botName string, err error)
}
```

- [ ] **Step 2: Implement the adapter**

```go
type Adapter struct {
    seam              clientSeam
    mu                sync.RWMutex
    status            channel.Status
    botID             string
    allowedChannelIDs map[string]struct{}
    allowedGuildIDs   map[string]struct{}
    enforceChannels   bool
    enforceGuilds     bool
    recentMsgs        map[string]time.Time
    recentMu          sync.Mutex
}

func NewProductionAdapter(cfg config.DiscordChannelConfig) (*Adapter, error) {
    token := cfg.BotToken
    if token == "" && cfg.TokenFile != "" {
        data, err := os.ReadFile(cfg.TokenFile)
        if err != nil {
            return nil, fmt.Errorf("read discord token file: %w", err)
        }
        token = strings.TrimSpace(string(data))
    }
    if token == "" {
        return nil, fmt.Errorf("discord bot token required")
    }

    seam, err := newDiscordSeam(token)
    if err != nil {
        return nil, err
    }

    allowedCh := make(map[string]struct{})
    for _, id := range cfg.AllowedChannelIDs {
        allowedCh[id] = struct{}{}
    }
    allowedG := make(map[string]struct{})
    for _, id := range cfg.AllowedGuildIDs {
        allowedG[id] = struct{}{}
    }

    return &Adapter{
        seam:              seam,
        allowedChannelIDs: allowedCh,
        allowedGuildIDs:   allowedG,
        enforceChannels:   len(cfg.AllowedChannelIDs) > 0,
        enforceGuilds:     len(cfg.AllowedGuildIDs) > 0,
        recentMsgs:        make(map[string]time.Time),
        status:            channel.Status{State: "disconnected"},
    }, nil
}

func (a *Adapter) Name() string { return "discord" }

func (a *Adapter) Start(ctx context.Context, handler channel.Handler) error {
    return a.seam.Open(ctx, func(channelID, authorID, content string) {
        // Ignore own messages
        if authorID == a.botID {
            return
        }

        // Allowlist checks
        // Note: guild filtering requires the seam to provide guild ID
        // For simplicity, filter on channel ID
        if a.enforceChannels {
            if _, ok := a.allowedChannelIDs[channelID]; !ok {
                return
            }
        }

        // Deduplication
        dedupKey := fmt.Sprintf("%s:%s:%s", channelID, authorID, content)
        a.recentMu.Lock()
        if _, dup := a.recentMsgs[dedupKey]; dup {
            a.recentMu.Unlock()
            return
        }
        a.recentMsgs[dedupKey] = time.Now()
        a.recentMu.Unlock()

        handler(ctx, channel.InboundMessage{
            Channel: "discord",
            ChatID:  channelID,
            Content: content,
        })
    })
}

func (a *Adapter) Send(ctx context.Context, msg channel.OutboundMessage) error {
    // Chunk at 2000 chars
    chunks := chunkMessage(msg.Content, 2000)
    for _, chunk := range chunks {
        if err := a.seam.Send(ctx, msg.ChatID, chunk); err != nil {
            return err
        }
    }
    return nil
}

func (a *Adapter) Stop(ctx context.Context) error {
    return a.seam.Close(ctx)
}
```

- [ ] **Step 3: Implement the production seam**

```go
type discordSeam struct {
    session *discordgo.Session
    token   string
}

func newDiscordSeam(token string) (*discordSeam, error) {
    dg, err := discordgo.New("Bot " + token)
    if err != nil {
        return nil, fmt.Errorf("create discord session: %w", err)
    }
    return &discordSeam{session: dg, token: token}, nil
}

func (s *discordSeam) Open(ctx context.Context, handler func(channelID, authorID, content string)) error {
    s.session.AddHandler(func(_ *discordgo.Session, m *discordgo.MessageCreate) {
        if m.Author == nil || m.Content == "" {
            return
        }
        handler(m.ChannelID, m.Author.ID, m.Content)
    })

    // Request Message Content intent
    s.session.Identify.Intents = discordgo.IntentsGuildMessages |
        discordgo.IntentsDirectMessages |
        discordgo.IntentMessageContent

    return s.session.Open()
}

func (s *discordSeam) Close(ctx context.Context) error {
    return s.session.Close()
}

func (s *discordSeam) Send(ctx context.Context, channelID, content string) error {
    _, err := s.session.ChannelMessageSend(channelID, content)
    return err
}

func (s *discordSeam) GetMe(ctx context.Context) (string, string, error) {
    user, err := s.session.User("@me")
    if err != nil {
        return "", "", err
    }
    return user.ID, user.Username, nil
}
```

- [ ] **Step 4: Implement Login with content intent detection**

```go
func (a *Adapter) LoginWithUpdates(ctx context.Context, report func(channel.Status) error) error {
    report(channel.Status{State: "connecting", Detail: "Validating bot token..."})

    botID, botName, err := a.seam.GetMe(ctx)
    if err != nil {
        report(channel.Status{State: "auth-required", Detail: fmt.Sprintf("Invalid token: %v", err)})
        return err
    }

    a.mu.Lock()
    a.botID = botID
    a.status = channel.Status{
        State:  "connected",
        Detail: fmt.Sprintf("Bot: %s\nIMPORTANT: Enable 'Message Content Intent' in Discord Developer Portal > Bot settings", botName),
    }
    a.mu.Unlock()

    return report(a.status)
}
```

- [ ] **Step 5: Write tests**

Same pattern as Telegram — fake seam, test interface compliance, test allowlists, test chunking at 2000 chars, test dedup, test self-message filtering.

---

### Task 6: Discord Config, Runtime & Installer

**Files:**
- Modify: `pkg/config/config.go`
- Modify: `cmd/smolbot/runtime.go`
- Modify: `cmd/installer/types.go`, `views.go`, `tasks.go`
- New: `cmd/smolbot/channels_discord_login.go`

- [ ] **Step 1: Add config struct**

```go
type DiscordChannelConfig struct {
    Enabled           bool     `json:"enabled"`
    BotToken          string   `json:"botToken,omitempty"`
    TokenFile         string   `json:"tokenFile,omitempty"`
    AllowedChannelIDs []string `json:"allowedChannelIDs,omitempty"`
    AllowedGuildIDs   []string `json:"allowedGuildIDs,omitempty"`
}

type ChannelsConfig struct {
    // ... existing
    Discord  DiscordChannelConfig  `json:"discord"`
}
```

- [ ] **Step 2: Add runtime factory and registration**

Same pattern as Telegram:

```go
import discordchannel "github.com/Nomadcxx/smolbot/pkg/channel/discord"

var newDiscordChannel = func(cfg config.DiscordChannelConfig) (channel.Channel, error) {
    return discordchannel.NewProductionAdapter(cfg)
}
```

- [ ] **Step 3: Add to installer**

Channel toggle screen:

```
  Signal Integration      [disabled]  Requires signal-cli
  WhatsApp Integration    [enabled]   QR code pairing
  Telegram Integration    [disabled]  Requires BotFather token
  Discord Integration     [disabled]  Requires bot token + invite
```

When Discord is enabled, show setup guide:

```
Discord Setup:
1. Go to discord.com/developers/applications
2. Create New Application → Bot tab → Copy Token
3. Enable "Message Content Intent" under Privileged Intents
4. OAuth2 → URL Generator → Select "bot" scope
5. Select permissions: Send Messages, Read Message History
6. Open the generated URL to invite bot to your server

Bot Token: ____________________
```

- [ ] **Step 4: Add CLI login command**

```go
// cmd/smolbot/channels_discord_login.go
func runDiscordLogin(ctx context.Context, opts rootOptions) error {
    // Same pattern: create adapter, call LoginWithUpdates, print statuses
}
```

- [ ] **Step 5: Add onboard prompts**

```go
fmt.Print("Enable Discord channel? (y/N): ")
if discordEnabled {
    fmt.Print("  Bot token: ")
    fmt.Print("  Allowed channel IDs (comma-separated, empty for all): ")
    fmt.Print("  Allowed guild IDs (comma-separated, empty for all): ")
}
```

---

### Task 7: Shared Message Chunking Utility

**Files:**
- New: `pkg/channel/chunk.go`

Both Telegram (4096) and Discord (2000) need message chunking. WhatsApp could benefit too. Factor out into a shared utility.

- [ ] **Step 1: Create shared chunking function**

```go
// pkg/channel/chunk.go
package channel

import "strings"

func ChunkMessage(content string, maxLen int) []string {
    if len(content) <= maxLen {
        return []string{content}
    }

    var chunks []string
    for len(content) > 0 {
        if len(content) <= maxLen {
            chunks = append(chunks, content)
            break
        }

        cut := maxLen
        // Try paragraph boundary first
        if idx := strings.LastIndex(content[:maxLen], "\n\n"); idx > maxLen/2 {
            cut = idx + 2
        } else if idx := strings.LastIndex(content[:maxLen], "\n"); idx > maxLen/2 {
            cut = idx + 1
        } else if idx := strings.LastIndex(content[:maxLen], " "); idx > maxLen/2 {
            cut = idx + 1
        }

        chunks = append(chunks, strings.TrimRight(content[:cut], " \n"))
        content = strings.TrimLeft(content[cut:], " \n")
    }
    return chunks
}
```

- [ ] **Step 2: Write tests**

```go
func TestChunkMessageShortPassthrough(t *testing.T) { ... }
func TestChunkMessageBreaksAtParagraph(t *testing.T) { ... }
func TestChunkMessageBreaksAtNewline(t *testing.T) { ... }
func TestChunkMessageBreaksAtSpace(t *testing.T) { ... }
func TestChunkMessageHardBreak(t *testing.T) { ... }  // no good break point
```

---

## Priority Order

| Priority | Task | Rationale |
|----------|------|-----------|
| P0 | Task 1: Signal installer setup | Completes existing partial feature, matches WhatsApp UX |
| P0 | Task 2: Signal CLI login TUI | Users need to re-link devices from CLI |
| P1 | Task 3: Telegram adapter | Simplest new channel, highest value (huge user base) |
| P1 | Task 4: Telegram config/runtime/installer | Wires Telegram end-to-end |
| P1 | Task 7: Shared chunking utility | Both new channels need it |
| P2 | Task 5: Discord adapter | More complex, smaller niche for personal assistant |
| P2 | Task 6: Discord config/runtime/installer | Wires Discord end-to-end |

---

## Dependencies

```
go get github.com/go-telegram/bot@latest       # Telegram (zero external deps)
go get github.com/bwmarrin/discordgo@latest     # Discord (gorilla/websocket already in go.mod)
```

---

## Testing Strategy

| Channel | Unit Tests | Integration Tests | Manual Verification |
|---------|-----------|-------------------|---------------------|
| Signal | Installer linker with mock signal-cli | CLI login with real signal-cli | Full link + send/receive |
| Telegram | Adapter with fake seam | CLI login with real token | Send message in Telegram, verify response |
| Discord | Adapter with fake seam | CLI login with real token | Send in Discord channel, verify response |
| Shared | Chunking at various limits | N/A | Long message sent via each channel |

---

## Risks

1. **signal-cli output format:** The `signal-cli link` command output format (provisioning URI on stdout) may vary between versions. Should pin to a known-working version in docs.

2. **signal-cli availability:** signal-cli requires Java runtime. The installer should check for both `signal-cli` AND `java` as prerequisites when Signal is selected.

3. **Telegram bot privacy mode:** By default, bots in groups only receive messages that start with `/` or mention the bot. Users need to disable privacy mode via BotFather's `/setprivacy` command. The onboarding guide should mention this prominently.

4. **Discord Message Content Intent:** Without enabling this in the Developer Portal, `m.Content` is always empty. The login validation should attempt to detect this (hard to detect programmatically — may need to document clearly).

5. **Token security:** Bot tokens in config.json are sensitive. The `tokenFile` alternative allows storing tokens in a separate file with restricted permissions. The installer should warn about token security.

6. **Rate limiting:** Long LLM responses may hit per-chat rate limits when chunked into multiple messages. Consider adding a small delay between chunks (100-200ms).

7. **Message format:** Telegram supports Markdown and HTML formatting. Discord supports a Markdown subset. Should we send plain text or formatted? Start with plain text, add formatting support later.

## Out of Scope (Future Work)

1. **Slack channel** — Would follow the same pattern but uses Slack Bot API with OAuth2 app installation flow. More complex onboarding than Telegram/Discord.
2. **Matrix channel** — Open protocol, good fit for privacy-focused users. Library: `maunium.net/go/mautrix`.
3. **IRC channel** — Simple protocol but declining user base.
4. **Media handling** — Currently all channels are text-only. Image/file support would require extending `InboundMessage`/`OutboundMessage` with attachment fields.
5. **Streaming responses** — Telegram and Discord both support editing messages after sending, which could enable real-time streaming of LLM output (like OpenClaw does).
6. **Multi-account** — Supporting multiple Telegram bots or Discord bots simultaneously.
