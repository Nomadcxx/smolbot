---
name: signal
description: Operating procedures for the Signal channel in smolbot. Use when sending messages, handling errors, or managing Signal linked devices.
always: false
---

## Signal Channel

smolbot connects to Signal as a **linked device** via the `signal-cli` command-line tool subprocess. All operations pass through `pkg/channel/signal/adapter.go`.

### Sending Messages

Use the `message` tool with `channel: "signal"`:

```
message(channel="signal", chat_id="+15551234567", content="Hello")
```

The `chat_id` is a phone number in international format (e.g., `+15551234567`).

### Message Content

**Plain text only.** Signal does not support markdown, formatting, or attachments in smolbot's current implementation. Structure responses with:
- Capitalised headings
- Numbered lists with blank lines between items
- Plain punctuation

### signal-cli Integration

smolbot wraps `signal-cli` as a subprocess:
- Send: `signal-cli --config <datadir> -a <account> send -m <content> <chatId>`
- Receive: `signal-cli --config <datadir> -a <account> --output json receive`

The `CLIPath` config option specifies the signal-cli binary location (default: `signal-cli` from `PATH`).

### Configuration

```json
{
  "enabled": true,
  "account": "+15551234567",
  "dataDir": "~/.smolbot/signal",
  "cliPath": "signal-cli"
}
```

The `account` must be a registered Signal number. The `dataDir` stores session state.

### Login / Linking

```
smolbot channels login signal
```

This generates a QR code URI (`tsdevice:///?uuid=...`). The user scans it via Signal > Settings > Linked Devices > Link New Device.

### Detecting Session Expiry

Errors from `signal-cli` are wrapped: `fmt.Errorf("signal send: %w", err)`.

Watch for these in error output:
- `NotLinked` — device not linked
- `AuthorizationFailedError` — auth failed
- `Invalid registration` — number not registered
- `device not found` — device was revoked

If session expired, tell the user:
> "Signal session expired. Please run `smolbot channels login signal` and scan the QR code."

### Reconnection

The adapter implements exponential backoff on receive crashes:
- Initial delay: 5 seconds
- Doubles on each crash: 5s → 10s → 20s → ...
- Maximum delay: 5 minutes

The adapter automatically reconnects when the receive loop exits.

### Rate Limiting

Signal rate-limits message sending. If a send fails with a rate limit error, back off for 60 seconds before retrying.

### Privacy

Signal provides end-to-end encryption. Messages are not stored on Signal's servers after delivery. Do not log message content to disk or external services.

### Error Output Format

When signal-cli fails, the error includes the full command and output:
```
signal-cli --config ~/.smolbot/signal -a +15551234567 send -m hello +15557654321: exit status 1: Error: device not linked
```

Parse this to determine the failure reason.
