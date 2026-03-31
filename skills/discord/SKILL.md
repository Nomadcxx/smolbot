---
name: discord
description: Operating procedures for the Discord channel in smolbot. Use when sending messages, handling errors, or managing Discord bot sessions.
always: false
---

## Discord Channel

smolbot connects to Discord via the `discordgo` library. All operations pass through `pkg/channel/discord/adapter.go`.

### Sending Messages

Use the `message` tool with `channel: "discord"`:

```
message(channel="discord", chat_id="123456789012345678", content="Hello")
```

The `chat_id` is a Discord snowflake ID (as a string). For DMs, this is the user's ID. For server channels, this is the channel ID.

### Message Content

**Only plain text is supported.** The adapter sends via `discordgo.Session.ChannelMessageSend()` which sends plain text only. There is no embed support implemented in smolbot's Discord adapter — embeds mentioned in skill documentation are NOT currently supported.

### Character Limit

Messages are chunked at **2000 characters** (Discord's hard limit). The adapter uses paragraph-aware splitting (finds last `\n\n`, `\n`, or space before the limit).

### Formatting

Discord uses its own markdown:
- Bold: `**text**`
- Italic: `*text*` or `_text_`
- Underline: `__text__`
- Strikethrough: `~~text~~`
- Inline code: `` `code` ``
- Code block: ` ```language\ncode\n``` ``

Standard Markdown features not supported: tables, headings.

### Token Management

Discord uses a bot token (passed to `discordgo.New("Bot " + token)`). Tokens are loaded from:
1. `BotToken` config field directly
2. `TokenFile` — reads from file, trims whitespace

Tokens never expire but can be revoked via the Discord Developer Portal.

### Status States

- `disconnected` — initial state
- `connecting` — during `Start()`
- `connected` — running
- `error` — error with `Detail` containing the error message
- `auth-required` — token validation failed via `Identify()`

### Error Handling

All errors are wrapped: `fmt.Errorf("discord send: %w", err)`.

Known Discord error codes:
- `50013 Missing Permissions` — bot lacks a required permission in that channel
- `10003 Unknown Channel` — channel ID is wrong or bot has no access
- `50007 Cannot send messages to this user` — user has DMs disabled
- `401 Unauthorized` — invalid bot token

### Rate Limiting

smolbot does not implement explicit rate limiting. Discord enforces:
- Per channel: 5 messages per 5 seconds
- Global: 50 requests per second

If rate limited, Discord returns `429 Too Many Requests` with a `retry_after` field. Always honour the backoff.

### Allowed Channel IDs

Optional whitelist in config:
```json
{
  "enabled": true,
  "botToken": "...",
  "allowedChannelIDs": ["123456789012345678"]
}
```

Messages from non-whitelisted channels are silently ignored by the adapter.

### Inbound Handling

The adapter listens for `*discordgo.MessageCreate` events and filters:
- Messages from bots (`m.Author.Bot`)
- Messages with nil content

Guild messages include `guild_id`; DM messages do not.

### Bot vs Webhook

smolbot uses a bot account (two-way interaction). Webhooks are separate and not managed by the adapter.
