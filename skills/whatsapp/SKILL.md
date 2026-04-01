---
name: whatsapp
description: "Operating procedures for the WhatsApp channel in smolbot. Use when sending messages, receiving inbound, handling errors, managing sessions, or troubleshooting WhatsApp connectivity."
always: false
---

## WhatsApp Channel

smolbot connects to WhatsApp via the `whatsmeow` library (go.mau.fi/whatsmeow). All operations pass through `pkg/channel/whatsapp/adapter.go`.

### Architecture Overview

```
smolbot → message tool → MessageRouter.Route() → WhatsApp Adapter.Send()
                                                    ↓
                                            whatsmeowSeam.Send()
                                                    ↓
                                            whatsmeow.Client.SendMessage()
                                                    ↓
                                            WhatsApp WebSocket (port 443)
```

### ⚡ Quick Start & Setup

**1. Link your device (run once):**
```
smolbot channels login whatsapp
```
Scan the QR code with WhatsApp → Settings → Linked Devices → Link a Device.
Session persists across restarts as long as `storePath` db file is intact.
If the device becomes unlinked, run `smolbot channels login whatsapp` again.

**2. Find your chat ID:**
Send yourself a message from the phone. smolbot logs the inbound `chat_id` — use that exact string.

**3. Minimal working config:**
```json
{
  "whatsapp": {
    "enabled": true,
    "deviceName": "smolbot",
    "storePath": "/home/nomadx/.smolbot/whatsapp.db",
    "allowedChatIDs": ["61435311397@s.whatsapp.net"]
  }
}
```

**Chat ID format (DM):** International phone number (no leading `0`, no spaces, no `+`) + `@s.whatsapp.net`
- 🇦🇺 Australia `04XXXXXXXX` → `614XXXXXXXX@s.whatsapp.net`
- 🇺🇸 USA `+1 555 123 4567` → `15551234567@s.whatsapp.net`
- 🇬🇧 UK `07XXX XXXXXX` → `447XXXXXXXXX@s.whatsapp.net`

**Group chat ID format:** `123456789-987654321@g.us` (visible in smolbot logs when a group message arrives)

---

### 🚨 Troubleshooting

| Symptom | Cause | Fix |
|---------|-------|-----|
| Send fails with `"unknown server"` | Wrong chat ID format (e.g. `0412...` instead of `61412...`, or wrong `@server`) | Reformat: strip leading `0`, prepend country code, append `@s.whatsapp.net` |
| Messages sent OK but bot never replies | `allowedChatIDs` is empty or your JID isn't in it | Add your chat ID to `allowedChatIDs` in config |
| Bot replies to nothing (no inbound) | Empty `allowedChatIDs` list — **all inbound is silently dropped** | Add at least one chat ID to the allowlist |
| `"whatsapp login required"` or `"not logged in"` | Session expired or device was unlinked | Run `smolbot channels login whatsapp` |
| QR code expires before scanning | 5-minute timeout elapsed | Run login command again |
| `"stream: unknown"` | Protocol version mismatch with WhatsApp servers | Update smolbot binary (whatsmeow library needs update) |
| No QR appears in TUI | Installer TUI goroutine issue (known bug) | Use `smolbot channels login whatsapp` directly instead |

**⚠️ Critical: `allowedChatIDs` behaviour**
- **Empty list** → all inbound messages are **dropped** (the bot hears nothing)
- **Populated list** → only exact JID matches are processed
- There is no "allow all" mode in production; you must always list at least one chat ID

---

### Sending Messages

Use the `message` tool with `channel: "whatsapp"`:

```
message(channel="whatsapp", chat_id="15551234567@s.whatsapp.net", content="Hello")
```

**JID Format**: WhatsApp identifiers are JIDs (Jabber IDs) in the format `user@server`.

| Server | Purpose |
|--------|---------|
| `s.whatsapp.net` | Regular users and devices |
| `g.us` | Groups |
| `c.us` | Legacy users |
| `lid` | Hidden accounts |

**For DMs**: Use `15551234567@s.whatsapp.net` (phone number + server).

