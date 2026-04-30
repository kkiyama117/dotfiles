#!/usr/bin/env bash
# usage: tmux-claude-new.sh <branch> [--from-root [<session-id>]] [--no-claude]
#                           [--worktree-base <dir>] [--prompt <text>]
# - resolves main repo path; session name = basename of main worktree
# - window name = sanitized branch name; idempotent via `tmux new-window -S`
# - 2-pane (left: shell, right: claude) inside the window
#     default        : `claude --continue --fork-session` if prior history exists,
#                       else plain `claude`
#     --from-root    : `claude --resume <id> --fork-session` (id from main repo's
#                       claude session history; fzf picker if <session-id> omitted)
#     --no-claude    : 1-pane shell window, skips claude entirely
#     --worktree-base <dir>
#                    : place new worktrees under <dir>/<repo>/<branch> instead
#                      of the default sibling `${main_repo}-${branch}` layout.
#                      Ignored if a worktree for the branch already exists.
#                      Recommended: ~/.local/share/worktrees (XDG-centralized).
#     --prompt <text>
#                    : seed the freshly-spawned claude session with <text>
#                      as the initial message (passed as a positional arg to
#                      claude; safely shell-quoted before send-keys). No-op
#                      when --no-claude.
# - tags the window with `@claude-managed=yes` so claude-kill-session.sh can
#   safely act window-scoped without the old `claude-*` session-name prefix
# - attaches or switch-clients to the window

set -euo pipefail

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

# tmux session/window name sanitizer (allows alnum, dot, dash, underscore)
sanitize() {
  printf '%s' "$1" | tr -c 'a-zA-Z0-9._-' '-'
}

