#!/usr/bin/env bash
# Claude Code popup dispatcher. Lives alongside the libnotify popup,
# reads `notify-send --print-id --wait` output, and on left-click
# ("default" action — wired.ron maps button 1 to notification_action1)
# focuses the originating tmux pane and dismisses the popup explicitly
# via CloseNotification.
#
# Right-click is wired to `notification_close` in wired.ron and is
# handled entirely by wired -> notify-send returns without an action
# line, and this script no-ops.
#
# Inputs (env, set by claude-notify-hook.sh):
#   CLAUDE_NOTIFY_TITLE / BODY / URGENCY
#   CLAUDE_NOTIFY_SESSION_ID
#   CLAUDE_NOTIFY_TMUX_PANE / TMUX_SESSION
#
# De-dup (F-3.next #1):
#   When CLAUDE_NOTIFY_SESSION_ID is set, the latest notif_id is persisted
#   under "$state_dir/<sid>.id" and replayed as --replace-id on the next
#   invocation. wired (or any FDO-spec-compliant daemon) then updates the
#   existing popup in place instead of stacking a new one per session.
set -euo pipefail

title="${CLAUDE_NOTIFY_TITLE:-Claude Code}"
body="${CLAUDE_NOTIFY_BODY:-}"
urgency="${CLAUDE_NOTIFY_URGENCY:-normal}"
session_id="${CLAUDE_NOTIFY_SESSION_ID:-}"
tmux_pane="${CLAUDE_NOTIFY_TMUX_PANE:-}"
tmux_session="${CLAUDE_NOTIFY_TMUX_SESSION:-}"

command -v notify-send >/dev/null 2>&1 || exit 0

# === de-dup state (per session_id) ===
state_dir="${XDG_RUNTIME_DIR:-/tmp}/claude-notify/sessions"
state_file=""
prev_id=""
if [[ -n "$session_id" ]]; then
  # Filename-safe sid: keep alnum/_/- only, fall back to "unknown".
  sid_safe="${session_id//[^a-zA-Z0-9_-]/_}"
  [[ -z "$sid_safe" ]] && sid_safe="unknown"
  state_file="$state_dir/$sid_safe.id"
  mkdir -p "$state_dir" 2>/dev/null || true
  if [[ -r "$state_file" ]]; then
    prev_id="$(cat "$state_file" 2>/dev/null || true)"
    # Reject anything that is not a positive integer — protects --replace-id.
    [[ "$prev_id" =~ ^[1-9][0-9]*$ ]] || prev_id=""
  fi
fi

replace_args=()
[[ -n "$prev_id" ]] && replace_args=(--replace-id="$prev_id")

# notify-send --print-id --wait stdout:
#   line 1: notification id (uint32, always)
#   line 2: action key when ActionInvoked fires (e.g. "default")
# Close-without-action (incl. being replaced by a newer popup) -> only line 1.
mapfile -t lines < <(
  notify-send \
    --app-name=ClaudeCode \
    --urgency="$urgency" \
    --expire-time=0 \
    --action=default=Focus \
    --hint=string:x-claude-session:"${session_id:-unknown}" \
    --print-id \
    --wait \
    "${replace_args[@]}" \
    -- "$title" "$body" 2>/dev/null
) || exit 0

notif_id="${lines[0]:-}"
action_key="${lines[1]:-}"

# Persist new id for the next dispatcher invocation (atomic replace).
# Stale state for an ended session is harmless: the next replace-id either
# updates the lingering popup or is ignored by the daemon if the id is gone.
# Cleanup at session end is a separate follow-up.
if [[ -n "$state_file" && "$notif_id" =~ ^[1-9][0-9]*$ ]]; then
  tmp_file="$(mktemp "$state_dir/.tmp.XXXXXX" 2>/dev/null || true)"
  if [[ -n "$tmp_file" ]]; then
    if printf '%s\n' "$notif_id" >"$tmp_file" 2>/dev/null; then
      mv -f "$tmp_file" "$state_file" 2>/dev/null || rm -f "$tmp_file" 2>/dev/null || true
    else
      rm -f "$tmp_file" 2>/dev/null || true
    fi
  fi
fi

# Right-click / closeall / timeout -> nothing more to do.
[[ "$action_key" != "default" ]] && exit 0

# === focus the originating tmux pane ===
if [[ -n "$tmux_pane" && -n "$tmux_session" ]] \
    && command -v tmux >/dev/null 2>&1 \
    && tmux has-session -t "$tmux_session" 2>/dev/null; then
  tmux_err="$(tmux switch-client -t "$tmux_session" \; select-pane -t "$tmux_pane" 2>&1 >/dev/null)" \
    && rc=0 || rc=$?
  logger -t claude-notify-dispatch \
    "focus tmux: rc=$rc session='$tmux_session' pane='$tmux_pane' sid=${session_id:-?}${tmux_err:+ err=$tmux_err}" \
    || true
else
  # TODO(F-3): auto-reopen tmux session if it was killed, then resume claude
  logger -t claude-notify-dispatch \
    "focus skipped: no tmux context (pane='${tmux_pane:-}' session='${tmux_session:-}' sid=${session_id:-?})" \
    || true
fi

# === window-manager focus: bring the terminal window to the foreground ===
# tmux switch-client only changes what the *currently attached* client shows;
# if the terminal window itself is in the background, the user will not see
# anything change. Use xdotool (X11) / swaymsg (Wayland) to raise the window.
wm_focused=0
if [[ "${DISPLAY:-}" != "" ]] && command -v xdotool >/dev/null 2>&1; then
  for wm_class in kitty ghostty wezterm Alacritty; do
    if xdotool search --class "$wm_class" windowactivate 2>/dev/null; then
      wm_focused=1
      logger -t claude-notify-dispatch "focus wm: xdotool class=$wm_class" || true
      break
    fi
  done
elif [[ "${WAYLAND_DISPLAY:-}" != "" ]] && command -v swaymsg >/dev/null 2>&1; then
  if swaymsg -t command '[app_id="kitty"] focus, [app_id="com.mitchellh.ghostty"] focus' \
       >/dev/null 2>&1; then
    wm_focused=1
    logger -t claude-notify-dispatch "focus wm: swaymsg" || true
  fi
fi
if [[ "$wm_focused" -eq 0 ]]; then
  logger -t claude-notify-dispatch \
    "focus wm: no tool available (install xdotool for X11)" || true
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
