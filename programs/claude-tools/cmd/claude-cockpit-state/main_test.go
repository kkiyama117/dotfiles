package main

import (
	"context"
	"os"
	"testing"

	"claude-tools/internal/proc"
)

func TestEventToAction(t *testing.T) {
	tests := []struct {
		event       string
		wantAction  hookAction
		wantPayload string
		wantOK      bool
	}{
		{"UserPromptSubmit", actionWrite, "working", true},
		{"PreToolUse", actionWrite, "working", true},
		{"Notification", actionWrite, "waiting", true},
		{"Stop", actionWrite, "done", true},
		{"SessionEnd", actionRemove, "", true},
		{"SubagentStop", actionNone, "", false},
		{"", actionNone, "", false},
		{"unknown", actionNone, "", false},
	}
	for _, tt := range tests {
		gotAction, gotPayload, gotOK := eventToAction(tt.event)
		if gotAction != tt.wantAction || gotPayload != tt.wantPayload || gotOK != tt.wantOK {
			t.Errorf("eventToAction(%q) = (%v, %q, %v), want (%v, %q, %v)",
				tt.event, gotAction, gotPayload, gotOK,
				tt.wantAction, tt.wantPayload, tt.wantOK)
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

func TestRun_sessionEndRemovesStatus(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", dir)

	// Seed: write a working status, then simulate SessionEnd → file gone.
	cacheDir := dir + "/claude-cockpit/panes"
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		t.Fatal(err)
	}
	statusFile := cacheDir + "/mysession_%5.status"
	if err := os.WriteFile(statusFile, []byte("working"), 0644); err != nil {
		t.Fatal(err)
	}

	fake := proc.NewFakeRunner()
	fake.Register("tmux", []string{"display-message", "-p", "-t", "%5", "#{session_name}"},
		[]byte("mysession\n"), nil)

	if err := run(context.Background(), fake, []string{"prog", "hook", "SessionEnd"}, "%5"); err != nil {
		t.Errorf("run returned error: %v", err)
	}

	if _, err := os.Stat(statusFile); !os.IsNotExist(err) {
		t.Errorf("status file should be removed after SessionEnd (Stat err = %v)", err)
	}
}

func TestRun_sessionEndOnMissingFileIsNoOp(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", dir)

	fake := proc.NewFakeRunner()
	fake.Register("tmux", []string{"display-message", "-p", "-t", "%9", "#{session_name}"},
		[]byte("mysession\n"), nil)

	// SessionEnd on a session that never had a hook fire: must not error.
	if err := run(context.Background(), fake, []string{"prog", "hook", "SessionEnd"}, "%9"); err != nil {
		t.Errorf("run on missing status file returned error: %v", err)
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
