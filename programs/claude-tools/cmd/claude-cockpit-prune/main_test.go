package main

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"claude-tools/internal/proc"
)

// liveClaudeFormat mirrors cockpit.LoadLiveClaudePanes' tmux -F string.
const liveClaudeFormat = "#{session_name}_#{pane_id}\t#{pane_current_command}"

func TestPrune_removesOrphans(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", dir)

	cacheDir := filepath.Join(dir, "claude-cockpit", "panes")
	_ = os.MkdirAll(cacheDir, 0755)

	// 3 cache files: 2 live claude, 1 orphan (pane gone).
	live1 := filepath.Join(cacheDir, "sess_%1.status")
	live2 := filepath.Join(cacheDir, "sess_%2.status")
	orphan := filepath.Join(cacheDir, "sess_%99.status")
	_ = os.WriteFile(live1, []byte("working"), 0644)
	_ = os.WriteFile(live2, []byte("done"), 0644)
	_ = os.WriteFile(orphan, []byte("done"), 0644)

	fake := proc.NewFakeRunner()
	fake.Register("tmux", []string{"list-panes", "-a", "-F", liveClaudeFormat},
		[]byte("sess_%1\tclaude\nsess_%2\tclaude\n"), nil)

	if err := prune(context.Background(), fake); err != nil {
		t.Fatalf("prune: %v", err)
	}

	if _, err := os.Stat(live1); err != nil {
		t.Errorf("live1 was deleted: %v", err)
	}
	if _, err := os.Stat(live2); err != nil {
		t.Errorf("live2 was deleted: %v", err)
	}
	if _, err := os.Stat(orphan); !os.IsNotExist(err) {
		t.Errorf("orphan was NOT deleted (Stat err = %v)", err)
	}
}

// F-8 (c): prune must also remove .status files for panes that still
// exist as tmux panes but are no longer running claude (the user
// /exit'd and now the pane runs zsh, or a script reused the pane).
// Without this, the file lingers indefinitely until the pane itself
// disappears.
func TestPrune_removesNonClaudeLivePane(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", dir)

	cacheDir := filepath.Join(dir, "claude-cockpit", "panes")
	_ = os.MkdirAll(cacheDir, 0755)

	// All three panes exist in tmux, but only one still runs claude.
	stillClaude := filepath.Join(cacheDir, "sess_%1.status")
	nowZsh := filepath.Join(cacheDir, "sess_%2.status")
	nowVim := filepath.Join(cacheDir, "sess_%3.status")
	_ = os.WriteFile(stillClaude, []byte("working"), 0644)
	_ = os.WriteFile(nowZsh, []byte("done"), 0644) // stale: claude /exited
	_ = os.WriteFile(nowVim, []byte("done"), 0644) // stale: pane now editing a file

	fake := proc.NewFakeRunner()
	fake.Register("tmux", []string{"list-panes", "-a", "-F", liveClaudeFormat},
		[]byte("sess_%1\tclaude\nsess_%2\tzsh\nsess_%3\tvim\n"), nil)

	if err := prune(context.Background(), fake); err != nil {
		t.Fatalf("prune: %v", err)
	}

	if _, err := os.Stat(stillClaude); err != nil {
		t.Errorf("stillClaude was deleted: %v", err)
	}
	if _, err := os.Stat(nowZsh); !os.IsNotExist(err) {
		t.Errorf("nowZsh stale file was NOT deleted (Stat err = %v)", err)
	}
	if _, err := os.Stat(nowVim); !os.IsNotExist(err) {
		t.Errorf("nowVim stale file was NOT deleted (Stat err = %v)", err)
	}
}

func TestPrune_emptyDir(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", dir)

	fake := proc.NewFakeRunner()
	fake.Register("tmux", []string{"list-panes", "-a", "-F", liveClaudeFormat},
		[]byte(""), nil)

	if err := prune(context.Background(), fake); err != nil {
		t.Errorf("prune on missing dir should not error: %v", err)
	}
}

func TestPrune_tmuxFailure(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", dir)

	cacheDir := filepath.Join(dir, "claude-cockpit", "panes")
	_ = os.MkdirAll(cacheDir, 0755)
	cached := filepath.Join(cacheDir, "sess_%1.status")
	_ = os.WriteFile(cached, []byte("done"), 0644)

	fake := proc.NewFakeRunner() // tmux not registered → error

	err := prune(context.Background(), fake)
	if err == nil {
		t.Fatal("prune should propagate tmux failure")
	}

	// Cached file must NOT be deleted on tmux failure (safety: no live set means we can't tell what's orphan).
	if _, statErr := os.Stat(cached); os.IsNotExist(statErr) {
		t.Error("cached file deleted despite tmux failure (unsafe)")
	}
}
