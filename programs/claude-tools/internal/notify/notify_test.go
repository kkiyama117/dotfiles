package notify

import (
	"path/filepath"
	"testing"
)

func TestStateDir_UsesXDGRuntimeDir(t *testing.T) {
	t.Setenv("XDG_RUNTIME_DIR", "/run/user/1000")
	got := StateDir()
	want := filepath.Join("/run/user/1000", "claude-notify", "sessions")
	if got != want {
		t.Errorf("StateDir() = %q, want %q", got, want)
	}
}

func TestStateDir_FallsBackToTmp(t *testing.T) {
	t.Setenv("XDG_RUNTIME_DIR", "")
	got := StateDir()
	want := filepath.Join("/tmp", "claude-notify", "sessions")
	if got != want {
		t.Errorf("StateDir() = %q, want %q (fallback to /tmp)", got, want)
	}
}

// TestSocketPath_UsesXDGRuntimeDir verifies SocketPath returns the correct
// path under XDG_RUNTIME_DIR.
func TestSocketPath_UsesXDGRuntimeDir(t *testing.T) {
	t.Setenv("XDG_RUNTIME_DIR", "/run/user/1000")
	got := SocketPath()
	want := "/run/user/1000/claude-notify/sock"
	if got != want {
		t.Errorf("SocketPath() = %q, want %q", got, want)
	}
}

// TestSocketPath_FallsBackToTmp verifies SocketPath falls back to /tmp when
// XDG_RUNTIME_DIR is unset.
func TestSocketPath_FallsBackToTmp(t *testing.T) {
	t.Setenv("XDG_RUNTIME_DIR", "")
	got := SocketPath()
	want := "/tmp/claude-notify/sock"
	if got != want {
		t.Errorf("SocketPath() = %q, want %q (fallback to /tmp)", got, want)
	}
}
