#!/usr/bin/env bash
# Switch to the next pane whose cockpit state is "done", in inbox order:
#   session-name asc -> window-index asc -> pane-index asc.
# Cycles past the current pane; wraps around to the first done if needed.

set -u

cache_dir="${XDG_CACHE_HOME:-$HOME/.cache}/claude-cockpit/panes"
[ -d "$cache_dir" ] || { tmux display-message "no ready claude pane"; exit 0; }

# Build inbox-ordered list of "session\twindow_idx\tpane_id\tpane_idx" rows
# whose cached state is "done".
build_done_list() {
  local sname w_idx p_id p_idx state
  while IFS= read -r sname; do
    [ -z "$sname" ] && continue
    while IFS= read -r w_idx; do
      while IFS=$'\t' read -r p_id p_idx; do
        state=$(cat "$cache_dir/${sname}_${p_id}.status" 2>/dev/null || echo "")
        [ "$state" = "done" ] && printf '%s\t%s\t%s\t%s\n' "$sname" "$w_idx" "$p_id" "$p_idx"
      done < <(tmux list-panes -t "${sname}:${w_idx}" -F '#{pane_id}'$'\t''#{pane_index}' 2>/dev/null | sort -t$'\t' -k2,2n)
    done < <(tmux list-windows -t "$sname" -F '#{window_index}' 2>/dev/null | sort -n)
  done < <(tmux list-sessions -F '#{session_name}' 2>/dev/null | sort)
}

list=$(build_done_list)
if [ -z "$list" ]; then
  tmux display-message "no ready claude pane"
  exit 0
fi

# Identify the current pane to find the "next" in the list.
cur_pane=$(tmux display-message -p '#{pane_id}')

# Find the index of cur_pane in $list; pick the line after it (cycling).
target=$(awk -v cur="$cur_pane" '
  { rows[NR] = $0; ids[NR] = $3 }
  END {
    n = NR
    if (n == 0) exit 1
    pick = 1
    for (i = 1; i <= n; i++) {
      if (ids[i] == cur) { pick = (i % n) + 1; break }
    }
    print rows[pick]
  }
' <<<"$list")

[ -z "$target" ] && exit 0

t_session=$(printf '%s' "$target" | cut -f1)
t_window=$(printf '%s'  "$target" | cut -f2)
t_pane=$(printf '%s'    "$target" | cut -f3)

tmux switch-client -t "$t_session" \
  \; select-window -t "${t_session}:${t_window}" \
  \; select-pane -t "$t_pane"