branch=""
from_root=0
no_claude=0
explicit_session=""
worktree_base=""
initial_prompt=""

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
    --no-claude) no_claude=1; shift ;;
    --worktree-base)
      shift
      (( $# )) || die "--worktree-base requires a directory argument"
      worktree_base="$1"
      shift
      ;;
    --prompt)
      shift
      (( $# )) || die "--prompt requires a text argument"
      initial_prompt="$1"
      shift
      ;;
    -h|--help)
      echo "usage: tmux-claude-new.sh <branch> [--from-root [<session-id>]] [--no-claude]"
      echo "                          [--worktree-base <dir>] [--prompt <text>]"
      exit 0
      ;;
    *) die "unknown arg: $1" ;;
  esac
done

[ -z "$branch" ] && die "usage: tmux-claude-new.sh <branch> [--from-root [<session-id>]] [--no-claude] [--worktree-base <dir>] [--prompt <text>]"
(( from_root )) && (( no_claude )) && die "--from-root and --no-claude are mutually exclusive"
(( no_claude )) && [ -n "$initial_prompt" ] && die "--prompt is incompatible with --no-claude"

safe="$(sanitize "$branch")"
window_name="$safe"

# Resolve main repo (the first worktree entry is always the main worktree).
main_repo=$(git worktree list --porcelain 2>/dev/null | awk '/^worktree / {print $2; exit}')
[ -n "$main_repo" ] || die "not inside a git repo (cwd=$PWD)"

repo_basename="$(basename "$main_repo")"
session="$(sanitize "$repo_basename")"
[ -n "$session" ] || die "failed to resolve repo basename"

# Resolve target worktree.
existing_worktree=$(git worktree list --porcelain | awk -v b="refs/heads/$branch" '
  /^worktree / { wt=$2 }
  $0 == "branch " b { print wt; exit }
')

if [ -n "$existing_worktree" ]; then
  worktree="$existing_worktree"
else
  if [ -n "$worktree_base" ]; then
    worktree="${worktree_base}/${repo_basename}/${safe}"
    mkdir -p "$(dirname "$worktree")" 2>>"$log_file" \
      || die "failed to mkdir worktree parent: $(dirname "$worktree")"
  else
    worktree="${main_repo}-${safe}"
  fi
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

# Resolve root session id when --from-root.
session_id=""
if (( from_root )); then
  encoded=$(printf '%s' "$main_repo" | tr '/.' '-')
  sessions_dir="$HOME/.claude/projects/$encoded"
  [ -d "$sessions_dir" ] || die "no claude sessions at $sessions_dir"
  if [ -n "$explicit_session" ]; then
    [ -f "$sessions_dir/$explicit_session.jsonl" ] || die "session id not found: $explicit_session"
    session_id="$explicit_session"
  else
    command -v fzf >/dev/null 2>&1 || die "fzf required for --from-root without an id"
    pick=$(ls -t "$sessions_dir"/*.jsonl 2>/dev/null | \
      fzf --prompt='root session> ' --preview 'head -50 {}' --height=80%) || exit 0
    [ -z "$pick" ] && exit 0
    session_id=$(basename "$pick" .jsonl)
  fi
fi

# Detect prior claude history for THIS worktree (not the repo session).
worktree_has_history=0
worktree_encoded=$(printf '%s' "$worktree" | tr '/.' '-')
worktree_sessions_dir="$HOME/.claude/projects/$worktree_encoded"
if [ -d "$worktree_sessions_dir" ]; then
  shopt -s nullglob
  jsonl_files=("$worktree_sessions_dir"/*.jsonl)
  shopt -u nullglob
  (( ${#jsonl_files[@]} > 0 )) && worktree_has_history=1
fi

# Idempotent session create.
# We deliberately avoid `new-session -A`: when the session already exists,
# -A falls through to attach-session, which requires a TTY and fails with
# "open terminal failed: not a terminal" when invoked from a tmux key-binding
# pipe. Use an explicit `has-session` probe instead.
# When creating fresh, the first window is the branch window itself
# (-n "$window_name", cwd = "$worktree"), so we don't leave behind an orphan
# default `zsh` window.
if ! tmux has-session -t "=$session" 2>/dev/null; then
  tmux new-session -d -s "$session" -n "$window_name" -c "$worktree" 2>>"$log_file" \
    || die "failed to create session $session"
fi

# Idempotent window create. tmux 3.6's new-window uses -S (NOT -A) to select
# an existing same-named window instead of creating a duplicate.
tmux new-window -S -t "${session}:" -n "$window_name" -c "$worktree" 2>>"$log_file" \
  || die "failed to create or attach window $session:$window_name"

# Mark window as claude-managed. Used by claude-kill-session.sh as a fallback
# when no claude pane is currently running (e.g., user manually closed it).
tmux set-option -w -t "${session}:${window_name}" -o '@claude-managed' yes 2>/dev/null || true

# Inspect window pane count; only set up panes on a fresh window.
pane_count=$(tmux list-panes -t "${session}:${window_name}" -F '.' 2>/dev/null | wc -l)
if [ "$pane_count" -le 1 ]; then
  if (( no_claude )); then
    tmux select-pane -t "${session}:${window_name}.0" -T work 2>/dev/null || true
  else
    tmux split-window -h -t "${session}:${window_name}" -c "$worktree" 2>>"$log_file" \
      || die "failed to split window in $session:$window_name"
    tmux select-pane -t "${session}:${window_name}.0" -T work 2>/dev/null || true
    tmux select-pane -t "${session}:${window_name}.1" -T claude 2>/dev/null || true
    if [ -n "$session_id" ]; then
      claude_cmd="claude --resume $session_id --fork-session"
    elif (( worktree_has_history )); then
      claude_cmd="claude --continue --fork-session"
    else
      claude_cmd="claude"
    fi
    if [ -n "$initial_prompt" ]; then
      # printf %q produces bash-quoted output; safe to feed to interactive
      # shell via send-keys for typical text (incl. UTF-8 / Japanese).
      claude_cmd+=" $(printf %q "$initial_prompt")"
    fi
    tmux send-keys -t "${session}:${window_name}.1" "$claude_cmd" Enter 2>>"$log_file" \
      || die "failed to send claude command"
  fi
fi

# Switch (or attach if outside tmux).
if [ -n "${TMUX:-}" ]; then
  tmux switch-client -t "${session}:${window_name}" 2>>"$log_file" \
    || die "switch-client to ${session}:${window_name} failed"
  echo "switched to ${session}:${window_name}" >> "$log_file"
else
  tmux attach-session -t "${session}:${window_name}"
fi
