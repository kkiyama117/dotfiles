// claude-branch prints "[<branch>] " for use in tmux status-right.
// Always exit 0 to keep status-right rendering safe even on errors
// (status-right invokes this on every refresh; a non-zero exit would
// break the entire status line).
package main

import (
	"context"
	"fmt"
	"os"

	"claude-tools/internal/gitwt"
	"claude-tools/internal/obslog"
	"claude-tools/internal/proc"
)

const progName = "claude-branch"

var logger = obslog.New(progName)

func main() {
	defer func() {
		if r := recover(); r != nil {
			logger.Error("panic recovered", "panic", fmt.Sprint(r))
		}
		os.Exit(0)
	}()

	cwd := ""
	if len(os.Args) > 1 {
		cwd = os.Args[1]
	}
	out, err := formatBranch(context.Background(), proc.RealRunner{}, cwd)
	if err != nil {
		logger.Debug("branch lookup failed", "cwd", cwd, "err", err)
		return
	}
	fmt.Print(out)
}

// formatBranch is the testable core: returns the formatted status fragment
// or an empty string on any non-fatal condition.
func formatBranch(ctx context.Context, r proc.Runner, cwd string) (string, error) {
	if cwd == "" {
		return "", nil
	}
	branch, err := gitwt.New(r).CurrentBranch(ctx, cwd)
	if err != nil {
		return "", err
	}
	if branch == "" {
		return "", nil
	}
	return fmt.Sprintf("[%s] ", branch), nil
}
