# Telegram Recovery And Integration Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Recover the partial Telegram channel work into a compile-clean, tested, fully wired feature that is integrated into runtime, installer, and channel status/login flows.

**Architecture:** Keep Telegram aligned with the existing channel abstraction: a small adapter in `pkg/channel/telegram`, configuration in `pkg/config`, runtime registration in `cmd/smolbot/runtime.go`, and installer/onboarding exposure in `cmd/installer`. Reuse shared helpers instead of channel-local duplicates, and add focused tests before each integration step so the partially implemented slice becomes safe to extend.

**Tech Stack:** Go, Cobra CLI, existing `pkg/channel` abstractions, existing installer Bubble Tea flow, `github.com/go-telegram/bot`

---

### Task 1: Establish a clean Telegram baseline

**Files:**
- Modify: `go.mod`
- Modify: `go.sum`
- Test: `pkg/channel/telegram/adapter_test.go`

**Step 1: Write the failing test**

Create `pkg/channel/telegram/adapter_test.go` with a compile-smoke test that imports the package and a constructor test that expects `NewProductionAdapter` to reject an empty token.

```go
func TestNewProductionAdapterRequiresToken(t *testing.T) {
	_, err := NewProductionAdapter(config.TelegramChannelConfig{})
	if err == nil {
		t.Fatal("expected token error")
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./pkg/channel/telegram -run 'TestNewProductionAdapterRequiresToken'`
Expected: FAIL because the Telegram module dependency is missing and/or the package does not build.

**Step 3: Write minimal implementation**

- Add `github.com/go-telegram/bot` to `go.mod`
- Refresh `go.sum`
- Keep the constructor behavior minimal: empty token must return an error

**Step 4: Run test to verify it passes**

Run: `go test ./pkg/channel/telegram -run 'TestNewProductionAdapterRequiresToken'`
Expected: PASS

**Step 5: Commit**

```bash
git add go.mod go.sum pkg/channel/telegram/adapter_test.go
git commit -m "fix(telegram): add adapter dependency baseline"
```

**Gate:** Do not continue until `go test ./pkg/channel/telegram` builds successfully.

---

### Task 2: Make the Telegram adapter correct and test-covered

**Files:**
- Modify: `pkg/channel/telegram/adapter.go`
- Modify: `pkg/channel/chunk.go`
- Test: `pkg/channel/telegram/adapter_test.go`

**Step 1: Write the failing tests**

Add focused tests for:
- `LoginWithUpdates(nil)` must not panic
- `Start`/`Stop` update adapter status
- `Send` uses the shared `channel.ChunkMessage`
- token file loading works
- allowlist filtering works
- duplicate inbound messages are suppressed

Include a fake seam so tests do not hit Telegram.

**Step 2: Run tests to verify they fail**

Run: `go test ./pkg/channel/telegram -run 'Test(Adapter|NewProductionAdapter)'`
Expected: FAIL on nil callback behavior, stale status behavior, or chunking mismatch.

**Step 3: Write minimal implementation**

- Guard all `report(...)` calls in `LoginWithUpdates`
- Update adapter status consistently in `Start`, `Stop`, and login flows
- Remove the private `chunkMessage` helper
- Use `channel.ChunkMessage(msg.Content, 4096)` in `Send`
- Keep allowlist and dedupe behavior explicit and testable

**Step 4: Run tests to verify they pass**

Run: `go test ./pkg/channel/telegram -run 'Test(Adapter|NewProductionAdapter)'`
Expected: PASS

**Step 5: Commit**

```bash
git add pkg/channel/telegram/adapter.go pkg/channel/chunk.go pkg/channel/telegram/adapter_test.go
git commit -m "fix(telegram): harden adapter behavior"
```

**Gate:** Run a spec review and a code-quality review on the Telegram adapter slice before moving on.

---

### Task 3: Wire Telegram into runtime channel registration

**Files:**
- Modify: `cmd/smolbot/runtime.go`
- Modify: `cmd/smolbot/runtime_test.go`
- Modify: `cmd/smolbot/runtime_services_test.go`
- Modify: `cmd/smolbot/runtime_tools_test.go` if needed

**Step 1: Write the failing tests**

Add runtime tests that verify:
- enabled Telegram config creates/registers the adapter
- Telegram can coexist with Signal/WhatsApp registration
- runtime surfaces Telegram status through the existing channel manager path

Use injectable constructor seams instead of the real Telegram client.

**Step 2: Run tests to verify they fail**

Run: `go test ./cmd/smolbot -run 'Test.*Telegram'`
Expected: FAIL because runtime does not yet register Telegram.

**Step 3: Write minimal implementation**

- Add a `newTelegramChannel` seam alongside existing channel seams
- Instantiate Telegram when `cfg.Channels.Telegram.Enabled`
- Register it with the existing `channel.Manager`
- Keep error handling consistent with existing Signal/WhatsApp setup

