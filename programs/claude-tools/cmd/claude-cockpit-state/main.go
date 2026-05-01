// claude-cockpit-state is the Claude Code hook entry that records the
// current pane's state (working / waiting / done) into the cockpit cache.
//
// Usage: claude-cockpit-state hook <Event>
//
// Absolute contract: this binary ALWAYS exits 0, regardless of internal
// errors, so a failure here never blocks Claude Code's hook pipeline.
// Errors are forwarded to syslog via internal/obslog instead.
package main

import (
	"context"
	"os"
	"strings"

	"claude-tools/internal/cockpit"
	"claude-tools/internal/obslog"
	"claude-tools/internal/proc"
)

const progName = "claude-cockpit-state"

func main() {
	// Defense in depth: even a panic must not propagate to Claude.
	defer func() {
		_ = recover()
		os.Exit(0)
	}()

	tmuxPane := os.Getenv("TMUX_PANE")
	if err := run(context.Background(), proc.RealRunner{}, os.Args, tmuxPane); err != nil {
		// Should not happen because run() swallows errors, but guard anyway.
		obslog.New(progName).Error("run failed", "err", err)
	}
	os.Exit(0)
}

// run implements the hook logic. It always returns nil because hook
// failures are silently logged (never propagated). Returning an error
// type is reserved for future signal-based test hooks; current callers
// should ignore it.
func run(ctx context.Context, runner proc.Runner, args []string, tmuxPane string) error {
	logger := obslog.New(progName)

	// args[0] = program name; args[1] = mode; args[2] = event
	if len(args) < 3 || args[1] != "hook" {
		return nil
	}
	event := args[2]

	status, ok := eventToStatus(event)
	if !ok {
		return nil
	}
	if tmuxPane == "" {
		return nil
	}

	// tmux display-message to look up session name for this pane.
	out, err := runner.Run(ctx, "tmux", "display-message", "-p", "-t", tmuxPane, "#{session_name}")
	if err != nil {
		logger.Error("tmux session lookup failed", "pane", tmuxPane, "err", err)
		return nil
	}
	session := strings.TrimSpace(string(out))
	if session == "" {
		return nil
	}

	if err := cockpit.WriteStatus(session, tmuxPane, cockpit.Status(status)); err != nil {
		logger.Error("write status failed",
			"session", session, "pane", tmuxPane, "status", status, "err", err)
	}
	return nil
}

// eventToStatus maps Claude hook events to cockpit Status string literals.
// Returns ("", false) for events we ignore (e.g., SubagentStop).
func eventToStatus(event string) (string, bool) {
	switch event {
	case "UserPromptSubmit", "PreToolUse":
		return "working", true
	case "Notification":
		return "waiting", true
	case "Stop":
		return "done", true
	}
	return "", false
}
