#!/usr/bin/env bash
# usage: tmux-claude-new.sh <branch>
# - normalizes branch name to a session name "claude-<safe>"
# - creates git worktree at <repo-root>-<safe> if missing
# - creates 2-pane tmux session if missing (left: shell, right: claude --continue --fork-session)
# - attaches or switch-clients

set -euo pipefail

branch="${1:-}"
if [ -z "$branch" ]; then
  echo "usage: tmux-claude-new.sh <branch>" >&2
  exit 1
fi

safe="${branch//\//-}"
session="claude-${safe}"

repo_root=$(git rev-parse --show-toplevel 2>/dev/null) || {
  echo "tmux-claude-new: not inside a git repo" >&2
  exit 1
}
worktree="${repo_root}-${safe}"

# Ensure worktree exists
if [ ! -d "$worktree" ]; then
  if git show-ref --verify --quiet "refs/heads/$branch"; then
    git worktree add "$worktree" "$branch"
  elif git show-ref --verify --quiet "refs/remotes/origin/$branch"; then
    git worktree add -b "$branch" "$worktree" "origin/$branch"
  else
    git worktree add -b "$branch" "$worktree" HEAD
  fi || {
    echo "tmux-claude-new: failed to create worktree at $worktree" >&2
    exit 1
  }
fi

# Ensure session exists
if ! tmux has-session -t "$session" 2>/dev/null; then
  tmux new-session -d -s "$session" -c "$worktree"
  tmux split-window -h -t "$session" -c "$worktree"
  tmux select-pane -t "${session}.0" -T work
  tmux select-pane -t "${session}.1" -T claude
  tmux send-keys -t "${session}.1" "claude --continue --fork-session" Enter
fi

# Attach or switch
if [ -n "${TMUX:-}" ]; then
  tmux switch-client -t "$session"
else
  tmux attach-session -t "$session"
fi
