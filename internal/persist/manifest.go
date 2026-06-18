package persist

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"shux/internal/protocol"
)

const manifestFile = "manifest.json"

// LayoutPaneSnapshot is one pane's geometry in window cell space.
type LayoutPaneSnapshot struct {
	PaneID string `json:"pane_id"`
	Col    int    `json:"col"`
	Row    int    `json:"row"`
	Cols   int    `json:"cols"`
	Rows   int    `json:"rows"`
}

// LayoutSnapshot is a window layout checkpoint for resurrection.
type LayoutSnapshot struct {
	WindowID     string               `json:"window_id"`
	Cols         int                  `json:"cols"`
	Rows         int                  `json:"rows"`
	Panes        []LayoutPaneSnapshot `json:"panes"`
	ZoomedPaneID string               `json:"zoomed_pane_id,omitempty"`
	SavedPanes   []LayoutPaneSnapshot `json:"saved_panes,omitempty"`
}

// Manifest is the on-disk resurrection checkpoint for a shux daemon.
type Manifest struct {
	Version      int                        `json:"version"`
	ShellPath    string                     `json:"shell_path"`
	SessionID    protocol.SessionID         `json:"session_id"`
	WindowIDs    []protocol.WindowID        `json:"window_ids"`
	Layouts      map[string]LayoutSnapshot  `json:"layouts"`
	PaneJournals map[string]string          `json:"pane_journals"`
}

// JournalMapKey identifies a pane journal within a manifest by session window ordinal.
func JournalMapKey(windowOrdinal int, paneID protocol.PaneID) string {
	return fmt.Sprintf("%d/%s", windowOrdinal, paneID)
}

// LayoutFromEvent converts a hub layout event into a manifest snapshot.
func LayoutFromEvent(e protocol.EventWindowLayoutChanged) LayoutSnapshot {
	panes := make([]LayoutPaneSnapshot, len(e.Panes))
	for i, p := range e.Panes {
		panes[i] = LayoutPaneSnapshot{
			PaneID: string(p.PaneID),
			Col:    p.Col,
			Row:    p.Row,
			Cols:   p.Cols,
			Rows:   p.Rows,
		}
	}
	savedPanes := make([]LayoutPaneSnapshot, len(e.SavedPanes))
	for i, p := range e.SavedPanes {
		savedPanes[i] = LayoutPaneSnapshot{
			PaneID: string(p.PaneID),
			Col:    p.Col,
			Row:    p.Row,
			Cols:   p.Cols,
			Rows:   p.Rows,
		}
	}
	return LayoutSnapshot{
		WindowID:     string(e.WindowID),
		Cols:         e.Cols,
		Rows:         e.Rows,
		Panes:        panes,
		ZoomedPaneID: string(e.ZoomedPaneID),
		SavedPanes:   savedPanes,
	}
}

// JournalPath returns the on-disk path for a pane journal.
// windowOrdinal is the 1-based index of the window in the session window list.
func JournalPath(stateDir string, windowOrdinal int, paneID protocol.PaneID) string {
	name := fmt.Sprintf("win%d_%s.journal", windowOrdinal, paneID)
	return filepath.Join(stateDir, "panes", name)
}

func OpenJournal(stateDir string, windowOrdinal int, paneID protocol.PaneID, maxBytes uint64) (*Journal, error) {
	if windowOrdinal <= 0 {
		return nil, fmt.Errorf("persist: invalid window ordinal %d", windowOrdinal)
	}
	dir := filepath.Join(stateDir, "panes")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, err
	}
	path := JournalPath(stateDir, windowOrdinal, paneID)
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return nil, err
	}
	return &Journal{path: path, f: f, maxBytes: maxBytes}, nil
}

// ClearResurrectionState removes the manifest and pane journals before a fresh bootstrap.
func ClearResurrectionState(stateDir string) error {
	if stateDir == "" {
		return nil
	}
	_ = os.Remove(filepath.Join(stateDir, manifestFile))
	_ = os.Remove(filepath.Join(stateDir, manifestFile + ".tmp"))
	paneDir := filepath.Join(stateDir, "panes")
	entries, err := os.ReadDir(paneDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		_ = os.Remove(filepath.Join(paneDir, e.Name()))
	}
	return nil
}

// BuildManifest assembles a manifest from exported session snapshots.
func BuildManifest(sessionID protocol.SessionID, shellPath, stateDir string, windows []protocol.WindowID, layouts map[string]LayoutSnapshot) Manifest {
	journals := make(map[string]string)
	for i, wid := range windows {
		ordinal := i + 1
		layout, ok := layouts[string(wid)]
		if !ok {
			continue
		}
		for _, p := range layout.Panes {
			pid := protocol.PaneID(p.PaneID)
			journals[JournalMapKey(ordinal, pid)] = JournalPath(stateDir, ordinal, pid)
		}
	}
	return Manifest{
		Version:      1,
		ShellPath:    shellPath,
		SessionID:    sessionID,
		WindowIDs:    append([]protocol.WindowID(nil), windows...),
		Layouts:      layouts,
		PaneJournals: journals,
	}
}

// SaveManifest atomically writes a resurrection checkpoint.
func SaveManifest(stateDir string, m Manifest) error {
	if stateDir == "" {
		return errors.New("persist: empty state dir")
	}
	if err := os.MkdirAll(stateDir, 0o700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	tmp := filepath.Join(stateDir, manifestFile+".tmp")
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, filepath.Join(stateDir, manifestFile))
}

// LoadManifest reads a resurrection checkpoint. ok is false when no manifest exists.
func LoadManifest(stateDir string) (Manifest, bool, error) {
	path := filepath.Join(stateDir, manifestFile)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return Manifest{}, false, nil
		}
		return Manifest{}, false, err
	}
	var m Manifest
	if err := json.Unmarshal(data, &m); err != nil {
		return Manifest{}, false, err
	}
	if m.Version != 1 || !m.SessionID.Valid() || len(m.WindowIDs) == 0 {
		return Manifest{}, false, errors.New("persist: invalid manifest")
	}
	return m, true, nil
}
