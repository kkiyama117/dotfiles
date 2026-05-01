package main

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"claude-tools/internal/proc"
)

func TestBuildDoneList_orderAndCycle(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", dir)

	cacheDir := filepath.Join(dir, "claude-cockpit", "panes")
	_ = os.MkdirAll(cacheDir, 0755)

	// 3 done panes across 2 sessions.
	_ = os.WriteFile(filepath.Join(cacheDir, "alpha_%1.status"), []byte("done"), 0644)
	_ = os.WriteFile(filepath.Join(cacheDir, "alpha_%3.status"), []byte("done"), 0644)
	_ = os.WriteFile(filepath.Join(cacheDir, "beta_%2.status"), []byte("done"), 0644)
	// Working: should NOT appear.
	_ = os.WriteFile(filepath.Join(cacheDir, "alpha_%2.status"), []byte("working"), 0644)

	fake := proc.NewFakeRunner()
	fake.Register("tmux", []string{"list-sessions", "-F", "#{session_name}"},
		[]byte("alpha\nbeta\n"), nil)
	fake.Register("tmux", []string{"list-windows", "-t", "alpha", "-F", "#{window_index}"},
		[]byte("0\n"), nil)
	fake.Register("tmux", []string{"list-windows", "-t", "beta", "-F", "#{window_index}"},
		[]byte("0\n"), nil)
	fake.Register("tmux",
		[]string{"list-panes", "-t", "alpha:0", "-F",
			"#{pane_id}\t#{pane_index}\t#{pane_current_command}"},
		[]byte("%1\t0\tclaude\n%2\t1\tclaude\n%3\t2\tclaude\n"), nil)
	fake.Register("tmux",
		[]string{"list-panes", "-t", "beta:0", "-F",
			"#{pane_id}\t#{pane_index}\t#{pane_current_command}"},
		[]byte("%2\t0\tclaude\n"), nil)

	got, err := buildDoneList(context.Background(), fake)
	if err != nil {
		t.Fatalf("buildDoneList: %v", err)
	}
	want := []doneRow{
		{session: "alpha", window: "0", paneID: "%1"},
		{session: "alpha", window: "0", paneID: "%3"},
		{session: "beta", window: "0", paneID: "%2"},
	}
	if len(got) != len(want) {
		t.Fatalf("got %d rows, want %d: %+v", len(got), len(want), got)
	}
	for i := range got {
		if got[i] != want[i] {
			t.Errorf("row[%d] = %+v, want %+v", i, got[i], want[i])
		}
	}
}

// F-8 (b2): a pane whose pane_current_command is no longer "claude"
// must NOT appear in the done list, even if its cache file still says
// "done" (claude was killed by SIGKILL / OOM / pane closed without
// /exit, leaving a phantom "done" entry).
func TestBuildDoneList_skipsNonClaudePane(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", dir)

	cacheDir := filepath.Join(dir, "claude-cockpit", "panes")
	_ = os.MkdirAll(cacheDir, 0755)

	// Both panes have a "done" cache file, but only one is still claude.
	_ = os.WriteFile(filepath.Join(cacheDir, "alpha_%1.status"), []byte("done"), 0644)
	_ = os.WriteFile(filepath.Join(cacheDir, "alpha_%2.status"), []byte("done"), 0644) // stale: now zsh

	fake := proc.NewFakeRunner()
	fake.Register("tmux", []string{"list-sessions", "-F", "#{session_name}"},
		[]byte("alpha\n"), nil)
	fake.Register("tmux", []string{"list-windows", "-t", "alpha", "-F", "#{window_index}"},
		[]byte("0\n"), nil)
	fake.Register("tmux",
		[]string{"list-panes", "-t", "alpha:0", "-F",
			"#{pane_id}\t#{pane_index}\t#{pane_current_command}"},
		[]byte("%1\t0\tclaude\n%2\t1\tzsh\n"), nil)

	got, err := buildDoneList(context.Background(), fake)
	if err != nil {
		t.Fatalf("buildDoneList: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("got %d rows, want 1 (only the live claude pane): %+v", len(got), got)
	}
	if got[0].paneID != "%1" {
		t.Errorf("kept the wrong pane: %+v (expected %%1)", got[0])
	}
}

func TestPickNext_cycles(t *testing.T) {
	rows := []doneRow{
		{session: "a", window: "0", paneID: "%1"},
		{session: "a", window: "0", paneID: "%2"},
		{session: "b", window: "0", paneID: "%3"},
	}
	cases := []struct {
		cur  string
		want string
	}{
		{"%1", "%2"},
		{"%2", "%3"},
		{"%3", "%1"},  // wrap
		{"%99", "%1"}, // not in list -> first
	}
	for _, c := range cases {
		got := pickNext(rows, c.cur)
		if got.paneID != c.want {
			t.Errorf("pickNext(cur=%s) = %s, want %s", c.cur, got.paneID, c.want)
		}
	}
}

func TestPickNext_emptyList(t *testing.T) {
	got := pickNext(nil, "%1")
	if got != (doneRow{}) {
		t.Errorf("empty list = %+v, want zero", got)
	}
}
