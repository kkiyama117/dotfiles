package main

import (
	"context"
	"testing"

	"claude-tools/internal/proc"
)

func TestFormatBranch_emptyCwd(t *testing.T) {
	got, _ := formatBranch(context.Background(), proc.NewFakeRunner(), "")
	if got != "" {
		t.Fatalf("empty cwd: got %q want empty", got)
	}
}

func TestFormatBranch_normalCase(t *testing.T) {
	r := proc.NewFakeRunner()
	r.Register("git", []string{"-C", "/tmp/repo", "branch", "--show-current"}, []byte("main\n"), nil)
	got, err := formatBranch(context.Background(), r, "/tmp/repo")
	if err != nil || got != "[main] " {
		t.Fatalf("got=%q err=%v, want '[main] '", got, err)
	}
}

func TestFormatBranch_gitFailure_returnsEmpty(t *testing.T) {
	r := proc.NewFakeRunner() // unregistered → returns error
	got, err := formatBranch(context.Background(), r, "/tmp/repo")
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if got != "" {
		t.Fatalf("got=%q want empty", got)
	}
}

func TestFormatBranch_detachedHEAD(t *testing.T) {
	r := proc.NewFakeRunner()
	r.Register("git", []string{"-C", "/tmp/repo", "branch", "--show-current"}, []byte(""), nil)
	got, err := formatBranch(context.Background(), r, "/tmp/repo")
	if err != nil || got != "" {
		t.Fatalf("got=%q err=%v, want empty nil", got, err)
	}
}
