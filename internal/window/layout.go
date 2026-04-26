package window

import (
	"fmt"

	"shux/internal/protocol"
)

const minPaneCells = 1

// Rect is pane placement in window cell coordinates, origin top-left.
type Rect struct {
	Col, Row uint16
	Cols     uint16
	Rows     uint16
}

type layoutPhase uint8

const (
	phaseEmpty layoutPhase = iota
	phaseOne
	phaseSplit
)

// Layout is window-local tiling: window size, per-pane geometry, and active pane.
// Invariants: panes are inside the window, do not overlap, and have non-zero size.
type Layout struct {
	WindowCols, WindowRows uint16
	phase                  layoutPhase
	panes                  map[protocol.PaneID]Rect
	active                 protocol.PaneID
	// phaseSplit: paneA is left or top, paneB is right or bottom.
	paneA, paneB  protocol.PaneID
	splitVertical bool
	// Vertical: width of left pane. Horizontal: height of top pane.
	splitPrimary uint16
}

// NewLayout returns an empty layout for a window of the given size.
func NewLayout(windowCols, windowRows uint16) Layout {
	if windowCols == 0 || windowRows == 0 {
		panic(fmt.Sprintf("window: NewLayout: invalid size %dx%d", windowCols, windowRows))
	}
	return Layout{
		WindowCols: windowCols,
		WindowRows: windowRows,
		phase:      phaseEmpty,
		panes:      make(map[protocol.PaneID]Rect),
	}
}

// SetWindowSize updates the window dimensions and refits existing panes without removing them.
func (l *Layout) SetWindowSize(cols, rows uint16) {
	if cols == 0 || rows == 0 {
		panic(fmt.Sprintf("window: SetWindowSize: invalid size %dx%d", cols, rows))
	}
	oldW, oldH := l.WindowCols, l.WindowRows
	l.WindowCols, l.WindowRows = cols, rows
	switch l.phase {
	case phaseEmpty:
		return
	case phaseOne:
		for id := range l.panes {
			r := Rect{Col: 0, Row: 0, Cols: cols, Rows: rows}
			if err := l.assertRectInWindow(r); err != nil {
				panic(err)
			}
			l.panes[id] = r
		}
	case phaseSplit:
		if l.splitVertical {
			l.splitPrimary = scaleSplit(uint32(l.splitPrimary), int(oldW), int(cols))
		} else {
			l.splitPrimary = scaleSplit(uint32(l.splitPrimary), int(oldH), int(rows))
		}
		l.refitSplitLocked()
	}
}

// scaleSplit maps a divider position when the window dimension along that axis changes.
func scaleSplit(oldPos uint32, oldDim, newDim int) uint16 {
	if newDim < 2 {
		return 1
	}
	if oldDim < 1 {
		return uint16(newDim / 2)
	}
	n := int((int64(oldPos)*int64(newDim) + int64(oldDim)/2) / int64(oldDim))
	if n < minPaneCells {
		n = minPaneCells
	}
	if n >= newDim {
		n = newDim - minPaneCells
	}
	if n < minPaneCells {
		n = minPaneCells
	}
	return uint16(n)
}

func (l *Layout) refitSplitLocked() {
	w, h := l.WindowCols, l.WindowRows
	if l.splitVertical {
		w0 := l.splitPrimary
		if w0 < minPaneCells {
			w0 = minPaneCells
		}
		if w0+minPaneCells > w {
			w0 = w - minPaneCells
		}
		if w0 < minPaneCells {
			panic("window: layout: window too narrow for split")
		}
		w1 := w - w0
		l.splitPrimary = w0
		ra := Rect{Col: 0, Row: 0, Cols: w0, Rows: h}
		rb := Rect{Col: w0, Row: 0, Cols: w1, Rows: h}
		if err := l.assertRectInWindow(ra); err != nil {
			panic(err)
		}
		if err := l.assertRectInWindow(rb); err != nil {
			panic(err)
		}
		l.panes[l.paneA] = ra
		l.panes[l.paneB] = rb
	} else {
		h0 := l.splitPrimary
		if h0 < minPaneCells {
			h0 = minPaneCells
		}
		if h0+minPaneCells > h {
			h0 = h - minPaneCells
		}
		if h0 < minPaneCells {
			panic("window: layout: window too short for split")
		}
		h1 := h - h0
		l.splitPrimary = h0
		ra := Rect{Col: 0, Row: 0, Cols: w, Rows: h0}
		rb := Rect{Col: 0, Row: h0, Cols: w, Rows: h1}
		if err := l.assertRectInWindow(ra); err != nil {
			panic(err)
		}
		if err := l.assertRectInWindow(rb); err != nil {
			panic(err)
		}
		l.panes[l.paneA] = ra
		l.panes[l.paneB] = rb
	}
}

