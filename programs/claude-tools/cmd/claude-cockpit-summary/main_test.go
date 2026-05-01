package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestSummary_emptyDir(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", dir)

	var buf bytes.Buffer
	if err := writeSummary(&buf); err != nil {
		t.Fatalf("writeSummary: %v", err)
	}
	if buf.String() != "" {
		t.Errorf("output = %q, want empty", buf.String())
	}
}

func TestSummary_byteExact(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", dir)

	cacheDir := filepath.Join(dir, "claude-cockpit", "panes")
	_ = os.MkdirAll(cacheDir, 0755)

	_ = os.WriteFile(filepath.Join(cacheDir, "s1_%1.status"), []byte("working"), 0644)
	_ = os.WriteFile(filepath.Join(cacheDir, "s1_%2.status"), []byte("working"), 0644)
	_ = os.WriteFile(filepath.Join(cacheDir, "s1_%3.status"), []byte("working"), 0644)
	_ = os.WriteFile(filepath.Join(cacheDir, "s2_%4.status"), []byte("waiting"), 0644)
	_ = os.WriteFile(filepath.Join(cacheDir, "s2_%5.status"), []byte("done"), 0644)
	_ = os.WriteFile(filepath.Join(cacheDir, "s2_%6.status"), []byte("done"), 0644)

	var buf bytes.Buffer
	if err := writeSummary(&buf); err != nil {
		t.Fatalf("writeSummary: %v", err)
	}
	want := "⚡ 3 ⏸ 1 ✓ 2 "
	if buf.String() != want {
		t.Errorf("output = %q, want %q", buf.String(), want)
	}
}
