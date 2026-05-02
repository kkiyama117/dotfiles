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
	t.Run("both unset returns empty", func(t *testing.T) {
		t.Setenv("XDG_CACHE_HOME", "")
		t.Setenv("HOME", "")
		if got := CacheDir(); got != "" {
			t.Errorf("CacheDir() = %q, want empty string when HOME and XDG_CACHE_HOME are both unset", got)
		}
	})
}

func TestClaudeCockpitCacheDir(t *testing.T) {
	t.Run("XDG_CACHE_HOME set", func(t *testing.T) {
		t.Setenv("XDG_CACHE_HOME", "/x/cache")
		want := "/x/cache/claude-cockpit/panes"
		if got := ClaudeCockpitCacheDir(); got != want {
			t.Errorf("ClaudeCockpitCacheDir() = %q, want %q", got, want)
		}
	})
	t.Run("both unset returns empty", func(t *testing.T) {
		t.Setenv("XDG_CACHE_HOME", "")
		t.Setenv("HOME", "")
		if got := ClaudeCockpitCacheDir(); got != "" {
			t.Errorf("ClaudeCockpitCacheDir() = %q, want empty string when CacheDir is empty", got)
		}
	})
}

func TestClaudeNotifyStateDir(t *testing.T) {
	t.Setenv("XDG_RUNTIME_DIR", "/run/user/1000")
	want := "/run/user/1000/claude-notify/sessions"
	if got := ClaudeNotifyStateDir(); got != want {
		t.Errorf("ClaudeNotifyStateDir() = %q, want %q", got, want)
	}
}

func TestConfigDir(t *testing.T) {
	t.Run("XDG_CONFIG_HOME set", func(t *testing.T) {
		t.Setenv("XDG_CONFIG_HOME", "/custom/config")
		t.Setenv("HOME", "/home/test")
		if got := ConfigDir(); got != "/custom/config" {
			t.Errorf("ConfigDir() = %q, want /custom/config", got)
		}
	})
	t.Run("fallback to HOME/.config", func(t *testing.T) {
		t.Setenv("XDG_CONFIG_HOME", "")
		t.Setenv("HOME", "/home/test")
		want := filepath.Join("/home/test", ".config")
		if got := ConfigDir(); got != want {
			t.Errorf("ConfigDir() = %q, want %q", got, want)
		}
	})
	t.Run("both unset returns empty", func(t *testing.T) {
		t.Setenv("XDG_CONFIG_HOME", "")
		t.Setenv("HOME", "")
		if got := ConfigDir(); got != "" {
			t.Errorf("ConfigDir() = %q, want empty string when HOME and XDG_CONFIG_HOME are both unset", got)
		}
	})
}

func TestLocalBinDir(t *testing.T) {
	t.Run("HOME set", func(t *testing.T) {
		t.Setenv("HOME", "/home/test")
		want := filepath.Join("/home/test", ".local", "bin")
		if got := LocalBinDir(); got != want {
			t.Errorf("LocalBinDir() = %q, want %q", got, want)
		}
	})
	t.Run("HOME unset returns empty", func(t *testing.T) {
		t.Setenv("HOME", "")
		if got := LocalBinDir(); got != "" {
			t.Errorf("LocalBinDir() = %q, want empty string when HOME is unset", got)
		}
	})
}
