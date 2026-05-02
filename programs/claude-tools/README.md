# claude-tools

chezmoi-managed Go binaries that replace the shell scripts originally under
`dot_local/bin/executable_claude-*.sh`, `dot_config/tmux/scripts/cockpit/executable_*.sh`,
and `dot_config/tmux/scripts/executable_*.sh`. As of G-2 only
`dot_config/tmux/scripts/executable_tpm-bootstrap.sh` remains as shell (it
manages tmux itself, out of scope).

## Binaries

| Binary | Backed slash command / hook | Notes |
|---|---|---|
| `claude-cockpit-state` | `hooks.UserPromptSubmit` / `PreToolUse` / `Notification` / `Stop` / `SessionEnd` | per-pane status writer |
| `claude-cockpit-prune` / `-summary` / `-next-ready` / `-switcher` | `prefix + C` ⇒ `s` / `N` / status-right | cockpit fzf + state aggregation |
| `claude-notify-hook` / `-sound` / `-dispatch` / `claude-notifyd` | `hooks.Notification` / `Stop` | popup pipeline + resident daemon (G-1.next #3) |
| `claude-tmux-new` | `/branch-out` slash command, `prefix + C` ⇒ `n` / `o` | spawn worktree + window + child claude |
| `claude-pick-branch` | `prefix + C` ⇒ `n` / `o` | fzf branch picker → `claude-tmux-new` |
| `claude-respawn-pane` | `prefix + C` ⇒ `r` | restart claude pane |
| `claude-kill-session` | `/branch-finish` slash command, `prefix + C` ⇒ `k` | tear down worktree + window |
| `claude-branch-merge` | `/branch-merge` slash command | rebase + merge current worktree into target (squash optional) |
| `claude-branch` | tmux status-right `[branch]` | git branch reader |

The slash commands listed above live in
[`programs/claude-plugins/plugins/kkiyama117-flow-tools/commands/`](../claude-plugins/plugins/kkiyama117-flow-tools/commands/);
see [`docs/manage_claude.md`](../../docs/manage_claude.md) §5.8.

## Layout

- `cmd/<name>/main.go` — thin entry points (1 binary per former shell script)
- `internal/{cockpit,xdg,atomicfile,proc,obslog,notify,notifyd,gitwt,tmux}` — 共有パッケージ

## Build & Deploy

Distributed via chezmoi's `run_onchange_after_build-claude-tools.sh.tmpl`:

    chezmoi apply  # rebuilds binaries to ~/.local/bin/ when source changes

To build manually:

    go build -trimpath -ldflags="-s -w" -o ~/.local/bin/ ./cmd/...

## Test

    go test ./...

## Spec

See `docs/superpowers/specs/2026-05-01-shell-to-go-migration-design.md`
in the chezmoi repo root.

## claude-notifyd

`cmd/claude-notifyd/` is a long-lived resident daemon (G-1.next #3, 2026-05-02)
that consolidates popup state for all concurrent Claude Code sessions into a
single process. It replaces the per-popup `claude-notify-dispatch` fork model
with a Unix socket + D-Bus architecture where one daemon holds all in-flight
notification IDs and handles `ActionInvoked` / `NotificationClosed` signals.

**Socket path:** `${XDG_RUNTIME_DIR}/claude-notify/sock` (mode 0600).
The helper `notify.SocketPath()` in `internal/notify/socket.go` returns this
path with `/tmp/claude-notify/sock` as fallback when `XDG_RUNTIME_DIR` is unset.

**Hook integration:** `claude-notify-hook` attempts a 100 ms dial to the
daemon socket before falling back to the legacy `claude-notify-dispatch` fork.
On success it writes a single newline-delimited JSON frame
(`internal/notifyd/protocol.go` `Frame{V:1, Op:"show", ...}`) and closes the
connection. The hook always exits 0 regardless of dial outcome.

**Fallback behaviour:** When the daemon is down, the socket path is absent, or
the dial exceeds 100 ms, `claude-notify-hook` transparently falls back to
`setsid fork claude-notify-dispatch`. Popup delivery is never silently dropped.
The fallback binary is kept as a warm spare; deprecation is a future concern.

**systemd units:**
- `dot_config/systemd/user/claude-notifyd.socket` — `ListenStream=%t/claude-notify/sock`, socket activation entry point
- `dot_config/systemd/user/claude-notifyd.service` — `Type=notify`, `Restart=on-failure`, sandbox hardening
- `.chezmoiscripts/run_onchange_after_enable-claude-notifyd.sh.tmpl` — idempotent bootstrap (`daemon-reload` + `enable --now claude-notifyd.socket`)

Design spec: `docs/superpowers/specs/2026-05-02-notify-dispatch-daemon-design.md`.
