#!/usr/bin/env bash
# tmux popup: pick a git branch via fzf, then call tmux-claude-new.sh.
set -euo pipefail

if ! command -v fzf >/dev/null 2>&1; then
  echo "fzf is required (install via paru -S fzf)" >&2
  read -r -p "Press Enter to close..."
  exit 1
fi

branch=$(git for-each-ref --format='%(refname:short)' refs/heads | fzf --prompt='claude branch> ' --height=100%) || exit 0
[ -z "$branch" ] && exit 0

exec ~/.config/tmux/scripts/tmux-claude-new.sh "$branch"
