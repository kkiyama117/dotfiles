// claude-kill-session removes the current claude-managed window and its
// matching git worktree. The 3-stage safety check (managed=yes /
// pane has 'claude' / legacy 'claude-' session prefix) must pass; the
// caller (tmux confirm-before binding) is responsible for user
// confirmation.
package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"claude-tools/internal/gitwt"
	"claude-tools/internal/obslog"
	"claude-tools/internal/proc"
	"claude-tools/internal/tmux"
	"claude-tools/internal/xdg"
)

const progName = "claude-kill-session"

var logger = obslog.New(progName)

func main() {
	ctx := context.Background()
	r := proc.RealRunner{}
	tc := tmux.New(r)

	explicit := ""
	if len(os.Args) > 1 {
		explicit = os.Args[1]
	}

	session, window, err := resolveTarget(ctx, tc, explicit)
	if err != nil {
		tc.Display(ctx, fmt.Sprintf("claude-kill-session: %s", err))
		os.Exit(1)
	}
	target := session + ":" + window

	managed, _ := tc.ShowWindowOption(ctx, target, "@claude-managed")
	panes, _ := tc.ListPanes(ctx, target, "#{pane_current_command}")
	if !isClaudeManaged(managed, panes, session) {
		tc.Display(ctx, fmt.Sprintf("claude-kill-session: refusing on non-claude window (%s)", target))
		os.Exit(1)
	}

	wtRoot, _ := tc.ShowWindowOption(ctx, target, "@claude-worktree")
	mainRepo, _ := tc.ShowWindowOption(ctx, target, "@claude-main-repo")

	// fallback: derive from active pane's pane_current_path
	if wtRoot == "" || mainRepo == "" {
		panePath, _ := tc.DisplayMessageGet(ctx, target, "#{pane_current_path}")
		if panePath != "" && dirExists(panePath) {
			gw := gitwt.New(r)
			if wtRoot == "" {
				if v, err := gw.TopLevel(ctx, panePath); err == nil {
					wtRoot = v
				}
			}
			if mainRepo == "" {
				if v, err := gw.MainRepo(ctx, panePath); err == nil {
					mainRepo = v
				}
			}
		}
	}

	// Pane id capture for cache cleanup
	paneIDs, _ := tc.ListPanes(ctx, target, "#{pane_id}")

	// Sanity check: warn if worktree is outside <main-repo>/.dmux/worktrees/.
	// Do not block — legacy paths or hand-managed worktrees still need to be
	// cleanable.
	if wtRoot != "" && mainRepo != "" {
		dmuxRoot := gitwt.DmuxWorktreeRoot(mainRepo) + string(os.PathSeparator)
		if !strings.HasPrefix(wtRoot, dmuxRoot) {
			logger.Warn("worktree path is outside <main-repo>/.dmux/worktrees/, proceeding anyway",
				"wtRoot", wtRoot, "expected_prefix", dmuxRoot)
		}
	}

	// Worktree remove BEFORE kill-window (so error display still has a client).
	if wtRoot != "" && mainRepo != "" && wtRoot != mainRepo {
		gw := gitwt.New(r)
		if dirExists(wtRoot) {
			if msg, err := gw.Remove(ctx, mainRepo, wtRoot); err != nil {
				tc.Display(ctx, fmt.Sprintf("kept worktree %s: %s", wtRoot, msg))
			}
		}
		gw.Prune(ctx, mainRepo)
	}

	if err := tc.KillWindow(ctx, target); err != nil {
		logger.Error("kill-window failed", "target", target, "err", err)
		tc.Display(ctx, fmt.Sprintf("claude-kill-session: kill-window failed: %s", err))
		os.Exit(1)
	}

	// Cockpit cache cleanup
	cacheDir := xdg.ClaudeCockpitCacheDir()
	if cacheDir != "" {
		for _, pid := range paneIDs {
			_ = os.Remove(filepath.Join(cacheDir, session+"_"+pid+".status"))
		}
	}
}

// resolveTarget returns the session and window names for the kill target.
// explicit is the optional first CLI arg (session-only or session:window form).
func resolveTarget(ctx context.Context, tc *tmux.Client, explicit string) (string, string, error) {
	if explicit != "" {
		s, err := tc.DisplayMessageGet(ctx, explicit, "#S")
		if err != nil {
			return "", "", fmt.Errorf("target not found (%s)", explicit)
		}
		w, err := tc.DisplayMessageGet(ctx, explicit, "#W")
		if err != nil {
			return "", "", fmt.Errorf("window not found (%s)", explicit)
		}
		return s, w, nil
	}
	s, err := tc.DisplayMessageGet(ctx, "", "#S")
	if err != nil {
		return "", "", err
	}
	w, err := tc.DisplayMessageGet(ctx, "", "#W")
	if err != nil {
		return "", "", err
	}
	return s, w, nil
}

// isClaudeManaged implements the 3-stage OR safety check.
//
//	managed == "yes" OR any pane runs 'claude' OR session starts with 'claude-'
func isClaudeManaged(managedOpt string, panes []string, session string) bool {
	if managedOpt == "yes" {
		return true
	}
	for _, p := range panes {
		if strings.TrimSpace(p) == "claude" {
			return true
		}
	}
	return strings.HasPrefix(session, "claude-")
}

func dirExists(p string) bool {
	st, err := os.Stat(p)
	return err == nil && st.IsDir()
}
