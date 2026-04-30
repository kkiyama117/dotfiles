#!/usr/bin/env bash
# usage: tmux-claude-new.sh <branch> [--from-root [<session-id>]] [--no-claude]
# - normalizes branch name to a session name "claude-<safe>"
# - resolves the target worktree:
#     1. if <branch> is already registered in `git worktree list`, reuses that path
#        (handles "current branch" / branches checked out elsewhere)
#     2. else creates a worktree at <repo-root>-<safe> (auto-creates branch from
#        HEAD if it doesn't exist locally or as origin/<branch>)
# - creates 2-pane tmux session if missing (left: shell, right: claude)
#     default        : `claude --continue --fork-session` if prior history exists,
#                       else plain `claude` (avoids "Fatal Error" on fresh worktree)
#     --from-root    : `claude --resume <id> --fork-session` (id from main worktree's
#                       session history; fzf picker if <session-id> is omitted)
#     --no-claude    : 1-pane shell session, skips claude entirely
# - attaches or switch-clients

set -euo pipefail

branch=""
from_root=0
no_claude=0
explicit_session=""

# First positional arg: branch (must not start with '-')
if [ $# -gt 0 ] && [[ "$1" != -* ]]; then
  branch="$1"
  shift
fi

while (( $# )); do
  case "$1" in
    --from-root)
      from_root=1
      shift
      if (( $# )) && [[ "$1" != -* ]]; then
        explicit_session="$1"
        shift
      fi
      ;;
    --no-claude)
      no_claude=1
      shift
      ;;
    -h|--help)
      echo "usage: tmux-claude-new.sh <branch> [--from-root [<session-id>]] [--no-claude]"
      exit 0
      ;;
    *)
      echo "tmux-claude-new: unknown arg: $1" >&2
      exit 1
      ;;
  esac
done

if [ -z "$branch" ]; then
  echo "usage: tmux-claude-new.sh <branch> [--from-root [<session-id>]] [--no-claude]" >&2
  exit 1
fi

if (( from_root )) && (( no_claude )); then
  echo "tmux-claude-new: --from-root and --no-claude are mutually exclusive" >&2
  exit 1
fi

safe="${branch//\//-}"
session="claude-${safe}"

repo_root=$(git rev-parse --show-toplevel 2>/dev/null) || {
  echo "tmux-claude-new: not inside a git repo" >&2
  exit 1
}

# Resolve target worktree.
# If <branch> is already registered in any worktree, reuse it. This handles
# the "current branch" case (main repo's checked-out branch) and any other
# already-checked-out branch.
existing_worktree=$(git worktree list --porcelain | awk -v b="refs/heads/$branch" '
  /^worktree / { wt=$2 }
  $0 == "branch " b { print wt; exit }
')

if [ -n "$existing_worktree" ]; then
  worktree="$existing_worktree"
else
  worktree="${repo_root}-${safe}"
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
fi

# Resolve root session id when --from-root is requested.
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

# Detect prior claude history for this worktree (avoids `--continue` Fatal Error
# when there is no session to continue).
worktree_has_history=0
worktree_encoded=$(printf '%s' "$worktree" | tr '/.' '-')
worktree_sessions_dir="$HOME/.claude/projects/$worktree_encoded"
if [ -d "$worktree_sessions_dir" ]; then
  shopt -s nullglob
  jsonl_files=("$worktree_sessions_dir"/*.jsonl)
  shopt -u nullglob
  if (( ${#jsonl_files[@]} > 0 )); then
    worktree_has_history=1
  fi
fi

# Ensure session exists
if ! tmux has-session -t "$session" 2>/dev/null; then
  tmux new-session -d -s "$session" -c "$worktree"
  if (( no_claude )); then
    tmux select-pane -t "${session}.0" -T work
  else
    tmux split-window -h -t "$session" -c "$worktree"
    tmux select-pane -t "${session}.0" -T work
    tmux select-pane -t "${session}.1" -T claude
    if [ -n "$session_id" ]; then
      claude_cmd="claude --resume $session_id --fork-session"
    elif (( worktree_has_history )); then
      claude_cmd="claude --continue --fork-session"
    else
      claude_cmd="claude"
    fi
    tmux send-keys -t "${session}.1" "$claude_cmd" Enter
  fi
fi

# Attach or switch
if [ -n "${TMUX:-}" ]; then
  tmux switch-client -t "$session"
else
  tmux attach-session -t "$session"
fi
