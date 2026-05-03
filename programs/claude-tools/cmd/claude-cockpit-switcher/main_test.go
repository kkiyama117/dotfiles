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

// recordingRunner records every Run invocation (name + args). Optional
// per-argv responses can be set via register; calls without a registered
// argv return (nil, nil). Unlike proc.FakeRunner this never errors on
// unregistered calls, which keeps dispatch* tests focused on argv shape.
type recordingRunner struct {
	calls     [][]string
	responses []recordedResponse
}

type recordedResponse struct {
	name string
	args []string
	out  []byte
	err  error
}

func (r *recordingRunner) register(name string, args []string, out []byte, err error) {
	r.responses = append(r.responses, recordedResponse{name, args, out, err})
}

func (r *recordingRunner) Run(_ context.Context, name string, args ...string) ([]byte, error) {
	// Copy args because the slice may be reused by the caller.
	argsCopy := append([]string(nil), args...)
	r.calls = append(r.calls, append([]string{name}, argsCopy...))
	for _, rr := range r.responses {
		if rr.name == name && eqStrings(rr.args, args) {
			return rr.out, rr.err
		}
	}
	return nil, nil
}

// fakePrompter records every prompt and returns a canned answer.
type fakePrompter struct {
	answer bool
	asked  []string
}

func (f *fakePrompter) Confirm(prompt string) bool {
	f.asked = append(f.asked, prompt)
	return f.answer
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

// G-1.next #7 (L-2): dispatchKill paths exercise the prompter +
// runner injection points. Coverage matrix:
//   P (yes)     : single tmux kill-pane after prompt
//   P (no)      : prompter declines, no tmux call
//   W unmanaged : prompter is asked the unmanaged question, kill-window
//                 fires only on yes
//   W managed   : prompter is asked the managed question (with worktree
//                 wording); the YES branch shells out to the external
//                 claude-kill-session.sh script and is NOT covered here
//                 (interactive smoke covers it — see todos.md F-3.next)
//   S (yes/no)  : single tmux kill-session after prompt
func TestDispatchKill_paneAccept(t *testing.T) {
	r := &recordingRunner{}
	p := &fakePrompter{answer: true}
	dispatchKill(context.Background(), r, p, selectedRow{kind: "P", paneID: "%5"})

	if len(p.asked) != 1 || !strings.Contains(p.asked[0], "kill pane %5") {
		t.Errorf("prompt mismatch: %v", p.asked)
	}
	want := []string{"tmux", "kill-pane", "-t", "%5"}
	if len(r.calls) != 1 || !eqStrings(r.calls[0], want) {
		t.Errorf("argv = %v, want %v", r.calls, want)
	}
}

func TestDispatchKill_paneDecline(t *testing.T) {
	r := &recordingRunner{}
	p := &fakePrompter{answer: false}
	dispatchKill(context.Background(), r, p, selectedRow{kind: "P", paneID: "%5"})

	if len(p.asked) != 1 || !strings.Contains(p.asked[0], "kill pane %5") {
		t.Errorf("prompt mismatch: %v", p.asked)
	}
	if len(r.calls) != 0 {
		t.Errorf("decline should not invoke tmux, got %v", r.calls)
	}
}

func TestDispatchKill_unmanagedWindowAccept(t *testing.T) {
	r := &recordingRunner{}
	r.register("tmux",
		[]string{"show-options", "-w", "-t", "alpha:0", "-v", "@claude-managed"},
		[]byte(""), nil)
	p := &fakePrompter{answer: true}
	dispatchKill(context.Background(), r, p, selectedRow{kind: "W", session: "alpha", window: "0"})

	if len(p.asked) != 1 || !strings.Contains(p.asked[0], "kill window alpha:0") {
		t.Errorf("expected unmanaged window prompt, got: %v", p.asked)
	}
	// Calls: [show-options, kill-window]
	if len(r.calls) != 2 {
		t.Fatalf("expected 2 tmux calls (show-options + kill-window), got %d: %v",
			len(r.calls), r.calls)
	}
	want := []string{"tmux", "kill-window", "-t", "alpha:0"}
	if !eqStrings(r.calls[1], want) {
		t.Errorf("kill-window argv = %v, want %v", r.calls[1], want)
	}
}

func TestDispatchKill_unmanagedWindowDecline(t *testing.T) {
	r := &recordingRunner{}
	r.register("tmux",
		[]string{"show-options", "-w", "-t", "alpha:0", "-v", "@claude-managed"},
		[]byte(""), nil)
	p := &fakePrompter{answer: false}
	dispatchKill(context.Background(), r, p, selectedRow{kind: "W", session: "alpha", window: "0"})

	if len(p.asked) != 1 || !strings.Contains(p.asked[0], "kill window alpha:0") {
		t.Errorf("expected unmanaged window prompt, got: %v", p.asked)
	}
	// Only show-options should fire; kill-window must not.
	if len(r.calls) != 1 {
		t.Errorf("expected only show-options, got %v", r.calls)
	}
}

func TestDispatchKill_managedWindowDecline(t *testing.T) {
	r := &recordingRunner{}
	r.register("tmux",
		[]string{"show-options", "-w", "-t", "alpha:0", "-v", "@claude-managed"},
		[]byte("yes\n"), nil)
	p := &fakePrompter{answer: false}
	dispatchKill(context.Background(), r, p, selectedRow{kind: "W", session: "alpha", window: "0"})

	if len(p.asked) != 1 || !strings.Contains(p.asked[0], "kill claude window alpha:0 and worktree") {
		t.Errorf("expected managed window prompt with worktree wording, got: %v", p.asked)
	}
	// Decline must short-circuit before any kill happens; only show-options
	// shows up. The accept path delegates to claude-kill-session.sh and is
	// covered by interactive smoke (see todos.md F-3.next).
	if len(r.calls) != 1 {
		t.Errorf("expected only show-options on decline, got %v", r.calls)
	}
}

func TestDispatchKill_sessionAccept(t *testing.T) {
	r := &recordingRunner{}
	p := &fakePrompter{answer: true}
	dispatchKill(context.Background(), r, p, selectedRow{kind: "S", session: "alpha"})

	if len(p.asked) != 1 || !strings.Contains(p.asked[0], "kill session alpha") {
		t.Errorf("prompt mismatch: %v", p.asked)
	}
	want := []string{"tmux", "kill-session", "-t", "alpha"}
	if len(r.calls) != 1 || !eqStrings(r.calls[0], want) {
		t.Errorf("argv = %v, want %v", r.calls, want)
	}
}

func TestDispatchKill_sessionDecline(t *testing.T) {
	r := &recordingRunner{}
	p := &fakePrompter{answer: false}
	dispatchKill(context.Background(), r, p, selectedRow{kind: "S", session: "alpha"})

	if len(p.asked) != 1 || !strings.Contains(p.asked[0], "kill session alpha") {
		t.Errorf("prompt mismatch: %v", p.asked)
	}
	if len(r.calls) != 0 {
		t.Errorf("decline should not invoke tmux, got %v", r.calls)
	}
}

func TestDispatchKill_unknownKindIsNoOp(t *testing.T) {
	r := &recordingRunner{}
	p := &fakePrompter{answer: true}
	dispatchKill(context.Background(), r, p, selectedRow{kind: "X"})

	if len(p.asked) != 0 {
		t.Errorf("unknown kind should not prompt, got %v", p.asked)
	}
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
