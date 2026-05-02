package main

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"claude-tools/internal/gitwt"
)

func TestParseArgs_table(t *testing.T) {
	cases := []struct {
		name    string
		argv    []string
		want    options
		wantErr string // substring; empty = no error; "HELP" = errHelpRequested
	}{
		{"no args", nil, options{}, "usage:"},
		{"target only", []string{"main"}, options{target: "main"}, ""},
		{"target + squash", []string{"main", "--squash"}, options{target: "main", squash: true}, ""},
		{"target + no-rebase", []string{"main", "--no-rebase"}, options{target: "main", noRebase: true}, ""},
		{"target + fetch", []string{"main", "--fetch"}, options{target: "main", fetch: true}, ""},
		{"all flags", []string{"develop", "--squash", "--no-rebase", "--fetch"},
			options{target: "develop", squash: true, noRebase: true, fetch: true}, ""},
		{"help short", []string{"-h"}, options{}, "HELP"},
		{"help long", []string{"--help"}, options{}, "HELP"},
		{"help after target", []string{"main", "-h"}, options{target: "main"}, "HELP"},
		{"flag first", []string{"--squash", "main"}, options{}, "first argument must be the target"},
		{"unknown flag", []string{"main", "--bogus"}, options{}, "unknown arg"},
	}
	for _, c := range cases {
		got, err := parseArgs(c.argv)
		switch c.wantErr {
		case "":
			if err != nil {
				t.Errorf("%s: got err=%v, want nil", c.name, err)
				continue
			}
			if got != c.want {
				t.Errorf("%s: got opts=%+v, want %+v", c.name, got, c.want)
			}
		case "HELP":
			if !errors.Is(err, errHelpRequested) {
				t.Errorf("%s: got err=%v, want errHelpRequested", c.name, err)
			}
		default:
			if err == nil || !strings.Contains(err.Error(), c.wantErr) {
				t.Errorf("%s: got err=%v, want substring %q", c.name, err, c.wantErr)
			}
		}
	}
}

func TestBuildSquashMessage(t *testing.T) {
	cases := []struct {
		name                   string
		source, target, online string
		want                   string
	}{
		{"no log", "feat/x", "main", "", "Squash merge feat/x into main"},
		{"single commit", "feat/x", "main", "abc1234 feat: do thing",
			"Squash merge feat/x into main\n\n* abc1234 feat: do thing"},
		{"multi commits", "feat/x", "develop", "abc feat: a\ndef fix: b\n",
			"Squash merge feat/x into develop\n\n* abc feat: a\n* def fix: b"},
		{"blank lines in log", "feat/x", "main", "abc one\n\n\ndef two",
			"Squash merge feat/x into main\n\n* abc one\n* def two"},
	}
	for _, c := range cases {
		if got := buildSquashMessage(c.source, c.target, c.online); got != c.want {
			t.Errorf("%s: got=%q want=%q", c.name, got, c.want)
		}
	}
}

// fakeGitOps records calls and returns pre-programmed responses.
type fakeGitOps struct {
	current    string
	currentErr error
	worktrees  map[string]gitwt.Worktree
	fetchOut   string
	fetchErr   error
	rebaseOut  string
	rebaseErr  error
	mergeOut   string
	mergeErr   error
	commitOut  string
	commitErr  error
	logOneline string
	calls      []string
}

func (f *fakeGitOps) CurrentBranch(_ context.Context, cwd string) (string, error) {
	f.calls = append(f.calls, "CurrentBranch:"+cwd)
	return f.current, f.currentErr
}
func (f *fakeGitOps) FindByBranch(_ context.Context, cwd, branch string) (gitwt.Worktree, bool, error) {
	f.calls = append(f.calls, "FindByBranch:"+cwd+":"+branch)
	wt, ok := f.worktrees[branch]
	return wt, ok, nil
}
func (f *fakeGitOps) Fetch(_ context.Context, cwd, remote string) (string, error) {
	f.calls = append(f.calls, "Fetch:"+cwd+":"+remote)
	return f.fetchOut, f.fetchErr
}
func (f *fakeGitOps) Rebase(_ context.Context, cwd, onto string) (string, error) {
	f.calls = append(f.calls, "Rebase:"+cwd+":"+onto)
	return f.rebaseOut, f.rebaseErr
}
func (f *fakeGitOps) Merge(_ context.Context, cwd, source string, opts gitwt.MergeOpts) (string, error) {
	f.calls = append(f.calls, fmt.Sprintf("Merge:%s:%s:squash=%v:noff=%v", cwd, source, opts.Squash, opts.NoFF))
	return f.mergeOut, f.mergeErr
}
func (f *fakeGitOps) Commit(_ context.Context, cwd, msg string) (string, error) {
	f.calls = append(f.calls, "Commit:"+cwd+":"+msg)
	return f.commitOut, f.commitErr
}
func (f *fakeGitOps) LogOneline(_ context.Context, cwd, rev string) (string, error) {
	f.calls = append(f.calls, "LogOneline:"+cwd+":"+rev)
	return f.logOneline, nil
}

