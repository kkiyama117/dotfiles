package tmux

import (
	"context"
	"os/exec"
	"testing"

	"claude-tools/internal/proc"
)

func TestDisplay_argv(t *testing.T) {
	// Display swallows runner errors, so this test only verifies the
	// call doesn't panic. argv correctness for Display is covered by
	// integration smoke (manual tmux verification), not unit tests.
	r := proc.NewFakeRunner()
	r.Register("tmux", []string{"display-message", "hello"}, nil, nil)
	New(r).Display(context.Background(), "hello")
}

func TestListPanes_splitsLines(t *testing.T) {
	r := proc.NewFakeRunner()
	r.Register("tmux", []string{"list-panes", "-t", "S:W", "-F", "#{pane_id}"}, []byte("%1\n%2\n"), nil)
	got, err := New(r).ListPanes(context.Background(), "S:W", "#{pane_id}")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(got) != 2 || got[0] != "%1" || got[1] != "%2" {
		t.Fatalf("got %v", got)
	}
}

func TestListPanes_emptyOutput_returnsNil(t *testing.T) {
	r := proc.NewFakeRunner()
	r.Register("tmux", []string{"list-panes", "-t", "S", "-F", "."}, []byte(""), nil)
	got, _ := New(r).ListPanes(context.Background(), "S", ".")
	if got != nil {
		t.Fatalf("got %v want nil", got)
	}
}

func TestDisplayMessageGet_withAndWithoutTarget(t *testing.T) {
	r := proc.NewFakeRunner()
	r.Register("tmux", []string{"display-message", "-p", "-t", "S:W", "#S"}, []byte("S\n"), nil)
	r.Register("tmux", []string{"display-message", "-p", "#S"}, []byte("X\n"), nil)
	c := New(r)
	if got, _ := c.DisplayMessageGet(context.Background(), "S:W", "#S"); got != "S" {
		t.Fatalf("with target: got %q", got)
	}
	if got, _ := c.DisplayMessageGet(context.Background(), "", "#S"); got != "X" {
		t.Fatalf("without target: got %q", got)
	}
}

func TestRespawnPaneKill_argv(t *testing.T) {
	r := proc.NewFakeRunner()
	r.Register("tmux", []string{"respawn-pane", "-k", "-t", "%3"}, nil, nil)
	if err := New(r).RespawnPaneKill(context.Background(), "%3"); err != nil {
		t.Fatal(err)
	}
}

func TestSendKeys_appendsKeys(t *testing.T) {
	r := proc.NewFakeRunner()
	r.Register("tmux", []string{"send-keys", "-t", "%3", "claude --continue", "Enter"}, nil, nil)
	if err := New(r).SendKeys(context.Background(), "%3", "claude --continue", "Enter"); err != nil {
		t.Fatal(err)
	}
}

func TestSanitize_table(t *testing.T) {
	cases := []struct{ in, want string }{
		{"feat/x", "feat-x"},
		{"abc.def_ghi-jkl", "abc.def_ghi-jkl"},
		{"hello world!", "hello-world-"},
		{"", ""},
		{"a//b", "a--b"},
	}
	for _, c := range cases {
		if got := Sanitize(c.in); got != c.want {
			t.Errorf("Sanitize(%q)=%q want %q", c.in, got, c.want)
		}
	}
}

func TestShellQuote_table(t *testing.T) {
	cases := []struct{ in, want string }{
		{"", "''"},
		{"plain", "'plain'"},
		{"with space", "'with space'"},
		{"it's", `'it'\''s'`},
		{"$HOME", "'$HOME'"},
		{`a\b`, `'a\b'`},
		{"line\nbreak", "'line\nbreak'"},
	}
	for _, c := range cases {
		if got := ShellQuote(c.in); got != c.want {
			t.Errorf("ShellQuote(%q)=%q want %q", c.in, got, c.want)
		}
	}
}

// TestShellQuote_roundTrip feeds the quoted form through bash and verifies
// the original bytes round-trip. Skips if bash is unavailable.
func TestShellQuote_roundTrip(t *testing.T) {
	if _, err := exec.LookPath("bash"); err != nil {
		t.Skip("bash not available")
	}
	for _, in := range []string{"hi", "it's", "$X", `a\b`, "テスト 'quote'", `hello "world"`} {
		quoted := ShellQuote(in)
		out, err := exec.Command("bash", "-c", "printf '%s' "+quoted).Output()
		if err != nil {
			t.Fatalf("bash failed for %q: %v", in, err)
		}
		if got := string(out); got != in {
			t.Errorf("roundtrip mismatch: in=%q quoted=%s out=%q", in, quoted, got)
		}
	}
}
