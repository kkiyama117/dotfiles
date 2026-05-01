#!/usr/bin/env bash
# Claude Code hook entry for tmux cockpit state tracking.
# Writes "working" / "waiting" / "done" to a per-pane cache file based on
# the hook event, and removes the file entirely on SessionEnd so the
# cockpit summary / next-ready don't keep counting a Claude pane that
# has gracefully exited (e.g. via /exit). Exits 0 unconditionally so
# Claude is never blocked.
#
# Usage:
#   claude-cockpit-state.sh hook <Event>
# Events recognized:
#   UserPromptSubmit -> working
#   PreToolUse       -> working
#   Stop             -> done
#   Notification     -> waiting
#   SessionEnd       -> remove status file (graceful exit cleanup)
# Other events: ignored.
#
# SessionEnd covers /exit, /clear, /logout and similar in-Claude commands.
# It does NOT fire when Claude is killed externally (SIGKILL / OOM / pane
# closed by user without /exit), so reader-side scripts (summary.sh,
# next-ready.sh, switcher.sh) and prune.sh additionally guard via
# `pane_current_command == claude` checks. See docs/todos.md for the
# eBPF-based proposal that would close the remaining gap (process death
# events fired by the kernel itself).
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
  SessionEnd)                  state="__delete__" ;;
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

# SessionEnd: remove the status file so the pane stops being counted as
# claude in summary/next-ready/switcher. rm -f is idempotent and safe even
# if the file was never created (e.g. claude crashed before its first hook
# fire).
if [ "$state" = "__delete__" ]; then
  rm -f "$file" 2>/dev/null
  exit 0
fi

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
