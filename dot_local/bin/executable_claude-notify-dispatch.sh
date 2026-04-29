#!/usr/bin/env bash
# Claude Code popup dispatcher. Lives alongside the libnotify popup,
# reads `notify-send --print-id --wait` output, and on left-click
# ("default" action) focuses the originating tmux pane and dismisses
# the popup explicitly via CloseNotification.
#
# Right-click is wired to `notification_close` in wired.ron and is
# handled entirely by wired -> notify-send returns without an action
# line, and this script no-ops.
#
# Inputs (env, set by claude-notify-hook.sh):
#   CLAUDE_NOTIFY_TITLE / BODY / URGENCY
#   CLAUDE_NOTIFY_SESSION_ID
#   CLAUDE_NOTIFY_TMUX_PANE / TMUX_SESSION
set -euo pipefail

title="${CLAUDE_NOTIFY_TITLE:-Claude Code}"
body="${CLAUDE_NOTIFY_BODY:-}"
urgency="${CLAUDE_NOTIFY_URGENCY:-normal}"
session_id="${CLAUDE_NOTIFY_SESSION_ID:-}"
tmux_pane="${CLAUDE_NOTIFY_TMUX_PANE:-}"
tmux_session="${CLAUDE_NOTIFY_TMUX_SESSION:-}"

command -v notify-send >/dev/null 2>&1 || exit 0

# notify-send --print-id --wait stdout:
#   line 1: notification id (uint32, always)
#   line 2: action key when ActionInvoked fires (e.g. "default")
# Close-without-action -> only line 1 is printed.
mapfile -t lines < <(
  notify-send \
    --app-name=ClaudeCode \
    --urgency="$urgency" \
    --expire-time=0 \
    --action=default=Focus \
    --hint=string:x-claude-session:"${session_id:-unknown}" \
    --print-id \
    --wait \
    -- "$title" "$body" 2>/dev/null
) || exit 0

notif_id="${lines[0]:-}"
action_key="${lines[1]:-}"

# Right-click / closeall / timeout -> nothing more to do.
[[ "$action_key" != "default" ]] && exit 0

# === focus the originating tmux pane ===
if [[ -n "$tmux_pane" && -n "$tmux_session" ]] \
    && command -v tmux >/dev/null 2>&1 \
    && tmux has-session -t "$tmux_session" 2>/dev/null; then
  tmux switch-client -t "$tmux_session" \; select-pane -t "$tmux_pane" \
    >/dev/null 2>&1 || true
else
  # TODO(F-3): bare terminal fallback via wmctrl / swaymsg using cwd
  # TODO(F-3): auto-reopen tmux session if it was killed, then resume claude
  logger -t claude-notify-dispatch \
    "focus skipped: no tmux context (sid=${session_id:-?})" || true
fi

# === auto-dismiss popup (FDO spec doesn't auto-close on ActionInvoked) ===
if [[ -n "$notif_id" ]] && command -v gdbus >/dev/null 2>&1; then
  gdbus call --session \
    --dest=org.freedesktop.Notifications \
    --object-path=/org/freedesktop/Notifications \
    --method=org.freedesktop.Notifications.CloseNotification "$notif_id" \
    >/dev/null 2>&1 || true
fi

exit 0
