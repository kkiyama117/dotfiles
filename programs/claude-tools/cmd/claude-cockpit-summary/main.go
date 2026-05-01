// claude-cockpit-summary writes the per-state count summary used by
// tmux status-right.
//
// Output format (byte-exact match to summary.sh):
//
//	"⚡ N ⏸ M ✓ K "  with trailing space; segments with count 0 are
//	omitted; empty string when no states exist.
package main

import (
	"fmt"
	"io"
	"os"

	"claude-tools/internal/cockpit"
	"claude-tools/internal/obslog"
)

const progName = "claude-cockpit-summary"

func main() {
	if err := writeSummary(os.Stdout); err != nil {
		// Status-right failure must not poison the bar — emit nothing.
		obslog.New(progName).Error("write summary failed", "err", err)
		os.Exit(1)
	}
}

func writeSummary(w io.Writer) error {
	states, err := cockpit.LoadAll()
	if err != nil {
		return fmt.Errorf("load all: %w", err)
	}
	_, err = io.WriteString(w, cockpit.Summary(states))
	return err
}
