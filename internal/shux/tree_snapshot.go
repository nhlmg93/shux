package shux

import (
	"context"

	"shux/internal/protocol"
	"shux/internal/ui"
)

func (a *Shux) uiTreeSnapshot(ctx context.Context, clientSession protocol.SessionID) (ui.TreeSnapshotData, error) {
	sessions, err := a.ListSessions(ctx)
	if err != nil {
		return ui.TreeSnapshotData{}, err
	}
	out := ui.TreeSnapshotData{
		Sessions: make([]ui.TreeSessionNode, 0, len(sessions)),
		Screens:  make(map[protocol.WindowID]map[protocol.PaneID]protocol.EventPaneScreenChanged),
	}
	for _, desc := range sessions {
		node := ui.TreeSessionNode{
			SessionID: desc.SessionID,
			Name:      desc.Name,
			Attached:  desc.SessionID == clientSession,
		}
		windowIDs := a.cache.WindowIDs(desc.SessionID)
		wNames := a.cache.WindowNames(desc.SessionID)
		for i, wid := range windowIDs {
			win := ui.TreeWindowNode{
				WindowID: wid,
				Index:    i + 1,
				Name:     wNames[wid],
			}
			if layout, ok := a.cache.LayoutSnapshot(desc.SessionID, wid); ok {
				win.Layout = ui.LayoutSnapshotFromEvent(layout)
				paneNames := a.cache.PaneNames(desc.SessionID, wid)
				for pi, p := range layout.Panes {
					win.Panes = append(win.Panes, ui.TreePaneNode{
						PaneID: p.PaneID,
						Index:  pi + 1,
						Name:   paneNames[p.PaneID],
						Col:    int(p.Col),
						Row:    int(p.Row),
						Cols:   int(p.Cols),
						Rows:   int(p.Rows),
					})
				}
			}
			for _, screen := range a.cache.ScreenSnapshots(desc.SessionID, wid) {
				if out.Screens[wid] == nil {
					out.Screens[wid] = make(map[protocol.PaneID]protocol.EventPaneScreenChanged)
				}
				out.Screens[wid][screen.PaneID] = screen
			}
			node.Windows = append(node.Windows, win)
		}
		out.Sessions = append(out.Sessions, node)
	}
	return out, nil
}
