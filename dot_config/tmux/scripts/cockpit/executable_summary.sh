#!/usr/bin/env bash
# Aggregate per-pane state files into a status-right summary string.
# Output examples:
#   "⚡ 3 ⏸ 1 ✓ 2 "    (mixed)
#   "✓ 2 "             (only done)
#   ""                 (no state files / all empty)
# A single trailing space is included when output is non-empty so the next
# status-right segment is separated visually.
#
# Defensive filter: a status file is only counted if the corresponding tmux
# pane is currently running `claude`. This handles cases where SessionEnd
# didn't fire (claude killed by SIGKILL / OOM / pane closed without /exit),
# so a stale "working" / "waiting" / "done" file no longer keeps inflating
# the summary forever.

set -u

cache_dir="${XDG_CACHE_HOME:-$HOME/.cache}/claude-cockpit/panes"

# No cache yet -> empty summary
[ -d "$cache_dir" ] || exit 0

# Build the set of "<session>_<pane_id>" keys for panes currently running
# claude. Anything else (zsh, vim, /exit'd shell, dead pane) is ignored.
# tmux is required; if it's missing we silently emit nothing.
command -v tmux >/dev/null 2>&1 || exit 0
live_claude=$(tmux list-panes -a -F '#{session_name}_#{pane_id}'$'\t''#{pane_current_command}' 2>/dev/null \
  | awk -F'\t' '$2 == "claude" { print $1 }')

working=0
waiting=0
done_=0

# nullglob so an empty dir doesn't iterate "*.status" literal
shopt -s nullglob
for f in "$cache_dir"/*.status; do
  base=$(basename "$f" .status)
  # Skip status files whose pane is not currently a live claude pane.
  printf '%s\n' "$live_claude" | grep -Fxq -- "$base" || continue
  state=$(cat "$f" 2>/dev/null || echo "")
  case "$state" in
    working) working=$((working + 1)) ;;
    waiting) waiting=$((waiting + 1)) ;;
    done)    done_=$((done_ + 1)) ;;
    *)       ;;
  esac
done

out=""
[ "$working" -gt 0 ] && out+="⚡ $working "
[ "$waiting" -gt 0 ] && out+="⏸ $waiting "
[ "$done_"   -gt 0 ] && out+="✓ $done_ "

printf '%s' "$out"
