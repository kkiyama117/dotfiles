package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"claude-tools/internal/proc"
)

func TestBadge(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"working", "⚡ working"},
		{"waiting", "⏸ waiting"},
		{"done", "✓ done"},
		{"", ""},
		{"unknown", ""},
	}
	for _, c := range cases {
		if got := badge(c.in); got != c.want {
			t.Errorf("badge(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestStateForPane(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", dir)

	cacheDir := filepath.Join(dir, "claude-cockpit", "panes")
	_ = os.MkdirAll(cacheDir, 0755)
	_ = os.WriteFile(filepath.Join(cacheDir, "sess_%5.status"), []byte("working"), 0644)

	if got := stateForPane("sess", "%5"); got != "working" {
		t.Errorf("stateForPane = %q, want working", got)
	}
	if got := stateForPane("sess", "%nope"); got != "" {
		t.Errorf("stateForPane on missing = %q, want empty", got)
	}
}

func TestStateForSession_priority(t *testing.T) {
	cases := []struct {
		name  string
		panes []string // pane state literals
		want  string
	}{
		{"any working dominates", []string{"done", "waiting", "working"}, "working"},
		{"waiting beats done", []string{"done", "waiting"}, "waiting"},
		{"only done", []string{"done", "done"}, "done"},
		{"all empty", []string{"", ""}, ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := stateForSessionFromPanes(c.panes); got != c.want {
				t.Errorf("got %q, want %q", got, c.want)
			}
		})
	}
}

func TestParseSelection(t *testing.T) {
	row := "P\talpha\t0\t%5\t    pane:%5  cwd=/x    ⚡ working"
	got, err := parseSelection(row)
	if err != nil {
		t.Fatal(err)
	}
	if got.kind != "P" || got.session != "alpha" || got.window != "0" || got.paneID != "%5" {
		t.Errorf("parsed wrongly: %+v", got)
	}
}

func TestBuildLines_emitsTreeOrder(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", dir)
	cacheDir := filepath.Join(dir, "claude-cockpit", "panes")
	_ = os.MkdirAll(cacheDir, 0755)
	_ = os.WriteFile(filepath.Join(cacheDir, "alpha_%1.status"), []byte("working"), 0644)

	fake := proc.NewFakeRunner()
	fake.Register("tmux", []string{"list-sessions", "-F", "#{session_name}"},
		[]byte("alpha\n"), nil)
	fake.Register("tmux", []string{"list-windows", "-t", "alpha", "-F", "#{window_index}\t#{window_name}"},
		[]byte("0\tmain\n"), nil)
	fake.Register("tmux", []string{"list-panes", "-t", "alpha:0", "-F", "#{pane_id}\t#{pane_current_path}"},
		[]byte("%1\t/home/test\n"), nil)
	fake.Register("tmux", []string{"list-panes", "-t", "alpha", "-s", "-F", "#{pane_id}"},
		[]byte("%1\n"), nil)

	lines, err := buildLines(context.Background(), fake)
	if err != nil {
		t.Fatal(err)
	}
	if len(lines) != 3 {
		t.Fatalf("got %d lines, want 3 (S/W/P): %v", len(lines), lines)
	}
	if !strings.HasPrefix(lines[0], "S\talpha\t") {
		t.Errorf("line[0] not S row: %q", lines[0])
	}
	if !strings.HasPrefix(lines[1], "W\talpha\t0\t") {
		t.Errorf("line[1] not W row: %q", lines[1])
	}
	if !strings.HasPrefix(lines[2], "P\talpha\t0\t%1\t") {
		t.Errorf("line[2] not P row: %q", lines[2])
	}
}
