package cockpit

import (
	"os"
	"path/filepath"
	"testing"
)

func TestStatusParse(t *testing.T) {
	tests := []struct {
		in   string
		want Status
		ok   bool
	}{
		{"working", StatusWorking, true},
		{"waiting", StatusWaiting, true},
		{"done", StatusDone, true},
		{"", "", false},
		{"unknown", "", false},
		{"WORKING", "", false}, // case-sensitive: shell が小文字限定
	}
	for _, tt := range tests {
		got, ok := ParseStatus(tt.in)
		if ok != tt.ok || got != tt.want {
			t.Errorf("ParseStatus(%q) = (%v, %v), want (%v, %v)", tt.in, got, ok, tt.want, tt.ok)
		}
	}
}

func TestCachePath(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", "/tmp/test-cache")
	got := CachePath("mysession", "%5")
	want := "/tmp/test-cache/claude-cockpit/panes/mysession_%5.status"
	if got != want {
		t.Errorf("CachePath = %q, want %q", got, want)
	}
}

func TestWriteStatus_thenLoad(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", dir)

	if err := WriteStatus("sess", "%1", StatusWorking); err != nil {
		t.Fatalf("WriteStatus: %v", err)
	}

	// Verify file content matches shell format (no trailing newline).
	data, err := os.ReadFile(CachePath("sess", "%1"))
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if string(data) != "working" {
		t.Errorf("file content = %q, want %q", data, "working")
	}
}

func TestLoadAll_skipsCorrupt(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", dir)

	cacheDir := filepath.Join(dir, "claude-cockpit", "panes")
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Valid: working
	_ = os.WriteFile(filepath.Join(cacheDir, "s1_%1.status"), []byte("working"), 0644)
	// Valid: done
	_ = os.WriteFile(filepath.Join(cacheDir, "s1_%2.status"), []byte("done"), 0644)
	// Corrupt: garbage content
	_ = os.WriteFile(filepath.Join(cacheDir, "s2_%3.status"), []byte("xyz"), 0644)
	// Wrong extension: ignored
	_ = os.WriteFile(filepath.Join(cacheDir, "s2_%4.txt"), []byte("waiting"), 0644)
	// Bad filename (no underscore): ignored
	_ = os.WriteFile(filepath.Join(cacheDir, "no-underscore.status"), []byte("done"), 0644)

	states, err := LoadAll()
	if err != nil {
		t.Fatalf("LoadAll: %v", err)
	}
	if len(states) != 2 {
		t.Errorf("got %d states, want 2 (working + done): %+v", len(states), states)
	}
}

func TestLoadAll_emptyWhenDirMissing(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", dir)

	states, err := LoadAll()
	if err != nil {
		t.Fatalf("LoadAll on missing dir should not error: %v", err)
	}
	if len(states) != 0 {
		t.Errorf("expected 0 states, got %d", len(states))
	}
}

func TestRemoveStatus_existing(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", dir)

	if err := WriteStatus("sess", "%1", StatusWorking); err != nil {
		t.Fatalf("WriteStatus: %v", err)
	}
	if _, err := os.Stat(CachePath("sess", "%1")); err != nil {
		t.Fatalf("precondition: file should exist: %v", err)
	}

	if err := RemoveStatus("sess", "%1"); err != nil {
		t.Fatalf("RemoveStatus: %v", err)
	}
	if _, err := os.Stat(CachePath("sess", "%1")); !os.IsNotExist(err) {
		t.Errorf("file still exists after RemoveStatus (Stat err = %v)", err)
	}
}

func TestRemoveStatus_missingIsIdempotent(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", dir)

	// File never written; remove must not be an error.
	if err := RemoveStatus("never", "%99"); err != nil {
		t.Errorf("RemoveStatus on missing file should be nil, got %v", err)
	}
}

func TestSummary(t *testing.T) {
	tests := []struct {
		name   string
		states []PaneState
		want   string
	}{
		{
			name:   "empty",
			states: nil,
			want:   "",
		},
		{
			name: "only working",
			states: []PaneState{
				{Status: StatusWorking}, {Status: StatusWorking},
			},
			want: "⚡ 2 ",
		},
		{
			name: "all three",
			states: []PaneState{
				{Status: StatusWorking}, {Status: StatusWorking}, {Status: StatusWorking},
				{Status: StatusWaiting},
				{Status: StatusDone}, {Status: StatusDone},
			},
			want: "⚡ 3 ⏸ 1 ✓ 2 ",
		},
		{
			name: "only done",
			states: []PaneState{
				{Status: StatusDone}, {Status: StatusDone},
			},
			want: "✓ 2 ",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Summary(tt.states)
			if got != tt.want {
				t.Errorf("Summary = %q, want %q", got, tt.want)
			}
		})
	}
}
