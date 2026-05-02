package notify

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSafeSessionID(t *testing.T) {
	tests := []struct {
		in, want string
	}{
		{"", "unknown"},
		{"abc-123_DEF", "abc-123_DEF"},
		{"a/b/c", "a_b_c"},
		{"foo bar", "foo_bar"},
		{"path/with/slashes", "path_with_slashes"},
		{"....", "____"},
		{"abc..def", "abc__def"},
	}
	for _, tc := range tests {
		got := SafeSessionID(tc.in)
		if got != tc.want {
			t.Errorf("SafeSessionID(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

// Multi-byte chars are non-ASCII so each underlying byte becomes `_`.
// Verify the contract (replacement happens) without hard-coding the
// exact byte count.
func TestSafeSessionID_MultibyteAllReplaced(t *testing.T) {
	got := SafeSessionID("日本語")
	if got == "" || strings.ContainsAny(got, "日本語") {
		t.Errorf("SafeSessionID(%q) = %q, expected all chars replaced", "日本語", got)
	}
	for _, r := range got {
		if r != '_' {
			t.Errorf("non-underscore in result %q at %q", got, string(r))
			break
		}
	}
}

func TestLoadReplaceID_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	sid := "session-abc"
	if err := SaveReplaceID(dir, sid, 42); err != nil {
		t.Fatalf("save: %v", err)
	}
	got := LoadReplaceID(dir, sid)
	if got != 42 {
		t.Errorf("got %d, want 42", got)
	}
}

func TestLoadReplaceID_MissingFile(t *testing.T) {
	dir := t.TempDir()
	if got := LoadReplaceID(dir, "nope"); got != 0 {
		t.Errorf("missing file should yield 0, got %d", got)
	}
}

func TestLoadReplaceID_MalformedContent(t *testing.T) {
	tests := []struct {
		name, content string
	}{
		{"alpha", "abc"},
		{"empty", ""},
		{"whitespace", "   \n"},
		{"negative", "-1"},
		{"zero", "0"},
		{"leading-zero", "042"},
		{"mixed", "12abc"},
		{"trailing-garbage", "12 garbage"},
	}
	dir := t.TempDir()
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(dir, SafeSessionID("sid-"+tc.name)+".id")
			if err := os.WriteFile(path, []byte(tc.content), 0644); err != nil {
				t.Fatalf("write: %v", err)
			}
			got := LoadReplaceID(dir, "sid-"+tc.name)
			if got != 0 {
				t.Errorf("LoadReplaceID(%q) = %d, want 0", tc.content, got)
			}
		})
	}
}

func TestSaveReplaceID_NoOpForEmptySid(t *testing.T) {
	dir := t.TempDir()
	if err := SaveReplaceID(dir, "", 5); err != nil {
		t.Errorf("empty sid should be no-op without error, got %v", err)
	}
	entries, _ := os.ReadDir(dir)
	if len(entries) != 0 {
		t.Errorf("no files should be created, got %d", len(entries))
	}
}

func TestSaveReplaceID_NoOpForZero(t *testing.T) {
	dir := t.TempDir()
	if err := SaveReplaceID(dir, "sid-1", 0); err != nil {
		t.Errorf("zero id should be no-op, got %v", err)
	}
	if got := LoadReplaceID(dir, "sid-1"); got != 0 {
		t.Errorf("expected no state for sid-1, got %d", got)
	}
}

func TestSaveReplaceID_CreatesParentDir(t *testing.T) {
	root := t.TempDir()
	stateDir := filepath.Join(root, "claude-notify", "sessions")
	if err := SaveReplaceID(stateDir, "sid-create", 99); err != nil {
		t.Fatalf("save with non-existent parent: %v", err)
	}
	if got := LoadReplaceID(stateDir, "sid-create"); got != 99 {
		t.Errorf("got %d, want 99", got)
	}
}

func TestSaveReplaceID_Overwrite(t *testing.T) {
	dir := t.TempDir()
	sid := "sid-overwrite"
	for _, id := range []uint32{1, 2, 100, 99999} {
		if err := SaveReplaceID(dir, sid, id); err != nil {
			t.Fatalf("save id=%d: %v", id, err)
		}
		if got := LoadReplaceID(dir, sid); got != id {
			t.Errorf("after save id=%d, load returned %d", id, got)
		}
	}
}

func TestStateFilePath_UsesSafeSessionID(t *testing.T) {
	got := stateFilePath("/state", "a/b c")
	want := "/state/a_b_c.id"
	if got != want {
		t.Errorf("stateFilePath = %q, want %q", got, want)
	}
}

func TestLoadAllReplaceIDs(t *testing.T) {
	t.Run("returns only valid positive ids", func(t *testing.T) {
		dir := t.TempDir()
		// valid: "100\n" — matches ^[1-9][0-9]*$
		if err := os.WriteFile(filepath.Join(dir, "valid.id"), []byte("100\n"), 0644); err != nil {
			t.Fatalf("write valid.id: %v", err)
		}
		// zero: "0\n" — invalid (leading zero / equals zero)
		if err := os.WriteFile(filepath.Join(dir, "zero.id"), []byte("0\n"), 0644); err != nil {
			t.Fatalf("write zero.id: %v", err)
		}
		// letters: "abc\n" — invalid
		if err := os.WriteFile(filepath.Join(dir, "letters.id"), []byte("abc\n"), 0644); err != nil {
			t.Fatalf("write letters.id: %v", err)
		}
		// no .id suffix: must be ignored entirely
		if err := os.WriteFile(filepath.Join(dir, "ignore.txt"), []byte("999\n"), 0644); err != nil {
			t.Fatalf("write ignore.txt: %v", err)
		}

		got, err := LoadAllReplaceIDs(dir)
		if err != nil {
			t.Fatalf("LoadAllReplaceIDs: unexpected error: %v", err)
		}
		if len(got) != 1 {
			t.Fatalf("LoadAllReplaceIDs: got %d entries, want 1; map=%v", len(got), got)
		}
		if got["valid"] != 100 {
			t.Errorf("got[\"valid\"] = %d, want 100", got["valid"])
		}
	})

	t.Run("empty dir returns empty map nil error", func(t *testing.T) {
		dir := t.TempDir()
		got, err := LoadAllReplaceIDs(dir)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(got) != 0 {
			t.Errorf("expected empty map, got %v", got)
		}
	})

	t.Run("missing dir returns empty map nil error", func(t *testing.T) {
		dir := t.TempDir()
		missing := filepath.Join(dir, "does-not-exist")
		got, err := LoadAllReplaceIDs(missing)
		if err != nil {
			t.Fatalf("missing dir should not error, got: %v", err)
		}
		if len(got) != 0 {
			t.Errorf("expected empty map for missing dir, got %v", got)
		}
	})
}
