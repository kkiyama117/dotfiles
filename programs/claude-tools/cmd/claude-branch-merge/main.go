// claude-branch-merge rebases the current branch onto a target branch and
// merges (or squash-merges) it back into target in the worktree where target
// is currently checked out.
//
// usage: claude-branch-merge <target> [--squash] [--no-rebase] [--fetch]
package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"claude-tools/internal/gitwt"
	"claude-tools/internal/proc"
)

const progName = "claude-branch-merge"

// errHelpRequested signals that -h/--help was passed.
var errHelpRequested = fmt.Errorf("help requested")

func usageString() string {
	return "usage: claude-branch-merge <target> [--squash] [--no-rebase] [--fetch]"
}

type options struct {
	target   string
	squash   bool
	noRebase bool
	fetch    bool
}

func main() {
	opts, err := parseArgs(os.Args[1:])
	if errors.Is(err, errHelpRequested) {
		fmt.Println(usageString())
		os.Exit(0)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, progName+":", err)
		os.Exit(1)
	}
	cwd, err := os.Getwd()
	if err != nil {
		fmt.Fprintln(os.Stderr, progName+": getwd:", err)
		os.Exit(1)
	}
	gw := gitwt.New(proc.RealRunner{})
	if err := run(context.Background(), gw, cwd, opts); err != nil {
		fmt.Fprintln(os.Stderr, progName+":", err)
		os.Exit(1)
	}
}

// parseArgs is the side-effect-free arg parser.
func parseArgs(argv []string) (options, error) {
	var o options
	if len(argv) == 0 {
		return options{}, fmt.Errorf("%s", usageString())
	}
	if argv[0] == "-h" || argv[0] == "--help" {
		return options{}, errHelpRequested
	}
	if strings.HasPrefix(argv[0], "-") {
		return options{}, fmt.Errorf("first argument must be the target branch name; got %q", argv[0])
	}
	o.target = argv[0]
	for i := 1; i < len(argv); i++ {
		switch argv[i] {
		case "--squash":
			o.squash = true
		case "--no-rebase":
			o.noRebase = true
		case "--fetch":
			o.fetch = true
		case "-h", "--help":
			return options{}, errHelpRequested
		default:
			return options{}, fmt.Errorf("unknown arg: %s", argv[i])
		}
	}
	return o, nil
}

// gitOps is the subset of gitwt.Client used by run. Lets tests inject a fake.
type gitOps interface {
	CurrentBranch(ctx context.Context, cwd string) (string, error)
	FindByBranch(ctx context.Context, cwd, branch string) (gitwt.Worktree, bool, error)
	Fetch(ctx context.Context, cwd, remote string) (string, error)
	Rebase(ctx context.Context, cwd, onto string) (string, error)
	Merge(ctx context.Context, cwd, source string, opts gitwt.MergeOpts) (string, error)
	Commit(ctx context.Context, cwd, msg string) (string, error)
	LogOneline(ctx context.Context, cwd, rev string) (string, error)
}

func run(ctx context.Context, gw gitOps, cwd string, opts options) error {
	current, err := gw.CurrentBranch(ctx, cwd)
	if err != nil {
		return fmt.Errorf("failed to read current branch: %w", err)
	}
	if current == "" {
		return fmt.Errorf("HEAD is detached; refusing to merge")
	}
	if current == opts.target {
		return fmt.Errorf("current branch is already %q; nothing to merge", opts.target)
	}

	wt, found, err := gw.FindByBranch(ctx, cwd, opts.target)
	if err != nil {
		return fmt.Errorf("failed to enumerate worktrees: %w", err)
	}
	if !found {
		return fmt.Errorf("target branch %q is not checked out in any worktree; create one first", opts.target)
	}
	targetPath := wt.Path

	if opts.fetch {
		if out, err := gw.Fetch(ctx, targetPath, "origin"); err != nil {
			return fmt.Errorf("git fetch origin failed in %s:\n%s\n(error: %v)", targetPath, out, err)
		}
	}

	if !opts.noRebase {
		if out, err := gw.Rebase(ctx, cwd, opts.target); err != nil {
			return fmt.Errorf("rebase onto %s failed; resolve conflicts manually:\n%s\n(error: %v)", opts.target, out, err)
		}
	}

	// For --squash, capture the commit log before the merge call so the auto
	// commit message lists each squashed commit.
	var oneline string
	if opts.squash {
		oneline, _ = gw.LogOneline(ctx, cwd, opts.target+".."+current)
	}

	mergeOpts := gitwt.MergeOpts{Squash: opts.squash}
	if out, err := gw.Merge(ctx, targetPath, current, mergeOpts); err != nil {
		return fmt.Errorf("merge %s into %s failed:\n%s\n(error: %v)", current, opts.target, out, err)
	}

	if opts.squash {
		msg := buildSquashMessage(current, opts.target, oneline)
		if out, err := gw.Commit(ctx, targetPath, msg); err != nil {
			return fmt.Errorf("squash commit failed:\n%s\n(error: %v)", out, err)
		}
	}

	mode := "merged"
	if opts.squash {
		mode = "squash-merged"
	}
	fmt.Printf("%s %s into %s at %s\n", mode, current, opts.target, targetPath)
	return nil
}

// buildSquashMessage composes the auto-commit message for `--squash`.
//
//	Squash merge <source> into <target>
//
//	* <abbrev> <subject>
//	* ...
func buildSquashMessage(source, target, oneline string) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "Squash merge %s into %s", source, target)
	if oneline != "" {
		sb.WriteString("\n\n")
		for _, line := range strings.Split(oneline, "\n") {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			fmt.Fprintf(&sb, "* %s\n", line)
		}
	}
	return strings.TrimRight(sb.String(), "\n")
}