**For Groups**: Use the group JID, typically `123456789-987654321@g.us` (numeric ID + `-` + random suffix + `g.us`).

The adapter uses `waTypes.ParseJID()` to parse the chat_id string.

### Message Content Types

**Inbound text extraction** (from `extractMessageText()`):

The adapter extracts text from these message types in priority order:

1. `GetConversation()` — Plain direct text message
2. `GetExtendedTextMessage().GetText()` — Text with link preview
3. `GetImageMessage().GetCaption()` — Image caption
4. `GetVideoMessage().GetCaption()` — Video caption
5. `GetDocumentMessage().GetCaption()` — Document caption

**Important**: The adapter does NOT download or parse media content. Images, videos, audio, stickers, and documents are silently ignored unless they have captions.

**Outbound**: Only plain text via `Conversation` message type. No media sending, no formatting, no buttons, no polls.

### Character Limit

WhatsApp's limit is **4096 characters** per message. The adapter does NOT auto-chunk — the agent must split long messages manually.

**Splitting strategy**: Find the last `\n\n`, `\n`, or space before 4000 chars. Prefix continuation with "(continued)".

### Formatting

**Do not use standard Markdown**. WhatsApp uses its own markup:

| Style | Syntax |
|-------|--------|
| Bold | `*text*` |
| Italic | `_text_` |
| Strikethrough | `~text~` |
| Monospace (inline) | `` `code` `` |
| Monospace (block) | ` ```code``` ` |

Standard Markdown (`**bold**`, `__italic__`) renders as literal characters.

### Session / Store

Sessions are stored in SQLite at `storePath` (default: `~/.smolbot/whatsapp.db`).

**Schema tables** (from whatsmeow sqlstore):
- `whatsmeow_sessions` — Signal Protocol sessions
- `whatsmeow_identity_keys` — Device identity keys
- `whatsmeow_pre_keys` — Pre-key pairs
- `whatsmeow_sender_keys` — Group sender keys
- `whatsmeow_contacts` — Contact names and push names
- `whatsmeow_message_secrets` — Message decryption keys

**Device identity**: After QR login, `client.Store.ID` is populated (e.g., `12345678910@s.whatsapp.net`). This identifies the linked device.

### QR Login Flow

```
smolbot channels login whatsapp
```

**State machine** (from `loginUpdateFromQR()`):

| State | Detail | Meaning |
|-------|--------|---------|
| `qr` | QR code string | Display as QR; user scans with phone |
| `device-link` | "Linking device..." | Pairing in progress |
| `connected` | — | Login successful |
| `auth-required` | "timed out" | QR expired before scan |
| `error` | error message | Login failed |

**QR login sequence** (from `whatsmeowSeam.Login()`):
1. Call `client.GetQRChannel(ctx)` — MUST be before Connect()
2. Call `client.Connect()`
3. QR codes arrive as `{Event: "code", Code: "..."}` in the channel
4. User scans with WhatsApp > Settings > Linked Devices > Link New Device
5. On success: `PairSuccess` → state `connected`
6. On timeout: `QRChannelTimeout` → state `auth-required`
7. On error: `QRChannelEventError` → state `error`

**JSON output** (for installer):
```
smolbot channels login whatsapp --json
```
Outputs newline-delimited JSON events to stdout.

### Connection Management

**Auto-reconnect** is enabled:
```go
client.EnableAutoReconnect = true
```

**Reconnection behavior**:
- Exponential backoff: 0s, 2s, 4s, 6s... (doubles on each failure)
- `AutoReconnectHook` can return false to stop retrying
- After `LoggedOut` or `StreamReplaced`, no auto-reconnect

**Connection states**:
- `IsConnected()` — WebSocket open (may not be authenticated)
- `IsLoggedIn()` — Fully authenticated and ready

### Event Handling

The adapter registers an event handler for:

**`Message`** — Inbound messages (handled)
**`Disconnected`** — WebSocket closed → triggers `onDisconnect` callback
**`Connected`** — Authentication complete → triggers `onReconnect` callback

