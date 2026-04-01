# Gateway & Protocol Contracts Bugfix Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Fix 4 protocol bugs that cause visible failures: sessions can't be reset, skills.list always returns empty, `smolbot status` channels field always wrong, and TUI can't detect when a tool response was already delivered to a channel.

**Architecture:** All fixes are isolated one- to three-line changes across the client protocol layer (`internal/client/`) and the runtime wiring (`cmd/smolbot/runtime.go`). No new types or files needed; two existing tests must be updated to match actual server wire format.

**Tech Stack:** Go standard library, `gorilla/websocket`, `encoding/json`

---

## File Map

| File | What changes |
|---|---|
| `internal/client/messages.go` | Fix `sessions.reset` param: `"key"` → `"session"` |
| `internal/client/messages_test.go` | Add test verifying `sessions.reset` sends `"session"` |
| `internal/client/protocol.go` | Add `DeliveredToRequestTarget bool` field to `ToolDonePayload` |
| `internal/client/protocol_test.go` | Add test verifying field decodes + omits when false |
| `cmd/smolbot/runtime.go` | (a) Add `Skills: skills, Cron: cronService` to `gateway.NewServer()`; (b) replace `statusReport` struct with correct JSON tags and channel type; update `formatStatus` and `fetchChannelStatusesImpl` |
| `cmd/smolbot/runtime_test.go` | Update two existing tests (`TestFetchStatusQueriesGateway`, `TestFetchChannelStatusesQueriesGatewayStatus`) to send correct server wire format; add `TestBuildRuntimeWiresSkillsToGateway` |

---

## Task 1: Fix sessions.reset parameter typo (C10)

**Root cause:** `messages.go:57` sends `{"key": sessionKey}` but the gateway's `sessions.reset` handler at `pkg/gateway/server.go:426` reads `params.Session`. The session field is always empty, so the reset is always a no-op.

**Files:**
- Modify: `internal/client/messages.go:57`
- Test: `internal/client/messages_test.go`

- [ ] **Step 1: Write the failing test**

Add after `TestModelsSetRejectsResponseWithoutCurrentModel` in `internal/client/messages_test.go`. Ensure `"strings"` is in the import list.

```go
func TestSessionsResetSendsSessionParam(t *testing.T) {
	upgrader := websocket.Upgrader{CheckOrigin: func(*http.Request) bool { return true }}
	received := make(chan json.RawMessage, 1)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Fatalf("Upgrade: %v", err)
		}
		defer conn.Close()

		if _, raw, err := conn.ReadMessage(); err != nil {
			t.Fatalf("Read hello: %v", err)
		} else {
			var hello Request
			if err := json.Unmarshal(raw, &hello); err != nil {
				t.Fatalf("Unmarshal hello: %v", err)
			}
			if err := conn.WriteJSON(Response{
				Type:    FrameRes,
				ID:      hello.ID,
				OK:      true,
				Payload: json.RawMessage(`{"server":"smolbot","version":"test","protocol":1,"methods":["sessions.reset"],"events":[]}`),
			}); err != nil {
				t.Fatalf("Write hello: %v", err)
			}
		}

		if _, raw, err := conn.ReadMessage(); err != nil {
			t.Fatalf("Read sessions.reset: %v", err)
		} else {
			var wire struct {
				Method string          `json:"method"`
				ID     string          `json:"id"`
				Params json.RawMessage `json:"params"`
			}
			if err := json.Unmarshal(raw, &wire); err != nil {
				t.Fatalf("Unmarshal wire: %v", err)
			}
			received <- append([]byte(nil), wire.Params...)
			if err := conn.WriteJSON(Response{
				Type:    FrameRes,
				ID:      wire.ID,
				OK:      true,
				Payload: json.RawMessage(`{"ok":true}`),
			}); err != nil {
				t.Fatalf("Write response: %v", err)
			}
		}
	}))
	defer srv.Close()

	c := New("ws" + strings.TrimPrefix(srv.URL, "http") + "/ws")
	defer c.Close()
	if _, err := c.Connect(); err != nil {
		t.Fatalf("Connect: %v", err)
	}

	if err := c.SessionsReset("my-session"); err != nil {
		t.Fatalf("SessionsReset: %v", err)
	}

	var params map[string]string
	if err := json.Unmarshal(<-received, &params); err != nil {
		t.Fatalf("Unmarshal params: %v", err)
	}
	if got, ok := params["session"]; !ok || got != "my-session" {
		t.Fatalf("sessions.reset params = %#v, want {\"session\":\"my-session\"}", params)
	}
	if _, hasKey := params["key"]; hasKey {
		t.Fatalf("sessions.reset still sends old 'key' field: %#v", params)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
cd /home/nomadx/Documents/smolbot
go test ./internal/client/ -run TestSessionsResetSendsSessionParam -v
```

