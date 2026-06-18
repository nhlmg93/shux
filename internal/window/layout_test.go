package window

import (
	"context"
	"fmt"
	"testing"
	"time"

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

func TestLayoutSwapPaneByDirectionAllDirections(t *testing.T) {
	layout := newFourPaneLayout(t)

	neighbor, err := layout.SwapPaneByDirection("p-1", protocol.PaneDirectionRight)
	if err != nil {
		t.Fatalf("SwapPaneByDirection(right) error: %v", err)
	}
	if neighbor != "p-2" {
		t.Fatalf("neighbor(right) = %q, want %q", neighbor, "p-2")
	}
	assertRect(t, layout, "p-1", Rect{Col: 40, Row: 0, Cols: 40, Rows: 12})

	neighbor, err = layout.SwapPaneByDirection("p-1", protocol.PaneDirectionDown)
	if err != nil {
		t.Fatalf("SwapPaneByDirection(down) error: %v", err)
	}
	if neighbor != "p-4" {
		t.Fatalf("neighbor(down) = %q, want %q", neighbor, "p-4")
	}
	assertRect(t, layout, "p-1", Rect{Col: 40, Row: 12, Cols: 40, Rows: 12})

	neighbor, err = layout.SwapPaneByDirection("p-1", protocol.PaneDirectionLeft)
	if err != nil {
		t.Fatalf("SwapPaneByDirection(left) error: %v", err)
	}
	if neighbor != "p-3" {
		t.Fatalf("neighbor(left) = %q, want %q", neighbor, "p-3")
	}
	assertRect(t, layout, "p-1", Rect{Col: 0, Row: 12, Cols: 40, Rows: 12})

	neighbor, err = layout.SwapPaneByDirection("p-1", protocol.PaneDirectionUp)
	if err != nil {
		t.Fatalf("SwapPaneByDirection(up) error: %v", err)
	}
	if neighbor != "p-2" {
		t.Fatalf("neighbor(up) = %q, want %q", neighbor, "p-2")
	}
	assertRect(t, layout, "p-1", Rect{Col: 0, Row: 0, Cols: 40, Rows: 12})
	assertLayoutInvariants(t, layout)
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

func testContext(t *testing.T) context.Context {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
	t.Cleanup(cancel)
	return ctx
}

func TestLayoutResizePaneDeltaVerticalSplit(t *testing.T) {
	_ = testContext(t)
	l := NewLayout(80, 24)
	if err := l.SetSinglePane("p-1"); err != nil {
		t.Fatal(err)
	}
	if err := l.SplitPane("p-1", protocol.SplitVertical, "p-2"); err != nil {
		t.Fatal(err)
	}
	if err := l.ResizePaneDelta("p-1", protocol.PaneResizeEdgeRight, 5); err != nil {
		t.Fatal(err)
	}
	left, ok := l.Rect("p-1")
	if !ok {
		t.Fatal("missing p-1 rect")
	}
	right, ok := l.Rect("p-2")
	if !ok {
		t.Fatal("missing p-2 rect")
	}
	if left.Cols != 45 || right.Cols != 35 {
		t.Fatalf("pane widths = (%d,%d), want (45,35)", left.Cols, right.Cols)
	}
}

func TestLayoutResizePaneDeltaRejectsMinSizeViolation(t *testing.T) {
	_ = testContext(t)
	l := NewLayout(80, 24)
	if err := l.SetSinglePane("p-1"); err != nil {
		t.Fatal(err)
	}
	if err := l.SplitPane("p-1", protocol.SplitVertical, "p-2"); err != nil {
		t.Fatal(err)
	}
	if err := l.ResizePaneDelta("p-1", protocol.PaneResizeEdgeRight, 80); err == nil {
		t.Fatal("expected min size rejection")
	}
}

func TestLayoutResizePaneDeltaUsesNearestMatchingAncestor(t *testing.T) {
	_ = testContext(t)
	l := NewLayout(80, 24)
	if err := l.SetSinglePane("p-1"); err != nil {
		t.Fatal(err)
	}
	if err := l.SplitPane("p-1", protocol.SplitVertical, "p-2"); err != nil {
		t.Fatal(err)
	}
	if err := l.SplitPane("p-1", protocol.SplitHorizontal, "p-3"); err != nil {
		t.Fatal(err)
	}
	if err := l.ResizePaneDelta("p-3", protocol.PaneResizeEdgeUp, 4); err != nil {
		t.Fatal(err)
	}
	top, ok := l.Rect("p-1")
	if !ok {
		t.Fatal("missing p-1 rect")
	}
	bottom, ok := l.Rect("p-3")
	if !ok {
		t.Fatal("missing p-3 rect")
	}
	if top.Rows != 8 || bottom.Rows != 16 {
		t.Fatalf("pane heights = (%d,%d), want (8,16)", top.Rows, bottom.Rows)
	}
}
