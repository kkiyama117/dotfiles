// claude-cockpit-next-ready jumps to the next pane whose cockpit state
// is "done", in inbox order: session-name asc → window-index asc →
// pane-index asc. Cycles past the current pane.
package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"claude-tools/internal/obslog"
	"claude-tools/internal/proc"
	"claude-tools/internal/xdg"
)

const progName = "claude-cockpit-next-ready"

type doneRow struct {
	session string
	window  string
	paneID  string
}

type paneRow struct {
	id    string
	index int
}

func main() {
	ctx := context.Background()
	runner := proc.RealRunner{}
	logger := obslog.New(progName)

	cacheDir := xdg.ClaudeCockpitCacheDir()
	if _, err := os.Stat(cacheDir); os.IsNotExist(err) {
		displayMessage(ctx, runner, "no ready claude pane")
		os.Exit(0)
	}

	rows, err := buildDoneList(ctx, runner)
	if err != nil {
		logger.Error("build done list failed", "err", err)
		displayMessage(ctx, runner, "no ready claude pane")
		os.Exit(0)
	}
	if len(rows) == 0 {
		displayMessage(ctx, runner, "no ready claude pane")
		os.Exit(0)
	}

	curPaneOut, err := runner.Run(ctx, "tmux", "display-message", "-p", "#{pane_id}")
	if err != nil {
		logger.Error("get current pane failed", "err", err)
		os.Exit(1)
	}
	cur := strings.TrimSpace(string(curPaneOut))

	target := pickNext(rows, cur)
	if target == (doneRow{}) {
		os.Exit(0)
	}

	if _, err := runner.Run(ctx, "tmux",
		"switch-client", "-t", target.session,
		";", "select-window", "-t", target.session+":"+target.window,
		";", "select-pane", "-t", target.paneID,
	); err != nil {
		logger.Error("switch failed", "target", target, "err", err)
		os.Exit(1)
	}
}

// buildDoneList enumerates panes whose cached state is "done", sorted in
// inbox order: session-name asc, window-index asc, pane-index asc.
func buildDoneList(ctx context.Context, runner proc.Runner) ([]doneRow, error) {
	cacheDir := xdg.ClaudeCockpitCacheDir()

	sessOut, err := runner.Run(ctx, "tmux", "list-sessions", "-F", "#{session_name}")
	if err != nil {
		return nil, fmt.Errorf("list-sessions: %w", err)
	}
	sessions := splitNonEmpty(string(sessOut))
	sort.Strings(sessions)

	var rows []doneRow
	for _, s := range sessions {
		winOut, err := runner.Run(ctx, "tmux", "list-windows", "-t", s, "-F", "#{window_index}")
		if err != nil {
			continue
		}
		windows := splitNonEmpty(string(winOut))
		sort.SliceStable(windows, func(i, j int) bool {
			return atoiOrZero(windows[i]) < atoiOrZero(windows[j])
		})

		for _, w := range windows {
			paneOut, err := runner.Run(ctx, "tmux", "list-panes", "-t", s+":"+w, "-F", "#{pane_id}\t#{pane_index}")
			if err != nil {
				continue
			}
			var panes []paneRow
			for _, line := range splitNonEmpty(string(paneOut)) {
				parts := strings.SplitN(line, "\t", 2)
				if len(parts) != 2 {
					continue
				}
				panes = append(panes, paneRow{id: parts[0], index: atoiOrZero(parts[1])})
			}
			sort.SliceStable(panes, func(i, j int) bool { return panes[i].index < panes[j].index })

			for _, p := range panes {
				file := filepath.Join(cacheDir, s+"_"+p.id+".status")
				data, err := os.ReadFile(file)
				if err != nil {
					continue
				}
				if strings.TrimSpace(string(data)) == "done" {
					rows = append(rows, doneRow{session: s, window: w, paneID: p.id})
				}
			}
		}
	}
	return rows, nil
}

// pickNext returns the row after the one matching cur paneID; wraps to
// the first row if cur is the last; returns the first row if cur is not
// in the list. Returns zero value when rows is empty.
func pickNext(rows []doneRow, cur string) doneRow {
	if len(rows) == 0 {
		return doneRow{}
	}
	for i, r := range rows {
		if r.paneID == cur {
			return rows[(i+1)%len(rows)]
		}
	}
	return rows[0]
}

func displayMessage(ctx context.Context, runner proc.Runner, msg string) {
	_, _ = runner.Run(ctx, "tmux", "display-message", "-d", "1000", msg)
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

func atoiOrZero(s string) int {
	n := 0
	for _, c := range s {
		if c < '0' || c > '9' {
			return 0
		}
		n = n*10 + int(c-'0')
	}
	return n
}