**Step 4: Run tests to verify they pass**

Run: `go test ./cmd/smolbot -run 'Test.*Telegram'`
Expected: PASS

**Step 5: Commit**

```bash
git add cmd/smolbot/runtime.go cmd/smolbot/runtime_test.go cmd/smolbot/runtime_services_test.go cmd/smolbot/runtime_tools_test.go
git commit -m "feat(runtime): register telegram channel"
```

**Gate:** `go test ./pkg/channel/telegram ./cmd/smolbot -run 'Test.*Telegram'` must pass before installer work starts.

---

### Task 4: Add Telegram installer and config UX

**Files:**
- Modify: `cmd/installer/types.go`
- Modify: `cmd/installer/main.go`
- Modify: `cmd/installer/views.go`
- Modify: `cmd/installer/tasks.go`
- Modify: `pkg/config/config_test.go`
- Test: installer tests adjacent to touched files

**Step 1: Write the failing tests**

Add tests that verify:
- installer model stores Telegram enabled/token-file values
- generated config includes Telegram fields when enabled
- disabled Telegram does not inject stray token values

**Step 2: Run tests to verify they fail**

Run: `go test ./cmd/installer ./pkg/config -run 'Test.*Telegram|TestLoad'`
Expected: FAIL because installer does not yet surface Telegram.

**Step 3: Write minimal implementation**

- Extend installer model/state with Telegram controls
- Add Telegram view copy and input handling
- Emit Telegram config in task/config generation
- Keep token handling file-based where possible; do not hardcode secrets into source

**Step 4: Run tests to verify they pass**

Run: `go test ./cmd/installer ./pkg/config -run 'Test.*Telegram|TestLoad'`
Expected: PASS

**Step 5: Commit**

```bash
git add cmd/installer/types.go cmd/installer/main.go cmd/installer/views.go cmd/installer/tasks.go pkg/config/config_test.go
git commit -m "feat(installer): add telegram configuration flow"
```

**Gate:** Run spec review on installer/runtime/config integration before moving on.

---

### Task 5: Expose Telegram in CLI login/status surfaces where applicable

**Files:**
- Modify: `cmd/smolbot/channels.go`
- Modify: `cmd/smolbot/channels_login.go`
- Modify: `cmd/smolbot/channels_status.go`
- Modify: related tests in `cmd/smolbot/*test.go`

**Step 1: Write the failing tests**

Add tests that verify:
- Telegram appears in channel status output when configured
- Telegram login/validation path is routed through the existing interactive login abstraction if supported
- unsupported login behavior is reported clearly if Telegram is status-only in the CLI surface

**Step 2: Run tests to verify they fail**

Run: `go test ./cmd/smolbot -run 'Test.*Channel.*Telegram|Test.*Telegram.*Status|Test.*Telegram.*Login'`
Expected: FAIL because the CLI surfaces do not yet mention Telegram.

**Step 3: Write minimal implementation**

- Extend channel lists/status formatting to include Telegram
- If Telegram supports `InteractiveLoginHandler`, route through it
- If not, keep messaging explicit and non-misleading

**Step 4: Run tests to verify they pass**

Run: `go test ./cmd/smolbot -run 'Test.*Channel.*Telegram|Test.*Telegram.*Status|Test.*Telegram.*Login'`
Expected: PASS

**Step 5: Commit**

```bash
git add cmd/smolbot/channels.go cmd/smolbot/channels_login.go cmd/smolbot/channels_status.go cmd/smolbot/*test.go
git commit -m "feat(cli): expose telegram channel surfaces"
```

**Gate:** Code-quality review must confirm no misleading login/status semantics before plan closeout.

---

### Task 6: Final Telegram QA and closure

**Files:**
- Verify touched files only

**Step 1: Run focused verification**

Run:

```bash
go test ./pkg/channel/... ./pkg/config/...
go test ./cmd/smolbot -run 'Test.*Telegram|Test.*Channel'
go test ./cmd/installer -run 'Test.*Telegram'
```

Expected: PASS

**Step 2: Run broader compilation check**

Run: `go build ./cmd/smolbot ./cmd/installer`
Expected: PASS

**Step 3: Run review gates**

- Spec-compliance review for Telegram recovery and integration
- Code-quality review for Telegram recovery and integration

Expected: both return no blocking findings

**Step 4: Commit any final QA fixes**

```bash
git add <touched-files>
git commit -m "test(telegram): close recovery QA findings"
```

**Step 5: Completion criteria**

- Telegram compiles
- Telegram adapter is tested
- shared chunk helper is actually used
- runtime and installer are wired
- CLI/status surfaces are consistent
- spec review = good
- code quality review = good

