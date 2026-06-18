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
	Version            int               `json:"version"`
	ShellPath          string            `json:"shell_path"`
	DefaultSessionName string            `json:"default_session_name,omitempty"`
	Sessions           []SessionManifest `json:"sessions,omitempty"`

	// Legacy single-session fields (v1) retained for backward compatibility.
	SessionID    protocol.SessionID        `json:"session_id,omitempty"`
	WindowIDs    []protocol.WindowID       `json:"window_ids,omitempty"`
	Layouts      map[string]LayoutSnapshot `json:"layouts,omitempty"`
	PaneJournals map[string]string         `json:"pane_journals,omitempty"`
}

type SessionManifest struct {
	Name         string                    `json:"name"`
	WindowIDs    []protocol.WindowID       `json:"window_ids"`
	Layouts      map[string]LayoutSnapshot `json:"layouts"`
	PaneJournals map[string]string         `json:"pane_journals"`
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

// JournalPath returns the on-disk path for a pane journal. Pane IDs are stable
// across break/join moves, so journals are keyed by pane ID rather than window ordinal.
func JournalPath(stateDir string, windowOrdinal int, paneID protocol.PaneID) string {
	_ = windowOrdinal
	name := fmt.Sprintf("%s.journal", paneID)
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
	_ = os.Remove(filepath.Join(stateDir, manifestFile+".tmp"))
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
	return BuildManifestForSessions(shellPath, string(sessionID), []SessionManifest{
		BuildSessionManifest(string(sessionID), stateDir, windows, layouts),
	})
}

func BuildManifestForSessions(shellPath, defaultSessionName string, sessions []SessionManifest) Manifest {
	journals := make(map[string]string)
	if len(sessions) == 1 {
		for k, v := range sessions[0].PaneJournals {
			journals[k] = v
		}
	}
	return Manifest{
		Version:            2,
		ShellPath:          shellPath,
		DefaultSessionName: defaultSessionName,
		Sessions:           append([]SessionManifest(nil), sessions...),
		PaneJournals:       journals,
	}
}

func BuildSessionManifest(name, stateDir string, windows []protocol.WindowID, layouts map[string]LayoutSnapshot) SessionManifest {
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
	return SessionManifest{
		Name:         name,
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
	switch m.Version {
	case 1:
		if !m.SessionID.Valid() || len(m.WindowIDs) == 0 {
			return Manifest{}, false, errors.New("persist: invalid manifest")
		}
		m.DefaultSessionName = string(m.SessionID)
		m.Sessions = []SessionManifest{{
			Name:         string(m.SessionID),
			WindowIDs:    append([]protocol.WindowID(nil), m.WindowIDs...),
			Layouts:      m.Layouts,
			PaneJournals: m.PaneJournals,
		}}
	case 2:
		if len(m.Sessions) == 0 {
			return Manifest{}, false, errors.New("persist: invalid manifest")
		}
		if m.DefaultSessionName == "" {
			m.DefaultSessionName = m.Sessions[0].Name
		}
		for i, session := range m.Sessions {
			if !protocol.ValidSessionName(session.Name) || len(session.WindowIDs) == 0 {
				return Manifest{}, false, fmt.Errorf("persist: invalid manifest session[%d]", i)
			}
		}
	default:
		return Manifest{}, false, errors.New("persist: invalid manifest")
	}
	return m, true, nil
}
