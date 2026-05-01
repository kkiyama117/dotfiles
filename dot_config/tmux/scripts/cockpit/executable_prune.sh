#!/usr/bin/env bash
# Remove cache files for tmux panes that no longer exist OR whose pane is
# no longer running claude. The second case covers gracefully-exited
# claude (where SessionEnd hook may not have fired in time, or fired with
# a stale TMUX_PANE) as well as ungraceful exits (SIGKILL / OOM / pane
# closed by user without /exit). Safe to run any time; idempotent.

set -u

cache_dir="${XDG_CACHE_HOME:-$HOME/.cache}/claude-cockpit/panes"
[ -d "$cache_dir" ] || exit 0

command -v tmux >/dev/null 2>&1 || exit 0

# Build the set of "<session>_<pane-id>" keys for panes currently running
# claude. Anything missing from this set -- because the pane is gone, OR
# because the pane now runs zsh / vim / a different command -- gets its
# status file deleted.
live_claude=$(tmux list-panes -a -F '#{session_name}_#{pane_id}'$'\t''#{pane_current_command}' 2>/dev/null \
  | awk -F'\t' '$2 == "claude" { print $1 }') || exit 0

# Compare each cached file's basename (minus .status suffix) to the live
# claude set. Drop any file whose corresponding pane is gone or no longer
# running claude.
shopt -s nullglob
for f in "$cache_dir"/*.status; do
  base=$(basename "$f" .status)
  if ! printf '%s\n' "$live_claude" | grep -Fxq -- "$base"; then
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
