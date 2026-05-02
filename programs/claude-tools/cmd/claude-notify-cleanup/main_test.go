package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// makeFile writes a file under dir with the given mtime offset from `now`.
// Negative offsets put the file in the past.
func makeFile(t *testing.T, dir, name string, mtimeOffset time.Duration, now time.Time) string {
	t.Helper()
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatalf("mkdir %s: %v", dir, err)
	}
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte("0\n"), 0644); err != nil {
		t.Fatalf("write %s: %v", p, err)
	}
	mt := now.Add(mtimeOffset)
	if err := os.Chtimes(p, mt, mt); err != nil {
		t.Fatalf("chtimes %s: %v", p, err)
	}
	return p
}

func sessionsDir(t *testing.T) string {
	t.Helper()
	return filepath.Join(t.TempDir(), "claude-notify", "sessions")
}

func TestCleanup_RemovesStaleIDFiles(t *testing.T) {
	now := time.Now()
	dir := sessionsDir(t)

	stale1 := makeFile(t, dir, "stale1.id", -8*24*time.Hour, now)
	stale2 := makeFile(t, dir, "stale2.id", -30*24*time.Hour, now)
	fresh1 := makeFile(t, dir, "fresh1.id", -3*24*time.Hour, now)
	fresh2 := makeFile(t, dir, "fresh2.id", -1*time.Hour, now)

	idRem, tmpRem, err := cleanup(dir, 7, now)
	if err != nil {
		t.Fatalf("cleanup: %v", err)
	}
	if idRem != 2 {
		t.Errorf("idRemoved = %d, want 2", idRem)
	}
	if tmpRem != 0 {
		t.Errorf("tmpRemoved = %d, want 0", tmpRem)
	}
	for _, p := range []string{stale1, stale2} {
		if _, err := os.Stat(p); !os.IsNotExist(err) {
			t.Errorf("%s should be deleted (Stat err = %v)", filepath.Base(p), err)
		}
	}
	for _, p := range []string{fresh1, fresh2} {
		if _, err := os.Stat(p); err != nil {
			t.Errorf("%s should be kept: %v", filepath.Base(p), err)
		}
	}
}

func TestCleanup_RemovesStaleTmpFiles(t *testing.T) {
	now := time.Now()
	dir := sessionsDir(t)

	staleTmp := makeFile(t, dir, ".tmp.abc123", -90*time.Minute, now)
	freshTmp := makeFile(t, dir, ".tmp.xyz789", -30*time.Minute, now)

	idRem, tmpRem, err := cleanup(dir, 7, now)
	if err != nil {
		t.Fatalf("cleanup: %v", err)
	}
	if idRem != 0 {
		t.Errorf("idRemoved = %d, want 0", idRem)
	}
	if tmpRem != 1 {
		t.Errorf("tmpRemoved = %d, want 1", tmpRem)
	}
	if _, err := os.Stat(staleTmp); !os.IsNotExist(err) {
		t.Errorf("staleTmp should be deleted (Stat err = %v)", err)
	}
	if _, err := os.Stat(freshTmp); err != nil {
		t.Errorf("freshTmp should be kept: %v", err)
	}
}

func TestCleanup_RejectsUnexpectedBaseDir(t *testing.T) {
	now := time.Now()
	// base_dir does NOT end with claude-notify/sessions
	bad := filepath.Join(t.TempDir(), "evil")
	victim := makeFile(t, bad, "stale.id", -30*24*time.Hour, now)

	idRem, tmpRem, err := cleanup(bad, 7, now)
	if err != nil {
		t.Fatalf("cleanup should not error on guard: %v", err)
	}
	if idRem != 0 || tmpRem != 0 {
		t.Errorf("guard breached: idRem=%d tmpRem=%d", idRem, tmpRem)
	}
	if _, err := os.Stat(victim); err != nil {
		t.Errorf("victim should NOT be deleted: %v", err)
	}
}

func TestCleanup_NonexistentBaseDir(t *testing.T) {
	now := time.Now()
	missing := filepath.Join(t.TempDir(), "claude-notify", "sessions")

	idRem, tmpRem, err := cleanup(missing, 7, now)
	if err != nil {
		t.Errorf("cleanup on missing dir should not error: %v", err)
	}
	if idRem != 0 || tmpRem != 0 {
		t.Errorf("nonexistent base_dir reported deletions: id=%d tmp=%d", idRem, tmpRem)
	}
}

func TestCleanup_IgnoresUnrelatedFiles(t *testing.T) {
	now := time.Now()
	dir := sessionsDir(t)

	keep1 := makeFile(t, dir, "session.log", -100*24*time.Hour, now)
	keep2 := makeFile(t, dir, "README.md", -100*24*time.Hour, now)

	idRem, tmpRem, err := cleanup(dir, 7, now)
	if err != nil {
		t.Fatalf("cleanup: %v", err)
	}
	if idRem != 0 || tmpRem != 0 {
		t.Errorf("unrelated files removed: id=%d tmp=%d", idRem, tmpRem)
	}
	for _, p := range []string{keep1, keep2} {
		if _, err := os.Stat(p); err != nil {
			t.Errorf("%s should be kept: %v", filepath.Base(p), err)
		}
	}
}

func TestCleanup_TTLBoundary(t *testing.T) {
	now := time.Now()
	dir := sessionsDir(t)

	// strictly-older semantics: shell `find -mtime +7` is "more than 7 * 24h ago"
	// → 7d exactly is kept, 7d + 1s is removed.
	exactly7d := makeFile(t, dir, "edge1.id", -7*24*time.Hour, now)
	just_over7d := makeFile(t, dir, "edge2.id", -7*24*time.Hour-time.Second, now)

	if _, _, err := cleanup(dir, 7, now); err != nil {
		t.Fatalf("cleanup: %v", err)
	}
	if _, err := os.Stat(exactly7d); err != nil {
		t.Errorf("exactly 7d should be kept: %v", err)
	}
	if _, err := os.Stat(just_over7d); !os.IsNotExist(err) {
		t.Errorf("just over 7d should be deleted (Stat err = %v)", err)
	}
}

func TestParseTTLDays(t *testing.T) {
	tests := []struct {
		env  string
		want int
	}{
		{"", 7},
		{"7", 7},
		{"1", 1},
		{"30", 30},
		{"0", 7},
		{"-1", 7},
		{"abc", 7},
		{"7d", 7},
		{" 7 ", 7},
		{"100000000", 100000000},
	}
	for _, tc := range tests {
		t.Run(tc.env, func(t *testing.T) {
			got := parseTTLDays(tc.env)
			if got != tc.want {
				t.Errorf("parseTTLDays(%q) = %d, want %d", tc.env, got, tc.want)
			}
		})
	}
}
