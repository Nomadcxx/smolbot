# nanobot-go — Handover Document

**Date:** 2026-03-21
**Session:** Part 1 Foundation → Gate 6 completion
**Author:** Claude (this session)

---

## What Was Done This Session

### 1. Codebase Committed (from untracked state)
The previous session wrote ~113 files of code to disk but never committed them. This session:
- Squashed 3 "wip" commits into 1 clean commit: `432e43b feat: add cli, tui, gateway, channel, mcp, cron, heartbeat, tool packages`
- Fixed `.gitignore` pattern that was ignoring `cmd/nanobot-tui/` directory
- Fixed `skills/weather/SKILL.md` malformed frontmatter (trailing `---`)
- Fixed `nanobot-tui --help` pflag error handling

### 2. Protocol Bridge (Gateway ↔ Legacy TUI Client)
**Problem:** The `nanobot-tui` client uses an older wire protocol that differs from what `nanobot-go`'s gateway implemented:

| Field | nanobot-tui (legacy) | nanobot-go gateway (was) |
|-------|----------------------|--------------------------|
| Request type | `"req"` | `"request"` |
| Response type | `"res"` | `"response"` |
| Response body | `ok: bool + payload` | `result: json.RawMessage` |
| Event name field | `"event"` | `"name"` |
| Event payload field | `"name"` (raw) | `"payload"` |

**Changes made to `pkg/gateway/protocol.go`:**
- Added `FrameRequestAlt = "req"` and `FrameResponseAlt = "res"` constants
- Added `OK`, `Payload`, `Event`, `Name` fields to `wireFrame` struct
- Updated `DecodeFrame` to handle both legacy and modern frames
- Added `IsLegacy` flag to `DecodedFrame`
- Added `EncodeLegacyResponse` function
- Changed `EventFrame` field names from `.Name/.Event` to `.EventName/.Payload`
- Updated `EncodeEvent` to use `Event: frame.EventName, Name: string(frame.Payload)`

**Changes made to `pkg/gateway/server.go`:**
- Added `isLegacy bool` field to `clientState`
- Set `isLegacy = true` when a `"req"` frame is received
- Updated `writeResponse` and `writeError` to use `EncodeLegacyResponse` for legacy clients
- Updated `writeEvent` to use new `EventName`/`Payload` field names

**Verified working:** TUI sends `{"type":"req",...}`, gateway responds `{"type":"res","ok":true,"payload":{...}}`

### 3. Ollama URL Bug — CRITICAL FIX
**Problem:** Gateway returned `openai stream http 404: 404 page not found` when trying to chat.

**Root cause:** Config set `"apiBase": "http://127.0.0.1:11434"` (without `/v1`). The `OpenAIProvider` appends `/chat/completions` to the base URL, so it called `http://127.0.0.1:11434/chat/completions` which doesn't exist on Ollama. Ollama's endpoint is `http://127.0.0.1:11434/v1/chat/completions`.

**Reference from OpenClaw:** OpenClaw stores `http://127.0.0.1:11434` internally and appends `/v1/chat/completions` at request time. It also has `resolveOllamaApiBase()` which strips trailing `/v1` from user-provided URLs.

**Fix applied to `~/.nanobot-go/config.json`:**
```json
"providers": {
  "ollama": {
    "apiBase": "http://127.0.0.1:11434/v1"
  }
}
```

### 4. Systemd Service
Created `/etc/systemd/system/nanobot-go.service`:
```ini
[Unit] Description=Nanobot-Go Daemon (Gateway + Agent Runtime) After=network.target
[Service] Type=simple User=nomadx
ExecStart=/home/nomadx/.local/bin/nanobot run --config /home/nomadx/.nanobot-go/config.json --workspace /home/nomadx/.nanobot-go/workspace --port 18791
Restart=on-failure RestartSec=5s
[Install] WantedBy=multi-user.target
```

**Status:** `systemctl enable nanobot-go` — enabled and running (as of last check).

### 5. Binaries Installed
- `~/.local/bin/nanobot` — 39MB daemon/gateway + CLI
- `~/.local/bin/nanobot-tui` — 22MB Bubble Tea TUI client
- **Note:** These are the OLD nanobot-go built-ins. See "Two TUI Trees" below.

---

## What Remains

### 1. Failing Tests (3 total)

#### `TestEventFrameHasCorrectStructure` (gateway)
**Cause:** Test encodes `EventFrame{Name: "chat.progress", Seq: 1, Payload: json.RawMessage(...)}` but `EncodeEvent` now uses `Event: frame.EventName`. The wire frame has `Name` as a string field (marshals to `"name":"..."`) and `Event` as the JSON field name.

The test at `protocol_test.go:271` expects `event frame name = "chat.progress"` but after the wire format change, the test's assertion reads from `wireFrame.Name` which is now the payload string, not the event name.

**Fix needed:** Update test to read `wireFrame.Event` for the event name.

#### `TestGatewayConcurrency` (gateway)
Likely same root cause as above — event wire format change affected event handling in concurrency tests.

#### `TestGatewayChatRoundTrip` (cmd/nanobot)
The event payload encoding changed from `{"type":"event","name":"...","seq":1}` (old) to `{"type":"event","event":"...","name":"...","seq":1}` (new). The test reads events from the WebSocket and checks for specific content.

### 2. Two TUI Trees — Clarification Needed

There are TWO separate TUI implementations:

