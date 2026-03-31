---
name: git-operations
description: Common git workflows: committing, branching, rebasing, conflict resolution, undoing mistakes, PR hygiene. Use when asked to perform git operations or when about to commit/push code.
always: false
---

## Git Operations

Follow consistent workflows. Always check `git diff --staged` before committing.

### Commit Message Format

Use conventional commits: `type(scope): subject`

Types: `feat`, `fix`, `docs`, `refactor`, `test`, `chore`, `build`, `ci`

Subject: imperative mood, no period, max 72 chars. Body: wrap at 72 chars, explain why not what.

### Amend vs New Commit

Amend only if the commit has not been pushed to a shared branch. After push, always create a new commit. Amending pushed commits forces others to rebase.

### Branching Strategy

Feature branches: `feat/<slug>`. Bug fix branches: `fix/<slug>`. Keep branches short-lived. Merge or rebase into main frequently. Delete branches after merge.

### Rebase vs Merge

Rebase for keeping a clean linear history on feature branches before PR. Merge for integrating into main (preserves topology). Never rebase a shared branch that others have checked out.

### Resolving Conflicts

Read both sides before choosing. `ours` means the current branch; `theirs` means the incoming branch. If unsure which is correct, ask — do not guess. After resolving, `git add` the file and continue the rebase or merge.

### Undoing Mistakes

- `git restore <file>`: discard unstaged changes
- `git reset HEAD <file>`: unstage without discarding
- `git reset --soft HEAD~1`: undo last commit, keep changes staged
- `git reset --hard` destroys work — confirm before using
- `git revert <hash>`: safe undo for pushed commits

### Stash Usage

`git stash` when you need to switch context mid-work. `git stash pop` to restore. Name stashes: `git stash save "wip: description"`. List with `git stash list`.

### Cherry-Pick

Use to port a specific commit to another branch: `git cherry-pick <hash>`. Use sparingly; prefer rebase for sequential commits. If the cherry-pick produces conflicts, resolve them the same way as merge conflicts.

### PR Hygiene

Squash noise commits before opening a PR. Ensure the PR description explains the why, not just the what. Link to the issue. Keep PRs small — one concern per PR.

### Check Before Committing

Always run `git diff --staged` before `git commit` to confirm exactly what is going into the commit.