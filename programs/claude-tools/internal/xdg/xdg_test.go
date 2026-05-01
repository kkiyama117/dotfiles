package xdg

import (
	"path/filepath"
	"testing"
)

func TestRuntimeDir(t *testing.T) {
	tests := []struct {
		name       string
		envRuntime string
		want       string
	}{
		{"env set", "/run/user/1000", "/run/user/1000"},
		{"env empty", "", "/tmp"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("XDG_RUNTIME_DIR", tt.envRuntime)
			if got := RuntimeDir(); got != tt.want {
				t.Errorf("RuntimeDir() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestCacheDir(t *testing.T) {
	t.Run("XDG_CACHE_HOME set", func(t *testing.T) {
		t.Setenv("XDG_CACHE_HOME", "/custom/cache")
		t.Setenv("HOME", "/home/test")
		if got := CacheDir(); got != "/custom/cache" {
			t.Errorf("CacheDir() = %q, want /custom/cache", got)
		}
	})
	t.Run("fallback to HOME/.cache", func(t *testing.T) {
		t.Setenv("XDG_CACHE_HOME", "")
		t.Setenv("HOME", "/home/test")
		want := filepath.Join("/home/test", ".cache")
		if got := CacheDir(); got != want {
			t.Errorf("CacheDir() = %q, want %q", got, want)
		}
	})
}

func TestClaudeCockpitCacheDir(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", "/x/cache")
	want := "/x/cache/claude-cockpit/panes"
	if got := ClaudeCockpitCacheDir(); got != want {
		t.Errorf("ClaudeCockpitCacheDir() = %q, want %q", got, want)
	}
}

func TestClaudeNotifyStateDir(t *testing.T) {
	t.Setenv("XDG_RUNTIME_DIR", "/run/user/1000")
	want := "/run/user/1000/claude-notify/sessions"
	if got := ClaudeNotifyStateDir(); got != want {
		t.Errorf("ClaudeNotifyStateDir() = %q, want %q", got, want)
	}
}
