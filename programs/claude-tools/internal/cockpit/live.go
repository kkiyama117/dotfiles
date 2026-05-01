package cockpit

import (
	"context"
	"fmt"
	"strings"

	"claude-tools/internal/proc"
)

// LoadLiveClaudePanes returns the set of "<session>_<pane_id>" keys for
// every tmux pane whose pane_current_command is "claude". Built by a
// single `tmux list-panes -a` call and used by summary / next-ready /
// switcher / prune as a defensive filter against stale cache files
// (claude killed by SIGKILL / OOM / pane closed without /exit).
//
// Errors from tmux are propagated; callers that want fail-closed
// behaviour (= treat tmux failure as "no live claude") can pass the
// returned (nil, err) into FilterByLive which collapses nil to empty.
func LoadLiveClaudePanes(ctx context.Context, runner proc.Runner) (map[string]struct{}, error) {
	out, err := runner.Run(ctx, "tmux",
		"list-panes", "-a", "-F",
		"#{session_name}_#{pane_id}\t#{pane_current_command}")
	if err != nil {
		return nil, fmt.Errorf("tmux list-panes: %w", err)
	}
	live := make(map[string]struct{})
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimRight(line, "\r")
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "\t", 2)
		if len(parts) != 2 {
			continue
		}
		if parts[1] == "claude" {
			live[parts[0]] = struct{}{}
		}
	}
	return live, nil
}

// FilterByLive returns the subset of states whose <session>_<pane_id>
// key is present in the live set. A nil or empty live map is treated
// as fail-closed: every state is filtered out. Pass an explicit
// non-empty map to keep matches.
func FilterByLive(states []PaneState, live map[string]struct{}) []PaneState {
	if len(live) == 0 {
		return nil
	}
	out := make([]PaneState, 0, len(states))
	for _, st := range states {
		key := st.Session + "_" + st.PaneID
		if _, ok := live[key]; ok {
			out = append(out, st)
		}
	}
	return out
}
