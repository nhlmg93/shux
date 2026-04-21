package shux

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSaveAndLoadSnapshot(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "test.gob")

	original := &SessionSnapshot{
		Version:      SnapshotVersion,
		SessionName:  "test-session",
		ID:           42,
		Shell:        "/bin/bash",
		ActiveWindow: 2,
		WindowOrder:  []uint32{1, 2, 3},
		Windows: []WindowSnapshot{
			{
				ID:         1,
				ActivePane: 1,
				PaneOrder:  []uint32{1},
				Panes: []PaneSnapshot{
					{ID: 1, Shell: "/bin/bash", Rows: 24, Cols: 80, CWD: "/home/user", WindowTitle: "bash"},
				},
				Layout: &SplitTreeSnapshot{PaneID: 1},
			},
			{
				ID:         2,
				ActivePane: 2,
				PaneOrder:  []uint32{1, 2},
				Panes: []PaneSnapshot{
					{ID: 1, Shell: "/bin/sh", Rows: 24, Cols: 80, CWD: "/tmp", WindowTitle: "sh"},
					{ID: 2, Shell: "/bin/zsh", Rows: 24, Cols: 80, CWD: "/home/user/projects", WindowTitle: "zsh"},
				},
				Layout: &SplitTreeSnapshot{
					Dir:    SplitV,
					First:  &SplitTreeSnapshot{PaneID: 1},
					Second: &SplitTreeSnapshot{PaneID: 2},
				},
			},
		},
	}

	// Save
	if err := SaveSnapshot(path, original); err != nil {
		t.Fatalf("SaveSnapshot failed: %v", err)
	}

	// Verify file exists
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("snapshot file not found: %v", err)
	}

	// Load
	loaded, err := LoadSnapshot(path)
	if err != nil {
		t.Fatalf("LoadSnapshot failed: %v", err)
	}

	// Verify
	if loaded.Version != original.Version {
		t.Errorf("Version: got %d, want %d", loaded.Version, original.Version)
	}
	if loaded.SessionName != original.SessionName {
		t.Errorf("SessionName: got %s, want %s", loaded.SessionName, original.SessionName)
	}
	if loaded.ID != original.ID {
		t.Errorf("ID: got %d, want %d", loaded.ID, original.ID)
	}
	if loaded.Shell != original.Shell {
		t.Errorf("Shell: got %s, want %s", loaded.Shell, original.Shell)
	}
	if loaded.ActiveWindow != original.ActiveWindow {
		t.Errorf("ActiveWindow: got %d, want %d", loaded.ActiveWindow, original.ActiveWindow)
	}

	if len(loaded.WindowOrder) != len(original.WindowOrder) {
		t.Errorf("WindowOrder length: got %d, want %d", len(loaded.WindowOrder), len(original.WindowOrder))
	} else {
		for i, id := range loaded.WindowOrder {
			if id != original.WindowOrder[i] {
				t.Errorf("WindowOrder[%d]: got %d, want %d", i, id, original.WindowOrder[i])
			}
		}
	}

	if len(loaded.Windows) != len(original.Windows) {
		t.Errorf("Windows length: got %d, want %d", len(loaded.Windows), len(original.Windows))
	} else {
		for i, win := range loaded.Windows {
			origWin := original.Windows[i]
			if win.ID != origWin.ID {
				t.Errorf("Window[%d].ID: got %d, want %d", i, win.ID, origWin.ID)
			}
			if win.ActivePane != origWin.ActivePane {
				t.Errorf("Window[%d].ActivePane: got %d, want %d", i, win.ActivePane, origWin.ActivePane)
			}
			if len(win.Panes) != len(origWin.Panes) {
				t.Errorf("Window[%d].Panes length: got %d, want %d", i, len(win.Panes), len(origWin.Panes))
			} else {
				for j, pane := range win.Panes {
					origPane := origWin.Panes[j]
					if pane.ID != origPane.ID {
						t.Errorf("Window[%d].Pane[%d].ID: got %d, want %d", i, j, pane.ID, origPane.ID)
					}
					if pane.Shell != origPane.Shell {
						t.Errorf("Window[%d].Pane[%d].Shell: got %s, want %s", i, j, pane.Shell, origPane.Shell)
					}
					if pane.CWD != origPane.CWD {
						t.Errorf("Window[%d].Pane[%d].CWD: got %s, want %s", i, j, pane.CWD, origPane.CWD)
					}
				}
			}
			if !equalSplitTreeSnapshot(win.Layout, origWin.Layout) {
				t.Errorf("Window[%d].Layout: got %#v, want %#v", i, win.Layout, origWin.Layout)
			}
		}
	}
}

