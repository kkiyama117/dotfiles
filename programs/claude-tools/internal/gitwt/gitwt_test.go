package gitwt

import (
	"context"
	"reflect"
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

func TestParsePorcelain_table(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want []Worktree
	}{
		{"empty", "", nil},
		{"single main", "worktree /home/u/r\nHEAD aaa\nbranch refs/heads/main\n", []Worktree{
			{Path: "/home/u/r", Branch: "main", HEAD: "aaa"},
		}},
		{"main + 1 wt", "worktree /a\nHEAD a1\nbranch refs/heads/main\n\nworktree /b\nHEAD b1\nbranch refs/heads/feat\n", []Worktree{
			{Path: "/a", Branch: "main", HEAD: "a1"},
			{Path: "/b", Branch: "feat", HEAD: "b1"},
		}},
		{"main + N", "worktree /a\nHEAD a1\nbranch refs/heads/main\n\nworktree /b\nHEAD b1\nbranch refs/heads/x\n\nworktree /c\nHEAD c1\nbranch refs/heads/y\n", []Worktree{
			{Path: "/a", Branch: "main", HEAD: "a1"},
			{Path: "/b", Branch: "x", HEAD: "b1"},
			{Path: "/c", Branch: "y", HEAD: "c1"},
		}},
		{"detached", "worktree /a\nHEAD a1\nbranch refs/heads/main\n\nworktree /tmp/d\nHEAD d1\ndetached\n", []Worktree{
			{Path: "/a", Branch: "main", HEAD: "a1"},
			{Path: "/tmp/d", Branch: "", HEAD: "d1"},
		}},
		{"branch ref non-heads (tag)", "worktree /a\nHEAD a1\nbranch refs/tags/v1\n", []Worktree{
			{Path: "/a", Branch: "", HEAD: "a1"}, // tag は無視されて Branch="" のまま
		}},
		{"no trailing blank", "worktree /a\nHEAD a1\nbranch refs/heads/main", []Worktree{
			{Path: "/a", Branch: "main", HEAD: "a1"},
		}},
		{"extra blank lines", "\n\nworktree /a\nHEAD a1\nbranch refs/heads/main\n\n\n\n", []Worktree{
			{Path: "/a", Branch: "main", HEAD: "a1"},
		}},
		{"branch slash in name", "worktree /a\nHEAD a1\nbranch refs/heads/feat/sub-x\n", []Worktree{
			{Path: "/a", Branch: "feat/sub-x", HEAD: "a1"},
		}},
		{"path only (no HEAD or branch)", "worktree /minimal\n", []Worktree{
			{Path: "/minimal", Branch: "", HEAD: ""},
		}},
	}
	for _, c := range cases {
		got := parsePorcelain([]byte(c.in))
		if !reflect.DeepEqual(got, c.want) {
			t.Errorf("%s: got %+v want %+v", c.name, got, c.want)
		}
	}
}

func TestMainRepo_returnsFirst(t *testing.T) {
	r := proc.NewFakeRunner()
	r.Register("git", []string{"-C", "/x", "worktree", "list", "--porcelain"},
		[]byte("worktree /m\nHEAD a\nbranch refs/heads/main\n\nworktree /w\nHEAD b\nbranch refs/heads/x\n"), nil)
	got, err := New(r).MainRepo(context.Background(), "/x")
	if err != nil || got != "/m" {
		t.Fatalf("got=%q err=%v", got, err)
	}
}

func TestFindByBranch_hit(t *testing.T) {
	r := proc.NewFakeRunner()
	r.Register("git", []string{"-C", "/x", "worktree", "list", "--porcelain"},
		[]byte("worktree /m\nHEAD a\nbranch refs/heads/main\n\nworktree /w\nHEAD b\nbranch refs/heads/feat\n"), nil)
	got, ok, err := New(r).FindByBranch(context.Background(), "/x", "feat")
	if err != nil || !ok || got.Path != "/w" {
		t.Fatalf("got=%+v ok=%v err=%v", got, ok, err)
	}
}

