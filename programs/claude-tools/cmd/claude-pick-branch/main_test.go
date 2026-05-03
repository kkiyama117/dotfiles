package main

import (
	"reflect"
	"testing"
)

func TestBuildExecArgs_table(t *testing.T) {
	cases := []struct {
		name        string
		branch      string
		passthrough []string
		want        []string
	}{
		{"branch only", "feat/x", nil, []string{"claude-tmux-new", "feat/x"}},
		{"with --no-claude", "feat/x", []string{"--no-claude"}, []string{"claude-tmux-new", "feat/x", "--no-claude"}},
		{"multiple flags", "feat/x", []string{"--from-root", "--prompt", "hi"}, []string{"claude-tmux-new", "feat/x", "--from-root", "--prompt", "hi"}},
	}
	for _, c := range cases {
		got := buildExecArgs(c.branch, c.passthrough)
		if !reflect.DeepEqual(got, c.want) {
			t.Errorf("%s: got %v want %v", c.name, got, c.want)
		}
	}
}

func TestPromptForFlags(t *testing.T) {
	cases := []struct {
		name    string
		passArg []string
		want    string
	}{
		{"default", nil, "claude branch> "},
		{"--no-claude switches prompt", []string{"--no-claude"}, "worktree branch> "},
	}
	for _, c := range cases {
		if got := promptForFlags(c.passArg); got != c.want {
			t.Errorf("%s: got %q want %q", c.name, got, c.want)
		}
	}
}
