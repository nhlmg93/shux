package ui

import "shux/internal/protocol"

// LayoutSnapshot is a read-only view of one window’s tiling for rendering (Bubble Tea + Lip Gloss).
// Build it from hub events and protocol data; do not use it as the authority for layout—actors own that.
type LayoutSnapshot struct {
	SessionID  protocol.SessionID
	WindowID   protocol.WindowID
	Revision   uint64
	WindowCols int
	WindowRows int
	ActivePane protocol.PaneID
	Panes      []LayoutPane
	Title      string
	Status     string
}

// LayoutPane is one pane’s rectangle in window cell coordinates (origin top-left).
type LayoutPane struct {
	PaneID protocol.PaneID
	Col    int
	Row    int
	Cols   int
	Rows   int
}

// EmptyLayoutSnapshot returns a snapshot with no panes and zero window size.
// Callers can fill fields when applying events.
func EmptyLayoutSnapshot(sessionID protocol.SessionID, windowID protocol.WindowID) LayoutSnapshot {
	return LayoutSnapshot{SessionID: sessionID, WindowID: windowID}
}

// LayoutSnapshotFromEvent maps a layout fanout event to a read-only render snapshot.
func LayoutSnapshotFromEvent(e protocol.EventWindowLayoutChanged) LayoutSnapshot {
	panes := make([]LayoutPane, 0, len(e.Panes))
	for _, p := range e.Panes {
		panes = append(panes, LayoutPane{
			PaneID: p.PaneID,
			Col:    p.Col, Row: p.Row,
			Cols: p.Cols, Rows: p.Rows,
		})
	}
	return LayoutSnapshot{
		SessionID:  e.SessionID,
		WindowID:   e.WindowID,
		Revision:   e.Revision,
		WindowCols: e.Cols,
		WindowRows: e.Rows,
		Panes:      panes,
	}
}
