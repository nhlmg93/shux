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

// SplitTreeSnapshot captures the split topology for a window.
type SplitTreeSnapshot struct {
	PaneID uint32
	Dir    SplitDir
	Ratio  float64
	First  *SplitTreeSnapshot
	Second *SplitTreeSnapshot
}

// WindowSnapshot captures the state of a window.
type WindowSnapshot struct {
	ID         uint32
	ActivePane uint32
	PaneOrder  []uint32
	Panes      []PaneSnapshot
	Layout     *SplitTreeSnapshot
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
// If logger is nil, logging is skipped.
func SaveSnapshot(path string, snapshot *SessionSnapshot, logger ShuxLogger) error {
	start := time.Now()
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return fmt.Errorf("failed to create snapshot directory: %w", err)
	}

	tmpPath := path + ".tmp"
	file, err := os.Create(tmpPath)
	if err != nil {
		return fmt.Errorf("failed to create snapshot file: %w", err)
	}

	encoder := gob.NewEncoder(file)
	if err := encoder.Encode(snapshot); err != nil {
		if closeErr := file.Close(); closeErr != nil {
			if logger != nil {
				logger.Warnf("snapshot: close temp file after encode failure path=%s err=%v", tmpPath, closeErr)
			}
		}
		if removeErr := os.Remove(tmpPath); removeErr != nil && !os.IsNotExist(removeErr) {
			if logger != nil {
				logger.Warnf("snapshot: remove temp file after encode failure path=%s err=%v", tmpPath, removeErr)
			}
		}
		return fmt.Errorf("failed to encode snapshot: %w", err)
	}

	if err := file.Close(); err != nil {
		if removeErr := os.Remove(tmpPath); removeErr != nil && !os.IsNotExist(removeErr) {
			if logger != nil {
				logger.Warnf("snapshot: remove temp file after close failure path=%s err=%v", tmpPath, removeErr)
			}
		}
		return fmt.Errorf("failed to close snapshot file: %w", err)
	}

	if err := os.Rename(tmpPath, path); err != nil {
		if removeErr := os.Remove(tmpPath); removeErr != nil && !os.IsNotExist(removeErr) {
			if logger != nil {
				logger.Warnf("snapshot: remove temp file after rename failure path=%s err=%v", tmpPath, removeErr)
			}
		}
		return fmt.Errorf("failed to rename snapshot file: %w", err)
	}

	if logger != nil {
		if info, err := os.Stat(path); err == nil {
			logger.Infof("snapshot: save session=%s path=%s bytes=%d windows=%d duration=%s", snapshot.SessionName, path, info.Size(), len(snapshot.Windows), time.Since(start))
		} else {
			logger.Infof("snapshot: save session=%s path=%s windows=%d duration=%s", snapshot.SessionName, path, len(snapshot.Windows), time.Since(start))
		}
	}
	return nil
}

// LoadSnapshot reads and decodes a session snapshot from disk.
// If logger is nil, logging is skipped.
func LoadSnapshot(path string, logger ShuxLogger) (*SessionSnapshot, error) {
	start := time.Now()
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open snapshot file: %w", err)
	}
	defer func() {
		if closeErr := file.Close(); closeErr != nil {
			if logger != nil {
				logger.Warnf("snapshot: close file path=%s err=%v", path, closeErr)
			}
		}
	}()

	snapshot, err := decodeSnapshot(file)
	if err != nil {
		return nil, err
	}

	if logger != nil {
		logger.Infof("snapshot: load session=%s path=%s windows=%d duration=%s", snapshot.SessionName, path, len(snapshot.Windows), time.Since(start))
	}
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
// If logger is nil, logging is skipped.
func DeleteSnapshot(path string, logger ShuxLogger) error {
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to delete snapshot: %w", err)
	}
	if logger != nil {
		logger.Infof("snapshot: delete path=%s", path)
	}
	return nil
}

