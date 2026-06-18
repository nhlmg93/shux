package shux

import (
	"testing"

	"shux/internal/persist"
)

func TestFindSplitTargetSnaps_fullWidthParent(t *testing.T) {
	current := []persist.LayoutPaneSnapshot{{PaneID: "p-1", Col: 0, Row: 0, Cols: 80, Rows: 24}}
	target := persist.LayoutPaneSnapshot{PaneID: "p-2", Col: 40, Row: 0, Cols: 40, Rows: 12}
	_, _, ok := findSplitTargetSnaps(current, target)
	if !ok {
		t.Fatal("expected vertical split match")
	}
}
