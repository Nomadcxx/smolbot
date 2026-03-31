---
name: telegram
description: Operating procedures for the Telegram channel in smolbot. Use when sending messages, handling errors, or managing Telegram bot sessions.
always: false
---

## Telegram Channel

smolbot connects to Telegram via the `go-telegram/bot` library using long polling. All operations pass through `pkg/channel/telegram/adapter.go`.

### Sending Messages

Use the `message` tool with `channel: "telegram"`:

```
message(channel="telegram", chat_id="123456789", content="Hello")
```

The `chat_id` is a numeric Telegram chat ID (as a string). For DMs, this is the user's numeric ID. For groups, this is the group's numeric chat ID.

### Message Content

**Only text is supported.** The adapter explicitly filters for text:
```go
if update.Message == nil || update.Message.Text == "" {
    return
}
```

No support for photos, videos, documents, inline keyboards, or reply keyboards — these are not implemented in smolbot's Telegram adapter.

### Character Limit

Messages are chunked at **4096 characters** using paragraph-aware splitting (finds last `\n\n`, `\n`, or space). Code blocks spanning splits should close and reopen the `<pre>` tag.

### Formatting

smolbot's Telegram adapter does **not** apply automatic formatting. Agents must construct messages with appropriate Telegram-native formatting if desired. HTML is supported by Telegram's API but not automatically generated:

- `<b>bold</b>`
- `<i>italic</i>`
- `<code>code</code>`
- `<pre>pre-formatted</pre>`

### Token Management

Telegram uses a bot token (format: `123456789:ABCdef...`). Tokens are loaded from:
1. `BotToken` config field directly
2. `TokenFile` — reads from file, trims whitespace

Tokens never expire but can be revoked via @BotFather.

### Status States

- `disconnected` — initial state
- `connecting` — during `Start()`
- `connected` — running
- `error` — error with `Detail` containing the error message
- `auth-required` — token validation failed via `GetMe()`

### Error Handling

Errors from the Telegram API propagate directly:
- `401 Unauthorized` — token invalid or revoked → `auth-required` status
- `403 Forbidden` — bot blocked by user or removed from group
- `400 Bad Request` — malformed request
- `429 Too Many Requests` — rate limited, check `retry_after`

### Rate Limiting

smolbot does not implement explicit rate limiting. Telegram enforces:
- Global: 30 messages/second across all chats
- Per-chat: 1 message/second

Space rapid messages 1 second apart to stay within per-chat limits.

### Allowed Chat IDs

Optional whitelist in config:
```json
{
  "enabled": true,
  "botToken": "...",
  "allowedChatIDs": ["123456789", "-100987654321"]
}
```

Messages from non-whitelisted chats are silently ignored by the adapter.

### Inbound Deduplication

Messages are deduped within a 5-minute window using a `recent` map. Duplicate messages (same `chat_id` + same text) are silently ignored.
