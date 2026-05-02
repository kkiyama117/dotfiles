// claude-notify-cleanup prunes stale dispatcher state files under
// ${XDG_RUNTIME_DIR:-/tmp}/claude-notify/sessions/.
//
// Behaviour (shell parity with the previous claude-notify-cleanup.sh):
//   - Removes "*.id" files whose mtime is older than $TTL_DAYS (default 7).
//   - Removes leftover ".tmp.*" mktemp artefacts older than 60 minutes.
//   - Refuses to operate outside the expected base path (suffix check) as
//     a guard against env injection. Mirrors the shell case statement.
//
// Wired into systemd --user via claude-notify-cleanup.{service,timer}.
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"claude-tools/internal/notify"
	"claude-tools/internal/obslog"
)

const (
	progName    = "claude-notify-cleanup"
	defaultTTL  = 7
	tmpMaxAge   = 60 * time.Minute
	expectedSfx = "claude-notify/sessions"
)

var logger = obslog.New(progName)

func main() {
	baseDir := notify.StateDir()
	ttlDays := parseTTLDays(os.Getenv("CLAUDE_NOTIFY_CLEANUP_TTL_DAYS"))

	idRem, tmpRem, err := cleanup(baseDir, ttlDays, time.Now())
	if err != nil {
		logger.Error("cleanup failed", "err", err, "base_dir", baseDir)
		os.Exit(1)
	}
	if idRem > 0 || tmpRem > 0 {
		logger.Info("removed stale state",
			"id", idRem,
			"tmp", tmpRem,
			"ttl_days", ttlDays,
			"base_dir", baseDir,
		)
	}
}

// parseTTLDays returns env parsed as positive integer, or defaultTTL on
// any malformed input. Mirrors the shell regex `^[1-9][0-9]*$` plus the
// fallback to 7.
func parseTTLDays(env string) int {
	if env == "" {
		return defaultTTL
	}
	for _, r := range env {
		if r < '0' || r > '9' {
			return defaultTTL
		}
	}
	var n int
	if _, err := fmt.Sscanf(env, "%d", &n); err != nil {
		return defaultTTL
	}
	if n <= 0 {
		return defaultTTL
	}
	return n
}

// cleanup deletes stale state files under baseDir. Returns counts of
// removed `*.id` and `.tmp.*` files. Errors are limited to ReadDir
// failures other than NotExist; per-file failures are logged but do not
// fail the run.
//
// The suffix guard rejects any baseDir whose cleaned path does not end
// in "claude-notify/sessions". Defence in depth against XDG_RUNTIME_DIR
// pointing somewhere unexpected.
func cleanup(baseDir string, ttlDays int, now time.Time) (int, int, error) {
	clean := filepath.Clean(baseDir)
	if !strings.HasSuffix(clean, expectedSfx) {
		logger.Warn("refusing unexpected base_dir", "base_dir", baseDir)
		return 0, 0, nil
	}

	entries, err := os.ReadDir(clean)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, 0, nil
		}
		return 0, 0, fmt.Errorf("readdir %s: %w", clean, err)
	}

	idCutoff := now.Add(-time.Duration(ttlDays) * 24 * time.Hour)
	tmpCutoff := now.Add(-tmpMaxAge)

	var idRem, tmpRem int
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		isID := strings.HasSuffix(name, ".id")
		isTmp := strings.HasPrefix(name, ".tmp.")
		if !isID && !isTmp {
			continue
		}

		info, err := e.Info()
		if err != nil {
			logger.Warn("stat failed", "file", name, "err", err)
			continue
		}

		var cutoff time.Time
		if isID {
			cutoff = idCutoff
		} else {
			cutoff = tmpCutoff
		}
		// `find -mtime +N` is strictly older than N. Replicate that:
		// keep mtime equal to the cutoff, remove anything before it.
		if !info.ModTime().Before(cutoff) {
			continue
		}

		full := filepath.Join(clean, name)
		if err := os.Remove(full); err != nil {
			logger.Warn("remove failed", "file", name, "err", err)
			continue
		}
		if isID {
			idRem++
		} else {
			tmpRem++
		}
	}
	return idRem, tmpRem, nil
}
