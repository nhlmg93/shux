package persist

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"shux/internal/protocol"
)

// JournalEntry describes an on-disk pane journal file.
type JournalEntry struct {
	Path      string
	Size      int64
	Referenced bool
}

// StateSummary describes resurrection artifacts in a state directory.
type StateSummary struct {
	StateDir       string
	ManifestExists bool
	Manifest       Manifest
	Journals       []JournalEntry
	OrphanCount    int
}

// RemoveJournal deletes a pane journal file. Missing files are ignored.
func RemoveJournal(stateDir string, paneID protocol.PaneID) error {
	if stateDir == "" || paneID == "" {
		return nil
	}
	path := JournalPath(stateDir, 1, paneID)
	err := os.Remove(path)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

// ReferencedJournalPaths returns manifest-referenced journal paths.
func ReferencedJournalPaths(m Manifest) map[string]struct{} {
	refs := make(map[string]struct{})
	add := func(path string) {
		if path == "" {
			return
		}
		refs[filepath.Clean(path)] = struct{}{}
	}
	for _, path := range m.PaneJournals {
		add(path)
	}
	for _, session := range m.Sessions {
		for _, path := range session.PaneJournals {
			add(path)
		}
	}
	return refs
}

// PruneOrphanJournals removes journal files not referenced by m.
// Returns paths removed. Errors on individual deletes are ignored.
func PruneOrphanJournals(stateDir string, m Manifest) ([]string, error) {
	if stateDir == "" {
		return nil, nil
	}
	paneDir := filepath.Join(stateDir, "panes")
	entries, err := os.ReadDir(paneDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	refs := ReferencedJournalPaths(m)
	removed := make([]string, 0)
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".journal") {
			continue
		}
		path := filepath.Clean(filepath.Join(paneDir, e.Name()))
		if _, ok := refs[path]; ok {
			continue
		}
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			continue
		}
		removed = append(removed, path)
	}
	return removed, nil
}

// InspectState summarizes manifest and journal files on disk.
func InspectState(stateDir string) (StateSummary, error) {
	if stateDir == "" {
		return StateSummary{}, fmt.Errorf("persist: empty state dir")
	}
	sum := StateSummary{StateDir: stateDir}
	m, ok, err := LoadManifest(stateDir)
	if err != nil {
		return sum, err
	}
	sum.ManifestExists = ok
	if ok {
		sum.Manifest = m
	}
	refs := ReferencedJournalPaths(m)
	paneDir := filepath.Join(stateDir, "panes")
	entries, err := os.ReadDir(paneDir)
	if err != nil {
		if os.IsNotExist(err) {
			return sum, nil
		}
		return sum, err
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".journal") {
			continue
		}
		path := filepath.Clean(filepath.Join(paneDir, e.Name()))
		info, err := e.Info()
		if err != nil {
			continue
		}
		_, referenced := refs[path]
		if !referenced {
			sum.OrphanCount++
		}
		sum.Journals = append(sum.Journals, JournalEntry{
			Path:       path,
			Size:       info.Size(),
			Referenced: referenced,
		})
	}
	return sum, nil
}
