package persist_test

import (
	"context"
	"testing"
	"time"

	"shux/internal/persist"
)

func TestLayoutValidForCheckpoint(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
	defer cancel()

	valid := persist.LayoutSnapshot{Cols: 80, Rows: 24, Panes: []persist.LayoutPaneSnapshot{{PaneID: "p-1", Cols: 80, Rows: 24}}}
	if !persist.LayoutValidForCheckpoint(valid) {
		t.Fatal("expected 80x24 layout to be valid for checkpoint")
	}
	tiny := persist.LayoutSnapshot{Cols: 1, Rows: 1, Panes: []persist.LayoutPaneSnapshot{{PaneID: "p-1", Cols: 1, Rows: 1}}}
	if persist.LayoutValidForCheckpoint(tiny) {
		t.Fatal("expected 1x1 layout to be rejected for checkpoint")
	}
	_ = ctx
}

func TestNormalizeLayoutForRestore_upgradesTinyLayout(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
	defer cancel()

	out := persist.NormalizeLayoutForRestore(persist.LayoutSnapshot{
		WindowID: "w-1",
		Cols:     1,
		Rows:     1,
		Panes:    []persist.LayoutPaneSnapshot{{PaneID: "p-1", Col: 0, Row: 0, Cols: 1, Rows: 1}},
	})
	if out.Cols != persist.DefaultLayoutCols || out.Rows != persist.DefaultLayoutRows {
		t.Fatalf("expected default window size, got %dx%d", out.Cols, out.Rows)
	}
	if len(out.Panes) != 1 || out.Panes[0].Cols != persist.DefaultLayoutCols {
		t.Fatalf("expected single full-window pane, got %+v", out.Panes)
	}
	_ = ctx
}
