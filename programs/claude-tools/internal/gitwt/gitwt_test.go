package gitwt

import (
	"context"
	"testing"

	"claude-tools/internal/proc"
)

func TestCurrentBranch_returnsTrimmedBranch(t *testing.T) {
	r := proc.NewFakeRunner()
	r.Register("git", []string{"-C", "/tmp/repo", "branch", "--show-current"}, []byte("feat/x\n"), nil)
	got, err := New(r).CurrentBranch(context.Background(), "/tmp/repo")
	if err != nil || got != "feat/x" {
		t.Fatalf("got=%q err=%v, want feat/x nil", got, err)
	}
}

func TestCurrentBranch_returnsEmptyOnDetached(t *testing.T) {
	r := proc.NewFakeRunner()
	r.Register("git", []string{"-C", "/tmp/repo", "branch", "--show-current"}, []byte(""), nil)
	got, err := New(r).CurrentBranch(context.Background(), "/tmp/repo")
	if err != nil || got != "" {
		t.Fatalf("got=%q err=%v, want empty nil", got, err)
	}
}

func TestCurrentBranch_returnsErrorOnGitFailure(t *testing.T) {
	r := proc.NewFakeRunner() // unregistered call → error
	got, err := New(r).CurrentBranch(context.Background(), "/tmp/repo")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if got != "" {
		t.Fatalf("got=%q want empty on error", got)
	}
}
