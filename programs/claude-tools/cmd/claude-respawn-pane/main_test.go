package main

import (
	"context"
	"testing"

	"claude-tools/internal/proc"
)

func TestPickTargetPane_findsClaudePane(t *testing.T) {
	r := proc.NewFakeRunner()
	r.Register("tmux", []string{"display-message", "-p", "#S"}, []byte("dev\n"), nil)
	r.Register("tmux", []string{"list-panes", "-t", "dev", "-F", "#{pane_id} #{pane_current_command}"},
		[]byte("%1 zsh\n%2 claude\n"), nil)
	got, err := pickTargetPane(context.Background(), r)
	if err != nil || got != "%2" {
		t.Fatalf("got=%q err=%v want %%2", got, err)
	}
}

func TestPickTargetPane_fallsBackToCurrentPane(t *testing.T) {
	r := proc.NewFakeRunner()
	r.Register("tmux", []string{"display-message", "-p", "#S"}, []byte("dev\n"), nil)
	r.Register("tmux", []string{"list-panes", "-t", "dev", "-F", "#{pane_id} #{pane_current_command}"},
		[]byte("%1 zsh\n%2 vim\n"), nil)
	r.Register("tmux", []string{"display-message", "-p", "#{pane_id}"}, []byte("%1\n"), nil)
	got, err := pickTargetPane(context.Background(), r)
	if err != nil || got != "%1" {
		t.Fatalf("got=%q err=%v want %%1", got, err)
	}
}
