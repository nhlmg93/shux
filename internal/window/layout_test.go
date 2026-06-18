package window

import (
	"context"
	"testing"
	"time"

	"shux/internal/protocol"
)

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
