package window

import (
	"fmt"
	"testing"

	"shux/internal/protocol"
)

func TestLayoutApplyPresetEvenVertical(t *testing.T) {
	layout := newFourPaneLayout(t)
	if err := layout.ApplyPreset("p-1", protocol.LayoutPresetEvenVertical); err != nil {
		t.Fatalf("ApplyPreset(even-vertical) error: %v", err)
	}
	assertRect(t, layout, "p-1", Rect{Col: 0, Row: 0, Cols: 20, Rows: 24})
	assertRect(t, layout, "p-3", Rect{Col: 20, Row: 0, Cols: 20, Rows: 24})
	assertRect(t, layout, "p-2", Rect{Col: 40, Row: 0, Cols: 20, Rows: 24})
	assertRect(t, layout, "p-4", Rect{Col: 60, Row: 0, Cols: 20, Rows: 24})
	assertLayoutInvariants(t, layout)
}

func TestLayoutApplyPresetMainHorizontalUsesActivePane(t *testing.T) {
	layout := newFourPaneLayout(t)
	if err := layout.ApplyPreset("p-2", protocol.LayoutPresetMainHorizontal); err != nil {
		t.Fatalf("ApplyPreset(main-horizontal) error: %v", err)
	}
	assertRect(t, layout, "p-2", Rect{Col: 0, Row: 0, Cols: 80, Rows: 12})
	assertRect(t, layout, "p-1", Rect{Col: 0, Row: 12, Cols: 26, Rows: 12})
	assertRect(t, layout, "p-3", Rect{Col: 26, Row: 12, Cols: 27, Rows: 12})
	assertRect(t, layout, "p-4", Rect{Col: 53, Row: 12, Cols: 27, Rows: 12})
	assertLayoutInvariants(t, layout)
}

func TestLayoutSwapPaneByDirection(t *testing.T) {
	layout := newFourPaneLayout(t)
	if err := layout.ApplyPreset("p-1", protocol.LayoutPresetEvenVertical); err != nil {
		t.Fatalf("ApplyPreset(even-vertical) error: %v", err)
	}
	neighbor, err := layout.SwapPaneByDirection("p-3", protocol.PaneDirectionLeft)
	if err != nil {
		t.Fatalf("SwapPaneByDirection error: %v", err)
	}
	if neighbor != "p-1" {
		t.Fatalf("neighbor = %q, want %q", neighbor, "p-1")
	}
	assertRect(t, layout, "p-3", Rect{Col: 0, Row: 0, Cols: 20, Rows: 24})
	assertRect(t, layout, "p-1", Rect{Col: 20, Row: 0, Cols: 20, Rows: 24})
	assertLayoutInvariants(t, layout)

	if _, err := layout.SwapPaneByDirection("p-3", protocol.PaneDirectionUp); err == nil {
		t.Fatal("expected swap up with no neighbor to fail")
	}
}

