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
	layouts := map[string]persist.LayoutSnapshot{
		"w-1": {
			WindowID: "w-1",
			Cols:     80,
			Rows:     24,
			Panes: []persist.LayoutPaneSnapshot{
				{PaneID: "p-1", Col: 0, Row: 0, Cols: 40, Rows: 24},
				{PaneID: "p-2", Col: 40, Row: 0, Cols: 40, Rows: 24},
			},
		},
	}
	m := persist.BuildManifestForSessions("/bin/sh", "s-1", []persist.SessionManifest{
		persist.BuildSessionManifest(
			"s-1",
			dir,
			[]protocol.WindowID{"w-1"},
			layouts,
			map[protocol.WindowID]string{"w-1": "editor"},
			map[protocol.WindowID]map[protocol.PaneID]string{
				"w-1": {"p-1": "logs"},
			},
		),
	})
	if err := persist.SaveManifest(dir, m); err != nil {
		t.Fatal(err)
	}
	got, ok, err := persist.LoadManifest(dir)
	if err != nil || !ok {
		t.Fatalf("load: ok=%v err=%v", ok, err)
	}
	if got.DefaultSessionName != "s-1" || len(got.Sessions) != 1 || got.Sessions[0].Name != "s-1" || len(got.Sessions[0].WindowIDs) != 1 || len(got.Sessions[0].Layouts["w-1"].Panes) != 2 {
		t.Fatalf("unexpected manifest: %+v", got)
	}
	wantPath := persist.JournalPath(dir, 1, "p-1")
	key := persist.JournalMapKey(1, "p-1")
	if got.Sessions[0].PaneJournals[key] != wantPath {
		t.Fatalf("journal map: %v", got.Sessions[0].PaneJournals)
	}
	if got.PaneJournals[key] != wantPath {
		t.Fatalf("legacy journal map: %v", got.PaneJournals)
	}
	if got.Sessions[0].WindowNames["w-1"] != "editor" {
		t.Fatalf("window name = %q", got.Sessions[0].WindowNames["w-1"])
	}
	if got.Sessions[0].PaneNames[persist.PaneNameMapKey("w-1", "p-1")] != "logs" {
		t.Fatalf("pane name = %q", got.Sessions[0].PaneNames[persist.PaneNameMapKey("w-1", "p-1")])
	}
}

func TestJournalPath_stablePerPaneID(t *testing.T) {
	dir := t.TempDir()
	j, err := persist.OpenJournal(dir, 1, "p-1", 0)
	if err != nil {
		t.Fatal(err)
	}
	if err := j.Append([]byte("hello")); err != nil {
		t.Fatal(err)
	}
	if err := j.Close(); err != nil {
		t.Fatal(err)
	}
	j2, err := persist.OpenJournal(dir, 2, "p-1", 0)
	if err != nil {
		t.Fatal(err)
	}
	if err := j2.Close(); err != nil {
		t.Fatal(err)
	}
	if j.Path() != j2.Path() {
		t.Fatal("expected stable journal path for moved pane")
	}
	data, err := persist.ReadJournal(filepath.Join(dir, "panes", "p-1.journal"))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "hello" {
		t.Fatalf("journal bytes: %q", data)
	}
}

func TestJournal_enforcesMaxBytes(t *testing.T) {
	dir := t.TempDir()
	j, err := persist.OpenJournal(dir, 1, "p-1", 8)
	if err != nil {
		t.Fatal(err)
	}
	if err := j.Append([]byte("123456789")); err != nil {
		t.Fatal(err)
	}
	if err := j.Close(); err != nil {
		t.Fatal(err)
	}
	data, err := persist.ReadJournal(j.Path())
	if err != nil {
		t.Fatal(err)
	}
	if len(data) != 8 || string(data) != "23456789" {
		t.Fatalf("expected tail 8 bytes, got %q", data)
	}
}

func TestBuildManifest_ordinalWithGapWindowID(t *testing.T) {
	dir := t.TempDir()
	m := persist.BuildManifestForSessions("/bin/sh", "s-1", []persist.SessionManifest{
		persist.BuildSessionManifest(
			"s-1",
			dir,
			[]protocol.WindowID{"w-2"},
			map[string]persist.LayoutSnapshot{
				"w-2": {
					WindowID: "w-2",
					Cols:     80,
					Rows:     24,
					Panes:    []persist.LayoutPaneSnapshot{{PaneID: "p-1", Col: 0, Row: 0, Cols: 80, Rows: 24}},
				},
			},
			nil,
			nil,
		),
	})
	want := persist.JournalPath(dir, 1, "p-1")
	if m.Sessions[0].PaneJournals[persist.JournalMapKey(1, "p-1")] != want {
		t.Fatalf("expected ordinal 1 path %q, got map %v", want, m.Sessions[0].PaneJournals)
	}
	if filepath.Base(want) != "p-1.journal" {
		t.Fatalf("unexpected path: %s", want)
	}
}

func TestClearResurrectionState(t *testing.T) {
	dir := t.TempDir()
	m := persist.BuildManifestForSessions("/bin/sh", "s-1", []persist.SessionManifest{
		persist.BuildSessionManifest(
			"s-1",
			dir,
			[]protocol.WindowID{"w-1"},
			map[string]persist.LayoutSnapshot{
				"w-1": {WindowID: "w-1", Cols: 80, Rows: 24, Panes: []persist.LayoutPaneSnapshot{{PaneID: "p-1"}}},
			},
			nil,
			nil,
		),
	})
	if err := persist.SaveManifest(dir, m); err != nil {
		t.Fatal(err)
	}
	j, err := persist.OpenJournal(dir, 1, "p-1", 0)
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

func TestLayoutFromEvent_persistsZoomState(t *testing.T) {
	layout := persist.LayoutFromEvent(protocol.EventWindowLayoutChanged{
		SessionID:    "s-1",
		WindowID:     "w-1",
		Revision:     9,
		Cols:         120,
		Rows:         40,
		ZoomedPaneID: "p-2",
		Panes: []protocol.EventLayoutPane{
			{PaneID: "p-2", Col: 0, Row: 0, Cols: 120, Rows: 40},
		},
		SavedPanes: []protocol.EventLayoutPane{
			{PaneID: "p-1", Col: 0, Row: 0, Cols: 60, Rows: 40},
			{PaneID: "p-2", Col: 60, Row: 0, Cols: 60, Rows: 40},
		},
	})
	if layout.ZoomedPaneID != "p-2" {
		t.Fatalf("zoomed pane = %q, want p-2", layout.ZoomedPaneID)
	}
	if len(layout.Panes) != 1 || len(layout.SavedPanes) != 2 {
		t.Fatalf("unexpected panes in layout snapshot: %+v", layout)
	}
}