Expected: FAIL — the test will either hang (if the server reads `"key"` and the reset silently succeeds) or fail the param assertion because `params["session"]` will be absent.

- [ ] **Step 3: Fix the implementation**

In `internal/client/messages.go:57`, change `"key"` to `"session"`:

```go
func (c *Client) SessionsReset(key string) error {
	_, err := c.sendRequest("sessions.reset", map[string]string{"session": key})
	return err
}
```

- [ ] **Step 4: Run test to verify it passes**

```bash
go test ./internal/client/ -run TestSessionsResetSendsSessionParam -v
```

Expected: PASS

- [ ] **Step 5: Run all client tests**

```bash
go test ./internal/client/ -v
```

Expected: all PASS

- [ ] **Step 6: Commit**

```bash
git add internal/client/messages.go internal/client/messages_test.go
git commit -m "fix(client): sessions.reset sends 'session' param not 'key'"
```

---

## Task 2: Add DeliveredToRequestTarget to ToolDonePayload (H3)

**Root cause:** Gateway emits `chat.tool.done` with a `deliveredToRequestTarget: true` field when a tool sends its output directly to the originating channel (e.g. WhatsApp). `ToolDonePayload` in `protocol.go` doesn't have this field, so the TUI always sees it as false and can't suppress the final chat response that would otherwise double-send to the channel.

**Files:**
- Modify: `internal/client/protocol.go:96-101`
- Test: `internal/client/protocol_test.go`

- [ ] **Step 1: Write the failing tests**

Add to `internal/client/protocol_test.go`. First add `"strings"` to the imports if not already present.

```go
func TestToolDonePayloadDecodesDeliveredField(t *testing.T) {
	raw := []byte(`{"name":"web_search","output":"results","id":"call-1","deliveredToRequestTarget":true}`)
	var p ToolDonePayload
	if err := json.Unmarshal(raw, &p); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if !p.DeliveredToRequestTarget {
		t.Fatal("DeliveredToRequestTarget was not decoded: field missing from struct")
	}
	if p.Name != "web_search" || p.ID != "call-1" {
		t.Fatalf("unexpected payload %#v", p)
	}
}

func TestToolDonePayloadOmitsDeliveredWhenFalse(t *testing.T) {
	p := ToolDonePayload{Name: "tool", Output: "out", ID: "id-1"}
	raw, err := json.Marshal(p)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	if strings.Contains(string(raw), "deliveredToRequestTarget") {
		t.Fatalf("deliveredToRequestTarget should be omitted when false, got: %s", raw)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
go test ./internal/client/ -run "TestToolDonePayload" -v
```

Expected: `TestToolDonePayloadDecodesDeliveredField` FAIL — field is absent from struct, JSON silently discards it, `p.DeliveredToRequestTarget` is always false.

- [ ] **Step 3: Add the field to ToolDonePayload**

In `internal/client/protocol.go:96-101`, replace:

```go
type ToolDonePayload struct {
	Name   string `json:"name"`
	Output string `json:"output"`
	Error  string `json:"error,omitempty"`
	ID     string `json:"id"`
}
```

With:

