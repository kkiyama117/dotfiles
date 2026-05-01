// claude-cockpit-prune removes per-pane cockpit cache files for tmux
// panes that no longer exist.
//
// Safe to run any time. Idempotent. Called from tmux.conf at server-start
// (`run -b`) and from cockpit-switcher at startup.
package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"claude-tools/internal/obslog"
	"claude-tools/internal/proc"
	"claude-tools/internal/xdg"
)

const progName = "claude-cockpit-prune"

func main() {
	if err := prune(context.Background(), proc.RealRunner{}); err != nil {
		obslog.New(progName).Error("prune failed", "err", err)
		os.Exit(1)
	}
}

// prune builds the live tmux pane key set and deletes cached files
// whose basename (minus .status) is not in the set.
//
// Returns an error from tmux failure; in that case it does NOT delete
// any cached files (we can't tell which are orphans without the live set).
func prune(ctx context.Context, runner proc.Runner) error {
	out, err := runner.Run(ctx, "tmux", "list-panes", "-a", "-F", "#{session_name}_#{pane_id}")
	if err != nil {
		return fmt.Errorf("tmux list-panes: %w", err)
	}
	live := make(map[string]struct{})
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			live[line] = struct{}{}
		}
	}

	pruneDir(xdg.ClaudeCockpitCacheDir(), live)
	// Defensive: shell prune.sh also cleans sessions/ if it exists.
	pruneDir(filepath.Join(xdg.CacheDir(), "claude-cockpit", "sessions"), live)
	return nil
}

func pruneDir(dir string, live map[string]struct{}) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return
		}
		obslog.New(progName).Error("readdir failed", "dir", dir, "err", err)
		return
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if !strings.HasSuffix(name, ".status") {
			continue
		}
		key := strings.TrimSuffix(name, ".status")
		if _, ok := live[key]; ok {
			continue
		}
		if err := os.Remove(filepath.Join(dir, name)); err != nil {
			obslog.New(progName).Error("remove failed", "file", name, "err", err)
		}
	}
}
