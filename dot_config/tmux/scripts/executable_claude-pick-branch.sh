#!/usr/bin/env bash
# tmux popup: pick a git branch via fzf, then call tmux-claude-new.sh.
# Any extra args ($@) are passed through to tmux-claude-new.sh
# (e.g., --no-claude to open shell-only worktree).
set -euo pipefail

if ! command -v fzf >/dev/null 2>&1; then
  echo "fzf is required (install via paru -S fzf)" >&2
  read -r -p "Press Enter to close..."
  exit 1
fi

prompt='claude branch> '
for a in "$@"; do
  [ "$a" = "--no-claude" ] && prompt='worktree branch> ' && break
done

branch=$(git for-each-ref --format='%(refname:short)' refs/heads | fzf --prompt="$prompt" --height=100%) || exit 0
[ -z "$branch" ] && exit 0

exec ~/.config/tmux/scripts/tmux-claude-new.sh "$branch" "$@"
