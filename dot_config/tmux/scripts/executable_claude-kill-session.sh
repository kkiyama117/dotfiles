#!/usr/bin/env bash
# Kill the current claude-managed window and remove its corresponding worktree.
# Caller (binding) wraps this in confirm-before, so we proceed unconditionally
# once the safety check passes.
#
# Safety check (in order, OR semantics):
#   1. window has user option `@claude-managed=yes` (set by tmux-claude-new.sh)
#   2. window has any pane whose pane_current_command == 'claude'
#   3. legacy: session name starts with 'claude-' (old flat scheme)
# If none match, refuse with display-message + exit 1.
#
# Worktree resolution (in order):
#   1. window options `@claude-worktree` and `@claude-main-repo` (pinned by
#      tmux-claude-new.sh at creation, authoritative)
#   2. `pane_current_path` of the active pane (legacy fallback for windows
#      that pre-date the tags)
# If the resolved worktree differs from the main repo path, attempt
# `git worktree remove --force` BEFORE kill-window so error messages still
# have a client to display on.

set -euo pipefail

# Optional positional: explicit window target.
# Accepted forms:
#   (no arg)         -> current window of current client
#   "<session>"      -> active window of <session>
#   "<session>:<W>"  -> specific window <W> in <session>
#                       (W is a window index or name, i.e. any tmux window-target)
explicit_target="${1:-}"

if [ -n "$explicit_target" ]; then
  # Probe the target by reading both #S and #W via display-message -t.
  # Works for session-only and session:window forms because tmux resolves
  # both to a window-target (defaulting to the active window for session-only).
  session=$(tmux display-message -p -t "$explicit_target" '#S' 2>/dev/null) || {
    tmux display-message "claude-kill-session: target not found ($explicit_target)"
    exit 1
  }
  window=$(tmux display-message -p -t "$explicit_target" '#W' 2>/dev/null) || {
    tmux display-message "claude-kill-session: window not found ($explicit_target)"
    exit 1
  }
else
  session=$(tmux display-message -p '#S')
  window=$(tmux display-message -p '#W')
fi

target="${session}:${window}"

# --- Safety check ---
managed=$(tmux show-options -w -t "$target" -v '@claude-managed' 2>/dev/null || echo "")
has_claude=""
if [ "$managed" != "yes" ]; then
  has_claude=$(tmux list-panes -t "$target" -F '#{pane_current_command}' 2>/dev/null \
    | grep -Fx claude || true)
fi
legacy=""
case "$session" in claude-*) legacy=1 ;; esac

if [ "$managed" != "yes" ] && [ -z "$has_claude" ] && [ -z "$legacy" ]; then
  tmux display-message "claude-kill-session: refusing on non-claude window ($target)"
  exit 1
fi

# --- Worktree resolution ---
# Prefer the @claude-worktree / @claude-main-repo tags pinned by
# tmux-claude-new.sh at window creation. They are authoritative even when
# pane_current_path has drifted (user `cd`'d out of the worktree, into a
# different repo, or into a deleted directory). Fall back to deriving from
# pane_current_path for legacy windows that pre-date the tags.
wt_root=$(tmux show-options -w -t "$target" -v '@claude-worktree' 2>/dev/null || echo "")
main_repo=$(tmux show-options -w -t "$target" -v '@claude-main-repo' 2>/dev/null || echo "")

if [ -z "$wt_root" ] || [ -z "$main_repo" ]; then
  pane_path=$(tmux display-message -p -t "$target" '#{pane_current_path}' 2>/dev/null || true)
  if [ -n "$pane_path" ] && [ -d "$pane_path" ]; then
    [ -z "$wt_root" ] && wt_root=$(git -C "$pane_path" rev-parse --show-toplevel 2>/dev/null || true)
    # First entry in `git worktree list --porcelain` is always the main worktree.
    [ -z "$main_repo" ] && main_repo=$(git -C "$pane_path" worktree list --porcelain 2>/dev/null \
      | awk '/^worktree / {print $2; exit}')
  fi
fi

# --- Pre-kill: capture pane ids for cache cleanup ---
pane_ids=$(tmux list-panes -t "$target" -F '#{pane_id}' 2>/dev/null || true)

# --- Worktree remove BEFORE kill-window so error messages still have a client
# to display on. (kill-window may destroy the session if this is the last
# window, leaving display-message with no audience.) ---
# Only when the window's worktree is a non-main worktree.
if [ -n "$wt_root" ] && [ -n "$main_repo" ] && [ "$wt_root" != "$main_repo" ]; then
  if [ -d "$wt_root" ]; then
    if ! err_msg=$(git -C "$main_repo" worktree remove "$wt_root" --force 2>&1); then
      tmux display-message "kept worktree $wt_root: ${err_msg}"
    fi
  fi
  # Defensive: prune stale .git/worktrees/<name>/ admin dirs even when the
  # working tree was already gone (e.g., user removed it manually).
  git -C "$main_repo" worktree prune 2>/dev/null || true
fi

# --- Kill the window. tmux destroys the session automatically if it was the
# last window. ---
tmux kill-window -t "$target"

# --- Cockpit: drop cached state for the killed panes ---
cache="${XDG_CACHE_HOME:-$HOME/.cache}/claude-cockpit"
for pid in $pane_ids; do
  rm -f "$cache/panes/${session}_${pid}.status" 2>/dev/null || true
done
