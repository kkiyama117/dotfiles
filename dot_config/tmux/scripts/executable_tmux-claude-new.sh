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

# When invoked from `display-popup -E`, stderr is hidden once the popup closes.
# Log everything to /tmp/tmux-claude-new.log so failures are debuggable, and on
# error surface the message via tmux display-message which the user can read.
log_file="/tmp/tmux-claude-new.log"
{
  echo "=== $(date -Iseconds) $$ ==="
  echo "argv: $*"
  echo "cwd: $PWD"
  echo "TMUX: ${TMUX:-(unset)}"
} >> "$log_file" 2>/dev/null || true

die() {
  local msg="tmux-claude-new: $*"
  echo "$msg" >&2
  echo "$msg" >> "$log_file" 2>/dev/null || true
  if [ -n "${TMUX:-}" ]; then
    tmux display-message "$msg" 2>/dev/null || true
  fi
  exit 1
}

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
      die "unknown arg: $1"
      ;;
  esac
done

if [ -z "$branch" ]; then
  die "usage: tmux-claude-new.sh <branch> [--from-root [<session-id>]] [--no-claude]"
fi

if (( from_root )) && (( no_claude )); then
  die "--from-root and --no-claude are mutually exclusive"
fi

safe="${branch//\//-}"
session="claude-${safe}"

repo_root=$(git rev-parse --show-toplevel 2>/dev/null) || die "not inside a git repo (cwd=$PWD)"

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
    fi || die "failed to create worktree at $worktree"
  fi
fi

# Resolve root session id when --from-root is requested.
session_id=""
if (( from_root )); then
  main_repo=$(git worktree list --porcelain | awk '/^worktree / {print $2; exit}')
  [ -n "$main_repo" ] || die "failed to resolve main worktree path"
  encoded=$(printf '%s' "$main_repo" | tr '/.' '-')
  sessions_dir="$HOME/.claude/projects/$encoded"

  [ -d "$sessions_dir" ] || die "no claude sessions at $sessions_dir"

  if [ -n "$explicit_session" ]; then
    [ -f "$sessions_dir/$explicit_session.jsonl" ] || die "session id not found: $explicit_session"
    session_id="$explicit_session"
  else
    command -v fzf >/dev/null 2>&1 || die "fzf required for --from-root without an id"
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

# Ensure session exists. -A makes new-session idempotent: attach (here, just keep)
# if a session of that name already exists, otherwise create it. Combined with -d
# we always end up with the session present and detached.
tmux new-session -A -d -s "$session" -c "$worktree" 2>>"$log_file" \
  || die "failed to create or attach session $session"

# If we just created a fresh session (single pane), set up panes.
pane_count=$(tmux list-panes -t "$session" -F '.' 2>/dev/null | wc -l)
if [ "$pane_count" -le 1 ]; then
  if (( no_claude )); then
    # Cosmetic title only; failures here must not abort the switch.
    tmux select-pane -t "$session" -T work 2>/dev/null || true
  else
    tmux split-window -h -t "$session" -c "$worktree" 2>>"$log_file" \
      || die "failed to split window in $session"
    tmux select-pane -t "${session}.0" -T work 2>/dev/null || true
    tmux select-pane -t "${session}.1" -T claude 2>/dev/null || true
    if [ -n "$session_id" ]; then
      claude_cmd="claude --resume $session_id --fork-session"
    elif (( worktree_has_history )); then
      claude_cmd="claude --continue --fork-session"
    else
      claude_cmd="claude"
    fi
    tmux send-keys -t "${session}.1" "$claude_cmd" Enter 2>>"$log_file" \
      || die "failed to send claude command"
  fi
fi

# Attach or switch
if [ -n "${TMUX:-}" ]; then
  tmux switch-client -t "$session" 2>>"$log_file" \
    || die "switch-client to $session failed"
  echo "switched to $session" >> "$log_file"
else
  tmux attach-session -t "$session"
fi
