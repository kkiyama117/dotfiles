// Package cockpit models per-pane Claude state for the tmux cockpit.
//
// Cache layout (shell 互換):
//
//	${XDG_CACHE_HOME}/claude-cockpit/panes/<session>_<paneID>.status
//	File content: single line "working" / "waiting" / "done".
//
// Status mapping intentionally matches tmux-agent-status' literal
// values (concept-only inspiration, no code copied).
package cockpit

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"claude-tools/internal/atomicfile"
	"claude-tools/internal/xdg"
)

// Status is the three-valued cockpit pane state.
type Status string

const (
	StatusWorking Status = "working"
	StatusWaiting Status = "waiting"
	StatusDone    Status = "done"
)

// ParseStatus accepts only the three known string literals (case-sensitive).
// Returns ("", false) for anything else (including empty / WORKING / etc.).
func ParseStatus(s string) (Status, bool) {
	switch Status(s) {
	case StatusWorking, StatusWaiting, StatusDone:
		return Status(s), true
	}
	return "", false
}

// PaneState captures one pane's status read from the cache directory.
type PaneState struct {
	Session string
	PaneID  string // tmux pane ID like "%5"
	Status  Status
}

// CachePath returns the absolute path to the cache file for a pane.
func CachePath(session, paneID string) string {
	return filepath.Join(xdg.ClaudeCockpitCacheDir(), session+"_"+paneID+".status")
}

// WriteStatus atomically writes status for the given (session, pane).
// Creates the cache directory if missing.
func WriteStatus(session, paneID string, s Status) error {
	dir := xdg.ClaudeCockpitCacheDir()
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("mkdir cache dir: %w", err)
	}
	return atomicfile.Write(CachePath(session, paneID), []byte(string(s)), 0644)
}

// RemoveStatus removes the cache file for the given (session, pane).
// Missing file is treated as success (idempotent / safe to call when the
// file was never created — e.g. claude crashed before the first hook
// fire). Used for the SessionEnd graceful-exit cleanup path.
func RemoveStatus(session, paneID string) error {
	err := os.Remove(CachePath(session, paneID))
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("remove status: %w", err)
	}
	return nil
}

// LoadAll scans the cache dir and returns parsed pane states.
// Files with corrupt content / unparseable filenames are silently skipped.
// Missing directory returns empty slice (not an error).
func LoadAll() ([]PaneState, error) {
	dir := xdg.ClaudeCockpitCacheDir()
	entries, err := os.ReadDir(dir)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read cache dir: %w", err)
	}

	var out []PaneState
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if !strings.HasSuffix(name, ".status") {
			continue
		}
		base := strings.TrimSuffix(name, ".status")
		// Filename format: <session>_<paneID>. Session may itself contain
		// underscores (e.g. "main_repo"); split on the LAST underscore.
		idx := strings.LastIndex(base, "_")
		if idx <= 0 || idx == len(base)-1 {
			continue
		}
		session := base[:idx]
		paneID := base[idx+1:]

		data, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			continue
		}
		s, ok := ParseStatus(strings.TrimSpace(string(data)))
		if !ok {
			continue
		}
		out = append(out, PaneState{Session: session, PaneID: paneID, Status: s})
	}
	return out, nil
}

// Summary aggregates pane states into the status-right format.
// Format (byte-exact match to summary.sh):
//
//	"⚡ N ⏸ M ✓ K "  with each segment "<emoji> <count> " present only
//	when count > 0; trailing space included so the next status-right
//	segment is visually separated. Empty string when all counts are zero.
func Summary(states []PaneState) string {
	var working, waiting, done int
	for _, s := range states {
		switch s.Status {
		case StatusWorking:
			working++
		case StatusWaiting:
			waiting++
		case StatusDone:
			done++
		}
	}
	var sb strings.Builder
	if working > 0 {
		fmt.Fprintf(&sb, "⚡ %d ", working)
	}
	if waiting > 0 {
		fmt.Fprintf(&sb, "⏸ %d ", waiting)
	}
	if done > 0 {
		fmt.Fprintf(&sb, "✓ %d ", done)
	}
	return sb.String()
}
