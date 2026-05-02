package main

import "testing"

func TestIsClaudeManaged_table(t *testing.T) {
	cases := []struct {
		name    string
		managed string
		panes   string
		session string
		want    bool
	}{
		{"managed=yes", "yes", "", "", true},
		{"claude pane present", "", "zsh\nclaude\n", "", true},
		{"legacy session", "", "zsh\nvim\n", "claude-old", true},
		{"all NG", "", "zsh\nvim\n", "dev", false},
		{"managed empty + claude in panes mid", "", "zsh\nclaude\nvim\n", "dev", true},
	}
	for _, c := range cases {
		if got := isClaudeManaged(c.managed, c.panes, c.session); got != c.want {
			t.Errorf("%s: got %v want %v", c.name, got, c.want)
		}
	}
}
