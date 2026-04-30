#!/usr/bin/env bash
# Kill the current "claude-*" session and remove its corresponding worktree.
# Caller (binding) wraps this in confirm-before, so we proceed unconditionally.
set -euo pipefail

session=$(tmux display-message -p '#S')

case "$session" in
  claude-*) ;;
  *)
    tmux display-message "claude-kill-session: refusing on non-claude session ($session)"
    exit 1
    ;;
esac

# Worktree path is computed the same way tmux-claude-new.sh did:
# repo_root + "-" + session_suffix
suffix="${session#claude-}"
pane_path=$(tmux display-message -p '#{pane_current_path}')
repo_root=$(git -C "$pane_path" rev-parse --show-toplevel 2>/dev/null || true)

# Detach clients on this session, then kill it, then remove worktree if found.
tmux switch-client -n 2>/dev/null || true
tmux kill-session -t "$session"

if [ -n "$repo_root" ]; then
  worktree="${repo_root}-${suffix}"
  if [ -d "$worktree" ]; then
    git -C "$repo_root" worktree remove "$worktree" --force 2>/dev/null \
      || tmux display-message "kept worktree $worktree (remove manually)"
  fi
fi

# Cockpit: drop cached state files for the killed session
cache="${XDG_CACHE_HOME:-$HOME/.cache}/claude-cockpit"
rm -f "$cache/sessions/${session}.status" "$cache/panes/${session}"_*.status 2>/dev/null || true
