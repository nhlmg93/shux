package shux

import (
	"shux/internal/protocol"
	"shux/internal/ui"
)

func (a *Shux) sessionSnapshotMsg(sessionID protocol.SessionID) ui.SessionSnapshotMsg {
	windowIDs := a.cache.WindowIDs(sessionID)
	msg := ui.SessionSnapshotMsg{
		SessionID:     sessionID,
		WindowIDs:     append([]protocol.WindowID(nil), windowIDs...),
		WindowNames:   make(map[protocol.WindowID]string),
		PaneNames:     make(map[protocol.WindowID]map[protocol.PaneID]string),
		Layouts:       make(map[protocol.WindowID]ui.LayoutSnapshot),
		WindowScreens: make(map[protocol.WindowID]map[protocol.PaneID]protocol.EventPaneScreenChanged),
	}
	for wid, name := range a.cache.WindowNames(sessionID) {
		msg.WindowNames[wid] = name
	}
	for _, windowID := range windowIDs {
		if layout, ok := a.cache.LayoutSnapshot(sessionID, windowID); ok {
			msg.Layouts[windowID] = ui.LayoutSnapshotFromEvent(layout)
		}
		if panes := a.cache.PaneNames(sessionID, windowID); panes != nil {
			msg.PaneNames[windowID] = panes
		}
		for _, screen := range a.cache.ScreenSnapshots(sessionID, windowID) {
			if msg.WindowScreens[windowID] == nil {
				msg.WindowScreens[windowID] = make(map[protocol.PaneID]protocol.EventPaneScreenChanged)
			}
			msg.WindowScreens[windowID][screen.PaneID] = screen
		}
	}
	if len(windowIDs) > 0 {
		msg.WindowID = windowIDs[0]
		if layout, ok := msg.Layouts[windowIDs[0]]; ok && len(layout.Panes) > 0 {
			msg.PaneID = layout.Panes[0].PaneID
		}
	}
	return msg
}
