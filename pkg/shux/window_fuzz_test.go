//go:build ci
// +build ci

package shux

import (
	"fmt"
	"os/exec"
	"testing"
)

// FuzzWindowStateMachine performs randomized window operations and validates
// structural/layout invariants after each step.
func FuzzWindowStateMachine(f *testing.F) {
	f.Add([]byte{})
	f.Add([]byte{0x01, 0x00, 0x02, 0x01, 0x03})
	f.Add([]byte{0x01, 0x01, 0x01, 0x01, 0x01, 0x01, 0x01})
	f.Add([]byte{0x00, 0x05, 0x02, 0x06, 0x03, 0x04, 0x01})

	f.Fuzz(func(t *testing.T, ops []byte) {
		oldStartPanePTY := startPanePTY
		startPanePTY = func(cmd *exec.Cmd, rows, cols int) (Pty, error) {
			return newFakePTY(), nil
		}
		defer func() { startPanePTY = oldStartPanePTY }()

		w := NewWindow(1)
		w.logger = NoOpLogger{}
		w.ref = &WindowRef{loopRef: newLoopRef(32)}
		defer shutdownWindowPanes(w)

		w.createPane(CreatePane{Rows: 24, Cols: 80, Shell: "/bin/sh"})
		assertWindowStateConsistent(t, w)

		for i, op := range ops {
			if i >= 40 {
				break
			}

			switch op % 7 {
			case 0:
				dir := SplitH
				if op&0x10 != 0 {
					dir = SplitV
				}
				w.splitPane(dir)
			case 1:
				if len(w.paneOrder) > 0 {
					idx := int(op) % len(w.paneOrder)
					paneID := w.paneOrder[idx]
					shutdownPane(w.panes[paneID])
					w.handlePaneExited(paneID)
				}
			case 2:
				rows := 5 + int(op%35)
				cols := 10 + int((op>>1)%90)
				w.resizeAllPanes(rows, cols)
			case 3:
				w.navigatePane(PaneNavDir(op % 4))
			case 4:
				if len(w.paneOrder) > 0 {
					w.switchToPane(int(op) % len(w.paneOrder))
				}
			case 5:
				w.resizePane(PaneNavDir(op%4), 1+int(op%4))
			case 6:
				assertSnapshotRestorePreservesPaneSet(t, w)
			}

			assertWindowStateConsistent(t, w)
		}

		assertSnapshotRestorePreservesPaneSet(t, w)
	})
}

func shutdownWindowPanes(w *Window) {
	if w == nil {
		return
	}
	for _, pane := range w.panes {
		shutdownPane(pane)
	}
}

func shutdownPane(pane *PaneRef) {
	if pane != nil {
		pane.Shutdown()
	}
}

func assertWindowStateConsistent(t *testing.T, w *Window) {
	t.Helper()

	seen := make(map[uint32]struct{}, len(w.paneOrder))
	for _, paneID := range w.paneOrder {
		if _, ok := seen[paneID]; ok {
			t.Fatalf("duplicate pane ID in order: %d", paneID)
		}
		seen[paneID] = struct{}{}
		if _, ok := w.panes[paneID]; !ok {
			t.Fatalf("paneOrder references missing pane %d", paneID)
		}
	}

	if len(w.paneOrder) > 0 {
		if w.active == 0 {
			t.Fatalf("active pane is zero with %d panes", len(w.paneOrder))
		}
		if _, ok := w.panes[w.active]; !ok {
			t.Fatalf("active pane %d missing from panes map", w.active)
		}
	} else if w.active != 0 {
		t.Fatalf("active pane %d set with no panes", w.active)
	}

	assertLayoutBoundsAndNoOverlap(t, w)
}

func assertLayoutBoundsAndNoOverlap(t *testing.T, w *Window) {
	t.Helper()

	if len(w.layout) == 0 {
		return
	}
	if w.rows <= 0 || w.cols <= 0 {
		t.Fatalf("non-empty layout with invalid window size %dx%d", w.rows, w.cols)
	}

	occupied := make(map[string]uint32, w.rows*w.cols)
	for _, pl := range w.layout {
		if _, ok := w.panes[pl.paneID]; !ok {
			t.Fatalf("layout references missing pane %d", pl.paneID)
		}
		if pl.rows <= 0 || pl.cols <= 0 {
			t.Fatalf("pane %d has non-positive size %dx%d", pl.paneID, pl.rows, pl.cols)
		}
		if pl.row < 0 || pl.col < 0 || pl.row+pl.rows > w.rows || pl.col+pl.cols > w.cols {
			t.Fatalf("pane %d out of bounds: rect=(%d,%d %dx%d) window=%dx%d", pl.paneID, pl.row, pl.col, pl.rows, pl.cols, w.rows, w.cols)
		}
		for r := pl.row; r < pl.row+pl.rows; r++ {
			for c := pl.col; c < pl.col+pl.cols; c++ {
				key := fmt.Sprintf("%d:%d", r, c)
				if other, ok := occupied[key]; ok {
					t.Fatalf("pane overlap at %s between panes %d and %d", key, other, pl.paneID)
				}
				occupied[key] = pl.paneID
			}
		}
	}
}

func assertSnapshotRestorePreservesPaneSet(t *testing.T, w *Window) {
	t.Helper()

	snapshot := w.gatherSnapshotData()
	original := make(map[uint32]struct{}, len(snapshot.PaneOrder))
	for _, paneID := range snapshot.PaneOrder {
		original[paneID] = struct{}{}
	}

	restored := NewWindow(2)
	restored.logger = NoOpLogger{}
	restored.ref = &WindowRef{loopRef: newLoopRef(32)}
	defer shutdownWindowPanes(restored)

	for _, pane := range snapshot.Panes {
		restored.createPane(CreatePane{
			ID:    pane.ID,
			Rows:  pane.Rows,
			Cols:  pane.Cols,
			Shell: pane.Shell,
			CWD:   pane.CWD,
		})
	}
	if snapshot.Layout != nil {
		restored.restoreWindowLayout(snapshot.Layout, snapshot.ActivePane)
	}
	assertWindowStateConsistent(t, restored)

	restoredSnapshot := restored.gatherSnapshotData()
	if len(restoredSnapshot.PaneOrder) != len(original) {
		t.Fatalf("restored pane count = %d, want %d", len(restoredSnapshot.PaneOrder), len(original))
	}
	for _, paneID := range restoredSnapshot.PaneOrder {
		if _, ok := original[paneID]; !ok {
			t.Fatalf("restored unexpected pane ID %d", paneID)
		}
	}
}