func TestFindByBranch_miss(t *testing.T) {
	r := proc.NewFakeRunner()
	r.Register("git", []string{"-C", "/x", "worktree", "list", "--porcelain"},
		[]byte("worktree /m\nHEAD a\nbranch refs/heads/main\n"), nil)
	_, ok, err := New(r).FindByBranch(context.Background(), "/x", "nope")
	if err != nil || ok {
		t.Fatalf("ok=%v err=%v want false nil", ok, err)
	}
}

func TestTopLevel(t *testing.T) {
	r := proc.NewFakeRunner()
	r.Register("git", []string{"-C", "/p", "rev-parse", "--show-toplevel"}, []byte("/m\n"), nil)
	got, err := New(r).TopLevel(context.Background(), "/p")
	if err != nil || got != "/m" {
		t.Fatalf("got=%q err=%v", got, err)
	}
}

func TestLocalBranches(t *testing.T) {
	r := proc.NewFakeRunner()
	r.Register("git", []string{"-C", "/p", "for-each-ref", "--format=%(refname:short)", "refs/heads"},
		[]byte("main\nfeat/x\nfeat/y\n"), nil)
	got, err := New(r).LocalBranches(context.Background(), "/p")
	if err != nil || len(got) != 3 || got[0] != "main" {
		t.Fatalf("got=%v err=%v", got, err)
	}
}

func TestLocalBranches_empty(t *testing.T) {
	r := proc.NewFakeRunner()
	r.Register("git", []string{"-C", "/p", "for-each-ref", "--format=%(refname:short)", "refs/heads"}, []byte(""), nil)
	got, _ := New(r).LocalBranches(context.Background(), "/p")
	if got != nil {
		t.Fatalf("got %v want nil", got)
	}
}

func TestHasLocalRef(t *testing.T) {
	r := proc.NewFakeRunner()
	r.Register("git", []string{"-C", "/p", "show-ref", "--verify", "--quiet", "refs/heads/x"}, nil, nil)
	if !New(r).HasLocalRef(context.Background(), "/p", "x") {
		t.Fatal("want true")
	}
}

func TestAddExistingLocal_argv(t *testing.T) {
	r := proc.NewFakeRunner()
	r.Register("git", []string{"-C", "/p", "worktree", "add", "/wt", "x"}, nil, nil)
	if err := New(r).AddExistingLocal(context.Background(), "/p", "/wt", "x"); err != nil {
		t.Fatal(err)
	}
}

func TestLogOneline_returnsTrimmedOutput(t *testing.T) {
	r := proc.NewFakeRunner()
	r.Register("git", []string{"-C", "/p", "log", "--oneline", "main..feat"},
		[]byte("abc1234 feat: do thing\ndef5678 fix: edge case\n"), nil)
	got, err := New(r).LogOneline(context.Background(), "/p", "main..feat")
	want := "abc1234 feat: do thing\ndef5678 fix: edge case"
	if err != nil || got != want {
		t.Fatalf("got=%q err=%v want=%q", got, err, want)
	}
}

func TestLogOneline_emptyRange(t *testing.T) {
	r := proc.NewFakeRunner()
	r.Register("git", []string{"-C", "/p", "log", "--oneline", "main..feat"}, []byte(""), nil)
	got, err := New(r).LogOneline(context.Background(), "/p", "main..feat")
	if err != nil || got != "" {
		t.Fatalf("got=%q err=%v want empty nil", got, err)
	}
}

func TestLogOneline_propagatesError(t *testing.T) {
	r := proc.NewFakeRunner() // unregistered → error
	_, err := New(r).LogOneline(context.Background(), "/p", "main..feat")
	if err == nil {
		t.Fatal("want error from unregistered runner call")
	}
}

func TestAddTrackingRemote_argv(t *testing.T) {
	r := proc.NewFakeRunner()
	r.Register("git", []string{"-C", "/p", "worktree", "add", "-b", "x", "/wt", "origin/x"}, nil, nil)
	if err := New(r).AddTrackingRemote(context.Background(), "/p", "/wt", "x"); err != nil {
		t.Fatal(err)
	}
}

func TestAddFromHead_argv(t *testing.T) {
	r := proc.NewFakeRunner()
	r.Register("git", []string{"-C", "/p", "worktree", "add", "-b", "x", "/wt", "HEAD"}, nil, nil)
	if err := New(r).AddFromHead(context.Background(), "/p", "/wt", "x"); err != nil {
		t.Fatal(err)
	}
}
