// Package tmux wraps tmux command invocations behind a proc.Runner so cmds
// can be unit-tested by injecting FakeRunner. Sanitize / ShellQuote are
// package-level helpers shared across all cmds.
package tmux

import (
	"context"
	"regexp"
	"strings"

	"claude-tools/internal/proc"
)

// Client wraps proc.Runner for tmux invocations.
type Client struct{ runner proc.Runner }

// New returns a Client backed by the given runner.
func New(r proc.Runner) *Client { return &Client{runner: r} }

// Display posts a short message to the status line. Failures are swallowed.
func (c *Client) Display(ctx context.Context, msg string) {
	_, _ = c.runner.Run(ctx, "tmux", "display-message", msg)
}

// ListPanes runs `tmux list-panes -t <target> -F <format>` and returns
// the trimmed lines.
func (c *Client) ListPanes(ctx context.Context, target, format string) ([]string, error) {
	out, err := c.runner.Run(ctx, "tmux", "list-panes", "-t", target, "-F", format)
	if err != nil {
		return nil, err
	}
	s := strings.TrimRight(string(out), "\n")
	if s == "" {
		return nil, nil
	}
	return strings.Split(s, "\n"), nil
}

// DisplayMessageGet runs `tmux display-message -p [-t <target>] <format>`.
// If target is "", -t flag is omitted.
func (c *Client) DisplayMessageGet(ctx context.Context, target, format string) (string, error) {
	args := []string{"display-message", "-p"}
	if target != "" {
		args = append(args, "-t", target)
	}
	args = append(args, format)
	out, err := c.runner.Run(ctx, "tmux", args...)
	if err != nil {
		return "", err
	}
	return strings.TrimRight(string(out), "\n"), nil
}

// RespawnPaneKill runs `tmux respawn-pane -k -t <target>`.
func (c *Client) RespawnPaneKill(ctx context.Context, target string) error {
	_, err := c.runner.Run(ctx, "tmux", "respawn-pane", "-k", "-t", target)
	return err
}

// SendKeys runs `tmux send-keys -t <target> <keys...>`. The caller appends
// "Enter" / "C-m" etc. as a separate key argument when needed.
func (c *Client) SendKeys(ctx context.Context, target string, keys ...string) error {
	args := append([]string{"send-keys", "-t", target}, keys...)
	_, err := c.runner.Run(ctx, "tmux", args...)
	return err
}

// sanitizeRe matches characters NOT allowed in tmux session/window names.
var sanitizeRe = regexp.MustCompile(`[^a-zA-Z0-9._-]`)

// Sanitize replaces every character outside [a-zA-Z0-9._-] with '-'.
// Mirrors the shell `tr -c 'a-zA-Z0-9._-' '-'` behaviour.
func Sanitize(s string) string {
	return sanitizeRe.ReplaceAllString(s, "-")
}

// ShellQuote returns a POSIX single-quoted form of s. Empty string becomes
// '' and any internal ' becomes '\''. Output is byte-different from
// `printf %q` but is semantically equivalent: feeding it back through bash
// reproduces the original string.
func ShellQuote(s string) string {
	if s == "" {
		return "''"
	}
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}
