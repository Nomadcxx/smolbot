# DCP Compliance Repair Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Make the DCP integration production-ready by fixing the critical correctness bugs and closing the highest-impact spec gaps without widening scope beyond the existing design.

**Architecture:** Keep DCP additive and config-gated. Repair the state model so message refs remain stable across transformed views, validate compression ranges against tool-call/result protocol boundaries, keep DCP runtime state scoped to live DCP sessions, and ensure the deferred `compress` tool operates against the current transformed conversation instead of stale or synthetic assumptions.

**Tech Stack:** Go, SQLite, existing smolbot agent loop/tool registry/session store, `pkg/agent/compression/dcp`

---

### Task 1: Lock Down Critical Failing Behaviors With Tests

**Files:**
- Modify: `pkg/agent/compression/dcp/pipeline_test.go`
- Modify: `pkg/agent/compression/dcp/prune_test.go`
- Modify: `pkg/agent/compression/dcp/compress_tool_test.go`
- Modify: `pkg/agent/loop_dcp_test.go`

**Step 1: Write the failing tests**

Add tests for:
- stable refs across repeated `Transform()` calls after a block has been applied
- rejecting compression ranges that split assistant tool calls from matching tool results
- clearing DCP state on `/new`
- `compress` tool computing non-zero token savings from real covered messages
- DCP-only tool visibility not leaking into legacy mode after a DCP request

**Step 2: Run the focused tests to verify they fail**

Run: `go test -count=1 -run 'TestTransformStableRefsAfterPrune|TestApplyBlocksPreservesToolPairs|TestCompressToolRejectsBrokenToolPairRange|TestNewClearsDCPState|TestCompressToolComputesTokenSavings|TestCompressToolDoesNotLeakIntoLegacyMode' ./pkg/agent/... ./pkg/agent/compression/dcp/...`

Expected: FAIL with the current implementation for the new behaviors above.

**Step 3: Keep test names behavior-specific**

Avoid asserting implementation details like internal map shapes. Assert on user-visible/runtime-visible behavior:
- same refs remain attached to the same logical messages
- invalid ranges return structured tool errors
- `/new` leaves no DCP state for that session
- legacy requests cannot discover `compress`

**Step 4: Commit**

```bash
git add pkg/agent/compression/dcp/pipeline_test.go pkg/agent/compression/dcp/prune_test.go pkg/agent/compression/dcp/compress_tool_test.go pkg/agent/loop_dcp_test.go
git commit -m "test(dcp): capture compliance-critical regressions"
```

### Task 2: Make Message Refs Stable Across Pruned Views

**Files:**
- Modify: `pkg/agent/compression/dcp/state.go`
- Modify: `pkg/agent/compression/dcp/message_ids.go`
- Modify: `pkg/agent/compression/dcp/pipeline.go`
- Test: `pkg/agent/compression/dcp/pipeline_test.go`

**Step 1: Write the minimal failing ref-stability test if Task 1 didn’t already cover the exact scenario**

Test scenario:
1. Seed messages and assign refs
2. Create a block covering a middle range
3. Transform twice
4. Verify the same logical uncompressed messages still expose the same `mNNNN` refs across both transforms

**Step 2: Run only that test and verify it fails**

Run: `go test -count=1 -run 'TestTransformStableRefsAfterPrune' ./pkg/agent/compression/dcp/...`

Expected: FAIL because refs are currently keyed by transformed slice index.

**Step 3: Implement the minimal fix**

Refactor state tracking so message refs are anchored to stable history positions or another durable identity derived from the original session history, not post-prune slice indexes.

Requirements:
- `AssignMessageIDs()` must remain idempotent
- system messages still receive no tag
- protected messages still surface `PROTECTED`
- pruning must not cause `mNNNN` reuse for different logical messages

**Step 4: Run the ref tests**

Run: `go test -count=1 -run 'TestAssignMessageIDs|TestTransformStableRefsAfterPrune' ./pkg/agent/compression/dcp/...`