// ValidateSnapshot performs comprehensive structural validation on a snapshot.
// Returns detailed error if snapshot is invalid or corrupted.
func ValidateSnapshot(snapshot *SessionSnapshot) error {
	if snapshot == nil {
		return fmt.Errorf("snapshot is nil")
	}

	if snapshot.Version != SnapshotVersion {
		return fmt.Errorf("version mismatch: got %d, expected %d", snapshot.Version, SnapshotVersion)
	}

	// Collect all IDs to check for uniqueness
	windowIDs := make(map[uint32]struct{}, len(snapshot.Windows))
	paneIDs := make(map[uint32]struct{})

	// Validate session-level invariants
	if len(snapshot.WindowOrder) != len(snapshot.Windows) {
		return fmt.Errorf("windowOrder length (%d) != windows count (%d)", len(snapshot.WindowOrder), len(snapshot.Windows))
	}

	// Validate active window exists in windows
	if snapshot.ActiveWindow != 0 {
		found := false
		for _, win := range snapshot.Windows {
			if win.ID == snapshot.ActiveWindow {
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("activeWindow %d not found in windows", snapshot.ActiveWindow)
		}
	}

	// Validate each window
	for _, win := range snapshot.Windows {
		// Check window ID uniqueness
		if _, exists := windowIDs[win.ID]; exists {
			return fmt.Errorf("duplicate window ID: %d", win.ID)
		}
		windowIDs[win.ID] = struct{}{}

		// Check window is in order list
		found := false
		for _, id := range snapshot.WindowOrder {
			if id == win.ID {
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("window %d not in windowOrder", win.ID)
		}

		// Validate pane order matches pane count
		if len(win.PaneOrder) != len(win.Panes) {
			return fmt.Errorf("window %d: paneOrder length (%d) != panes count (%d)", win.ID, len(win.PaneOrder), len(win.Panes))
		}

		// Validate active pane exists in panes
		if win.ActivePane != 0 {
			found := false
			for _, pane := range win.Panes {
				if pane.ID == win.ActivePane {
					found = true
					break
				}
			}
			if !found {
				return fmt.Errorf("window %d: activePane %d not found in panes", win.ID, win.ActivePane)
			}
		}

		// Validate split tree if present
		if win.Layout != nil {
			treePaneIDs := make(map[uint32]struct{})
			if err := validateSplitTree(win.Layout, treePaneIDs, win.ID); err != nil {
				return fmt.Errorf("window %d: invalid split tree: %w", win.ID, err)
			}
			// Verify all tree pane IDs exist in panes
			for paneID := range treePaneIDs {
				found := false
				for _, pane := range win.Panes {
					if pane.ID == paneID {
						found = true
						break
					}
				}
				if !found {
					return fmt.Errorf("window %d: split tree references missing pane %d", win.ID, paneID)
				}
			}
		}

		// Validate each pane
		for _, pane := range win.Panes {
			// Check pane ID uniqueness across all windows
			if _, exists := paneIDs[pane.ID]; exists {
				return fmt.Errorf("duplicate pane ID: %d", pane.ID)
			}
			paneIDs[pane.ID] = struct{}{}

			// Check pane is in order list
			found := false
			for _, id := range win.PaneOrder {
				if id == pane.ID {
					found = true
					break
				}
			}
			if !found {
				return fmt.Errorf("window %d: pane %d not in paneOrder", win.ID, pane.ID)
			}

			// Validate terminal dimensions
			if pane.Rows <= 0 || pane.Rows > MaxTermDimension {
				return fmt.Errorf("window %d: pane %d has invalid rows: %d", win.ID, pane.ID, pane.Rows)
			}
			if pane.Cols <= 0 || pane.Cols > MaxTermDimension {
				return fmt.Errorf("window %d: pane %d has invalid cols: %d", win.ID, pane.ID, pane.Cols)
			}
		}
	}

	return nil
}

// validateSplitTree validates a split tree and collects all referenced pane IDs.
func validateSplitTree(node *SplitTreeSnapshot, paneIDs map[uint32]struct{}, windowID uint32) error {
	if node == nil {
		return nil
	}

	// Leaf node
	if node.First == nil && node.Second == nil {
		if node.PaneID == 0 {
			return fmt.Errorf("leaf node has zero paneID")
		}
		paneIDs[node.PaneID] = struct{}{}
		return nil
	}

	// Internal node - must have both children
	if node.First == nil || node.Second == nil {
		return fmt.Errorf("internal node missing child (first=%v, second=%v)", node.First != nil, node.Second != nil)
	}

	// Validate split ratio
	if node.Ratio <= 0 || node.Ratio >= 1 {
		return fmt.Errorf("invalid split ratio: %f (must be in (0,1))", node.Ratio)
	}

	// Validate split direction
	if node.Dir != SplitH && node.Dir != SplitV {
		return fmt.Errorf("invalid split direction: %d", node.Dir)
	}

	// Recursively validate children
	if err := validateSplitTree(node.First, paneIDs, windowID); err != nil {
		return err
	}
	if err := validateSplitTree(node.Second, paneIDs, windowID); err != nil {
		return err
	}

	return nil
}
