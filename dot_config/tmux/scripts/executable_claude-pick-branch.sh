#!/usr/bin/env bash
# tmux popup: pick a git branch via fzf, then call tmux-claude-new.sh.
# Any extra args ($@) are passed through to tmux-claude-new.sh
# (e.g., --no-claude to open shell-only worktree).
set -euo pipefail

log_file="/tmp/claude-pick-branch.log"
{
  echo "=== $(date -Iseconds) $$ ==="
  echo "argv: $*"
  echo "cwd: $PWD"
  echo "PATH: $PATH"
  echo "TMUX: ${TMUX:-(unset)}"
} >> "$log_file" 2>/dev/null || true

if ! command -v fzf >/dev/null 2>&1; then
  echo "fzf-missing" >> "$log_file" 2>/dev/null || true
  if [ -n "${TMUX:-}" ]; then
    tmux display-message "fzf is required (install via paru -S fzf)" 2>/dev/null || true
  fi
  echo "fzf is required (install via paru -S fzf)" >&2
  read -r -p "Press Enter to close..."
  exit 1
fi

prompt='claude branch> '
for a in "$@"; do
  [ "$a" = "--no-claude" ] && prompt='worktree branch> ' && break
done

branches=$(git for-each-ref --format='%(refname:short)' refs/heads 2>>"$log_file") || {
  echo "git-for-each-ref failed (cwd=$PWD)" >> "$log_file" 2>/dev/null || true
  if [ -n "${TMUX:-}" ]; then
    tmux display-message "claude-pick-branch: git for-each-ref failed (cwd=$PWD)" 2>/dev/null || true
  fi
  exit 1
}

if [ -z "$branches" ]; then
  echo "no-branches-found" >> "$log_file" 2>/dev/null || true
  if [ -n "${TMUX:-}" ]; then
    tmux display-message "claude-pick-branch: no local branches" 2>/dev/null || true
  fi
  exit 0
fi

branch=$(printf '%s\n' "$branches" | fzf --prompt="$prompt" --height=100%) || {
  echo "fzf-canceled (exit=$?)" >> "$log_file" 2>/dev/null || true
  exit 0
}

if [ -z "$branch" ]; then
  echo "empty-pick" >> "$log_file" 2>/dev/null || true
  exit 0
fi

echo "picked: $branch" >> "$log_file" 2>/dev/null || true
echo "exec: ~/.config/tmux/scripts/tmux-claude-new.sh $branch $*" >> "$log_file" 2>/dev/null || true
exec ~/.config/tmux/scripts/tmux-claude-new.sh "$branch" "$@"
