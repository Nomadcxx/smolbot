# Signal Login And Discord Completion Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Finish the remaining channel work by implementing the pending Signal login TUI flow, wiring QR rendering into the real login experience, and adding Discord end-to-end support through config, runtime, installer, and CLI/status surfaces.

**Architecture:** Keep Signal login inside the existing channel login command flow, using the already-present Signal adapter and the shared QR renderer rather than inventing a parallel UX. Add Discord as another `pkg/channel` adapter with the same runtime/installer/config pattern used by Telegram and the existing channels. Keep each channel independently testable, then finish with cross-channel integration checks.

**Tech Stack:** Go, Cobra CLI, existing `pkg/channel` abstractions, installer Bubble Tea flow, shared QR rendering helper

---

### Task 1: Define the Signal login UX in tests

**Files:**
- Create: `cmd/smolbot/channels_signal_login.go`
- Test: `cmd/smolbot/channels_login_test.go`
- Test: `cmd/smolbot/channels_status.go` adjacent tests if needed

**Step 1: Write the failing tests**

Add tests that describe the intended Signal login flow:
- invoking Signal login uses the adapter’s interactive login path
- provisioning URI updates are rendered to the user
- QR output is shown via the shared renderer
- cancellation exits cleanly

Use a fake interactive login handler so tests stay deterministic.

**Step 2: Run tests to verify they fail**

Run: `go test ./cmd/smolbot -run 'Test.*Signal.*Login'`
Expected: FAIL because the dedicated Signal login flow file/path does not exist yet.

**Step 3: Write minimal implementation**

- Add `channels_signal_login.go`
- Route Signal channel login through the existing CLI command stack
- Render provisioning updates and QR output without logging secrets unnecessarily

**Step 4: Run tests to verify they pass**

Run: `go test ./cmd/smolbot -run 'Test.*Signal.*Login'`
Expected: PASS

**Step 5: Commit**

```bash
git add cmd/smolbot/channels_signal_login.go cmd/smolbot/channels_login_test.go
git commit -m "feat(signal): add channel login flow"
```

**Gate:** Spec review must confirm the UX matches the intended Signal login flow before moving on.

---

### Task 2: Integrate the shared QR renderer into real Signal login

**Files:**
- Modify: `pkg/channel/qr/renderer.go`
- Modify: `cmd/smolbot/channels_signal_login.go`
- Test: `pkg/channel/qr/renderer_test.go`
- Test: `cmd/smolbot/channels_login_test.go`

**Step 1: Write the failing tests**

Add tests that verify:
- QR renderer returns stable, non-empty output for valid provisioning URIs
- Signal login uses the shared QR renderer instead of inline duplicate logic
- empty provisioning URIs do not render bogus output

**Step 2: Run tests to verify they fail**

Run: `go test ./pkg/channel/qr ./cmd/smolbot -run 'Test.*QR|Test.*Signal.*Login'`
Expected: FAIL because the renderer is not yet integrated into the login flow and may lack direct tests.

**Step 3: Write minimal implementation**

- Add renderer tests
- Use `pkg/channel/qr` from the Signal login path
- Keep QR data out of logs and only show it in the interactive output path

**Step 4: Run tests to verify they pass**

Run: `go test ./pkg/channel/qr ./cmd/smolbot -run 'Test.*QR|Test.*Signal.*Login'`
Expected: PASS

**Step 5: Commit**

```bash
git add pkg/channel/qr/renderer.go pkg/channel/qr/renderer_test.go cmd/smolbot/channels_signal_login.go cmd/smolbot/channels_login_test.go
git commit -m "feat(signal): render provisioning QR in login flow"
```

**Gate:** Code-quality review must confirm the login flow does not leak QR/provisioning secrets via logs.

---

### Task 3: Build the Discord adapter with tests

**Files:**
- Create: `pkg/channel/discord/adapter.go`
- Create: `pkg/channel/discord/adapter_test.go`
- Modify: `pkg/config/config.go`
- Modify: `pkg/config/config_test.go`

**Step 1: Write the failing tests**

Add adapter tests for:
- token loading/validation
- send path
- inbound message handling
- allowlist/channel filtering if supported by the config shape
- status/login behavior

Also add config tests for Discord defaults and load behavior.

**Step 2: Run tests to verify they fail**

Run: `go test ./pkg/channel/discord ./pkg/config -run 'Test.*Discord|TestLoad'`
Expected: FAIL because the Discord adapter/config do not exist yet.

