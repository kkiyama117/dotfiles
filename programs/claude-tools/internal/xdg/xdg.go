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

// CacheDir returns $XDG_CACHE_HOME or "$HOME/.cache" if XDG_CACHE_HOME
// is unset/empty. Returns "" when HOME is also unset/empty so that the
// caller's write-time error surfaces a missing-environment problem
// rather than silently writing to a relative path.
func CacheDir() string {
	if v := os.Getenv("XDG_CACHE_HOME"); v != "" {
		return v
	}
	home := os.Getenv("HOME")
	if home == "" {
		return ""
	}
	return filepath.Join(home, ".cache")
}

// ClaudeCockpitCacheDir is the per-pane status cache directory.
// Layout matches tmux-agent-status (intentional, for future migration option).
// Returns "" when CacheDir is empty (HOME and XDG_CACHE_HOME both unset)
// so the caller surfaces a missing-environment error rather than writing
// to a relative path under the binary's CWD.
func ClaudeCockpitCacheDir() string {
	base := CacheDir()
	if base == "" {
		return ""
	}
	return filepath.Join(base, "claude-cockpit", "panes")
}

// ClaudeNotifyStateDir is the per-session notify replace-id state directory.
func ClaudeNotifyStateDir() string {
	return filepath.Join(RuntimeDir(), "claude-notify", "sessions")
}

// ConfigDir returns $XDG_CONFIG_HOME or "$HOME/.config" if XDG_CONFIG_HOME
// is unset/empty. Returns "" when HOME is also unset/empty so the caller
// surfaces a missing-environment error rather than building a relative
// path under the binary's CWD.
func ConfigDir() string {
	if v := os.Getenv("XDG_CONFIG_HOME"); v != "" {
		return v
	}
	home := os.Getenv("HOME")
	if home == "" {
		return ""
	}
	return filepath.Join(home, ".config")
}

// LocalBinDir returns "$HOME/.local/bin" or "" when HOME is unset/empty.
// XDG does not specify a user-bin variable, so this is HOME-only by design.
func LocalBinDir() string {
	home := os.Getenv("HOME")
	if home == "" {
		return ""
	}
	return filepath.Join(home, ".local", "bin")
}
