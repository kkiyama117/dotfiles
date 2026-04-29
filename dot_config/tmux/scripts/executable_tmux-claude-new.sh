#!/usr/bin/env bash
# usage: tmux-claude-new.sh <branch> [--from-root [<session-id>]]
# - normalizes branch name to a session name "claude-<safe>"
# - creates git worktree at <repo-root>-<safe> if missing (auto-creates branch
#   from HEAD if it doesn't exist locally or as origin/<branch>)
# - creates 2-pane tmux session if missing (left: shell, right: claude)
# - right pane runs:
#     default        : `claude --continue --fork-session`
#     --from-root    : `claude --resume <id> --fork-session`, where <id> comes
#                       from the main worktree's session history
#                       (fzf picker if <session-id> is omitted)
# - attaches or switch-clients

set -euo pipefail

branch=""
from_root=0
explicit_session=""

# First positional arg: branch
if [ $# -gt 0 ] && [[ "$1" != --* ]]; then
  branch="$1"
  shift
fi

while (( $# )); do
  case "$1" in
    --from-root)
      from_root=1
      shift
      if (( $# )) && [[ "$1" != --* ]]; then
        explicit_session="$1"
        shift
      fi
      ;;
    -h|--help)
      echo "usage: tmux-claude-new.sh <branch> [--from-root [<session-id>]]"
      exit 0
      ;;
    *)
      echo "tmux-claude-new: unknown arg: $1" >&2
      exit 1
      ;;
  esac
done

if [ -z "$branch" ]; then
  echo "usage: tmux-claude-new.sh <branch> [--from-root [<session-id>]]" >&2
  exit 1
fi

safe="${branch//\//-}"
session="claude-${safe}"

repo_root=$(git rev-parse --show-toplevel 2>/dev/null) || {
  echo "tmux-claude-new: not inside a git repo" >&2
  exit 1
}
worktree="${repo_root}-${safe}"

# Resolve root session id when --from-root is requested.
# Picks from the *main* worktree's claude project dir, regardless of where this
# script is invoked from (so it works even if called from inside a worktree).
session_id=""
if (( from_root )); then
  main_repo=$(git worktree list --porcelain | awk '/^worktree / {print $2; exit}')
  if [ -z "$main_repo" ]; then
    echo "tmux-claude-new: failed to resolve main worktree path" >&2
    exit 1
  fi
  encoded=$(printf '%s' "$main_repo" | tr '/.' '-')
  sessions_dir="$HOME/.claude/projects/$encoded"

  if [ ! -d "$sessions_dir" ]; then
    echo "tmux-claude-new: no claude sessions at $sessions_dir" >&2
    exit 1
  fi

  if [ -n "$explicit_session" ]; then
    if [ ! -f "$sessions_dir/$explicit_session.jsonl" ]; then
      echo "tmux-claude-new: session id not found: $explicit_session" >&2
      exit 1
    fi
    session_id="$explicit_session"
  else
    if ! command -v fzf >/dev/null 2>&1; then
      echo "tmux-claude-new: fzf required for --from-root without an id" >&2
      exit 1
    fi
    pick=$(ls -t "$sessions_dir"/*.jsonl 2>/dev/null | \
      fzf --prompt='root session> ' \
          --preview 'head -50 {}' \
          --height=80%) || exit 0
    [ -z "$pick" ] && exit 0
    session_id=$(basename "$pick" .jsonl)
  fi
fi

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
  if [ -n "$session_id" ]; then
    tmux send-keys -t "${session}.1" "claude --resume $session_id --fork-session" Enter
  else
    tmux send-keys -t "${session}.1" "claude --continue --fork-session" Enter
  fi
fi

# Attach or switch
if [ -n "${TMUX:-}" ]; then
  tmux switch-client -t "$session"
else
  tmux attach-session -t "$session"
fi