**Step 3: Write minimal implementation**

- Add `DiscordChannelConfig` to config
- Create a Discord adapter that matches the existing `pkg/channel` interfaces
- Keep transport-specific code behind a seam so tests stay offline

**Step 4: Run tests to verify they pass**

Run: `go test ./pkg/channel/discord ./pkg/config -run 'Test.*Discord|TestLoad'`
Expected: PASS

**Step 5: Commit**

```bash
git add pkg/channel/discord/adapter.go pkg/channel/discord/adapter_test.go pkg/config/config.go pkg/config/config_test.go
git commit -m "feat(discord): add channel adapter"
```

**Gate:** Do not wire Discord into runtime until adapter and config tests are green.

---

### Task 4: Wire Discord into runtime and CLI channel surfaces

**Files:**
- Modify: `cmd/smolbot/runtime.go`
- Modify: `cmd/smolbot/channels.go`
- Modify: `cmd/smolbot/channels_login.go`
- Modify: `cmd/smolbot/channels_status.go`
- Test: related tests in `cmd/smolbot/*test.go`

**Step 1: Write the failing tests**

Add tests that verify:
- Discord registers when enabled
- Discord status is visible in channel status output
- Discord login/status behavior is surfaced consistently with the adapter capabilities

**Step 2: Run tests to verify they fail**

Run: `go test ./cmd/smolbot -run 'Test.*Discord|Test.*Channel'`
Expected: FAIL because runtime and CLI surfaces do not yet include Discord.

**Step 3: Write minimal implementation**

- Add a `newDiscordChannel` seam
- Register Discord in runtime
- Extend CLI channel listings/status/login routing

**Step 4: Run tests to verify they pass**

Run: `go test ./cmd/smolbot -run 'Test.*Discord|Test.*Channel'`
Expected: PASS

**Step 5: Commit**

```bash
git add cmd/smolbot/runtime.go cmd/smolbot/channels.go cmd/smolbot/channels_login.go cmd/smolbot/channels_status.go cmd/smolbot/*test.go
git commit -m "feat(runtime): wire discord channel surfaces"
```

**Gate:** Spec review must confirm Discord is exposed everywhere the plan expects.

---

### Task 5: Add Discord installer/onboarding support

**Files:**
- Modify: `cmd/installer/types.go`
- Modify: `cmd/installer/main.go`
- Modify: `cmd/installer/views.go`
- Modify: `cmd/installer/tasks.go`
- Test: installer tests adjacent to touched files

**Step 1: Write the failing tests**

Add tests that verify:
- Discord can be enabled in the installer flow
- Discord token/config fields are emitted correctly
- disabled Discord leaves config clean

**Step 2: Run tests to verify they fail**

Run: `go test ./cmd/installer -run 'Test.*Discord'`
Expected: FAIL because the installer does not yet support Discord.

**Step 3: Write minimal implementation**

- Extend installer model and views
- Generate Discord config in the installer task pipeline
- Keep secret handling file- or env-based where practical

**Step 4: Run tests to verify they pass**

Run: `go test ./cmd/installer -run 'Test.*Discord'`
Expected: PASS

**Step 5: Commit**

```bash
git add cmd/installer/types.go cmd/installer/main.go cmd/installer/views.go cmd/installer/tasks.go
git commit -m "feat(installer): add discord configuration flow"
```

**Gate:** Code-quality review must confirm secret handling and config emission are safe.

---

### Task 6: Cross-channel integration QA

**Files:**
- Verify touched files only

**Step 1: Run focused verification**

Run:

```bash
go test ./pkg/channel/...
go test ./pkg/config/...
go test ./cmd/smolbot -run 'Test.*Signal|Test.*Discord|Test.*Channel'
go test ./cmd/installer -run 'Test.*Signal|Test.*Discord'
```

Expected: PASS

**Step 2: Run broader compilation check**

Run: `go build ./cmd/smolbot ./cmd/installer`
Expected: PASS

**Step 3: Run review gates**

- Spec-compliance review for Signal login + Discord completion
- Code-quality review for Signal login + Discord completion

Expected: both return no blocking findings

**Step 4: Commit any final QA fixes**

```bash
git add <touched-files>
git commit -m "test(channels): close signal and discord QA findings"
```

**Step 5: Completion criteria**

- Signal login UX is implemented and uses the shared QR renderer
- Discord exists end-to-end
- runtime, installer, and CLI surfaces are coherent across channels
- spec review = good
- code quality review = good