Expected: PASS

**Step 5: Commit**

```bash
git add pkg/agent/compression/dcp/state.go pkg/agent/compression/dcp/message_ids.go pkg/agent/compression/dcp/pipeline.go pkg/agent/compression/dcp/pipeline_test.go
git commit -m "fix(dcp): stabilize message refs across pruning"
```

### Task 3: Enforce Tool-Call/Result Pair Integrity For Blocks and Compression Ranges

**Files:**
- Modify: `pkg/agent/compression/dcp/blocks.go`
- Modify: `pkg/agent/compression/dcp/prune.go`
- Modify: `pkg/agent/compression/dcp/compress_tool.go`
- Modify: `pkg/agent/compression/dcp/testutil_test.go`
- Test: `pkg/agent/compression/dcp/prune_test.go`
- Test: `pkg/agent/compression/dcp/compress_tool_test.go`
- Test: `pkg/agent/compression/dcp/dcp_integration_test.go`

**Step 1: Write failing tests for pair integrity**

Add tests that prove:
- block creation rejects ranges that split a tool call/result pair
- `compress` rejects such ranges with a structured tool error
- pruned output still has every tool result paired with a preceding assistant tool call

**Step 2: Run those tests and verify they fail**

Run: `go test -count=1 -run 'TestApplyBlocksPreservesToolPairs|TestCompressToolRejectsBrokenToolPairRange|TestDCPIntegration_ToolCallResultPairIntegrity' ./pkg/agent/compression/dcp/...`

Expected: FAIL

**Step 3: Implement the minimal validation**

Add explicit helpers that:
- map tool calls to matching tool results
- detect whether a range cuts through a protocol pair
- are used consistently by block creation and compress-tool execution

Do not rely on turn protection alone; validate the actual range.

**Step 4: Re-run the integrity tests**

Run: `go test -count=1 -run 'TestApplyBlocksPreservesToolPairs|TestCompressToolRejectsBrokenToolPairRange|TestDCPIntegration_ToolCallResultPairIntegrity' ./pkg/agent/compression/dcp/...`

Expected: PASS

**Step 5: Commit**

```bash
git add pkg/agent/compression/dcp/blocks.go pkg/agent/compression/dcp/prune.go pkg/agent/compression/dcp/compress_tool.go pkg/agent/compression/dcp/testutil_test.go pkg/agent/compression/dcp/prune_test.go pkg/agent/compression/dcp/compress_tool_test.go pkg/agent/compression/dcp/dcp_integration_test.go
git commit -m "fix(dcp): preserve tool call and result integrity"
```

### Task 4: Make the Compress Tool Operate on Real Runtime Context

**Files:**
- Modify: `pkg/agent/compression/dcp/compress_tool.go`
- Modify: `pkg/agent/loop.go`
- Test: `pkg/agent/compression/dcp/compress_tool_test.go`
- Test: `pkg/agent/loop_dcp_test.go`

**Step 1: Write the failing runtime-context tests**

Add tests for:
- non-zero token savings when compressing a real covered range
- nested compression consuming existing active blocks correctly
- no tool leakage into legacy mode after a prior DCP request

**Step 2: Run the focused tests and verify they fail**

Run: `go test -count=1 -run 'TestCompressToolComputesTokenSavings|TestCompressToolNestedCompression|TestCompressToolDoesNotLeakIntoLegacyMode' ./pkg/agent/... ./pkg/agent/compression/dcp/...`

Expected: FAIL

**Step 3: Implement the minimal fix**

Requirements:
- the `compress` tool must validate and calculate against the current DCP-visible conversation, not a nil `messages` field
- tool registration must be scoped so legacy sessions do not retain discoverable `compress`
- nested block consumption must deactivate superseded blocks deterministically

Prefer wiring a per-request/per-run tool instance or equivalent scoping rather than mutating the global registry permanently.

**Step 4: Re-run the focused tests**

