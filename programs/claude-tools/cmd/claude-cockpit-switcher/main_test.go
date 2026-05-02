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

	if got := stateForPane("sess", "%5", "claude"); got != "working" {
		t.Errorf("stateForPane = %q, want working", got)
	}
	if got := stateForPane("sess", "%nope", "claude"); got != "" {
		t.Errorf("stateForPane on missing = %q, want empty", got)
	}
}

// F-8 (b3): when the pane is no longer running claude, stateForPane
// must return "" even if the cache file still says "working". The
// switcher row is still rendered (handled in buildLines), but the
// badge column blanks out so the user can tell the pane is stale.
func TestStateForPane_blankWhenNotClaude(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", dir)

	cacheDir := filepath.Join(dir, "claude-cockpit", "panes")
	_ = os.MkdirAll(cacheDir, 0755)
	_ = os.WriteFile(filepath.Join(cacheDir, "sess_%5.status"), []byte("working"), 0644)

	if got := stateForPane("sess", "%5", "zsh"); got != "" {
		t.Errorf("stateForPane on non-claude pane = %q, want empty", got)
	}
	if got := stateForPane("sess", "%5", ""); got != "" {
		t.Errorf("stateForPane on unknown cmd = %q, want empty", got)
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
	fake.Register("tmux",
		[]string{"list-panes", "-t", "alpha:0", "-F",
			"#{pane_id}\t#{pane_current_path}\t#{pane_current_command}"},
		[]byte("%1\t/home/test\tclaude\n"), nil)
	fake.Register("tmux",
		[]string{"list-panes", "-t", "alpha", "-s", "-F",
			"#{pane_id}\t#{pane_current_command}"},
		[]byte("%1\tclaude\n"), nil)

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
	// Pane cmd is claude → badge column populated.
	if !strings.Contains(lines[2], "⚡ working") {
		t.Errorf("expected ⚡ working badge on live claude pane: %q", lines[2])
	}
}

// recordingRunner records every Run invocation (name + args) and always
// returns success. Unlike proc.FakeRunner this does not require pre-
// registration, which keeps dispatchSwitch tests focused on argv shape.
type recordingRunner struct {
	calls [][]string
}

func (r *recordingRunner) Run(_ context.Context, name string, args ...string) ([]byte, error) {
	r.calls = append(r.calls, append([]string{name}, args...))
	return nil, nil
}

func eqStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestDispatchSwitch(t *testing.T) {
	cases := []struct {
		name string
		row  selectedRow
		want []string
	}{
		{
			name: "S row switches client only",
			row:  selectedRow{kind: "S", session: "alpha"},
			want: []string{"tmux", "switch-client", "-t", "alpha"},
		},
		{
			name: "W row also selects window",
			row:  selectedRow{kind: "W", session: "alpha", window: "0"},
			want: []string{"tmux", "switch-client", "-t", "alpha",
				";", "select-window", "-t", "alpha:0"},
		},
		{
			name: "P row also selects pane",
			row:  selectedRow{kind: "P", session: "alpha", window: "0", paneID: "%5"},
			want: []string{"tmux", "switch-client", "-t", "alpha",
				";", "select-window", "-t", "alpha:0",
				";", "select-pane", "-t", "%5"},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			r := &recordingRunner{}
			dispatchSwitch(context.Background(), r, c.row)
			if len(r.calls) != 1 {
				t.Fatalf("expected 1 tmux call, got %d: %v", len(r.calls), r.calls)
			}
			if !eqStrings(r.calls[0], c.want) {
				t.Errorf("argv = %v, want %v", r.calls[0], c.want)
			}
		})
	}
}

func TestDispatchSwitch_unknownKindIsNoOp(t *testing.T) {
	r := &recordingRunner{}
	dispatchSwitch(context.Background(), r, selectedRow{kind: "X"})
	if len(r.calls) != 0 {
		t.Errorf("unknown kind should not invoke tmux, got %v", r.calls)
	}
}

// F-8 (b3): when a pane has a status file but its current command is
// no longer "claude", buildLines must still emit the row (so the user
// can switch to it / kill it from the switcher) but with a blank badge.
// This protects against the cockpit listing phantom claude states.
func TestBuildLines_blankBadgeForNonClaudePane(t *testing.T) {
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
	fake.Register("tmux",
		[]string{"list-panes", "-t", "alpha:0", "-F",
			"#{pane_id}\t#{pane_current_path}\t#{pane_current_command}"},
		[]byte("%1\t/home/test\tzsh\n"), nil) // cmd != claude
	fake.Register("tmux",
		[]string{"list-panes", "-t", "alpha", "-s", "-F",
			"#{pane_id}\t#{pane_current_command}"},
		[]byte("%1\tzsh\n"), nil)

	lines, err := buildLines(context.Background(), fake)
	if err != nil {
		t.Fatal(err)
	}
	if len(lines) != 3 {
		t.Fatalf("got %d lines, want 3 (S/W/P): %v", len(lines), lines)
	}
	pRow := lines[2]
	if !strings.HasPrefix(pRow, "P\talpha\t0\t%1\t") {
		t.Errorf("P row not emitted: %q", pRow)
	}
	if strings.Contains(pRow, "⚡") || strings.Contains(pRow, "⏸") || strings.Contains(pRow, "✓") {
		t.Errorf("non-claude pane should have blank badge, got: %q", pRow)
	}
}
