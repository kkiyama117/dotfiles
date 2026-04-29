#!/usr/bin/env bash
# Restart claude in the current session's claude pane.
# Strategy: find a pane in the current session whose pane_current_command is
# 'claude'; if found, respawn-pane -k and start a fresh claude --continue.
# If none found, do it in the current pane.
set -euo pipefail

session=$(tmux display-message -p '#S')
target=$(tmux list-panes -t "$session" -F '#{pane_id} #{pane_current_command}' \
  | awk '$2 == "claude" {print $1; exit}')

if [ -z "$target" ]; then
  target=$(tmux display-message -p '#{pane_id}')
fi

tmux respawn-pane -k -t "$target"
tmux send-keys -t "$target" "claude --continue" Enter
