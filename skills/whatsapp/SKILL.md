---
name: whatsapp
description: Operating procedures for the WhatsApp channel in smolbot. Use when sending messages, handling errors, or managing WhatsApp sessions.
always: false
---

## WhatsApp Channel

smolbot connects to WhatsApp via the `whatsmeow` library. All outbound and inbound messages pass through `pkg/channel/whatsapp/adapter.go`.

### Sending Messages

Use the `message` tool with `channel: "whatsapp"`:

```
message(channel="whatsapp", chat_id="15551234567", content="Hello")
```

The `message` tool routes through `MessageRouter.Route()` which dispatches to the WhatsApp adapter's `Send()` method. The adapter sends via `whatsmeow.Client.SendMessage()`.

### Message Content

**Only text is supported.** Inbound messages are parsed from:
- Direct text (`msg.GetConversation()`)
- Extended text (`msg.GetExtendedTextMessage().GetText()`)
- Image/video/document captions

No media files are parsed for content — only text with optional captions.

### Character Limit

Messages are chunked at **4096 characters** using paragraph-aware splitting. Find the last `\n\n`, `\n`, or space before the limit. For code blocks spanning splits, close the block in the first message and reopen in the second.

### Formatting

WhatsApp uses its own markup, **not standard Markdown**:
- Bold: `*text*`
- Italic: `_text_`
- Strikethrough: `~text~`
- Monospace: `` `text` `` (inline) or ` ```text``` ` (block)

Standard Markdown (`**bold**`, `__italic__`) renders as literal characters. Never use standard Markdown in WhatsApp messages.

### Session Management

Sessions are stored in SQLite at `~/.smolbot/whatsapp.db` by default. The session is tied to the device link established during QR login.

Session states (from `adapter.Status()`):
- `connecting` — during startup
- `connected` — active and ready
- `disconnected` — not connected
- `error` — error with detail message

### Detecting Session Expiry

Errors are categorized in `categorizeError()`:
- `"auth"` category — non-retryable, session expired
- `"connection"` category — retryable, temporary disconnect
- `"rate"` category — retryable, rate limited

If the send tool returns an error containing "logged out" or "auth", the session has expired. Tell the user:
> "WhatsApp session expired. Please run `smolbot channels login whatsapp` on the server and scan the QR code."

### Login Command

```
smolbot channels login whatsapp
```

This displays a QR code via the TUI. The user scans it with WhatsApp > Settings > Linked Devices > Link New Device.

### Rate Limiting

No explicit rate limiting is implemented in smolbot. The adapter recognizes rate limit errors from whatsmeow. If rate limited:
- Retry once after 10 seconds
- If retry fails, back off for 60 seconds

### Error Wrapping

All WhatsApp send errors are wrapped: `fmt.Errorf("whatsapp send: %w", err)`. Read the wrapped error to determine the category.

### Inbound Handling

The adapter filters:
- Own messages (via `shouldIgnoreInboundMessage()`)
- Duplicate messages within a 5-minute sliding window

Group messages arrive from multiple senders. Track sender identity via the `ChatID` field if needed.
