// Package safefile provides atomic file write operations that prevent
// data corruption on crash. Writes go to a temp file in the same
// directory, then rename into place — on most OSes rename is atomic.
package safefile

import (
	"fmt"
	"os"
	"path/filepath"
)

// Write writes data to path atomically. It creates a temp file in the
// same directory, writes the data, syncs to disk, then renames into
// place. If any step fails the temp file is removed and the original
// file (if any) is left untouched.
func Write(path string, data []byte, perm os.FileMode) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("safefile: mkdir: %w", err)
	}

	f, err := os.CreateTemp(dir, ".safefile-*")
	if err != nil {
		return fmt.Errorf("safefile: temp: %w", err)
	}
	tmpPath := f.Name()

	cleanup := func() { _ = os.Remove(tmpPath) }

	if _, err := f.Write(data); err != nil {
		_ = f.Close()
		cleanup()
		return fmt.Errorf("safefile: write: %w", err)
	}
	if err := f.Sync(); err != nil {
		_ = f.Close()
		cleanup()
		return fmt.Errorf("safefile: sync: %w", err)
	}
	if err := f.Close(); err != nil {
		cleanup()
		return fmt.Errorf("safefile: close: %w", err)
	}
	if err := os.Chmod(tmpPath, perm); err != nil {
		cleanup()
		return fmt.Errorf("safefile: chmod: %w", err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		cleanup()
		return fmt.Errorf("safefile: rename: %w", err)
	}
	return nil
}
