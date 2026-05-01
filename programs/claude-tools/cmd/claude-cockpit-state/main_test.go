package main

import (
	"context"
	"os"
	"testing"

	"claude-tools/internal/proc"
)

func TestEventToStatus(t *testing.T) {
	tests := []struct {
		event string
		want  string
		ok    bool
	}{
		{"UserPromptSubmit", "working", true},
		{"PreToolUse", "working", true},
		{"Notification", "waiting", true},
		{"Stop", "done", true},
		{"SubagentStop", "", false},
		{"", "", false},
		{"unknown", "", false},
	}
	for _, tt := range tests {
		got, ok := eventToStatus(tt.event)
		if got != tt.want || ok != tt.ok {
			t.Errorf("eventToStatus(%q) = (%q, %v), want (%q, %v)",
				tt.event, got, ok, tt.want, tt.ok)
		}
	}
}

func TestRun_modeNotHook(t *testing.T) {
	if err := run(context.Background(), proc.NewFakeRunner(), []string{"prog", "other-mode"}, "%1"); err != nil {
		t.Errorf("run with non-hook mode returned error: %v", err)
	}
}

func TestRun_emptyTmuxPane(t *testing.T) {
	if err := run(context.Background(), proc.NewFakeRunner(), []string{"prog", "hook", "Stop"}, ""); err != nil {
		t.Errorf("run with empty TMUX_PANE returned error: %v", err)
	}
}

func TestRun_unknownEvent(t *testing.T) {
	if err := run(context.Background(), proc.NewFakeRunner(), []string{"prog", "hook", "SubagentStop"}, "%1"); err != nil {
		t.Errorf("run with unknown event returned error: %v", err)
	}
}

func TestRun_sessionLookupFailure(t *testing.T) {
	fake := proc.NewFakeRunner()
	// Don't register tmux: lookup will fail. run() should still complete.
	err := run(context.Background(), fake, []string{"prog", "hook", "Stop"}, "%1")
	// Returns nil because hook contract is exit 0; inner errors are logged not bubbled.
	if err != nil {
		t.Errorf("run with tmux failure returned error: %v", err)
	}
}

func TestRun_writesStatus(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", dir)

	fake := proc.NewFakeRunner()
	fake.Register("tmux", []string{"display-message", "-p", "-t", "%5", "#{session_name}"},
		[]byte("mysession\n"), nil)

	if err := run(context.Background(), fake, []string{"prog", "hook", "UserPromptSubmit"}, "%5"); err != nil {
		t.Errorf("run returned error: %v", err)
	}

	want := dir + "/claude-cockpit/panes/mysession_%5.status"
	data, err := os.ReadFile(want)
	if err != nil {
		t.Fatalf("read cache file %q: %v", want, err)
	}
	if string(data) != "working" {
		t.Errorf("cache content = %q, want %q", data, "working")
	}
}
