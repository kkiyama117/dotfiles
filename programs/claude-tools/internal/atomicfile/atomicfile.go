// Package atomicfile provides atomic file write via tmp + rename.
//
// Shell parity: cockpit-state.sh / notify-cleanup.sh の
//
//	printf '%s' "$data" > "$tmp" && mv "$tmp" "$file"
//
// パターンと等価。
package atomicfile

import (
	"fmt"
	"os"
	"path/filepath"
)

// Write writes data to path atomically:
//  1. CreateTemp in the same directory as path (so rename is on same fs)
//  2. Write data and close
//  3. Chmod to perm
//  4. Rename tmp -> path
//
// Caller is responsible for ensuring the parent directory exists.
// On any failure the tmp file is removed.
func Write(path string, data []byte, perm os.FileMode) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".atomic.*")
	if err != nil {
		return fmt.Errorf("create tmp: %w", err)
	}
	tmpName := tmp.Name()

	cleanup := func() {
		_ = os.Remove(tmpName)
	}

	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		cleanup()
		return fmt.Errorf("write tmp: %w", err)
	}
	if err := tmp.Close(); err != nil {
		cleanup()
		return fmt.Errorf("close tmp: %w", err)
	}
	if err := os.Chmod(tmpName, perm); err != nil {
		cleanup()
		return fmt.Errorf("chmod tmp: %w", err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		cleanup()
		return fmt.Errorf("rename tmp: %w", err)
	}
	return nil
}
