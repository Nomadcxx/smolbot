# TUI Port Implementation TODO

## 🔴 GATE 0: Foundation Layer (BLOCKING) - ✅ COMPLETE

- [x] **Task 0.1:** Create internal/client/types.go with UsageInfo, ChannelStatus, StatusPayload
- [x] **Task 0.2:** Update Status() and ModelsSet() signatures in protocol.go and messages.go
- [x] **Task 0.3:** Add 20 new color fields to Theme struct using image/color.Color

## 🟡 GATE 1: Theme System (PARALLEL) - ✅ COMPLETE

- [x] **Slice 1.1:** Add darkenHex, themeOption support to register.go
- [x] **Slice 1.2:** Add slices.Sort to theme manager List() method
- [x] **Slice 1.3:** Add new color fields to 7 standard themes
- [x] **Slice 1.4:** Add monochrome, rama, tokyo-night with custom colors

## 🟡 GATE 2: Foundation Components (PARALLEL) - 🔄 IN PROGRESS

- [ ] **Slice 2.1:** Create common.go with visibleBounds, matchesQuery, hasWordPrefix
- [ ] **Slice 2.2:** Create footer.go with Height() method and usage display
- [ ] **Slice 2.3:** Add colorHex, subtleWash, transcriptRoleAccent to message.go

## 🟡 GATE 3: Component Enhancements (PARALLEL)

- [ ] **Slice 3.1:** Add SetCompact and Height methods to Header
- [ ] **Slice 3.2:** Add SetCompact, SetShowHint, Height to Editor
- [ ] **Slice 3.3:** Add windowing, vim keys (j/k, Ctrl+n/p) to all dialogs
- [ ] **Slice 3.4:** Add renderRoleBlock function with themed colors
- [ ] **Slice 3.5:** Create test files for commands, models, sessions dialogs

## 🔴 GATE 4: Menu Dialog (BLOCKING)

- [ ] **Task 4.1:** Create menu_dialog.go with cursor preservation per page

## 🔴 GATE 5: Integration (BLOCKING)

- [ ] **Task 5.1:** Integrate footer, menu, compact mode in tui.go

## 🟢 GATE 6: Verification (PARALLEL)

- [ ] **Slice 6.1:** Verify go test ./internal/... passes
- [ ] **Slice 6.2:** Build smolbot-tui binary
- [ ] **Slice 6.3:** Commit all changes and push to main
