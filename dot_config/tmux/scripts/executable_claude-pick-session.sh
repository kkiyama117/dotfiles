#!/usr/bin/env bash
# tmux popup: pick a session whose name matches "claude-*" and switch-client to it.
set -euo pipefail

if ! command -v fzf >/dev/null 2>&1; then
  echo "fzf is required (install via paru -S fzf)" >&2
  read -r -p "Press Enter to close..."
  exit 1
fi

target=$(tmux list-sessions -F '#S' 2>/dev/null | grep '^claude-' | fzf --prompt='claude session> ' --height=100%) || exit 0
[ -z "$target" ] && exit 0

tmux switch-client -t "$target"
