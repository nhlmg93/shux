package ui

import "shux/internal/protocol"

// SessionSnapshotMsg replaces the client model state for a session switch.
type SessionSnapshotMsg struct {
	SessionID     protocol.SessionID
	WindowIDs     []protocol.WindowID
	WindowID      protocol.WindowID
	PaneID        protocol.PaneID
	WindowNames   map[protocol.WindowID]string
	PaneNames     map[protocol.WindowID]map[protocol.PaneID]string
	Layouts       map[protocol.WindowID]LayoutSnapshot
	WindowScreens map[protocol.WindowID]map[protocol.PaneID]protocol.EventPaneScreenChanged
}
