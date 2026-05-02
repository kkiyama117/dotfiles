package main

import "testing"

func TestIsClaudeManaged_table(t *testing.T) {
	cases := []struct {
		name    string
		managed string
		panes   []string
		session string
		want    bool
	}{
		{"managed=yes", "yes", nil, "", true},
		{"claude pane present", "", []string{"zsh", "claude"}, "", true},
		{"legacy session", "", []string{"zsh", "vim"}, "claude-old", true},
		{"all NG", "", []string{"zsh", "vim"}, "dev", false},
		{"managed empty + claude in panes mid", "", []string{"zsh", "claude", "vim"}, "dev", true},
	}
	for _, c := range cases {
		if got := isClaudeManaged(c.managed, c.panes, c.session); got != c.want {
			t.Errorf("%s: got %v want %v", c.name, got, c.want)
		}
	}
}
