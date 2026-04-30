#!/usr/bin/env bash
# Claude Code hook entry for tmux cockpit state tracking.
# Writes "working" / "waiting" / "done" to a per-pane cache file based on
# the hook event. Exits 0 unconditionally so Claude is never blocked.
#
# Usage:
#   claude-cockpit-state.sh hook <Event>
# Events recognized:
#   UserPromptSubmit -> working
#   PreToolUse       -> working
#   Stop             -> done
#   Notification     -> waiting
# Other events: ignored.
#
# Stdin: claude hook payload (JSON). Currently unused; reserved.

set -u  # NOT -e: never let a tool failure propagate to claude

mode="${1:-}"
event="${2:-}"

# Only "hook" mode is implemented for now.
[ "$mode" != "hook" ] && exit 0

case "$event" in
  UserPromptSubmit|PreToolUse) state="working" ;;
  Notification)                state="waiting" ;;
  Stop)                        state="done" ;;
  *)                           exit 0 ;;
esac

# tmux 外で動いている場合は no-op
tmux_pane="${TMUX_PANE:-}"
[ -z "$tmux_pane" ] && exit 0

command -v tmux >/dev/null 2>&1 || exit 0

tmux_session=$(tmux display-message -p -t "$tmux_pane" '#{session_name}' 2>/dev/null) || exit 0
[ -z "$tmux_session" ] && exit 0

cache_dir="${XDG_CACHE_HOME:-$HOME/.cache}/claude-cockpit/panes"
if ! mkdir -p "$cache_dir" 2>/dev/null; then
  command -v logger >/dev/null 2>&1 && \
    logger -t claude-cockpit-state "mkdir failed: $cache_dir"
  exit 0
fi

file="$cache_dir/${tmux_session}_${tmux_pane}.status"
tmp="$file.$$.tmp"

# atomic write: write to tmp, then rename
if printf '%s' "$state" > "$tmp" 2>/dev/null; then
  if ! mv "$tmp" "$file" 2>/dev/null; then
    rm -f "$tmp"
    command -v logger >/dev/null 2>&1 && \
      logger -t claude-cockpit-state "atomic mv failed: $file"
  fi
else
  command -v logger >/dev/null 2>&1 && \
    logger -t claude-cockpit-state "tmp write failed: $tmp"
fi

exit 0
