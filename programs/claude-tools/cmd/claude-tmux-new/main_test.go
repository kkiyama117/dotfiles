package main

import (
	"reflect"
	"testing"
)

func TestParseArgs_table(t *testing.T) {
	cases := []struct {
		name string
		argv []string
		want options
		err  bool
	}{
		{"branch only", []string{"feat/x"}, options{branch: "feat/x"}, false},
		{"--no-claude", []string{"feat/x", "--no-claude"}, options{branch: "feat/x", noClaude: true}, false},
		{"--from-root no id", []string{"feat/x", "--from-root"}, options{branch: "feat/x", fromRoot: true}, false},
		{"--from-root with id", []string{"feat/x", "--from-root", "abc-123"}, options{branch: "feat/x", fromRoot: true, explicitSession: "abc-123"}, false},
		{"--worktree-base", []string{"feat/x", "--worktree-base", "/tmp/wt"}, options{branch: "feat/x", worktreeBase: "/tmp/wt"}, false},
		{"--prompt", []string{"feat/x", "--prompt", "hi"}, options{branch: "feat/x", initialPrompt: "hi"}, false},
		{"missing branch", []string{}, options{}, true},
		{"--from-root + --no-claude", []string{"feat/x", "--from-root", "--no-claude"}, options{}, true},
		{"--prompt + --no-claude", []string{"feat/x", "--no-claude", "--prompt", "hi"}, options{}, true},
		{"--worktree-base missing arg", []string{"feat/x", "--worktree-base"}, options{}, true},
	}
	for _, c := range cases {
		got, err := parseArgs(c.argv)
		if (err != nil) != c.err {
			t.Errorf("%s: err=%v wantErr=%v", c.name, err, c.err)
			continue
		}
		if err == nil && !reflect.DeepEqual(got, c.want) {
			t.Errorf("%s: got %+v want %+v", c.name, got, c.want)
		}
	}
}

func TestBuildClaudeCommand_table(t *testing.T) {
	cases := []struct {
		name      string
		sessionID string
		history   bool
		prompt    string
		want      string
	}{
		{"plain", "", false, "", "claude"},
		{"continue", "", true, "", "claude --continue --fork-session"},
		{"resume", "abc-123", false, "", "claude --resume abc-123 --fork-session"},
		{"plain + prompt", "", false, "hi", "claude 'hi'"},
		{"continue + prompt with quote", "", true, "it's", `claude --continue --fork-session 'it'\''s'`},
	}
	for _, c := range cases {
		if got := buildClaudeCommand(c.sessionID, c.history, c.prompt); got != c.want {
			t.Errorf("%s: got %q want %q", c.name, got, c.want)
		}
	}
}