// SetSinglePane is the initial layout: one pane fills the window (replaces any prior layout).
func (l *Layout) SetSinglePane(id protocol.PaneID) {
	if !id.Valid() {
		panic("window: SetSinglePane: invalid PaneID")
	}
	l.phase = phaseOne
	l.splitVertical = false
	l.paneA, l.paneB = "", ""
	l.panes = make(map[protocol.PaneID]Rect)
	r := Rect{Col: 0, Row: 0, Cols: l.WindowCols, Rows: l.WindowRows}
	if err := l.assertRectInWindow(r); err != nil {
		panic(err)
	}
	l.panes[id] = r
	l.active = id
}

// SplitActive replaces the single-pane layout with a 2-pane split. newPane becomes active.
// Only valid in phaseOne with exactly one pane.
func (l *Layout) SplitActive(dir protocol.SplitDirection, newPane protocol.PaneID) {
	if l.phase != phaseOne {
		panic("window: SplitActive: expected single-pane layout")
	}
	if len(l.panes) != 1 {
		panic("window: SplitActive: expected exactly one pane")
	}
	if !newPane.Valid() {
		panic("window: SplitActive: invalid new PaneID")
	}
	var oldID protocol.PaneID
	for id := range l.panes {
		oldID = id
		break
	}
	w, h := l.WindowCols, l.WindowRows
	l.paneA = oldID
	l.paneB = newPane
	l.phase = phaseSplit
	l.splitVertical = dir == protocol.SplitVertical
	if l.splitVertical {
		if w < 2 {
			panic("window: SplitActive: window too narrow")
		}
		w0 := int(w) / 2
		if w0 < minPaneCells {
			w0 = minPaneCells
		}
		if w0 >= int(w) {
			w0 = int(w) - minPaneCells
		}
		l.splitPrimary = uint16(w0)
	} else {
		if h < 2 {
			panic("window: SplitActive: window too short")
		}
		h0 := int(h) / 2
		if h0 < minPaneCells {
			h0 = minPaneCells
		}
		if h0 >= int(h) {
			h0 = int(h) - minPaneCells
		}
		l.splitPrimary = uint16(h0)
	}
	l.panes = make(map[protocol.PaneID]Rect)
	l.panes[l.paneA] = Rect{}
	l.panes[l.paneB] = Rect{}
	l.refitSplitLocked()
	l.active = newPane
}

// CycleActive switches the active pane between the two split panes; no-op if not split.
func (l *Layout) CycleActive() {
	if l.phase != phaseSplit {
		return
	}
	if l.active == l.paneA {
		l.active = l.paneB
	} else {
		l.active = l.paneA
	}
}

// SetActive sets the active pane if it exists in the layout.
func (l *Layout) SetActive(id protocol.PaneID) {
	if !id.Valid() {
		panic("window: SetActive: invalid PaneID")
	}
	if _, ok := l.panes[id]; !ok {
		panic("window: SetActive: unknown pane")
	}
	l.active = id
}

// Rect returns a copy of the pane’s rectangle, or false if the pane is unknown.
func (l *Layout) Rect(id protocol.PaneID) (Rect, bool) {
	if l.panes == nil {
		return Rect{}, false
	}
	r, ok := l.panes[id]
	return r, ok
}

// ActivePane returns the active pane, or false if none.
func (l *Layout) ActivePane() (protocol.PaneID, bool) {
	if !l.active.Valid() {
		return "", false
	}
	if _, ok := l.panes[l.active]; !ok {
		return "", false
	}
	return l.active, true
}

// PaneIDs returns pane ids in stable order (for events and resize fanout).
func (l *Layout) PaneIDs() []protocol.PaneID {
	switch l.phase {
	case phaseEmpty:
		return nil
	case phaseOne:
		for id := range l.panes {
			return []protocol.PaneID{id}
		}
		return nil
	case phaseSplit:
		return []protocol.PaneID{l.paneA, l.paneB}
	default:
		return nil
	}
}

func (l *Layout) assertRectInWindow(r Rect) error {
	if r.Cols == 0 || r.Rows == 0 {
		return fmt.Errorf("window: layout: zero pane size")
	}
	endCol := int(r.Col) + int(r.Cols)
	endRow := int(r.Row) + int(r.Rows)
	if endCol > int(l.WindowCols) || endRow > int(l.WindowRows) {
		return fmt.Errorf("window: layout: pane out of bounds")
	}
	return nil
}
