#!/usr/bin/env bash
# Claude Code hook handler: emits a desktop notification (libnotify -> wired)
# and plays a freedesktop sound for the given event.
#
# Argument $1: event key (notification | stop | subagent-stop | error).
# Stdin: Claude Code hook payload (JSON). Drained even when unused.
set -euo pipefail

event="${1:-notification}"
sound_dir="${CLAUDE_NOTIFY_SOUND_DIR:-/usr/share/sounds/freedesktop/stereo}"

payload=""
if [[ ! -t 0 ]]; then
  payload="$(cat || true)"
fi

case "$event" in
  notification)
    sound="$sound_dir/message.oga"
    title="Claude Code"
    default_body="Awaiting input"
    urgency="normal"
    ;;
  stop)
    sound="$sound_dir/complete.oga"
    title="Claude Code"
    default_body="Turn complete"
    urgency="normal"
    ;;
  subagent-stop)
    sound="$sound_dir/bell.oga"
    title="Claude Code"
    default_body="Subagent finished"
    urgency="low"
    ;;
  error)
    sound="$sound_dir/dialog-error.oga"
    title="Claude Code"
    default_body="Error"
    urgency="critical"
    ;;
  *)
    sound="$sound_dir/message.oga"
    title="Claude Code"
    default_body="$event"
    urgency="normal"
    ;;
esac

body="$default_body"
if [[ -n "$payload" ]] && command -v jq >/dev/null 2>&1; then
  parsed="$(printf '%s' "$payload" | jq -r '.message // empty' 2>/dev/null || true)"
  [[ -n "$parsed" ]] && body="$parsed"
fi

if command -v notify-send >/dev/null 2>&1; then
  # expire-time=0 -> never expire (FDO spec). Keeps Claude popups visible
  # until the user dismisses them, regardless of wired.ron default_timeout.
  notify-send \
    --app-name=ClaudeCode \
    --urgency="$urgency" \
    --expire-time=0 \
    -- \
    "$title" "$body" >/dev/null 2>&1 || true
fi

if [[ -r "$sound" ]]; then
  if command -v pw-play >/dev/null 2>&1; then
    pw-play --volume=0.6 "$sound" >/dev/null 2>&1 &
  elif command -v paplay >/dev/null 2>&1; then
    paplay --volume=39322 "$sound" >/dev/null 2>&1 &
  elif command -v ffplay >/dev/null 2>&1; then
    ffplay -nodisp -autoexit -loglevel quiet -volume 60 "$sound" >/dev/null 2>&1 &
  fi
fi

disown 2>/dev/null || true
exit 0
