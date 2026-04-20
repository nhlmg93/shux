package shux

import (
	"os"
	"path/filepath"
)

// DataDir returns the base data directory for shux (~/.local/share/shux/).
func DataDir() string {
	home := os.Getenv("HOME")
	if home == "" {
		home = "."
	}
	return filepath.Join(home, ".local", "share", "shux")
}

// SessionDir returns the directory path for a named session's data.
func SessionDir(name string) string {
	return filepath.Join(DataDir(), "sessions", name)
}

// SessionSnapshotPath returns the full path to a session's snapshot file.
func SessionSnapshotPath(name string) string {
	return filepath.Join(SessionDir(name), "snapshot.gob")
}

// EnsureSessionDir creates the session directory if it doesn't exist.
func EnsureSessionDir(name string) error {
	dir := SessionDir(name)
	return os.MkdirAll(dir, 0o750)
}

// SessionSnapshotExists checks if a snapshot file exists for the given session name.
func SessionSnapshotExists(name string) bool {
	path := SessionSnapshotPath(name)
	_, err := os.Stat(path)
	return err == nil
}
