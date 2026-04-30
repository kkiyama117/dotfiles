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
# Worktree resolution: read pane_current_path of the active pane in the window.
# If it differs from main repo path, attempt git worktree remove.
# If only a session name is passed (legacy callers), fall back to old behavior.

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
# Resolve the active pane's worktree root (wt_root) and the repo's MAIN
# worktree (main_repo). The window's worktree should be removed only if it
# is a non-main worktree (i.e. wt_root != main_repo).
pane_path=$(tmux display-message -p -t "$target" '#{pane_current_path}' 2>/dev/null || true)
wt_root=""
main_repo=""
if [ -n "$pane_path" ]; then
  wt_root=$(git -C "$pane_path" rev-parse --show-toplevel 2>/dev/null || true)
  # First entry in `git worktree list --porcelain` is always the main worktree.
  main_repo=$(git -C "$pane_path" worktree list --porcelain 2>/dev/null \
    | awk '/^worktree / {print $2; exit}')
fi

# --- Pre-kill: capture pane ids for cache cleanup ---
pane_ids=$(tmux list-panes -t "$target" -F '#{pane_id}' 2>/dev/null || true)

# --- Kill the window. tmux destroys the session automatically if it was the
# last window. ---
tmux kill-window -t "$target"

# --- Worktree remove (only when the killed window's pane was in a non-main
# worktree). ---
if [ -n "$wt_root" ] && [ -n "$main_repo" ] && [ "$wt_root" != "$main_repo" ] && [ -d "$wt_root" ]; then
  git -C "$main_repo" worktree remove "$wt_root" --force 2>/dev/null \
    || tmux display-message "kept worktree $wt_root (remove manually)"
fi

# --- Cockpit: drop cached state for the killed panes ---
cache="${XDG_CACHE_HOME:-$HOME/.cache}/claude-cockpit"
for pid in $pane_ids; do
  rm -f "$cache/panes/${session}_${pid}.status" 2>/dev/null || true
done
