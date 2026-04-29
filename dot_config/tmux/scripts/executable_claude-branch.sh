#!/usr/bin/env bash
# usage: claude-branch.sh <pane-current-path>
# stdout: "[<branch>] " when inside a git repo with a current branch, else empty.
[ -z "$1" ] && exit 0
b=$(git -C "$1" branch --show-current 2>/dev/null)
[ -n "$b" ] && printf "[%s] " "$b"
exit 0