```go
type ToolDonePayload struct {
	Name                    string `json:"name"`
	Output                  string `json:"output"`
	Error                   string `json:"error,omitempty"`
	ID                      string `json:"id"`
	DeliveredToRequestTarget bool   `json:"deliveredToRequestTarget,omitempty"`
}
```

- [ ] **Step 4: Run tests to verify they pass**

```bash
go test ./internal/client/ -run "TestToolDonePayload" -v
```

Expected: both PASS

- [ ] **Step 5: Run all client tests**

```bash
go test ./internal/client/ -v
```

Expected: all PASS

- [ ] **Step 6: Commit**

```bash
git add internal/client/protocol.go internal/client/protocol_test.go
git commit -m "fix(protocol): add DeliveredToRequestTarget field to ToolDonePayload"
```

---

## Task 3: Wire Skills and Cron to gateway (K2)

**Root cause:** `gateway.NewServer()` in `cmd/smolbot/runtime.go:746` is called without `Skills:` or `Cron:` fields. Both `skills` (line 618) and `cronService` (used at line 740) are already in scope. The gateway's `skills.list` handler returns `[]` when `s.skills == nil`, and `cron.list` returns `[]` when `s.cron == nil`.

**Files:**
- Modify: `cmd/smolbot/runtime.go:746-759`
- Test: `cmd/smolbot/runtime_test.go`

- [ ] **Step 1: Write a failing integration test**

Add to `cmd/smolbot/runtime_test.go`. Note: `freePort(t)` is defined in `runtime_test.go:1043`, `writeConfigFile` exists in the same file, and `connectGatewayClient` is in `runtime_model_test.go` (same package).

```go
func TestBuildRuntimeWiresSkillsToGateway(t *testing.T) {
	port := freePort(t)
	cfg := config.DefaultConfig()
	cfg.Agents.Defaults.Model = "gpt-test"
	cfg.Agents.Defaults.Workspace = filepath.Join(t.TempDir(), "workspace")
	cfg.Gateway.Host = "127.0.0.1"
	cfg.Gateway.Port = port

	cfgPath := filepath.Join(t.TempDir(), "config.json")
	if err := writeConfigFile(cfgPath, &cfg); err != nil {
		t.Fatalf("writeConfigFile: %v", err)
	}

	app, err := buildRuntime(daemonLaunchOptions{
		ConfigPath: cfgPath,
		Port:       port,
	}, runtimeDeps{
		Provider: &fakeRuntimeProvider{},
	})
	if err != nil {
		t.Fatalf("buildRuntime: %v", err)
	}
	defer app.Close()

	httpServer := httptest.NewServer(app.gateway.Handler())
	defer httpServer.Close()

	cl := connectGatewayClient(t, httpServer.URL)
	defer cl.Close()

	skills, err := cl.Skills()
	if err != nil {
		t.Fatalf("Skills: %v", err)
	}
	if len(skills) == 0 {
		t.Fatal("skills.list returned empty list — Skills not wired to gateway in buildRuntime")
	}
}
```

- [ ] **Step 2: Run the test to verify it fails**

```bash
go test ./cmd/smolbot/ -run TestBuildRuntimeWiresSkillsToGateway -v
```

Expected: FAIL — `skills.list` returns `[]`, test reports "Skills not wired to gateway in buildRuntime"

- [ ] **Step 3: Wire Skills and Cron**

In `cmd/smolbot/runtime.go:746-759`, add `Skills: skills, Cron: cronService`:

```go
gateway: gateway.NewServer(gateway.ServerDeps{
    Agent:     loop,
    Sessions:  sessions,
    Channels:  channels,
    Config:    cfg,
    Usage:     usageStore,
    Skills:    skills,
    Cron:      cronService,
    Version:   version,
    StartedAt: time.Now(),
    SetModelCallback: func(model string) (string, error) {
        loop.SetActiveModel(model)
        heartbeatService.SetActiveModel(model)
        return loop.EffectiveModel(), nil
    },
}),
```

- [ ] **Step 4: Run the test to verify it passes**

