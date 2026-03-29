# MiniMax OAuth Implementation - Corrected Handover

This handover supersedes the earlier stale version that claimed OAuth work had not started.

## Branch And Worktree

- Branch: `feat/provider-ux-minimax`
- Worktree: `/home/nomadx/Documents/smolbot/.worktrees/provider-ux-minimax`

Do not continue this work from the root checkout.

## What Is Already True

Provider UX and MiniMax API-key work are already in place on this branch. Relevant recent commits:

- `a8f6937 feat(provider): expand model discovery across providers`
- `24a571e feat(tui): upgrade model picker workflow`
- `84ee4d9 feat(tui): improve provider detail dialog`
- `ab89b1a feat(provider): add minimax configuration support`
- `3b601ce test(provider): add end-to-end provider regressions`

OAuth work has also already started:

- `8b52282 feat(provider): add OAuth type definitions`
- `172f84e feat(provider): add PKCE utilities`
- `e9d96cc docs: comprehensive oauth implementation handover`

Current local branch state may also include an untracked spike:

- `pkg/provider/minimax_oauth.go`

Treat that file as exploratory unless it has been committed and reviewed.

## Why The Earlier Handover Was Wrong

The old handover incorrectly claimed:

- OAuth tasks completed: `0`
- committed OAuth files: none
- MiniMax OAuth should be implemented as a generic `device_code` flow using `device_code`

That is now known to be wrong in three ways:

1. Types and PKCE utilities are already committed.
2. MiniMax OAuth in the OpenClaw reference uses a PKCE-backed **user-code/device-style** flow, not the generic `device_code` grant assumed by the spike.
3. The correct product shape is to keep API-key MiniMax and Token Plan OAuth MiniMax as separate provider identities.

## Correct Design Direction

The authoritative plan is now:

- `docs/plans/2026-03-29-minimax-oauth-implementation.md`

The important design decisions are:

1. `minimax` remains the API-key provider.
2. `minimax-portal` is the new OAuth/Token Plan provider identity.
3. Backend comes first:
   - config/auth metadata
   - token store
   - MiniMax auth client
   - registry/runtime wiring
4. Installer and TUI follow only after backend correctness is established.
5. `MiniMax-M2.7-highspeed` must not become the default OAuth model.

## Known Pitfalls To Avoid

1. Do not overload `providers["minimax"]` with both API key and OAuth semantics.
2. Do not use `device_code` in token polling just because it is a familiar RFC shape.
3. Do not set token expiry from auth-code expiry.
4. Do not assume `/oauth/revoke` exists or works without evidence.
5. Do not store OAuth access tokens directly in the main config JSON.
6. Do not start with installer/TUI work before backend tests are green.

## Files That Already Exist

Committed and usable:

- `pkg/provider/oauth.go`
- `pkg/provider/oauth_test.go`
- `pkg/provider/pkce.go`
- `pkg/provider/pkce_test.go`

Existing surrounding integration points:

- `pkg/config/config.go`
- `pkg/provider/registry.go`
- `pkg/provider/discovery.go`
- `cmd/installer/types.go`
- `cmd/installer/tasks.go`
- `internal/components/dialog/providers.go`
- `internal/components/dialog/models.go`

Possibly present as local-only spike:

- `pkg/provider/minimax_oauth.go`

## First Commands To Run

```bash
git -C /home/nomadx/Documents/smolbot/.worktrees/provider-ux-minimax status --short
git -C /home/nomadx/Documents/smolbot/.worktrees/provider-ux-minimax log --oneline -12
go test ./pkg/provider ./pkg/config ./cmd/installer
```

## Success Criteria

This OAuth slice is only complete when all of the following are true:

- MiniMax OAuth login works via a correct PKCE-backed user-code flow
- `minimax-portal` resolves as a distinct provider from `minimax`
- tokens persist outside main config and refresh correctly
- installer can configure MiniMax OAuth cleanly
- provider UI shows auth mode/status accurately
- model picker avoids obviously invalid Token Plan defaults
- focused tests pass
- build passes
- final spec review passes
- final code-quality review passes
