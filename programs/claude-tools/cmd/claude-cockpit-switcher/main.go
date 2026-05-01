// claude-cockpit-switcher provides a hierarchical fzf switcher over
// tmux sessions/windows/panes annotated with cockpit state.
//
// Keys (fzf --expect):
//
//	Enter   -> switch-client (+ select-window / select-pane as needed)
//	Ctrl-X  -> kill the selected scope (worktree-aware for windows)
//	Ctrl-R  -> reload (re-exec self)
package main

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"syscall"

	"claude-tools/internal/obslog"
	"claude-tools/internal/proc"
	"claude-tools/internal/xdg"
)

const progName = "claude-cockpit-switcher"

func main() {
	if _, err := exec.LookPath("fzf"); err != nil {
		runTmux("display-message", "fzf required (paru -S fzf)")
		os.Exit(1)
	}

	// Fire-and-forget prune so orphans don't show up.
	_ = exec.Command(filepath.Join(os.Getenv("HOME"), ".local/bin/claude-cockpit-prune")).Start()

	ctx := context.Background()
	runner := proc.RealRunner{}
	logger := obslog.New(progName)

	lines, err := buildLines(ctx, runner)
	if err != nil {
		logger.Error("build lines failed", "err", err)
		os.Exit(1)
	}

	selection, key, err := runFzf(strings.Join(lines, "\n"))
	if err != nil {
		// User cancelled or fzf error: silent exit 0.
		os.Exit(0)
	}
	if selection == "" {
		os.Exit(0)
	}

	row, err := parseSelection(selection)
	if err != nil {
		logger.Error("parse selection failed", "raw", selection, "err", err)
		os.Exit(1)
	}

	switch key {
	case "ctrl-r":
		bin, _ := os.Executable()
		_ = syscall.Exec(bin, []string{bin}, os.Environ())
		os.Exit(0)
	case "ctrl-x":
		dispatchKill(ctx, runner, row)
	default:
		dispatchSwitch(ctx, runner, row)
	}
}

type selectedRow struct {
	kind    string // "S" / "W" / "P"
	session string
	window  string
	paneID  string
}

// badge returns the visual badge for a pane state literal.
func badge(state string) string {
	switch state {
	case "working":
		return "⚡ working"
	case "waiting":
		return "⏸ waiting"
	case "done":
		return "✓ done"
	}
	return ""
}

