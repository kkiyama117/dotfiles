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
