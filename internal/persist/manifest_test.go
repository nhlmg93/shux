package persist_test

import (
	"os"
	"path/filepath"
	"testing"

	"shux/internal/persist"
	"shux/internal/protocol"
)

func TestManifest_roundtrip(t *testing.T) {
	dir := t.TempDir()
	m := persist.BuildManifest(
		protocol.SessionID("s-1"),
		"/bin/sh",
		dir,
		[]protocol.WindowID{"w-1"},
		map[string]persist.LayoutSnapshot{
			"w-1": {
				WindowID: "w-1",
				Cols:     80,
				Rows:     24,
				Panes: []persist.LayoutPaneSnapshot{
					{PaneID: "p-1", Col: 0, Row: 0, Cols: 40, Rows: 24},
					{PaneID: "p-2", Col: 40, Row: 0, Cols: 40, Rows: 24},
				},
			},
		},
	)
	if err := persist.SaveManifest(dir, m); err != nil {
		t.Fatal(err)
	}
	got, ok, err := persist.LoadManifest(dir)
	if err != nil || !ok {
		t.Fatalf("load: ok=%v err=%v", ok, err)
	}
	if got.SessionID != m.SessionID || len(got.WindowIDs) != 1 || len(got.Layouts["w-1"].Panes) != 2 {
		t.Fatalf("unexpected manifest: %+v", got)
	}
	wantPath := persist.JournalPath(dir, "w-1", "p-1")
	if got.PaneJournals[persist.JournalMapKey("w-1", "p-1")] != wantPath {
		t.Fatalf("journal map: %v", got.PaneJournals)
	}
}

func TestJournalPath_perWindow(t *testing.T) {
	dir := t.TempDir()
	j, err := persist.OpenJournal(dir, "w-1", "p-1")
	if err != nil {
		t.Fatal(err)
	}
	if err := j.Append([]byte("hello")); err != nil {
		t.Fatal(err)
	}
	if err := j.Close(); err != nil {
		t.Fatal(err)
	}
	j2, err := persist.OpenJournal(dir, "w-2", "p-1")
	if err != nil {
		t.Fatal(err)
	}
	if err := j2.Close(); err != nil {
		t.Fatal(err)
	}
	if j.Path() == j2.Path() {
		t.Fatal("expected distinct journal paths for different windows")
	}
	data, err := persist.ReadJournal(filepath.Join(dir, "panes", "w-1_p-1.journal"))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "hello" {
		t.Fatalf("journal bytes: %q", data)
	}
}

func TestClearResurrectionState(t *testing.T) {
	dir := t.TempDir()
	m := persist.BuildManifest("s-1", "/bin/sh", dir, []protocol.WindowID{"w-1"}, map[string]persist.LayoutSnapshot{
		"w-1": {WindowID: "w-1", Cols: 80, Rows: 24, Panes: []persist.LayoutPaneSnapshot{{PaneID: "p-1"}}},
	})
	if err := persist.SaveManifest(dir, m); err != nil {
		t.Fatal(err)
	}
	j, err := persist.OpenJournal(dir, "w-1", "p-1")
	if err != nil {
		t.Fatal(err)
	}
	_ = j.Close()
	if err := persist.ClearResurrectionState(dir); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "manifest.json")); !os.IsNotExist(err) {
		t.Fatalf("manifest still present: %v", err)
	}
	entries, err := os.ReadDir(filepath.Join(dir, "panes"))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("expected empty panes dir, got %d entries", len(entries))
	}
}