**Ignored** (not currently handled):
- `Receipt` — delivery/read receipts
- `ChatPresence` — typing, recording indicators
- `Presence` — online/offline status
- `GroupInfo` — group metadata changes
- `Picture` — profile picture changes
- `PollCreationMessage` / `PollUpdateMessage`
- `ReactionMessage`
- `EditedMessage`
- `EphemeralMessage`

### Error Handling

**Error categorization** (from `categorizeError()`):

| Category | Keywords | Retryable |
|----------|----------|-----------|
| `auth` | "logged out", "auth" | No |
| `connection` | "connection", "disconnected" | Yes |
| `timeout` | "timeout" | Yes |
| `rate` | "rate limit" | Yes |
| `unknown` | everything else | Yes |

**Common error strings from whatsmeow**:
- `"whatsapp login required"` — `Store.ID` is nil, not linked
- `"not logged in"` — session expired
- `"connection closed"` — WebSocket dropped
- `"stream: unknown"` — protocol version mismatch

**Error wrapping**:
- Send errors: `fmt.Errorf("whatsapp send: %w", err)`
- Connect errors: `fmt.Errorf("whatsapp connect [%s]: %w", category, err)`

### Deduplication

The adapter maintains a `recentMessages` map with a **5-minute sliding window** (key: message ID, value: receive time).

Duplicate messages (same ID within 5 minutes) are silently dropped.

**Own messages** are filtered by `shouldIgnoreInboundMessage()`:
- Messages from bots are ignored
- Messages from own device (same `user` and `device` in JID) are ignored

### Allowlist

If `AllowedChatIDs` is configured, only messages from those JIDs are processed:

```go
// Config
{
  "enabled": true,
  "storePath": "~/.smolbot/whatsapp.db",
  "allowedChatIDs": ["15551234567@s.whatsapp.net", "123456789-987654321@g.us"]
}
```

If allowlist is empty and `enforceAllowlist=true`, all inbound is dropped (logged at INFO level).

### Status Reporting

Adapter status states:

| State | Meaning |
|-------|---------|
| `connecting` | `Start()` called, connecting |
| `connected` | WebSocket connected and authenticated |
| `disconnected` | Not connected |
| `error` | Error occurred; `Detail` has message |
| `auth-required` | Login required (from `LoginWithUpdates`) |

### Rate Limiting

WhatsApp does not publish explicit rate limits. Best practices:
- No explicit limiting in smolbot
- If send fails with rate-related error, retry after 60s
- Avoid sending many rapid messages to the same chat

### Message ID Format

Message IDs from inbound are strings like `"3EBXXXXXXXXXXXXXXX"`. These can be used for:
- Deduplication (handled automatically)
- Referencing in reactions (not implemented)

### Detecting Session Expiry

**Programmatic detection**: Check for `auth` category errors or `"not logged in"` / `"login required"` strings.

**Signs of expiry**:
- Send returns error containing "logged out" or "auth"
- `web_whatsapp_status` (if exposed) returns `unauthenticated`
- User reports not receiving messages

**Recovery**:
```
smolbot channels login whatsapp
```

### Privacy Considerations

- Messages are E2E encrypted by WhatsApp (smolbot cannot read them without key exchange)
- Session store (`whatsapp.db`) contains sensitive key material — protect file permissions
- Logging of message content should be minimal
- Contact names are cached in store (`whatsmeow_contacts` table)

### Debugging

**Check if linked**:
```go
if client.Store.ID == nil {
    // Not linked
}
```

**Check connection**:
```go
if !client.IsConnected() {
    // Disconnected
}
```

**Force reconnect**:
```go
client.Disconnect()
client.Connect()
```

### Relevant Files

| File | Purpose |
|------|---------|
| `pkg/channel/whatsapp/adapter.go` | Main adapter |
| `pkg/channel/whatsapp/adapter_test.go` | Tests with fakeSeam |
| `cmd/smolbot/channels_whatsapp_login.go` | TUI for QR login |
| `pkg/channel/qr/renderer.go` | QR code rendering |
| `pkg/config/config.go` | `WhatsAppChannelConfig` struct |
