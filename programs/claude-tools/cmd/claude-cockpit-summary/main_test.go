package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"

	"claude-tools/internal/proc"
)

// liveAll registers a fake `tmux list-panes -a` call advertising every
// passed key as a live claude pane. Use in tests where the live-claude
// filter is not the focus.
func liveAll(fake *proc.FakeRunner, keys ...string) {
	var sb bytes.Buffer
	for _, k := range keys {
		sb.WriteString(k)
		sb.WriteString("\tclaude\n")
	}
	fake.Register("tmux",
		[]string{"list-panes", "-a", "-F", "#{session_name}_#{pane_id}\t#{pane_current_command}"},
		sb.Bytes(), nil)
}

func TestSummary_emptyDir(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", dir)

	fake := proc.NewFakeRunner()
	liveAll(fake) // tmux returns no panes

	var buf bytes.Buffer
	if err := writeSummary(context.Background(), fake, &buf); err != nil {
		t.Fatalf("writeSummary: %v", err)
	}
	if buf.String() != "" {
		t.Errorf("output = %q, want empty", buf.String())
	}
}

func TestSummary_byteExact(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", dir)

	cacheDir := filepath.Join(dir, "claude-cockpit", "panes")
	_ = os.MkdirAll(cacheDir, 0755)

	_ = os.WriteFile(filepath.Join(cacheDir, "s1_%1.status"), []byte("working"), 0644)
	_ = os.WriteFile(filepath.Join(cacheDir, "s1_%2.status"), []byte("working"), 0644)
	_ = os.WriteFile(filepath.Join(cacheDir, "s1_%3.status"), []byte("working"), 0644)
	_ = os.WriteFile(filepath.Join(cacheDir, "s2_%4.status"), []byte("waiting"), 0644)
	_ = os.WriteFile(filepath.Join(cacheDir, "s2_%5.status"), []byte("done"), 0644)
	_ = os.WriteFile(filepath.Join(cacheDir, "s2_%6.status"), []byte("done"), 0644)

	fake := proc.NewFakeRunner()
	liveAll(fake, "s1_%1", "s1_%2", "s1_%3", "s2_%4", "s2_%5", "s2_%6")

	var buf bytes.Buffer
	if err := writeSummary(context.Background(), fake, &buf); err != nil {
		t.Fatalf("writeSummary: %v", err)
	}
	want := "⚡ 3 ⏸ 1 ✓ 2 "
	if buf.String() != want {
		t.Errorf("output = %q, want %q", buf.String(), want)
	}
}

// F-8 (b1): a stale status file whose pane is no longer running claude
// must NOT be counted in the summary. Here "s2_%5" still has a "done"
// status file but the live tmux pane runs zsh, so the ✓ count drops
// from 2 to 1.
func TestSummary_filtersStaleNonClaudePane(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", dir)

	cacheDir := filepath.Join(dir, "claude-cockpit", "panes")
	_ = os.MkdirAll(cacheDir, 0755)
	_ = os.WriteFile(filepath.Join(cacheDir, "s1_%1.status"), []byte("working"), 0644)
	_ = os.WriteFile(filepath.Join(cacheDir, "s2_%5.status"), []byte("done"), 0644) // stale: pane no longer claude
	_ = os.WriteFile(filepath.Join(cacheDir, "s2_%6.status"), []byte("done"), 0644)

	fake := proc.NewFakeRunner()
	fake.Register("tmux",
		[]string{"list-panes", "-a", "-F", "#{session_name}_#{pane_id}\t#{pane_current_command}"},
		[]byte("s1_%1\tclaude\ns2_%5\tzsh\ns2_%6\tclaude\n"), nil)

	var buf bytes.Buffer
	if err := writeSummary(context.Background(), fake, &buf); err != nil {
		t.Fatalf("writeSummary: %v", err)
	}
	want := "⚡ 1 ✓ 1 " // s2_%5 dropped (zsh)
	if buf.String() != want {
		t.Errorf("output = %q, want %q", buf.String(), want)
	}
}

// Tmux failure is treated as fail-closed (empty live set → empty
// summary). Matches shell summary.sh behaviour from F-8 v1.
func TestSummary_tmuxFailureFailsClosed(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", dir)
	cacheDir := filepath.Join(dir, "claude-cockpit", "panes")
	_ = os.MkdirAll(cacheDir, 0755)
	_ = os.WriteFile(filepath.Join(cacheDir, "s1_%1.status"), []byte("working"), 0644)

	fake := proc.NewFakeRunner() // tmux NOT registered → error

	var buf bytes.Buffer
	if err := writeSummary(context.Background(), fake, &buf); err != nil {
		t.Fatalf("writeSummary should fail-closed (no error): %v", err)
	}
	if buf.String() != "" {
		t.Errorf("output = %q, want empty (fail-closed)", buf.String())
	}
}