Run: `go test -count=1 -run 'TestCompressToolComputesTokenSavings|TestCompressToolNestedCompression|TestCompressToolDoesNotLeakIntoLegacyMode|TestLoopDCPRegistersCompressTool' ./pkg/agent/... ./pkg/agent/compression/dcp/...`

Expected: PASS

**Step 5: Commit**

```bash
git add pkg/agent/compression/dcp/compress_tool.go pkg/agent/loop.go pkg/agent/compression/dcp/compress_tool_test.go pkg/agent/loop_dcp_test.go
git commit -m "fix(dcp): scope compress tool to live runtime context"
```

### Task 5: Clean Up DCP Session Lifecycle

**Files:**
- Modify: `pkg/agent/loop.go`
- Test: `pkg/agent/loop_dcp_test.go`

**Step 1: Write the failing lifecycle test if not already added**

Add a test that:
1. runs a DCP session
2. confirms DCP state exists
3. invokes `/new`
4. confirms both message history and DCP state are cleared for that session

**Step 2: Run the lifecycle test to verify it fails**

Run: `go test -count=1 -run 'TestNewClearsDCPState' ./pkg/agent`

Expected: FAIL

**Step 3: Implement the minimal fix**

On `/new`, clear DCP state for that session when the DCP state manager exists or can be initialized.

Do not clear unrelated sessions.

**Step 4: Re-run the lifecycle test**

Run: `go test -count=1 -run 'TestNewClearsDCPState' ./pkg/agent`

Expected: PASS

**Step 5: Commit**

```bash
git add pkg/agent/loop.go pkg/agent/loop_dcp_test.go
git commit -m "fix(dcp): clear session state on new conversation"
```

### Task 6: Fill the Highest-Value Missing Spec Coverage

**Files:**
- Modify: `pkg/agent/compression/dcp/dcp_integration_test.go`
- Modify: `pkg/agent/compression/dcp/compress_tool_test.go`
- Modify: `pkg/agent/compression/dcp/nudge_test.go`

**Step 1: Add missing tests from the plan that cover the repaired behavior**

Minimum set:
- overlapping ranges
- nested compression
- nudge no-double-nudge behavior
- mode switch regression
- end-to-end transform → compress → transform flow

**Step 2: Run the focused test subset**

Run: `go test -count=1 -run 'TestCompressTool_OverlappingRanges|TestCompressTool_NestedCompression|TestNudge_NoDoubleNudge|TestDCPIntegration_CompressToolEndToEnd|TestDCPIntegration_ModeSwitch' ./pkg/agent/... ./pkg/agent/compression/dcp/...`

Expected: PASS after implementation

**Step 3: Commit**

```bash
git add pkg/agent/compression/dcp/dcp_integration_test.go pkg/agent/compression/dcp/compress_tool_test.go pkg/agent/compression/dcp/nudge_test.go
git commit -m "test(dcp): add missing compliance coverage"
```

### Task 7: Final Verification

**Files:**
- Verify only

**Step 1: Run DCP package tests**

Run: `go test -count=1 ./pkg/agent/compression/dcp/...`

Expected: PASS

**Step 2: Run DCP race test**

Run: `go test -race ./pkg/agent/compression/dcp/...`

Expected: PASS

**Step 3: Run focused agent regression tests**

Run: `go test -count=1 -run 'TestLoopDCP|TestLoopDefaultEngineIsLegacy|TestNewClearsDCPState' ./pkg/agent`

Expected: PASS

**Step 4: Build the whole project**

Run: `go build ./...`

Expected: PASS

**Step 5: Run broader agent/config checks**

Run: `go test -count=1 ./pkg/config/... ./pkg/agent/compression/...`

Expected: PASS

**Step 6: Run full project tests and record residual failures exactly**

Run: `go test ./...`

Expected: DCP-related packages stay green. If unrelated existing failures remain, record them explicitly and do not claim the repo is fully green.