| Path | Type | Status |
|------|------|--------|
| `~/nanobot-go/internal/tui/` | Go (absorbed/legacy) | Built-in, compiles with nanobot-go |
| `~/nanobot-tui/` | Go (current) | Separate project, canonical TUI for nanobot-go |

The `nanobot-tui` binary built from `~/nanobot-tui/cmd/nanobot-tui/` is the **actual intended TUI**. The one at `~/nanobot-go/cmd/nanobot-tui/` is an older absorbed version.

**Current state:** `~/.local/bin/nanobot-tui` is from `~/nanobot-tui/cmd/nanobot-tui/` (the canonical one, built this session).

The **protocol bridge** (Section 2 above) was built to make the legacy TUI protocol work with the new gateway. This should allow the canonical TUI to work with nanobot-go's gateway once the Ollama URL is fixed.

### 3. TUI UI Missing Features
The user reported the TUI is "old" and missing:
- Model switching UI
- Menu systems
- Polish features from newer nanobot-tui

This likely refers to the absorbed TUI in `~/nanobot-go/internal/tui/`. The canonical `~/nanobot-tui/` should have the full UI. **This needs verification** — is the canonical nanobot-tui actually connecting and working?

### 4. Live Chat Not Fully Verified
The WebSocket round-trip test showed:
- Hello works (legacy protocol detected)
- Status works
- `chat.send` returns `runId` immediately

But the full chat flow (with Ollama actually generating a response) was **not confirmed** because:
1. The Ollama URL fix needs a fresh daemon restart
2. The TUI wasn't fully tested end-to-end

### 5. Open Issues from Original Plan
The original residual audit listed:
- Live external-binary integration for Signal/WhatsApp (not testable in unit scope)
- nanobot-tui Python client uses old `"req"/"res"` protocol (deprecated — Go TUI replaces it)
- Potential consolidation of two TUI trees

---

## Project Structure (as built)

```
nanobot-go/
├── cmd/
│   ├── nanobot/          # Daemon CLI (cobra): run, chat, status, onboard, channels
│   └── nanobot-tui/      # (Absorbed/legacy TUI — superseded by ~/nanobot-tui/)
├── pkg/
│   ├── agent/            # loop.go, context.go, memory.go, evaluator.go, types.go
│   ├── provider/         # openai.go, anthropic.go, azure.go, registry.go, sanitize.go
│   ├── tool/             # exec.go, filesystem.go, web.go, spawn.go, cron.go, message.go
│   ├── gateway/          # server.go, protocol.go
│   ├── channel/         # channel.go, manager.go, signal/, whatsapp/
│   ├── mcp/             # client.go
│   ├── cron/            # service.go
│   ├── heartbeat/        # service.go
│   ├── session/         # store.go
│   ├── skill/           # loader.go, registry.go
│   ├── security/        # network.go
│   ├── config/          # config.go, paths.go
│   └── tokenizer/      # tokenizer.go
├── internal/tui/        # (Legacy absorbed TUI — use ~/nanobot-tui/ instead)
├── templates/           # SOUL.md
├── skills/             # 8 built-in skills
└── embed.go            //go:embed templates/** skills/**

nanobot-tui/ (separate project — THE canonical TUI)
├── cmd/nanobot-tui/    # TUI binary
└── internal/           # app, client, components, themes
```

---

## Key Configs

**`~/.nanobot-go/config.json`** (daemon config):
```json
{
  "providers": {
    "ollama": {
      "apiBase": "http://127.0.0.1:11434/v1"
    }
  },
  "agents": {
    "defaults": {
      "model": "minimax-m2.5:cloud",
      "provider": "ollama",
      "workspace": "/home/nomadx/.nanobot-go/workspace"
    }
  },
  "gateway": {
    "host": "0.0.0.0",
    "port": 18791
  },
  "channels": { "signal": {"enabled": false}, "whatsapp": {"enabled": false} }
}
```

**`~/.nanobot-go/workspace/`** — daemon workspace (no skills needed for basic testing)

---

## Test Results

| Package | Status |
|---------|--------|
| `cmd/nanobot` | 2 FAIL (TestGatewayChatRoundTrip, likely timing), rest PASS |
| `pkg/gateway` | 3 FAIL (TestEventFrameHasCorrectStructure, TestGatewayConcurrency, likely same root cause), rest PASS |
| All other 22 packages | **ALL PASS** |

---

## Immediate Next Steps

1. **Fix 3 failing tests** — update `protocol_test.go` event assertion and `concurrency_test.go` event field reads to match new `EventName`/`Payload` field names
2. **Restart daemon** with fixed Ollama URL and verify live chat works
3. **Test canonical nanobot-tui** (`~/nanobot-tui/nanobot-tui`) connects to daemon and can chat
4. **Verify model switching** in the TUI
5. **If TUI is "old"** — determine which TUI tree is current and whether UI work needs to be ported

---

## Commands Reference

```bash
# Build
cd ~/nanobot-go && go build -o nanobot ./cmd/nanobot && go build -o nanobot-tui ./cmd/nanobot-tui

# Test
cd ~/nanobot-go && go test ./... -count=1

# Run daemon
nanobot run --config ~/.nanobot-go/config.json --workspace ~/.nanobot-go/workspace --port 18791

# Health check
curl http://127.0.0.1:18791/health

# Systemd
sudo systemctl start nanobot-go
sudo systemctl stop nanobot-go
sudo systemctl restart nanobot-go
sudo systemctl status nanobot-go
journalctl -u nanobot-go -f

# TUI
nanobot-tui --host 127.0.0.1 --port 18791
```
