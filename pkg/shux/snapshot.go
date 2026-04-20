package shux

import (
	"encoding/gob"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"
)

const SnapshotVersion = 1

// SessionSnapshot captures the complete state of a session for persistence.
type SessionSnapshot struct {
	Version      int
	SessionName  string
	ID           uint32
	Shell        string
	ActiveWindow uint32
	WindowOrder  []uint32
	Windows      []WindowSnapshot
}

// WindowSnapshot captures the state of a window.
type WindowSnapshot struct {
	ID         uint32
	ActivePane uint32
	PaneOrder  []uint32
	Panes      []PaneSnapshot
}

// PaneSnapshot captures the state of a pane.
type PaneSnapshot struct {
	ID          uint32
	Shell       string
	Rows        int
	Cols        int
	CWD         string
	WindowTitle string
}

// SaveSnapshot atomically writes a session snapshot to disk.
func SaveSnapshot(path string, snapshot *SessionSnapshot) error {
	start := time.Now()
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0750); err != nil {
		return fmt.Errorf("failed to create snapshot directory: %w", err)
	}

	tmpPath := path + ".tmp"
	file, err := os.Create(tmpPath)
	if err != nil {
		return fmt.Errorf("failed to create snapshot file: %w", err)
	}

	encoder := gob.NewEncoder(file)
	if err := encoder.Encode(snapshot); err != nil {
		file.Close()
		os.Remove(tmpPath)
		return fmt.Errorf("failed to encode snapshot: %w", err)
	}

	if err := file.Close(); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("failed to close snapshot file: %w", err)
	}

	if err := os.Rename(tmpPath, path); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("failed to rename snapshot file: %w", err)
	}

	if info, err := os.Stat(path); err == nil {
		Infof("snapshot: save session=%s path=%s bytes=%d windows=%d duration=%s", snapshot.SessionName, path, info.Size(), len(snapshot.Windows), time.Since(start))
	} else {
		Infof("snapshot: save session=%s path=%s windows=%d duration=%s", snapshot.SessionName, path, len(snapshot.Windows), time.Since(start))
	}
	return nil
}

// LoadSnapshot reads and decodes a session snapshot from disk.
func LoadSnapshot(path string) (*SessionSnapshot, error) {
	start := time.Now()
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open snapshot file: %w", err)
	}
	defer file.Close()

	snapshot, err := decodeSnapshot(file)
	if err != nil {
		return nil, err
	}

	Infof("snapshot: load session=%s path=%s windows=%d duration=%s", snapshot.SessionName, path, len(snapshot.Windows), time.Since(start))
	return snapshot, nil
}

func decodeSnapshot(r io.Reader) (*SessionSnapshot, error) {
	var snapshot SessionSnapshot
	decoder := gob.NewDecoder(r)
	if err := decoder.Decode(&snapshot); err != nil {
		return nil, fmt.Errorf("failed to decode snapshot: %w", err)
	}

	if snapshot.Version != SnapshotVersion {
		return nil, fmt.Errorf("snapshot version mismatch: got %d, expected %d", snapshot.Version, SnapshotVersion)
	}

	return &snapshot, nil
}

// DeleteSnapshot removes a snapshot file from disk.
func DeleteSnapshot(path string) error {
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to delete snapshot: %w", err)
	}
	Infof("snapshot: delete path=%s", path)
	return nil
}
