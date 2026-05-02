// Package tmux wraps tmux command invocations behind a proc.Runner so cmds
// can be unit-tested by injecting FakeRunner. Sanitize / ShellQuote are
// package-level helpers shared across all cmds.
package tmux

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"syscall"

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
		return nil, fmt.Errorf("tmux list-panes -t %q: %w", target, err)
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
		return "", fmt.Errorf("tmux display-message -p %q: %w", format, err)
	}
	return strings.TrimRight(string(out), "\n"), nil
}

// RespawnPaneKill runs `tmux respawn-pane -k -t <target>`.
func (c *Client) RespawnPaneKill(ctx context.Context, target string) error {
	if _, err := c.runner.Run(ctx, "tmux", "respawn-pane", "-k", "-t", target); err != nil {
		return fmt.Errorf("tmux respawn-pane -k -t %q: %w", target, err)
	}
	return nil
}

// SendKeys runs `tmux send-keys -t <target> <keys...>`. The caller appends
// "Enter" / "C-m" etc. as a separate key argument when needed.
func (c *Client) SendKeys(ctx context.Context, target string, keys ...string) error {
	args := make([]string, 0, 3+len(keys))
	args = append(args, "send-keys", "-t", target)
	args = append(args, keys...)
	if _, err := c.runner.Run(ctx, "tmux", args...); err != nil {
		return fmt.Errorf("tmux send-keys -t %q: %w", target, err)
	}
	return nil
}

// KillWindow runs `tmux kill-window -t <target>`.
func (c *Client) KillWindow(ctx context.Context, target string) error {
	if _, err := c.runner.Run(ctx, "tmux", "kill-window", "-t", target); err != nil {
		return fmt.Errorf("tmux kill-window -t %q: %w", target, err)
	}
	return nil
}

// ShowWindowOption returns the value of a tmux window option and whether
// it was set. show-options -v exits non-zero when the option is unset,
// which is also when tmux itself fails to start; we treat both as "unset".
func (c *Client) ShowWindowOption(ctx context.Context, target, key string) (string, bool) {
	out, err := c.runner.Run(ctx, "tmux", "show-options", "-w", "-t", target, "-v", key)
	if err != nil {
		return "", false
	}
	return strings.TrimRight(string(out), "\n"), true
}

// HasSession returns true if `tmux has-session -t "=<name>"` exits zero.
func (c *Client) HasSession(ctx context.Context, name string) bool {
	_, err := c.runner.Run(ctx, "tmux", "has-session", "-t", "="+name)
	return err == nil
}

// SetWindowOption sets a tmux window-scope option.
func (c *Client) SetWindowOption(ctx context.Context, target, key, value string) error {
	if _, err := c.runner.Run(ctx, "tmux", "set-option", "-w", "-t", target, "-o", key, value); err != nil {
		return fmt.Errorf("tmux set-option -w -t %q %s: %w", target, key, err)
	}
	return nil
}

// NewSessionDetached creates a detached tmux session.
func (c *Client) NewSessionDetached(ctx context.Context, session, window, cwd string) error {
	if _, err := c.runner.Run(ctx, "tmux", "new-session", "-d", "-s", session, "-n", window, "-c", cwd); err != nil {
		return fmt.Errorf("tmux new-session -d -s %q: %w", session, err)
	}
	return nil
}

// NewWindowSelectExisting runs `tmux new-window -S` (selects same-named
// window if it already exists, instead of creating a duplicate).
func (c *Client) NewWindowSelectExisting(ctx context.Context, session, window, cwd string) error {
	if _, err := c.runner.Run(ctx, "tmux", "new-window", "-S", "-t", session+":", "-n", window, "-c", cwd); err != nil {
		return fmt.Errorf("tmux new-window -S -t %q -n %q: %w", session+":", window, err)
	}
	return nil
}

// SplitWindowH splits the window horizontally.
func (c *Client) SplitWindowH(ctx context.Context, target, cwd string) error {
	if _, err := c.runner.Run(ctx, "tmux", "split-window", "-h", "-t", target, "-c", cwd); err != nil {
		return fmt.Errorf("tmux split-window -h -t %q: %w", target, err)
	}
	return nil
}

// SelectPaneTitle sets the pane title (failures swallowed — cosmetic).
func (c *Client) SelectPaneTitle(ctx context.Context, target, title string) {
	_, _ = c.runner.Run(ctx, "tmux", "select-pane", "-t", target, "-T", title)
}

// SwitchClient switches the current client to the given target.
func (c *Client) SwitchClient(ctx context.Context, target string) error {
	if _, err := c.runner.Run(ctx, "tmux", "switch-client", "-t", target); err != nil {
		return fmt.Errorf("tmux switch-client -t %q: %w", target, err)
	}
	return nil
}

// AttachSessionExec replaces the current process with `tmux attach-session`,
// preserving the TTY. Returns only on syscall.Exec failure.
func (c *Client) AttachSessionExec(target string) error {
	bin, err := exec.LookPath("tmux")
	if err != nil {
		return fmt.Errorf("tmux binary not found: %w", err)
	}
	if err := syscall.Exec(bin, []string{"tmux", "attach-session", "-t", target}, os.Environ()); err != nil {
		return fmt.Errorf("exec tmux attach-session -t %q: %w", target, err)
	}
	return nil
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
