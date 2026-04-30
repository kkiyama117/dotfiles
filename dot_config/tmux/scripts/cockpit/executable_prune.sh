#!/usr/bin/env bash
# Remove cache files for tmux panes that no longer exist.
# Safe to run any time; idempotent.

set -u

cache_dir="${XDG_CACHE_HOME:-$HOME/.cache}/claude-cockpit/panes"
[ -d "$cache_dir" ] || exit 0

command -v tmux >/dev/null 2>&1 || exit 0

# Build the set of currently-live "<session>_<pane-id>" keys
live=$(tmux list-panes -a -F '#{session_name}_#{pane_id}' 2>/dev/null) || exit 0

# Compare each cached file's basename (minus .status suffix) to live set
shopt -s nullglob
for f in "$cache_dir"/*.status; do
  base=$(basename "$f" .status)
  if ! printf '%s\n' "$live" | grep -Fxq -- "$base"; then
    rm -f -- "$f"
  fi
done

# Also clean up the (currently unused) sessions/ dir defensively
sessions_dir="${XDG_CACHE_HOME:-$HOME/.cache}/claude-cockpit/sessions"
if [ -d "$sessions_dir" ]; then
  live_sessions=$(tmux list-sessions -F '#{session_name}' 2>/dev/null) || exit 0
  for f in "$sessions_dir"/*.status; do
    base=$(basename "$f" .status)
    if ! printf '%s\n' "$live_sessions" | grep -Fxq -- "$base"; then
      rm -f -- "$f"
    fi
  done
fi

exit 0
