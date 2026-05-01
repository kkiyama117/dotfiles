package atomicfile

import (
	"os"
	"path/filepath"
	"testing"
)

func TestWrite_basic(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "out.status")
	data := []byte("working")

	if err := Write(target, data, 0644); err != nil {
		t.Fatalf("Write: %v", err)
	}

	got, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if string(got) != "working" {
		t.Errorf("content = %q, want %q", got, "working")
	}

	info, err := os.Stat(target)
	if err != nil {
		t.Fatalf("Stat: %v", err)
	}
	if info.Mode().Perm() != 0644 {
		t.Errorf("perm = %v, want 0644", info.Mode().Perm())
	}
}

func TestWrite_dirMissing(t *testing.T) {
	target := filepath.Join(t.TempDir(), "nonexistent-subdir", "out.status")
	err := Write(target, []byte("x"), 0644)
	if err == nil {
		t.Fatal("Write should fail when parent dir missing")
	}
}

func TestWrite_overwrite(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "out.status")
	if err := Write(target, []byte("first"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := Write(target, []byte("second"), 0644); err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(target)
	if string(got) != "second" {
		t.Errorf("content = %q, want %q", got, "second")
	}
}

func TestWrite_noTmpLeftBehind(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "out.status")
	if err := Write(target, []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		if e.Name() != "out.status" {
			t.Errorf("unexpected leftover file: %s", e.Name())
		}
	}
}
