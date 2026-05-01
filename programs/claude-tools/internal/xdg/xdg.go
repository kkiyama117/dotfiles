// Package xdg resolves XDG Base Directory paths used by claude-tools.
//
// Shell parity: ${XDG_RUNTIME_DIR:-/tmp} と ${XDG_CACHE_HOME:-$HOME/.cache}
// を 1:1 再現する。
package xdg

import (
	"os"
	"path/filepath"
)

// RuntimeDir returns $XDG_RUNTIME_DIR or "/tmp" if unset/empty.
func RuntimeDir() string {
	if v := os.Getenv("XDG_RUNTIME_DIR"); v != "" {
		return v
	}
	return "/tmp"
}

// CacheDir returns $XDG_CACHE_HOME or "$HOME/.cache" if unset/empty.
func CacheDir() string {
	if v := os.Getenv("XDG_CACHE_HOME"); v != "" {
		return v
	}
	return filepath.Join(os.Getenv("HOME"), ".cache")
}

// ClaudeCockpitCacheDir is the per-pane status cache directory.
// Layout matches tmux-agent-status (intentional, for future migration option).
func ClaudeCockpitCacheDir() string {
	return filepath.Join(CacheDir(), "claude-cockpit", "panes")
}

// ClaudeNotifyStateDir is the per-session notify replace-id state directory.
func ClaudeNotifyStateDir() string {
	return filepath.Join(RuntimeDir(), "claude-notify", "sessions")
}
