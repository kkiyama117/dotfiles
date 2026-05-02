// Package gitwt wraps git worktree / branch operations behind a proc.Runner
// for testability. Initial set: CurrentBranch only; later PRs add the
// ListPorcelain parser and worktree mutation methods.
package gitwt

import (
	"context"
	"strings"

	"claude-tools/internal/proc"
)

// Worktree represents one entry from `git worktree list --porcelain`.
// Defined in this skeleton; consumed by ListPorcelain in PR-C-3.
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
