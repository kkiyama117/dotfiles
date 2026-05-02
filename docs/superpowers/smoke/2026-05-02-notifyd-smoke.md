# `claude-notifyd` smoke (G-1.next #3 / PR-D4)

**Date:** 2026-05-02
**Branch:** `feat/notify-dispatch-daemon` @ `dd68813`
**Host:** Manjaro (Linux 7.0.3-1-MANJARO), systemd --user, wired-notify, X11+Wayland
**Method:** Manual deploy (skipped `chezmoi apply` because Bitwarden was locked — units are literal files, equivalent result)

## Pre-state
- `~/.local/bin/claude-notifyd` did not exist (build script had not run)
- `~/.config/systemd/user/claude-notifyd.{socket,service}` did not exist
- `$XDG_RUNTIME_DIR/claude-notify/sessions/` already contained 1 active session id from current shell

## Steps

### 1. Build + install
```
cd programs/claude-tools
go build -o ~/.local/bin/claude-notifyd ./cmd/claude-notifyd
cp dot_config/systemd/user/claude-notifyd.{socket,service} ~/.config/systemd/user/
systemctl --user daemon-reload
systemctl --user enable --now claude-notifyd.socket
```
- Binary: 5.3 MiB.
- Symlink: `sockets.target.wants/claude-notifyd.socket` created.
- `is-active claude-notifyd.socket` → **active**.
- `is-active claude-notifyd.service` → **inactive** (waiting for socket activation).

### 2. Socket activation (cold start)
```
python3 -c '... AF_UNIX SOCK_STREAM connect $XDG_RUNTIME_DIR/claude-notify/sock; sendall {Show v=1 sid=smoke-d4 ...}\n'
```
- `is-active claude-notifyd.service` → **active** (auto-started by socket activation).
- `cat sessions/smoke-d4.id` → **3** (notif_id assigned by wired-notify via D-Bus Notify).
- Journal:
  - `Starting Claude Code notifyd daemon (G-1.next #3)...`
  - `state warm-start ... loaded=1`
  - `Started Claude Code notifyd daemon (G-1.next #3).`

### 3. Replace flow (in-place update)
```
# 3 sends in succession with same sid=smoke-d4, varying title/body
```
- `sessions/smoke-d4.id` remained **3** across all sends → wired-notify popup updated in place via `replaces_id=3`.

### 4. Restart=on-failure
```
kill -9 $(systemctl --user show claude-notifyd.service --property=MainPID --value)   # PID 122596
sleep 13
```
- Journal:
  - `Main process exited, code=killed, status=9/KILL`
  - `Failed with result 'signal'`
  - `Scheduled restart job, restart counter is at 1`
  - `Started Claude Code notifyd daemon (G-1.next #3).`
- New `MainPID` = **128207** (was 122596).
- `is-active` → **active**.
- Warm-start picked up 2 sessions (smoke-d4 + the existing one).

### 5. Post-restart frame
```
# fresh sid: smoke-after-restart
```
- `sessions/smoke-after-restart.id` → **4** (wired-notify assigned next id).
- Daemon still active.

### 6. Malformed frame resilience
```
sendall '{this is not valid json\n'
```
- Daemon logged WARN: `frame unmarshal error err="invalid character 't' looking for beginning of object key string"`.
- Connection dropped, daemon stayed **active**.

## Final state
- `claude-notifyd.socket`: `active (running)` since 20:23:13.
- `claude-notifyd.service`: `active (running)` since 20:24:31 (post-restart). Memory peak 7.8 MiB / current 4.9 MiB. Tasks: 9.
- Smoke session ids cleaned from `sessions/`. Only the active shell's session id + `notifyd.lock` remain.

## Acceptance criteria check
- [x] `chezmoi apply` (or equivalent) → `claude-notifyd.socket` active. **PASS** (manual install).
- [x] socket → daemon → wired-notify popup → state file write end-to-end. **PASS** (id=3 written, popup observed by user).
- [x] Disk format byte-equivalent to PR-9. **PASS** (file content is decimal id + `\n`, no schema change).
- [x] Daemon survives malformed frame. **PASS**.
- [x] `Restart=on-failure` recovers daemon within RestartSec=10. **PASS**.

## Deferred (not in this smoke)
- **Hook → daemon dial path** end-to-end (would require deploying the new Go `claude-notify-hook` to `~/.local/bin/`, which overwrites the user's current shell-script hook). Verified instead by `cmd/claude-notify-hook` unit tests (3 dialDaemon cases + 2 notifyHookWithDial cases).
- **Daemon down → fallback dispatch** end-to-end (same reason — needs new Go hook deployed). Verified by `notifyHookWithDial` unit test asserting `forkDispatchFn` is invoked when dial fails.
- **`chezmoi apply` smoke** (Bitwarden was locked at smoke time). User to run `bw_session && chezmoi diff && chezmoi apply` themselves; the run_onchange script will call `daemon-reload + enable --now claude-notifyd.socket`, which is idempotent vs. the current state (already enabled).

## 1-week observation hook (Task 5.4)
After running `chezmoi apply` for full integration, observe for 7 days:
```
journalctl --user -t claude-notify-dispatch --since '7 days ago'
```
If 0 fallback invocations → daemon is stable → consider deprecating legacy `claude-notify-dispatch` in a follow-up plan.
