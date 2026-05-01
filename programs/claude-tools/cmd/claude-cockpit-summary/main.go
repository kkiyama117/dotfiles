// claude-cockpit-summary writes the per-state count summary used by
// tmux status-right.
//
// Output format (byte-exact match to summary.sh):
//
//	"⚡ N ⏸ M ✓ K "  with trailing space; segments with count 0 are
//	omitted; empty string when no states exist.
package main

import (
	"context"
	"fmt"
	"io"
	"os"

	"claude-tools/internal/cockpit"
	"claude-tools/internal/obslog"
	"claude-tools/internal/proc"
)

const progName = "claude-cockpit-summary"

func main() {
	if err := writeSummary(context.Background(), proc.RealRunner{}, os.Stdout); err != nil {
		// Status-right failure must not poison the bar — emit nothing.
		obslog.New(progName).Error("write summary failed", "err", err)
		os.Exit(1)
	}
}

// writeSummary loads cached pane states, intersects them with the live
// claude pane set (tmux list-panes -a, F-8 v1 defensive filter), and
// writes the cockpit summary. Tmux failure is treated as fail-closed
// (empty live set → empty summary), so a broken tmux server can't
// freeze the status-right with stale ⚡/⏸/✓ counts.
func writeSummary(ctx context.Context, runner proc.Runner, w io.Writer) error {
	states, err := cockpit.LoadAll()
	if err != nil {
		return fmt.Errorf("load all: %w", err)
	}
	live, err := cockpit.LoadLiveClaudePanes(ctx, runner)
	if err != nil {
		// Fail-closed: drop the summary rather than show stale counts.
		obslog.New(progName).Error("live claude lookup failed", "err", err)
		live = map[string]struct{}{}
	}
	filtered := cockpit.FilterByLive(states, live)
	_, err = io.WriteString(w, cockpit.Summary(filtered))
	return err
}
