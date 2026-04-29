#!/usr/bin/env bash
# Claude Code hook entry. Parses event/payload, forks the sound player
# and the popup dispatcher, then exits immediately so the hook caller
# (claude) is not blocked.
#
# Argument $1: event key (notification | stop | subagent-stop | error).
# Stdin: Claude Code hook payload (JSON).
set -euo pipefail

event="${1:-notification}"
sound_dir="${CLAUDE_NOTIFY_SOUND_DIR:-/usr/share/sounds/freedesktop/stereo}"

payload=""
if [[ ! -t 0 ]]; then
  payload="$(cat || true)"
fi

case "$event" in
  notification)  sound="$sound_dir/message.oga";      title="Claude Code"; default_body="Awaiting input";    urgency="normal"   ;;
  stop)          sound="$sound_dir/complete.oga";     title="Claude Code"; default_body="Turn complete";     urgency="normal"   ;;
  subagent-stop) sound="$sound_dir/bell.oga";         title="Claude Code"; default_body="Subagent finished"; urgency="low"      ;;
  error)         sound="$sound_dir/dialog-error.oga"; title="Claude Code"; default_body="Error";             urgency="critical" ;;
  *)             sound="$sound_dir/message.oga";      title="Claude Code"; default_body="$event";            urgency="normal"   ;;
esac

# === payload extraction ===
body="$default_body"
session_id=""
if [[ -n "$payload" ]] && command -v jq >/dev/null 2>&1; then
  parsed_msg="$(printf '%s' "$payload" | jq -r '.message // empty' 2>/dev/null || true)"
  [[ -n "$parsed_msg" ]] && body="$parsed_msg"
  session_id="$(printf '%s' "$payload" | jq -r '.session_id // empty' 2>/dev/null || true)"
fi

# === tmux context (empty -> bare terminal) ===
tmux_pane="${TMUX_PANE:-}"
tmux_session=""
if [[ -n "$tmux_pane" ]] && command -v tmux >/dev/null 2>&1; then
  tmux_session="$(tmux display-message -p -t "$tmux_pane" '#{session_name}' 2>/dev/null || true)"
fi

# === fire & forget: sound ===
sound_bin="${CLAUDE_NOTIFY_SOUND_BIN:-$HOME/.local/bin/claude-notify-sound.sh}"
if [[ -x "$sound_bin" ]]; then
  "$sound_bin" "$sound" >/dev/null 2>&1 &
  disown 2>/dev/null || true
fi

# === fork dispatcher (popup + action loop) ===
dispatch_bin="${CLAUDE_NOTIFY_DISPATCH:-$HOME/.local/bin/claude-notify-dispatch.sh}"
if [[ -x "$dispatch_bin" ]] && command -v notify-send >/dev/null 2>&1; then
  CLAUDE_NOTIFY_TITLE="$title" \
  CLAUDE_NOTIFY_BODY="$body" \
  CLAUDE_NOTIFY_URGENCY="$urgency" \
  CLAUDE_NOTIFY_SESSION_ID="$session_id" \
  CLAUDE_NOTIFY_TMUX_PANE="$tmux_pane" \
  CLAUDE_NOTIFY_TMUX_SESSION="$tmux_session" \
    setsid "$dispatch_bin" </dev/null >/dev/null 2>&1 &
  disown 2>/dev/null || true
elif command -v notify-send >/dev/null 2>&1; then
  # fallback: dispatcher missing -> still show a (non-interactive) popup
  notify-send --app-name=ClaudeCode --urgency="$urgency" --expire-time=0 \
    -- "$title" "$body" >/dev/null 2>&1 || true
fi

exit 0
