package ui

import "shux/internal/protocol"

func (m Model) applySessionSnapshot(msg SessionSnapshotMsg) Model {
	if !msg.SessionID.Valid() {
		return m
	}
	m.SessionID = msg.SessionID
	m.WindowIDs = append([]protocol.WindowID(nil), msg.WindowIDs...)
	m.WindowNames = msg.WindowNames
	if m.WindowNames == nil {
		m.WindowNames = make(map[protocol.WindowID]string)
	}
	m.PaneNames = msg.PaneNames
	if m.PaneNames == nil {
		m.PaneNames = make(map[protocol.WindowID]map[protocol.PaneID]string)
	}
	m.Layouts = msg.Layouts
	if m.Layouts == nil {
		m.Layouts = make(map[protocol.WindowID]LayoutSnapshot)
	}
	m.WindowScreens = msg.WindowScreens
	if m.WindowScreens == nil {
		m.WindowScreens = make(map[protocol.WindowID]map[protocol.PaneID]protocol.EventPaneScreenChanged)
	}
	if msg.WindowID.Valid() {
		m = m.switchWindow(msg.WindowID)
	}
	if msg.PaneID.Valid() {
		m.ActivePaneID = msg.PaneID
		m.Layout.ActivePane = msg.PaneID
	}
	return m
}
