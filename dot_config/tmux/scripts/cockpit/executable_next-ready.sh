#!/usr/bin/env bash
# Switch to the next pane whose cockpit state needs attention, in two
# priority buckets:
#   1. waiting  (Notification fired -- Claude is asking for input/permission)
#   2. done     (Stop fired -- Claude finished its turn)
# Within each bucket, inbox order = session-name asc -> window-index asc ->
# pane-index asc. The combined list is `<all waiting> ++ <all done>`, so a
# `waiting` pane is always preferred over a `done` pane regardless of which
# session it lives in. Cycles past the current pane; wraps around to the
# first entry if needed.

set -u

cache_dir="${XDG_CACHE_HOME:-$HOME/.cache}/claude-cockpit/panes"
[ -d "$cache_dir" ] || { tmux display-message -d 1000 "no ready claude pane"; exit 0; }

# Build the priority-ordered list of "session\twindow_idx\tpane_id\tpane_idx"
# rows. `waiting` panes come first (more urgent), `done` panes after. Each
# bucket is filled in inbox order (sessions sorted asc, windows by index,
# panes by index).
#
# Defensive filter: only consider panes whose `pane_current_command` is
# `claude`. This skips stale status files left behind when SessionEnd
# didn't fire (e.g. claude killed by SIGKILL / OOM / pane closed without
# /exit). The hook-driven SessionEnd cleanup handles graceful exits;
# this filter handles the rest.
build_ready_list() {
  local sname w_idx p_id p_idx p_cmd state
  local waiting_rows="" done_rows=""
  while IFS= read -r sname; do
    [ -z "$sname" ] && continue
    while IFS= read -r w_idx; do
      while IFS=$'\t' read -r p_id p_idx p_cmd; do
        [ "$p_cmd" = "claude" ] || continue
        state=$(cat "$cache_dir/${sname}_${p_id}.status" 2>/dev/null || echo "")
        case "$state" in
          waiting) waiting_rows+="${sname}"$'\t'"${w_idx}"$'\t'"${p_id}"$'\t'"${p_idx}"$'\n' ;;
          done)    done_rows+="${sname}"$'\t'"${w_idx}"$'\t'"${p_id}"$'\t'"${p_idx}"$'\n' ;;
        esac
      done < <(tmux list-panes -t "${sname}:${w_idx}" -F '#{pane_id}'$'\t''#{pane_index}'$'\t''#{pane_current_command}' 2>/dev/null | sort -t$'\t' -k2,2n)
    done < <(tmux list-windows -t "$sname" -F '#{window_index}' 2>/dev/null | sort -n)
  done < <(tmux list-sessions -F '#{session_name}' 2>/dev/null | sort)
  printf '%s%s' "$waiting_rows" "$done_rows"
}

list=$(build_ready_list)
if [ -z "$list" ]; then
  tmux display-message -d 1000 "no ready claude pane"
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
