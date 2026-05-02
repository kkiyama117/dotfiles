// claude-respawn-pane restarts claude in the current session's claude pane.
// Strategy: find a pane in the current session whose pane_current_command
// is 'claude'; if found, respawn-pane -k and start a fresh
// `claude --continue`. If none found, do it in the current pane.
package main

import (
	"context"
	"os"
	"strings"

	"claude-tools/internal/obslog"
	"claude-tools/internal/proc"
	"claude-tools/internal/tmux"
)

const progName = "claude-respawn-pane"

var logger = obslog.New(progName)

func main() {
	ctx := context.Background()
	r := proc.RealRunner{}
	target, err := pickTargetPane(ctx, r)
	if err != nil {
		logger.Error("pick target pane failed", "err", err)
		os.Exit(1)
	}
	tc := tmux.New(r)
	if err := tc.RespawnPaneKill(ctx, target); err != nil {
		logger.Error("respawn-pane failed", "target", target, "err", err)
		os.Exit(1)
	}
	if err := tc.SendKeys(ctx, target, "claude --continue", "Enter"); err != nil {
		logger.Error("send-keys failed", "target", target, "err", err)
		os.Exit(1)
	}
}

// pickTargetPane returns the pane_id of the first pane in the current
// session whose pane_current_command == "claude", or the active pane_id
// when no such pane exists.
func pickTargetPane(ctx context.Context, r proc.Runner) (string, error) {
	tc := tmux.New(r)
	session, err := tc.DisplayMessageGet(ctx, "", "#S")
	if err != nil {
		return "", err
	}
	lines, err := tc.ListPanes(ctx, session, "#{pane_id} #{pane_current_command}")
	if err != nil {
		return "", err
	}
	for _, line := range lines {
		fields := strings.Fields(line)
		if len(fields) >= 2 && fields[1] == "claude" {
			return fields[0], nil
		}
	}
	return tc.DisplayMessageGet(ctx, "", "#{pane_id}")
}
