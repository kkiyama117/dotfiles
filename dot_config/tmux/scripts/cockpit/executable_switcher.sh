#!/usr/bin/env bash
# Hierarchical fzf switcher: lists every tmux session/window/pane with its
# claude-cockpit state badge. Acts on the selected scope:
#   Enter   -> switch-client (+ select-window / select-pane as needed)
#   Ctrl-X  -> kill the selected scope (worktree-aware for sessions)
#   Ctrl-R  -> reload (re-runs prune + redraws)

set -u

if ! command -v fzf >/dev/null 2>&1; then
  tmux display-message "fzf required (paru -S fzf)"
  exit 1
fi

cache_dir="${XDG_CACHE_HOME:-$HOME/.cache}/claude-cockpit/panes"

# Run prune first so orphans don't show up.
~/.config/tmux/scripts/cockpit/prune.sh 2>/dev/null || true

state_for_pane() {
  local s="$1" p="$2" f
  f="$cache_dir/${s}_${p}.status"
  [ -f "$f" ] && cat "$f" 2>/dev/null || true
}

badge() {
  case "${1:-}" in
    working) printf '⚡ working' ;;
    waiting) printf '⏸ waiting' ;;
    done)    printf '✓ done'    ;;
    *)       printf ''          ;;
  esac
}

# Aggregate per-session state from its panes.
state_for_session() {
  local s="$1"
  local has_w=0 has_q=0 has_d=0
  while IFS= read -r p; do
    case "$(state_for_pane "$s" "$p")" in
      working) has_w=1 ;;
      waiting) has_q=1 ;;
      done)    has_d=1 ;;
    esac
  done < <(tmux list-panes -t "$s" -s -F '#{pane_id}' 2>/dev/null)
  if   [ "$has_w" -eq 1 ]; then echo "working"
  elif [ "$has_q" -eq 1 ]; then echo "waiting"
  elif [ "$has_d" -eq 1 ]; then echo "done"
  fi
}

# Build the rendered tree. Each output line is tab-separated with metadata
# columns hidden via fzf's --with-nth=5..  Format:
#   <kind>\t<session>\t<window-idx>\t<pane-id>\t<display>
build_lines() {
  local sname session_state w_idx w_name p_id p_path p_state
  while IFS= read -r sname; do
    [ -z "$sname" ] && continue
    session_state=$(state_for_session "$sname")
    printf 'S\t%s\t\t\t%-30s  %s\n' "$sname" "$sname" "$(badge "$session_state")"

    while IFS=$'\t' read -r w_idx w_name; do
      printf 'W\t%s\t%s\t\t  window:%s %s\n' "$sname" "$w_idx" "$w_idx" "$w_name"

      while IFS=$'\t' read -r p_id p_path; do
        p_state=$(state_for_pane "$sname" "$p_id")
        printf 'P\t%s\t%s\t%s\t    pane:%s  cwd=%s    %s\n' \
          "$sname" "$w_idx" "$p_id" "$p_id" "$p_path" "$(badge "$p_state")"
      done < <(tmux list-panes -t "${sname}:${w_idx}" -F '#{pane_id}'$'\t''#{pane_current_path}' 2>/dev/null)

    done < <(tmux list-windows -t "$sname" -F '#{window_index}'$'\t''#{window_name}' 2>/dev/null)
  done < <(tmux list-sessions -F '#{session_name}' 2>/dev/null | sort)
}

selection=$(build_lines | fzf \
  --prompt='cockpit> ' \
  --height=100% \
  --delimiter=$'\t' \
  --with-nth='5..' \
  --expect='ctrl-x,ctrl-r' \
  --header='enter=switch  ctrl-x=kill  ctrl-r=reload') || exit 0

key=$(printf '%s\n' "$selection" | sed -n '1p')
row=$(printf '%s\n' "$selection" | sed -n '2p')
[ -z "$row" ] && exit 0

kind=$(printf '%s' "$row"   | cut -f1)
sname=$(printf '%s' "$row"  | cut -f2)
w_idx=$(printf '%s' "$row"  | cut -f3)
p_id=$(printf '%s' "$row"   | cut -f4)

case "$key" in
  ctrl-r)
    exec ~/.config/tmux/scripts/cockpit/switcher.sh
    ;;
  ctrl-x)
    case "$kind" in
      P)
        tmux kill-pane -t "$p_id"
        ;;
      W)
        tmux confirm-before -p "kill window ${sname}:${w_idx}? (y/n) " \
          "kill-window -t ${sname}:${w_idx}"
        ;;
      S)
        # delegate to existing claude-kill-session.sh which removes worktree too
        case "$sname" in
          claude-*)
            tmux confirm-before -p "kill claude session ${sname} and worktree? (y/n) " \
              "run-shell '~/.config/tmux/scripts/claude-kill-session.sh ${sname}'"
            ;;
          *)
            tmux confirm-before -p "kill session ${sname}? (y/n) " \
              "kill-session -t ${sname}"
            ;;
        esac
        ;;
    esac
    ;;
  *)
    case "$kind" in
      S) tmux switch-client -t "$sname" ;;
      W) tmux switch-client -t "$sname" \; select-window -t "${sname}:${w_idx}" ;;
      P) tmux switch-client -t "$sname" \; select-window -t "${sname}:${w_idx}" \; select-pane -t "$p_id" ;;
    esac
    ;;
esac
