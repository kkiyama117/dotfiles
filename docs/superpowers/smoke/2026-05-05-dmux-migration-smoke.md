# dmux Migration — Manual Smoke Checklist (2026-05-05)

Spec: `docs/superpowers/specs/2026-05-05-dmux-migration-design.md` §9.4
Plan: `docs/superpowers/plans/2026-05-05-dmux-migration.md`

## Preconditions

- Active session inside tmux (any session)
- Working tree of chezmoi repo at `~/.local/share/chezmoi/` is clean (or only
  this plan's commits applied)
- `claude` CLI is on PATH
- `dmux` is optional — items 1-5 must work without it

## Items

### 1. `/branch-out` creates worktree at the dmux-compatible path

```
/branch-out "test dmux migration smoke"
```

Expected:
- A new tmux window is spawned and focused
- `git worktree list` (run in the new window's left pane) shows
  `~/.local/share/chezmoi/.dmux/worktrees/feat-test-dmux-migration-smoke`
  as a worktree
- The new pane runs `claude` and the message `test dmux migration smoke` is
  visible in the prompt area (pre-fill)

### 2. `.gitignore` has `.dmux/` appended automatically

```
grep -nE '^\.dmux/?$' ~/.local/share/chezmoi/.gitignore
```

Expected: 1 match. The change is uncommitted (user will commit when ready).

### 3. Pre-fill prompt is exact

In the new pane (claude), the first message displayed must be exactly
`test dmux migration smoke` — not a quoted variant, not truncated.

### 4. `/branch-merge main fetch` integrates back

In the spawned pane:
```
/branch-merge main fetch
```

Expected:
- `claude-branch-merge` outputs `merged feat/test-dmux-migration-smoke into main at /home/kiyama/.local/share/chezmoi`
- `git -C ~/.local/share/chezmoi log --oneline -1` shows the merged commit on `main`

### 5. `/branch-finish` removes worktree + window

In the spawned pane:
```
/branch-finish
```

Expected:
- The tmux window disappears (focus returns to the previous window)
- `git worktree list` no longer lists the spawned worktree
- `~/.local/share/chezmoi/.dmux/worktrees/feat-test-dmux-migration-smoke/` no
  longer exists

### 6. `dmux` UI sees existing worktrees (failure tolerated)

```
cd ~/.local/share/chezmoi
/branch-out "test dmux ui pickup"
# In a separate terminal:
cd ~/.local/share/chezmoi
dmux
```

Expected:
- The dmux TUI lists `feat-test-dmux-ui-pickup` as a manageable pane
- If it does not, file a follow-up under spec §11 D-1 / D-6 — but this is not
  a blocker for accepting the migration

After verifying, run `/branch-finish` from the spawned pane.

## Sign-off

When items 1-5 PASS, append to this file:

```
<!-- Smoke verified by kiyama on YYYY-MM-DD; item 6 result: PASS / FAIL / N/A -->
```
