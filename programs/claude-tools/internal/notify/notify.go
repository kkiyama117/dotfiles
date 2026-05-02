// Package notify provides shared types and helpers for the Claude Code
// notify pipeline (hook → dispatch → cleanup → sound). The popup state
// machine and D-Bus action loop live in dispatch.go (added in PR-9).
//
// Shell parity:
//   - state dir layout: ${XDG_RUNTIME_DIR:-/tmp}/claude-notify/sessions/
//   - replace-id state file: <sid>.id (1 行に notify-send notification id)
//   - tmp suffix: .tmp.* (mktemp 残骸)
package notify

import "claude-tools/internal/xdg"

// StateDir is the runtime directory holding per-session replace-id state
// files (`<sid>.id`). Mirrors the shell variable `state_dir` in the
// dispatch / cleanup scripts.
func StateDir() string {
	return xdg.ClaudeNotifyStateDir()
}
