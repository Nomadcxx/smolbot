---
name: channel-triage
description: Routing and triage across multiple messaging channels in smolbot. Use when deciding which channel to use, handling simultaneous inbound, or escalating between channels.
always: false
---

## Channel Triage in smolbot

smolbot supports four channels: WhatsApp, Telegram, Discord, and Signal. Each has different capabilities and limitations.

### Channel Capabilities Matrix

| Capability | WhatsApp | Telegram | Discord | Signal |
|------------|----------|----------|---------|--------|
| Library | whatsmeow | go-telegram/bot | discordgo | signal-cli subprocess |
| Text | Yes | Yes | Yes | Yes |
| Formatting | WhatsApp markup | HTML/plain | Discord markdown | Plain text only |
| Media inbound | Captions only | Not supported | Not supported | Not supported |
| Character limit | 4096 | 4096 | 2000 | No hard limit |
| Token/session | QR link (14 days) | Bot token (persistent) | Bot token (persistent) | QR link (revocable) |
| Deduplication | 5 min window | 5 min window | None | None |
| Allowed list | No | Yes (chat IDs) | Yes (channel IDs) | No |

### Routing by Message Type

**Sensitive content** (credentials, personal data): Signal first (E2E encrypted, no logging), WhatsApp second.

**Long technical output** (code, logs, reports):
- Telegram: 4096 char limit, paragraph-aware chunking, plain text
- Discord: 2000 char limit, plain text only

**Quick casual requests**: WhatsApp or Telegram DM.

**Group/community**: Discord (server channels) or WhatsApp groups.

**Automation triggers**: Telegram (persistent bot token, no session expiry).

### User Preference

If the user has expressed a channel preference, honour it. When unknown, ask once and store in memory via the `memory` tool.

### Handling Simultaneous Inbound

Each channel operates independently. Requests arriving on multiple channels simultaneously are processed separately. Each channel has its own conversation context.

Do not merge sessions across channels. If the same user sends the same request on two channels, handle both and note the duplication.

### Consistent Identity

The agent presents the same persona across channels. Formatting adapts to channel capabilities, but tone and capabilities remain consistent. Do not promise a capability on one channel that is unavailable on another.

### When to Suggest a Different Channel

Suggest switching when:
- A long code review is requested on WhatsApp (Signal or Telegram better for length)
- Detailed technical output on WhatsApp (better on Telegram or Discord)
- Signal user requests formatted output (Signal only supports plain text)

### Escalation Paths

If the primary channel fails:

**WhatsApp failure** → Telegram (if configured) → inform user of options

**Telegram failure** → WhatsApp (if configured)

**Discord failure** → Telegram (if configured)

**Signal failure** → No automatic fallback (by design — privacy-conscious users may not want fallback). Inform the user manually.

### Channel Status

Each channel reports status via `adapter.Status()`:
- `connecting` — starting up
- `connected` — active
- `disconnected` — not connected
- `error` — error with detail
- `auth-required` — session/token issue (QR expired, token revoked)

Check channel status when troubleshooting delivery failures.

### Sending to Multiple Channels

To send the same message to multiple channels, call the `message` tool separately for each channel with the appropriate `chat_id`. Do not assume a single `chat_id` works across channels — they are distinct identifiers.

### Configuration Discovery

Channel configuration is in `pkg/config/config.go`. The enabled channels and their settings are loaded at startup. Use `smolbot status` to see channel states.