func FuzzLayoutInvariants(f *testing.F) {
	f.Add([]byte{0, 0, 3, 2, 4, 1})
	f.Add([]byte{0, 0, 0, 3, 4, 4, 2, 1, 3})
	f.Add([]byte{3, 3, 4, 0, 0, 2, 1, 3, 4, 2})
	f.Fuzz(func(t *testing.T, ops []byte) {
		layout := NewLayout(80, 24)
		if err := layout.SetSinglePane("p-1"); err != nil {
			t.Fatalf("SetSinglePane error: %v", err)
		}
		ids := []protocol.PaneID{"p-1"}
		nextID := 1
		for i, b := range ops {
			op := int(b % 5)
			switch op {
			case 0: // split
				if len(ids) == 0 {
					continue
				}
				target := ids[int(b)%len(ids)]
				dir := protocol.SplitVertical
				if b&1 == 1 {
					dir = protocol.SplitHorizontal
				}
				if layout.CanSplitPane(target, dir) != nil {
					continue
				}
				nextID++
				newPane := protocol.PaneID(fmt.Sprintf("p-%d", nextID))
				if err := layout.SplitPane(target, dir, newPane); err == nil {
					ids = append(ids, newPane)
				}
			case 1: // remove
				if len(ids) <= 1 {
					continue
				}
				idx := int(b) % len(ids)
				id := ids[idx]
				if err := layout.RemovePane(id); err == nil {
					ids = append(ids[:idx], ids[idx+1:]...)
				}
			case 2: // resize
				cols := uint16(20 + int(b)%120)
				rows := uint16(10 + int(ops[(i+1)%len(ops)])%60)
				_ = layout.SetWindowSize(cols, rows)
			case 3: // preset
				active := ids[int(b)%len(ids)]
				preset := protocol.LayoutPresetEvenHorizontal
				switch b % 3 {
				case 1:
					preset = protocol.LayoutPresetEvenVertical
				case 2:
					preset = protocol.LayoutPresetMainHorizontal
				}
				_ = layout.ApplyPreset(active, preset)
			case 4: // swap
				pane := ids[int(b)%len(ids)]
				dir := protocol.PaneDirection(int(b) % 4)
				_, _ = layout.SwapPaneByDirection(pane, dir)
			}
			assertLayoutInvariants(t, layout)
		}
	})
}

func newFourPaneLayout(t *testing.T) Layout {
	t.Helper()
	layout := NewLayout(80, 24)
	if err := layout.SetSinglePane("p-1"); err != nil {
		t.Fatalf("SetSinglePane error: %v", err)
	}
	if err := layout.SplitPane("p-1", protocol.SplitVertical, "p-2"); err != nil {
		t.Fatalf("SplitPane p-1 vertical error: %v", err)
	}
	if err := layout.SplitPane("p-1", protocol.SplitHorizontal, "p-3"); err != nil {
		t.Fatalf("SplitPane p-1 horizontal error: %v", err)
	}
	if err := layout.SplitPane("p-2", protocol.SplitHorizontal, "p-4"); err != nil {
		t.Fatalf("SplitPane p-2 horizontal error: %v", err)
	}
	return layout
}

func assertRect(t *testing.T, layout Layout, paneID protocol.PaneID, want Rect) {
	t.Helper()
	got, ok := layout.Rect(paneID)
	if !ok {
		t.Fatalf("missing rect for %s", paneID)
	}
	if got != want {
		t.Fatalf("rect %s = %+v, want %+v", paneID, got, want)
	}
}

func assertLayoutInvariants(t *testing.T, layout Layout) {
	t.Helper()
	cols := int(layout.WindowCols)
	rows := int(layout.WindowRows)
	if cols <= 0 || rows <= 0 {
		t.Fatalf("invalid window size %dx%d", cols, rows)
	}
	cells := make([]protocol.PaneID, cols*rows)
	for _, paneID := range layout.PaneIDs() {
		rect, ok := layout.Rect(paneID)
		if !ok {
			t.Fatalf("missing rect for pane %s", paneID)
		}
		if rect.Cols == 0 || rect.Rows == 0 {
			t.Fatalf("pane %s has zero size: %+v", paneID, rect)
		}
		if int(rect.Col)+int(rect.Cols) > cols || int(rect.Row)+int(rect.Rows) > rows {
			t.Fatalf("pane %s out of bounds: %+v window=%dx%d", paneID, rect, cols, rows)
		}
		for y := int(rect.Row); y < int(rect.Row)+int(rect.Rows); y++ {
			for x := int(rect.Col); x < int(rect.Col)+int(rect.Cols); x++ {
				idx := y*cols + x
				if cells[idx].Valid() {
					t.Fatalf("overlap at (%d,%d): %s and %s", x, y, cells[idx], paneID)
				}
				cells[idx] = paneID
			}
		}
	}
	for idx, paneID := range cells {
		if !paneID.Valid() {
			x := idx % cols
			y := idx / cols
			t.Fatalf("uncovered cell at (%d,%d)", x, y)
		}
	}
}
