// Package gitwt wraps git worktree / branch operations behind a proc.Runner
// for testability. PR-C-1 introduced CurrentBranch; PR-C-3 adds the
// ListPorcelain parser and worktree mutation methods (Remove, Prune,
// TopLevel) consumed by claude-kill-session.
package gitwt

import (
	"context"
	"fmt"
	"os/exec"
	"strings"

	"claude-tools/internal/proc"
)

// Worktree represents one entry from `git worktree list --porcelain`.
type Worktree struct {
	Path   string
	Branch string // "refs/heads/<x>" の <x> 部分。detached は ""
	HEAD   string
}

// Client wraps proc.Runner for git invocations.
type Client struct{ runner proc.Runner }

// New returns a Client backed by the given runner.
func New(r proc.Runner) *Client { return &Client{runner: r} }

// CurrentBranch returns the current branch of the working tree at cwd.
// Returns ("", nil) if HEAD is detached (git --show-current outputs empty).
// Returns ("", err) if the git command itself fails; callers decide handling.
func (c *Client) CurrentBranch(ctx context.Context, cwd string) (string, error) {
	out, err := c.runner.Run(ctx, "git", "-C", cwd, "branch", "--show-current")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

// ListPorcelain runs `git -C <cwd> worktree list --porcelain` and parses
// the output into Worktree entries.
func (c *Client) ListPorcelain(ctx context.Context, cwd string) ([]Worktree, error) {
	out, err := c.runner.Run(ctx, "git", "-C", cwd, "worktree", "list", "--porcelain")
	if err != nil {
		return nil, fmt.Errorf("git worktree list: %w", err)
	}
	return parsePorcelain(out), nil
}

// parsePorcelain parses the multi-line, blank-line-separated record format
// emitted by `git worktree list --porcelain`. The first record is always
// the main worktree.
func parsePorcelain(b []byte) []Worktree {
	var out []Worktree
	var cur Worktree
	flush := func() {
		if cur.Path != "" {
			out = append(out, cur)
		}
		cur = Worktree{}
	}
	for _, line := range strings.Split(string(b), "\n") {
		switch {
		case strings.HasPrefix(line, "worktree "):
			flush()
			cur.Path = strings.TrimPrefix(line, "worktree ")
		case strings.HasPrefix(line, "HEAD "):
			cur.HEAD = strings.TrimPrefix(line, "HEAD ")
		case strings.HasPrefix(line, "branch refs/heads/"):
			cur.Branch = strings.TrimPrefix(line, "branch refs/heads/")
		case line == "" && cur.Path != "":
			flush()
		}
	}
	flush()
	return out
}

// MainRepo returns the path of the first worktree (always the main repo).
func (c *Client) MainRepo(ctx context.Context, cwd string) (string, error) {
	wts, err := c.ListPorcelain(ctx, cwd)
	if err != nil {
		return "", err
	}
	if len(wts) == 0 {
		return "", fmt.Errorf("no worktrees found at %q", cwd)
	}
	return wts[0].Path, nil
}

// FindByBranch returns the worktree whose Branch == branch.
func (c *Client) FindByBranch(ctx context.Context, cwd, branch string) (Worktree, bool, error) {
	wts, err := c.ListPorcelain(ctx, cwd)
	if err != nil {
		return Worktree{}, false, err
	}
	for _, w := range wts {
		if w.Branch == branch {
			return w, true, nil
		}
	}
	return Worktree{}, false, nil
}

// Remove runs `git -C <mainRepo> worktree remove <target> --force` and
// returns the captured stderr if it failed. Uses exec.CommandContext
// directly because proc.Runner.Run drops stderr — kill-session needs it
// for the "kept worktree" display message.
func (c *Client) Remove(ctx context.Context, mainRepo, target string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", "-C", mainRepo, "worktree", "remove", target, "--force")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return strings.TrimSpace(string(out)), err
	}
	return "", nil
}

// Prune runs `git -C <mainRepo> worktree prune` (failure ignored).
func (c *Client) Prune(ctx context.Context, mainRepo string) {
	_, _ = c.runner.Run(ctx, "git", "-C", mainRepo, "worktree", "prune")
}

// TopLevel returns `git -C <cwd> rev-parse --show-toplevel`.
func (c *Client) TopLevel(ctx context.Context, cwd string) (string, error) {
	out, err := c.runner.Run(ctx, "git", "-C", cwd, "rev-parse", "--show-toplevel")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

// HasLocalRef returns true when refs/heads/<branch> exists.
func (c *Client) HasLocalRef(ctx context.Context, cwd, branch string) bool {
	_, err := c.runner.Run(ctx, "git", "-C", cwd, "show-ref", "--verify", "--quiet", "refs/heads/"+branch)
	return err == nil
}

// HasRemoteRef returns true when refs/remotes/origin/<branch> exists.
func (c *Client) HasRemoteRef(ctx context.Context, cwd, branch string) bool {
	_, err := c.runner.Run(ctx, "git", "-C", cwd, "show-ref", "--verify", "--quiet", "refs/remotes/origin/"+branch)
	return err == nil
}

// AddExistingLocal: git worktree add <path> <branch>
func (c *Client) AddExistingLocal(ctx context.Context, cwd, path, branch string) error {
	if _, err := c.runner.Run(ctx, "git", "-C", cwd, "worktree", "add", path, branch); err != nil {
		return fmt.Errorf("git worktree add %q %q: %w", path, branch, err)
	}
	return nil
}

// AddTrackingRemote: git worktree add -b <branch> <path> origin/<branch>
func (c *Client) AddTrackingRemote(ctx context.Context, cwd, path, branch string) error {
	if _, err := c.runner.Run(ctx, "git", "-C", cwd, "worktree", "add", "-b", branch, path, "origin/"+branch); err != nil {
		return fmt.Errorf("git worktree add -b %q %q origin/%s: %w", branch, path, branch, err)
	}
	return nil
}

// AddFromHead: git worktree add -b <branch> <path> HEAD
func (c *Client) AddFromHead(ctx context.Context, cwd, path, branch string) error {
	if _, err := c.runner.Run(ctx, "git", "-C", cwd, "worktree", "add", "-b", branch, path, "HEAD"); err != nil {
		return fmt.Errorf("git worktree add -b %q %q HEAD: %w", branch, path, err)
	}
	return nil
}