func TestRun_refusesDetachedHead(t *testing.T) {
	gw := &fakeGitOps{current: ""}
	err := run(context.Background(), gw, "/wt", options{target: "main"})
	if err == nil || !strings.Contains(err.Error(), "detached") {
		t.Fatalf("got err=%v, want 'detached' error", err)
	}
}

func TestRun_refusesSelfMerge(t *testing.T) {
	gw := &fakeGitOps{current: "main"}
	err := run(context.Background(), gw, "/wt", options{target: "main"})
	if err == nil || !strings.Contains(err.Error(), "already") {
		t.Fatalf("got err=%v, want 'already' error", err)
	}
}

func TestRun_refusesMissingTargetWorktree(t *testing.T) {
	gw := &fakeGitOps{current: "feat/x", worktrees: map[string]gitwt.Worktree{}}
	err := run(context.Background(), gw, "/wt", options{target: "main"})
	if err == nil || !strings.Contains(err.Error(), "not checked out") {
		t.Fatalf("got err=%v, want 'not checked out' error", err)
	}
}

func TestRun_normalMergeFlow(t *testing.T) {
	gw := &fakeGitOps{
		current:   "feat/x",
		worktrees: map[string]gitwt.Worktree{"main": {Path: "/main", Branch: "main"}},
	}
	err := run(context.Background(), gw, "/wt", options{target: "main"})
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	wantCalls := []string{
		"CurrentBranch:/wt",
		"FindByBranch:/wt:main",
		"Rebase:/wt:main",
		"Merge:/main:feat/x:squash=false:noff=false",
	}
	if !equalStringSlice(gw.calls, wantCalls) {
		t.Errorf("calls=%v, want=%v", gw.calls, wantCalls)
	}
}

func TestRun_squashFlow_capturesLogAndCommits(t *testing.T) {
	gw := &fakeGitOps{
		current:    "feat/x",
		worktrees:  map[string]gitwt.Worktree{"main": {Path: "/main", Branch: "main"}},
		logOneline: "abc one\ndef two",
	}
	err := run(context.Background(), gw, "/wt", options{target: "main", squash: true})
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	wantPrefix := []string{
		"CurrentBranch:/wt",
		"FindByBranch:/wt:main",
		"Rebase:/wt:main",
		"LogOneline:/wt:main..feat/x",
		"Merge:/main:feat/x:squash=true:noff=false",
	}
	for i, want := range wantPrefix {
		if i >= len(gw.calls) || gw.calls[i] != want {
			t.Fatalf("calls[%d]=%q want %q (full=%v)", i, gw.calls[i], want, gw.calls)
		}
	}
	if len(gw.calls) != 6 {
		t.Fatalf("expected 6 calls, got %d (%v)", len(gw.calls), gw.calls)
	}
	if !strings.HasPrefix(gw.calls[5], "Commit:/main:Squash merge feat/x into main\n\n* abc one\n* def two") {
		t.Errorf("commit call=%q", gw.calls[5])
	}
}

func TestRun_skipsRebaseWhenRequested(t *testing.T) {
	gw := &fakeGitOps{
		current:   "feat/x",
		worktrees: map[string]gitwt.Worktree{"main": {Path: "/main", Branch: "main"}},
	}
	err := run(context.Background(), gw, "/wt", options{target: "main", noRebase: true})
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range gw.calls {
		if strings.HasPrefix(c, "Rebase:") {
			t.Fatalf("Rebase should not have been called; calls=%v", gw.calls)
		}
	}
}

func TestRun_fetchWhenRequested(t *testing.T) {
	gw := &fakeGitOps{
		current:   "feat/x",
		worktrees: map[string]gitwt.Worktree{"main": {Path: "/main", Branch: "main"}},
	}
	err := run(context.Background(), gw, "/wt", options{target: "main", fetch: true})
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, c := range gw.calls {
		if c == "Fetch:/main:origin" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("Fetch:/main:origin not in calls=%v", gw.calls)
	}
}

func TestRun_rebaseFailureSurfacesOutput(t *testing.T) {
	gw := &fakeGitOps{
		current:   "feat/x",
		worktrees: map[string]gitwt.Worktree{"main": {Path: "/main", Branch: "main"}},
		rebaseOut: "CONFLICT (content): Merge conflict in foo.go",
		rebaseErr: errors.New("exit 1"),
	}
	err := run(context.Background(), gw, "/wt", options{target: "main"})
	if err == nil || !strings.Contains(err.Error(), "CONFLICT") {
		t.Fatalf("got err=%v, want CONFLICT message", err)
	}
}

func TestRun_mergeFailureSurfacesOutput(t *testing.T) {
	gw := &fakeGitOps{
		current:   "feat/x",
		worktrees: map[string]gitwt.Worktree{"main": {Path: "/main", Branch: "main"}},
		mergeOut:  "Automatic merge failed",
		mergeErr:  errors.New("exit 1"),
	}
	err := run(context.Background(), gw, "/wt", options{target: "main"})
	if err == nil || !strings.Contains(err.Error(), "Automatic merge failed") {
		t.Fatalf("got err=%v", err)
	}
}

func equalStringSlice(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
