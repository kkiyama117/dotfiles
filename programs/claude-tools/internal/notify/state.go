package notify

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"claude-tools/internal/atomicfile"
)

// SafeSessionID sanitises a Claude session_id for use as a filename.
// Mirrors the shell substitution `${session_id//[^a-zA-Z0-9_-]/_}`
// followed by the empty-string fallback to "unknown".
func SafeSessionID(sid string) string {
	if sid == "" {
		return "unknown"
	}
	var b strings.Builder
	b.Grow(len(sid))
	for _, r := range sid {
		switch {
		case r >= '0' && r <= '9',
			r >= 'a' && r <= 'z',
			r >= 'A' && r <= 'Z',
			r == '_' || r == '-':
			b.WriteRune(r)
		default:
			b.WriteRune('_')
		}
	}
	if b.Len() == 0 {
		return "unknown"
	}
	return b.String()
}

// stateFilePath returns the per-session state file path. stateDir is
// usually `StateDir()` but tests pass a t.TempDir() override.
func stateFilePath(stateDir, sid string) string {
	return filepath.Join(stateDir, SafeSessionID(sid)+".id")
}

// LoadReplaceID reads the previous notification id for sid. Returns 0
// on any error (missing, unreadable, malformed) — matches shell parity
// where a missing or invalid prev_id silently disables --replace-id.
//
// Empty sid → 0 (no replace-id state to track).
func LoadReplaceID(stateDir, sid string) uint32 {
	if sid == "" || stateDir == "" {
		return 0
	}
	data, err := os.ReadFile(stateFilePath(stateDir, sid))
	if err != nil {
		return 0
	}
	s := strings.TrimSpace(string(data))
	// Shell regex: ^[1-9][0-9]*$ — strict positive integer (no leading 0).
	if s == "" || s[0] == '0' {
		return 0
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return 0
		}
	}
	n, err := strconv.ParseUint(s, 10, 32)
	if err != nil {
		return 0
	}
	return uint32(n)
}

// SaveReplaceID atomically writes id under sid's state file. mkdir -p
// the state dir first (idempotent). No-op for empty sid or id==0.
//
// Returns the error from atomicfile.Write so the caller can log it;
// shell behaviour is silent on save failure.
func SaveReplaceID(stateDir, sid string, id uint32) error {
	if sid == "" || stateDir == "" {
		return nil
	}
	if id == 0 {
		// Don't persist zero — would round-trip-load as "no prev id".
		return nil
	}
	if err := os.MkdirAll(stateDir, 0o755); err != nil {
		return fmt.Errorf("mkdir state dir: %w", err)
	}
	data := []byte(strconv.FormatUint(uint64(id), 10) + "\n")
	return atomicfile.Write(stateFilePath(stateDir, sid), data, 0o644)
}
