// Package appdir provides the canonical application configuration directory.
package appdir

import (
	"os"
	"path/filepath"
)

// ConfigDir returns the path to the application configuration directory
// (%APPDATA%/cursorbridge on Windows, ~/.config/cursorbridge elsewhere).
// The directory is NOT created automatically — callers should use os.MkdirAll
// if they need it to exist.
func ConfigDir() (string, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "cursorbridge"), nil
}