// stateForPane reads the cache file for (session, paneID) and returns
// its trimmed content (empty string if missing).
//
// F-8 (b3) defensive filter: when paneCmd is anything other than
// "claude", the cache value is treated as stale and "" is returned so
// the badge column blanks out. The switcher still renders the row so
// the user can switch to / kill the pane.
func stateForPane(session, paneID, paneCmd string) string {
	if paneCmd != "claude" {
		return ""
	}
	file := filepath.Join(xdg.ClaudeCockpitCacheDir(), session+"_"+paneID+".status")
	data, err := os.ReadFile(file)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

// stateForSessionFromPanes aggregates pane states with the priority
// working > waiting > done. Empty if no pane has a known state.
func stateForSessionFromPanes(states []string) string {
	hasW, hasQ, hasD := false, false, false
	for _, s := range states {
		switch s {
		case "working":
			hasW = true
		case "waiting":
			hasQ = true
		case "done":
			hasD = true
		}
	}
	switch {
	case hasW:
		return "working"
	case hasQ:
		return "waiting"
	case hasD:
		return "done"
	}
	return ""
}

// buildLines emits one tab-separated line per S/W/P entry.
// Format: "<kind>\t<session>\t<w_idx>\t<p_id>\t<display>"
func buildLines(ctx context.Context, runner proc.Runner) ([]string, error) {
	sessOut, err := runner.Run(ctx, "tmux", "list-sessions", "-F", "#{session_name}")
	if err != nil {
		return nil, fmt.Errorf("list-sessions: %w", err)
	}
	sessions := splitNonEmpty(string(sessOut))
	sort.Strings(sessions)

	var lines []string
	for _, s := range sessions {
		// Aggregate session state from its panes (server-wide, -s flag).
		// F-8 (b3): include pane_current_command so we can ignore
		// non-claude panes when computing the session badge.
		paneListOut, _ := runner.Run(ctx, "tmux",
			"list-panes", "-t", s, "-s", "-F",
			"#{pane_id}\t#{pane_current_command}")
		var paneStates []string
		for _, line := range splitNonEmpty(string(paneListOut)) {
			parts := strings.SplitN(line, "\t", 2)
			if len(parts) != 2 {
				continue
			}
			paneStates = append(paneStates, stateForPane(s, parts[0], parts[1]))
		}
		sBadge := badge(stateForSessionFromPanes(paneStates))
		lines = append(lines, fmt.Sprintf("S\t%s\t\t\t%-30s  %s", s, s, sBadge))

		winOut, err := runner.Run(ctx, "tmux", "list-windows", "-t", s, "-F", "#{window_index}\t#{window_name}")
		if err != nil {
			continue
		}
		for _, wline := range splitNonEmpty(string(winOut)) {
			parts := strings.SplitN(wline, "\t", 2)
			if len(parts) != 2 {
				continue
			}
			wIdx, wName := parts[0], parts[1]
			lines = append(lines, fmt.Sprintf("W\t%s\t%s\t\t  window:%s %s", s, wIdx, wIdx, wName))

			// F-8 (b3): extend per-window list-panes to carry
			// pane_current_command. Non-claude panes still appear
			// (so the user can navigate / kill), but their badge
			// blanks out via stateForPane.
			paneOut, err := runner.Run(ctx, "tmux",
				"list-panes", "-t", s+":"+wIdx, "-F",
				"#{pane_id}\t#{pane_current_path}\t#{pane_current_command}")
			if err != nil {
				continue
			}
			for _, pline := range splitNonEmpty(string(paneOut)) {
				pp := strings.SplitN(pline, "\t", 3)
				if len(pp) != 3 {
					continue
				}
				pID, pPath, pCmd := pp[0], pp[1], pp[2]
				pBadge := badge(stateForPane(s, pID, pCmd))
				lines = append(lines, fmt.Sprintf("P\t%s\t%s\t%s\t    pane:%s  cwd=%s    %s",
					s, wIdx, pID, pID, pPath, pBadge))
			}
		}
	}
	return lines, nil
}

// runFzf invokes fzf with our flags, pipes input, and returns
// (selection_row, key, err). On user-cancel returns ("", "", err).
func runFzf(input string) (string, string, error) {
	cmd := exec.Command("fzf",
		"--prompt=cockpit> ",
		"--height=100%",
		"--layout=reverse",
		"--no-sort",
		"--tiebreak=index",
		"--delimiter=\t",
		"--with-nth=5..",
		"--expect=ctrl-x,ctrl-r",
		"--header=enter=switch  ctrl-x=kill  ctrl-r=reload",
	)
	cmd.Stdin = strings.NewReader(input)
	cmd.Stderr = os.Stderr // fzf draws on stderr
	out, err := cmd.Output()
	if err != nil {
		return "", "", err
	}
	// fzf with --expect: first line = key (or empty for default), second = row.
	parts := strings.SplitN(strings.TrimRight(string(out), "\n"), "\n", 2)
	key := ""
	row := ""
	if len(parts) >= 1 {
		key = parts[0]
	}
	if len(parts) >= 2 {
		row = parts[1]
	}
	return row, key, nil
}

func parseSelection(row string) (selectedRow, error) {
	cols := strings.SplitN(row, "\t", 5)
	if len(cols) < 4 {
		return selectedRow{}, fmt.Errorf("malformed row (need 4+ cols): %q", row)
	}
	return selectedRow{
		kind:    cols[0],
		session: cols[1],
		window:  cols[2],
		paneID:  cols[3],
	}, nil
}

func dispatchSwitch(ctx context.Context, runner proc.Runner, row selectedRow) {
	switch row.kind {
	case "S":
		_, _ = runner.Run(ctx, "tmux", "switch-client", "-t", row.session)
	case "W":
		_, _ = runner.Run(ctx, "tmux", "switch-client", "-t", row.session,
			";", "select-window", "-t", row.session+":"+row.window)
	case "P":
		_, _ = runner.Run(ctx, "tmux", "switch-client", "-t", row.session,
			";", "select-window", "-t", row.session+":"+row.window,
			";", "select-pane", "-t", row.paneID)
	}
}

func dispatchKill(ctx context.Context, runner proc.Runner, row selectedRow) {
	switch row.kind {
	case "P":
		if !confirmYesNo(fmt.Sprintf("kill pane %s? (y/N) ", row.paneID)) {
			return
		}
		_, _ = runner.Run(ctx, "tmux", "kill-pane", "-t", row.paneID)
	case "W":
		// Check @claude-managed window option.
		out, _ := runner.Run(ctx, "tmux", "show-options", "-w", "-t", row.session+":"+row.window, "-v", "@claude-managed")
		managed := strings.TrimSpace(string(out)) == "yes"
		if managed {
			if !confirmYesNo(fmt.Sprintf("kill claude window %s:%s and worktree? (y/N) ", row.session, row.window)) {
				return
			}
			cmd := exec.CommandContext(ctx,
				filepath.Join(os.Getenv("HOME"), ".config/tmux/scripts/claude-kill-session.sh"),
				row.session+":"+row.window)
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
			_ = cmd.Run()
			return
		}
		if !confirmYesNo(fmt.Sprintf("kill window %s:%s? (y/N) ", row.session, row.window)) {
			return
		}
		_, _ = runner.Run(ctx, "tmux", "kill-window", "-t", row.session+":"+row.window)
	case "S":
		if !confirmYesNo(fmt.Sprintf("kill session %s? (worktrees kept) (y/N) ", row.session)) {
			return
		}
		_, _ = runner.Run(ctx, "tmux", "kill-session", "-t", row.session)
	}
}

// confirmYesNo prompts on stderr and reads from /dev/tty so the popup
// keeps focus. Matches shell switcher.sh's `IFS= read -r ans </dev/tty`.
func confirmYesNo(prompt string) bool {
	tty, err := os.Open("/dev/tty")
	if err != nil {
		return false
	}
	defer tty.Close()
	fmt.Fprint(os.Stderr, prompt)
	r := bufio.NewReader(tty)
	line, _ := r.ReadString('\n')
	line = strings.TrimSpace(line)
	return len(line) > 0 && (line[0] == 'y' || line[0] == 'Y')
}

// runTmux is a small helper for one-shot tmux invocations that don't
// need the proc.Runner abstraction (display-message at startup).
func runTmux(args ...string) {
	_ = exec.Command("tmux", args...).Run()
}

func splitNonEmpty(s string) []string {
	var out []string
	for _, line := range strings.Split(s, "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			out = append(out, line)
		}
	}
	return out
}