func equalSplitTreeSnapshot(a, b *SplitTreeSnapshot) bool {
	if a == nil || b == nil {
		return a == b
	}
	if a.PaneID != b.PaneID || a.Dir != b.Dir {
		return false
	}
	return equalSplitTreeSnapshot(a.First, b.First) && equalSplitTreeSnapshot(a.Second, b.Second)
}

func TestLoadSnapshotVersionMismatch(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "test.gob")

	// Create snapshot with wrong version
	snapshot := &SessionSnapshot{
		Version: 999, // Wrong version
		ID:      1,
	}

	if err := SaveSnapshot(path, snapshot); err != nil {
		t.Fatalf("SaveSnapshot failed: %v", err)
	}

	_, err := LoadSnapshot(path)
	if err == nil {
		t.Error("Expected error for version mismatch, got nil")
	}
}

func TestLoadSnapshotNotFound(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "nonexistent.gob")

	_, err := LoadSnapshot(path)
	if err == nil {
		t.Error("Expected error for missing file, got nil")
	}
}

func TestDeleteSnapshot(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "test.gob")

	// Create file
	snapshot := &SessionSnapshot{
		Version: SnapshotVersion,
		ID:      1,
	}
	if err := SaveSnapshot(path, snapshot); err != nil {
		t.Fatalf("SaveSnapshot failed: %v", err)
	}

	// Delete
	if err := DeleteSnapshot(path); err != nil {
		t.Fatalf("DeleteSnapshot failed: %v", err)
	}

	// Verify gone
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Error("Expected file to not exist after delete")
	}

	// Deleting non-existent should not error
	if err := DeleteSnapshot(path); err != nil {
		t.Errorf("DeleteSnapshot on non-existent file should not error: %v", err)
	}
}

func TestDataDirFunctions(t *testing.T) {
	// Test that DataDir returns expected format
	dir := DataDir()
	if dir == "" {
		t.Error("DataDir returned empty string")
	}

	// Test SessionDir
	sessionDir := SessionDir("test-session")
	if sessionDir == "" {
		t.Error("SessionDir returned empty string")
	}

	// Test SessionSnapshotPath
	snapshotPath := SessionSnapshotPath("test-session")
	if snapshotPath == "" {
		t.Error("SessionSnapshotPath returned empty string")
	}
	if !contains(snapshotPath, "snapshot.gob") {
		t.Error("SessionSnapshotPath should include 'snapshot.gob'")
	}
}

func TestEnsureSessionDir(t *testing.T) {
	tmpDir := t.TempDir()

	// Override DataDir for testing
	oldHome := os.Getenv("HOME")
	if err := os.Setenv("HOME", tmpDir); err != nil {
		t.Fatalf("set HOME: %v", err)
	}
	defer func() {
		if err := os.Setenv("HOME", oldHome); err != nil {
			t.Fatalf("restore HOME: %v", err)
		}
	}()

	name := "test-session"
	if err := EnsureSessionDir(name); err != nil {
		t.Fatalf("EnsureSessionDir failed: %v", err)
	}

	dir := SessionDir(name)
	if _, err := os.Stat(dir); err != nil {
		t.Errorf("Session directory not created: %v", err)
	}
}

func TestSessionSnapshotExists(t *testing.T) {
	tmpDir := t.TempDir()

	oldHome := os.Getenv("HOME")
	if err := os.Setenv("HOME", tmpDir); err != nil {
		t.Fatalf("set HOME: %v", err)
	}
	defer func() {
		if err := os.Setenv("HOME", oldHome); err != nil {
			t.Fatalf("restore HOME: %v", err)
		}
	}()

	name := "test-session"

	// Should return false for non-existent
	if SessionSnapshotExists(name) {
		t.Error("SessionSnapshotExists should return false for non-existent")
	}

	// Create snapshot
	if err := EnsureSessionDir(name); err != nil {
		t.Fatalf("EnsureSessionDir failed: %v", err)
	}
	snapshot := &SessionSnapshot{Version: SnapshotVersion, ID: 1}
	if err := SaveSnapshot(SessionSnapshotPath(name), snapshot); err != nil {
		t.Fatalf("SaveSnapshot failed: %v", err)
	}

	// Should return true now
	if !SessionSnapshotExists(name) {
		t.Error("SessionSnapshotExists should return true for existing")
	}
}