```bash
go test ./cmd/smolbot/ -run TestBuildRuntimeWiresSkillsToGateway -v
```

Expected: PASS

- [ ] **Step 5: Run all gateway tests**

```bash
go test ./pkg/gateway/ ./cmd/smolbot/ -v -count=1 2>&1 | tail -30
```

Expected: all PASS

- [ ] **Step 6: Commit**

```bash
git add cmd/smolbot/runtime.go cmd/smolbot/runtime_test.go
git commit -m "fix(runtime): wire Skills and Cron to gateway so skills.list returns real data"
```

---

## Task 4: Fix statusReport JSON struct (K4)

**Root cause:** `statusReport` in `cmd/smolbot/runtime.go:47-53` has three bugs:
1. `Channels []string` — server sends `[]{"name":..., "status":...}` objects, not strings. JSON silently discards them; channel list is always empty.
2. No JSON tags on `UptimeSeconds` — server sends `"uptime"` but the field maps to `"uptimeseconds"` (case-insensitive); uptime is always 0.
3. `ChannelStates map[string]map[string]string` — server never sends this field; `fetchChannelStatusesImpl` reads it to look up channel state, but it's always nil, so channel state is always "registered".

The two existing tests (`TestFetchStatusQueriesGateway`, `TestFetchChannelStatusesQueriesGatewayStatus`) assert against the old broken format and must be updated to match what the real server sends.

**Files:**
- Modify: `cmd/smolbot/runtime.go` (struct definition + `formatStatus` + `fetchChannelStatusesImpl`)
- Modify: `cmd/smolbot/runtime_test.go` (update two existing tests; add one new assertion to `TestFetchStatusQueriesGateway`)

- [ ] **Step 1: Update the existing tests to use correct server wire format**

The two tests currently have fake servers that send:
```go
"channels":      []string{"slack", "discord"},
"channelStates": map[string]map[string]string{"slack": {"state": "connected"}, ...},
"uptimeSeconds": 42,
"connectedClients": 2,
```

This was the old format. The real gateway (server.go:262-270) sends:
```json
"channels": [{"name": "slack", "status": "connected"}, {"name": "discord", "status": "error"}],
"uptime": 42
```

In `TestFetchStatusQueriesGateway` (around line 72), replace the `json.Marshal(map[string]any{...})` block with:

```go
payload, err := json.Marshal(map[string]any{
    "model":    "claude-sonnet",
    "uptime":   42,
    "channels": []map[string]string{
        {"name": "slack", "status": "connected"},
        {"name": "discord", "status": "error"},
    },
})
```

And replace the assertion block (around line 107-112):

```go
if report.Model != "claude-sonnet" || report.Uptime != 42 {
    t.Fatalf("unexpected status report %#v", report)
}
if len(report.Channels) != 2 || report.Channels[0].Name != "slack" || report.Channels[1].Name != "discord" {
    t.Fatalf("unexpected channels %#v", report.Channels)
}
```

In `TestFetchChannelStatusesQueriesGatewayStatus` (around line 134), replace the `json.Marshal(map[string]any{...})` block with the same new payload:

```go
payload, err := json.Marshal(map[string]any{
    "model":    "claude-sonnet",
    "uptime":   42,
    "channels": []map[string]string{
        {"name": "slack", "status": "connected"},
        {"name": "discord", "status": "error"},
    },
})
```

(Assertions in this test — `statuses[0].State != "connected"`, `statuses[1].State != "error"` — remain correct.)

- [ ] **Step 2: Run the updated tests to confirm they fail**

```bash
go test ./cmd/smolbot/ -run "TestFetchStatus|TestFetchChannel" -v
```

Expected: FAIL — `report.Uptime` is always 0 (wrong JSON tag), `report.Channels` always empty (wrong type)

- [ ] **Step 3: Replace the statusReport struct and fix its consumers**

In `cmd/smolbot/runtime.go` (search for "type statusReport struct" — line numbers may shift during implementation), replace the `statusReport` and `channelStatus` types with:

