package ui

import "shux/internal/protocol"

func SessionSnapshotFromTree(data TreeSnapshotData, sessionID protocol.SessionID) SessionSnapshotMsg {
	msg := SessionSnapshotMsg{
		SessionID:     sessionID,
		WindowNames:   make(map[protocol.WindowID]string),
		PaneNames:     make(map[protocol.WindowID]map[protocol.PaneID]string),
		Layouts:       make(map[protocol.WindowID]LayoutSnapshot),
		WindowScreens: make(map[protocol.WindowID]map[protocol.PaneID]protocol.EventPaneScreenChanged),
	}
	for _, sess := range data.Sessions {
		if sess.SessionID != sessionID {
			continue
		}
		for _, w := range sess.Windows {
			msg.WindowIDs = append(msg.WindowIDs, w.WindowID)
			if name := w.Name; name != "" {
				msg.WindowNames[w.WindowID] = name
			}
			if w.Layout.WindowID.Valid() {
				msg.Layouts[w.WindowID] = w.Layout
			}
			if len(w.Panes) > 0 {
				names := make(map[protocol.PaneID]string, len(w.Panes))
				for _, p := range w.Panes {
					if p.Name != "" {
						names[p.PaneID] = p.Name
					}
				}
				if len(names) > 0 {
					msg.PaneNames[w.WindowID] = names
				}
			}
		}
		break
	}
	for wid, screens := range data.Screens {
		for _, windowID := range msg.WindowIDs {
			if windowID != wid {
				continue
			}
			if msg.WindowScreens[wid] == nil {
				msg.WindowScreens[wid] = make(map[protocol.PaneID]protocol.EventPaneScreenChanged, len(screens))
			}
			for paneID, screen := range screens {
				msg.WindowScreens[wid][paneID] = screen
			}
			break
		}
	}
	if len(msg.WindowIDs) > 0 {
		msg.WindowID = msg.WindowIDs[0]
		if layout, ok := msg.Layouts[msg.WindowIDs[0]]; ok && len(layout.Panes) > 0 {
			msg.PaneID = layout.Panes[0].PaneID
		}
	}
	return msg
}

func (m Model) ApplySessionSnapshot(msg SessionSnapshotMsg) Model {
	if !msg.SessionID.Valid() {
		return m
	}
	m.Prefix = false
	m.CommandOpen = false
	m.CommandInput = ""
	m.RenameMode = renameNone
	m.RenameInput = ""
	m.CopyMode = false
	m.CopySelection = copySelection{}
	m.PaneQuickSelectEnabled = false
	m.TreeView = TreeView{}
	m.ClosedWindows = make(map[protocol.WindowID]bool)
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
		m.PaneID = msg.PaneID
		m.ActivePaneID = msg.PaneID
		m.Layout.ActivePane = msg.PaneID
	}
	return m
}
