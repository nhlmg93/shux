package persist_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"shux/internal/persist"
	"shux/internal/protocol"
)

func TestRemoveJournal(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
	defer cancel()

	dir := t.TempDir()
	j, err := persist.OpenJournal(dir, 1, "p-1", 0)
	if err != nil {
		t.Fatal(err)
	}
	if err := j.Append([]byte("data")); err != nil {
		t.Fatal(err)
	}
	_ = j.Close()

	if err := persist.RemoveJournal(dir, "p-1"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "panes", "p-1.journal")); !os.IsNotExist(err) {
		t.Fatalf("expected journal removed, stat err=%v", err)
	}
	_ = ctx
}

func TestPruneOrphanJournals(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
	defer cancel()

	dir := t.TempDir()
	m := persist.BuildManifestForSessions("/bin/sh", "main", []persist.SessionManifest{
		persist.BuildSessionManifest(
			"main",
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
	j1, err := persist.OpenJournal(dir, 1, "p-1", 0)
	if err != nil {
		t.Fatal(err)
	}
	_ = j1.Close()
	j2, err := persist.OpenJournal(dir, 1, "p-2", 0)
	if err != nil {
		t.Fatal(err)
	}
	_ = j2.Close()

	removed, err := persist.PruneOrphanJournals(dir, m)
	if err != nil {
		t.Fatal(err)
	}
	if len(removed) != 1 {
		t.Fatalf("expected 1 removed, got %v", removed)
	}
	if _, err := os.Stat(filepath.Join(dir, "panes", "p-2.journal")); !os.IsNotExist(err) {
		t.Fatal("expected orphan p-2 removed")
	}
	if _, err := os.Stat(filepath.Join(dir, "panes", "p-1.journal")); err != nil {
		t.Fatal("expected referenced p-1 kept")
	}
	_ = ctx
}

func TestInspectState_orphans(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
	defer cancel()

	dir := t.TempDir()
	j, err := persist.OpenJournal(dir, 1, "p-9", 0)
	if err != nil {
		t.Fatal(err)
	}
	_ = j.Close()

	sum, err := persist.InspectState(dir)
	if err != nil {
		t.Fatal(err)
	}
	if sum.OrphanCount != 1 || len(sum.Journals) != 1 || sum.Journals[0].Referenced {
		t.Fatalf("unexpected summary: %+v", sum)
	}
	_ = ctx
}