```go
type statusReport struct {
	Model    string         `json:"model"`
	Uptime   int            `json:"uptime"`
	Channels []channelEntry `json:"channels"`
}

type channelEntry struct {
	Name   string `json:"name"`
	Status string `json:"status"`
}

type channelStatus struct {
	Name   string
	State  string
	Detail string
}
```

Note: `channelEntry.Status` (field name) maps from JSON key `"status"` and is copied to `channelStatus.State` in `fetchChannelStatusesImpl`.

Update `formatStatus` (search for the function name — location may shift):

```go
func formatStatus(report *statusReport) string {
	channels := make([]string, 0, len(report.Channels))
	for _, ch := range report.Channels {
		channels = append(channels, ch.Name+"="+ch.Status)
	}
	return fmt.Sprintf(
		"model: %s\nuptime: %d\nchannels: %s\n",
		report.Model,
		report.Uptime,
		strings.Join(channels, ", "),
	)
}
```

Update `fetchChannelStatusesImpl` (search for the function name — location may shift):

```go
func fetchChannelStatusesImpl(ctx context.Context, opts rootOptions) ([]channelStatus, error) {
	report, err := fetchStatus(ctx, opts)
	if err != nil {
		return nil, err
	}
	statuses := make([]channelStatus, 0, len(report.Channels))
	for _, ch := range report.Channels {
		statuses = append(statuses, channelStatus{
			Name:  ch.Name,
			State: ch.Status,
		})
	}
	return statuses, nil
}
```

- [ ] **Step 4: Check the build compiles cleanly**

```bash
go build ./cmd/smolbot/
```

Expected: no errors. If there are references to removed fields (`UptimeSeconds`, `ConnectedClients`, `ChannelStates`), fix them now.

- [ ] **Step 5: Run the updated tests to verify they pass**

```bash
go test ./cmd/smolbot/ -run "TestFetchStatus|TestFetchChannel" -v
```

Expected: all PASS

- [ ] **Step 6: Run all smolbot tests**

```bash
go test ./cmd/smolbot/ -v -count=1 2>&1 | tail -40
```

Expected: all PASS

- [ ] **Step 7: Commit**

```bash
git add cmd/smolbot/runtime.go cmd/smolbot/runtime_test.go
git commit -m "fix(runtime): correct statusReport JSON struct so smolbot status shows channels and uptime"
```

---

## Final verification

- [ ] **Run the full test suite**

```bash
go test ./... 2>&1 | grep -E "FAIL|ok"
```

Expected: all packages `ok`, no `FAIL`

- [ ] **Manual smoke test**

```bash
go build -o /tmp/smolbot ./cmd/smolbot && \
systemctl --user stop smolbot && \
install -m755 /tmp/smolbot ~/.local/bin/smolbot && \
systemctl --user start smolbot && \
sleep 1 && \
smolbot status
```

Expected: `smolbot status` shows uptime (non-zero) and channels list (e.g. `whatsapp=connected`).

---

## Self-Review

**Spec coverage:**
- C10 (sessions.reset param) → Task 1 ✓
- H3 (ToolDonePayload missing field) → Task 2 ✓
- K2 (Skills not wired to gateway) → Task 3 ✓
- K4 (statusReport type mismatch) → Task 4 ✓
- C11 (hello advertises unimplemented methods) → not applicable; current hello response does NOT advertise cron.add/delete/keybindings methods
- C12 (channel events never emitted) → already fixed in a prior session; `BroadcastEvent` calls exist at runtime.go:1133-1195

**Placeholder scan:** No TBD or TODO in plan steps. All code blocks are complete.

**Type consistency:**
- `channelEntry.Status` (new type, server's JSON key) → mapped to `channelStatus.State` in `fetchChannelStatusesImpl` ✓
- `ToolDonePayload.DeliveredToRequestTarget` → `omitempty` tag matches gateway emission behavior ✓
- `statusReport.Channels []channelEntry` → `channelEntry{Name, Status}` matches server wire format `{"name":..., "status":...}` ✓
