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

	"claude-tools/internal/cockpit"
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

// prune builds the live *claude* tmux pane key set
// (cockpit.LoadLiveClaudePanes — F-8 v1 sweep) and deletes cached
// files whose basename (minus .status) is not in that set. This
// removes status files for panes that have either disappeared OR
// are now running zsh / vim / a different command.
//
// Returns an error from tmux failure; in that case it does NOT delete
// any cached files (we can't tell which are orphans without the live set).
func prune(ctx context.Context, runner proc.Runner) error {
	live, err := cockpit.LoadLiveClaudePanes(ctx, runner)
	if err != nil {
		return fmt.Errorf("tmux list-panes: %w", err)
	}

	pruneDir(xdg.ClaudeCockpitCacheDir(), live)
	// Defensive: shell prune.sh also cleans sessions/ if it exists.
	pruneDir(filepath.Join(xdg.CacheDir(), "claude-cockpit", "sessions"), live)
	return nil
}

// pruneDir removes .status files in dir whose basename is not in live.
// Empty live (= no live claude pane) deletes everything; that matches
// shell prune.sh and the F-8 v1 contract.
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
