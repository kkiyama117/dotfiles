#!/usr/bin/env bash
# Aggregate per-pane state files into a status-right summary string.
# Output examples:
#   "⚡ 3 ⏸ 1 ✓ 2 "    (mixed)
#   "✓ 2 "             (only done)
#   ""                 (no state files / all empty)
# A single trailing space is included when output is non-empty so the next
# status-right segment is separated visually.

set -u

cache_dir="${XDG_CACHE_HOME:-$HOME/.cache}/claude-cockpit/panes"

# No cache yet -> empty summary
[ -d "$cache_dir" ] || exit 0

working=0
waiting=0
done_=0

# nullglob so an empty dir doesn't iterate "*.status" literal
shopt -s nullglob
for f in "$cache_dir"/*.status; do
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
