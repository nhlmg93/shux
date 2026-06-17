package shux

import (
	"context"
	"fmt"

	"shux/internal/actor"
	"shux/internal/persist"
	"shux/internal/protocol"
)

func (a *Shux) checkpoint() {
	if !a.Config.Resurrection || a.Config.StateDir == "" || a.cache == nil {
		return
	}
	if a.getState() != stateReady {
		return
	}
	sessionID := a.DefaultSessionID
	windows := a.cache.WindowIDs(sessionID)
	layouts := make(map[string]persist.LayoutSnapshot, len(windows))
	for _, wid := range windows {
		if lay, ok := a.cache.LayoutSnapshot(sessionID, wid); ok {
			layouts[string(wid)] = persist.LayoutFromEvent(lay)
		}
	}
	m := persist.BuildManifest(sessionID, a.Config.ShellPath, a.Config.StateDir, windows, layouts)
	if err := persist.SaveManifest(a.Config.StateDir, m); err != nil {
		a.Logger.Printf("shux: checkpoint failed: %v", err)
		return
	}
	a.Logger.Info("shux: resurrection checkpoint saved")
}

func (a *Shux) bootstrapFresh(ctx context.Context) error {
	if a.Config.StateDir != "" {
		_ = persist.ClearResurrectionState(a.Config.StateDir)
	}

	events := make(protocol.EventChanAdapter, 16)
	if err := a.hub.Send(ctx, protocol.EventRegisterSubscriber{ClientID: bootstrapClientID, Sink: events}); err != nil {
		return err
	}
	defer a.hub.Send(ctx, protocol.EventUnregisterSubscriber{ClientID: bootstrapClientID})

	session, err := bootstrapStep[protocol.EventSessionCreated](ctx, a.supervisor, events, protocol.CommandCreateSession{})
	if err != nil {
		return err
	}
	window, err := bootstrapStep[protocol.EventWindowCreated](ctx, a.supervisor, events, protocol.CommandCreateWindow{SessionID: session.SessionID})
	if err != nil {
		return err
	}
	pane, err := bootstrapStep[protocol.EventPaneCreated](ctx, a.supervisor, events, protocol.CommandCreatePane{SessionID: session.SessionID, WindowID: window.WindowID})
	if err != nil {
		return err
	}

	a.DefaultSessionID = session.SessionID
	a.DefaultWindowID = window.WindowID
	a.DefaultPaneID = pane.PaneID
	a.setState(stateReady)
	a.Logger.Info("shux: bootstrap default session ready")
	a.emitDaemonStarted(ctx)
	return nil
}

func (a *Shux) restoreFromManifest(ctx context.Context, m persist.Manifest) error {
	events := make(protocol.EventChanAdapter, 32)
	if err := a.hub.Send(ctx, protocol.EventRegisterSubscriber{ClientID: bootstrapClientID, Sink: events}); err != nil {
		return err
	}
	defer a.hub.Send(ctx, protocol.EventUnregisterSubscriber{ClientID: bootstrapClientID})

	session, err := bootstrapStep[protocol.EventSessionCreated](ctx, a.supervisor, events, protocol.CommandCreateSession{})
	if err != nil {
		return fmt.Errorf("restore session: %w", err)
	}

	var firstWindow protocol.WindowID
	var firstPane protocol.PaneID
	for i, wid := range m.WindowIDs {
		layout, ok := m.Layouts[string(wid)]
		if !ok || len(layout.Panes) == 0 {
			return fmt.Errorf("restore window %s: missing layout", wid)
		}
		windowID, paneID, err := restoreWindow(ctx, a.supervisor, events, session.SessionID, layout)
		if err != nil {
			return fmt.Errorf("restore window %s: %w", wid, err)
		}
		if i == 0 {
			firstWindow = windowID
			firstPane = paneID
		}
	}

	a.DefaultSessionID = session.SessionID
	a.DefaultWindowID = firstWindow
	a.DefaultPaneID = firstPane
	a.setState(stateReady)
	a.Logger.Info("shux: restored session from manifest")
	a.emitDaemonStarted(ctx)
	return nil
}

func (a *Shux) emitDaemonStarted(ctx context.Context) {
	if a.Autocmds != nil {
		a.Autocmds.Emit(ctx, EventDaemonStarted, map[string]any{
			"session_id": string(a.DefaultSessionID),
		})
	}
}

func restoreWindow(ctx context.Context, super actor.Ref[protocol.Command], events <-chan protocol.Event, sessionID protocol.SessionID, layout persist.LayoutSnapshot) (protocol.WindowID, protocol.PaneID, error) {
	cols, rows := uint16(layout.Cols), uint16(layout.Rows)
	if cols == 0 || rows == 0 {
		cols, rows = 80, 24
	}
	window, err := bootstrapStep[protocol.EventWindowCreated](ctx, super, events, protocol.CommandCreateWindow{
		SessionID: sessionID,
		Cols:      cols,
		Rows:      rows,
		AutoPane:  true,
	})
	if err != nil {
		return "", "", err
	}
	if _, err := bootstrapWait[protocol.EventPaneCreated](ctx, events); err != nil {
		return "", "", err
	}
	current, err := bootstrapWaitLayout(ctx, events, sessionID, window.WindowID, 1)
	if err != nil {
		return "", "", err
	}

	firstPane := protocol.PaneID(layout.Panes[0].PaneID)
	for len(current.Panes) < len(layout.Panes) {
		next := layout.Panes[len(current.Panes)]
		parent, dir, ok := findSplitTarget(current, next)
		if !ok {
			return "", "", fmt.Errorf("cannot infer split for pane %s", next.PaneID)
		}
		if err := super.Send(ctx, protocol.CommandPaneSplit{
			SessionID:    sessionID,
			WindowID:     window.WindowID,
			TargetPaneID: parent,
			Direction:    dir,
		}); err != nil {
			return "", "", err
		}
		if _, err := bootstrapWait[protocol.EventPaneSplitCompleted](ctx, events); err != nil {
			return "", "", err
		}
		if _, err := bootstrapWait[protocol.EventPaneCreated](ctx, events); err != nil {
			return "", "", err
		}
		current, err = bootstrapWaitLayout(ctx, events, sessionID, window.WindowID, len(current.Panes)+1)
		if err != nil {
			return "", "", err
		}
	}

	return window.WindowID, firstPane, nil
}

func bootstrapWaitLayout(ctx context.Context, events <-chan protocol.Event, sessionID protocol.SessionID, windowID protocol.WindowID, minPanes int) (protocol.EventWindowLayoutChanged, error) {
	var zero protocol.EventWindowLayoutChanged
	for {
		select {
		case <-ctx.Done():
			return zero, ctx.Err()
		case event := <-events:
			layout, ok := event.(protocol.EventWindowLayoutChanged)
			if !ok {
				continue
			}
			if layout.SessionID != sessionID || layout.WindowID != windowID {
				continue
			}
			if len(layout.Panes) >= minPanes {
				return layout, nil
			}
		}
	}
}

func findSplitTarget(current protocol.EventWindowLayoutChanged, target persist.LayoutPaneSnapshot) (protocol.PaneID, protocol.SplitDirection, bool) {
	for _, p := range current.Panes {
		if target.Col == p.Col+p.Cols && target.Row == p.Row && target.Rows == p.Rows {
			return p.PaneID, protocol.SplitVertical, true
		}
		if target.Row == p.Row+p.Rows && target.Col == p.Col && target.Cols == p.Cols {
			return p.PaneID, protocol.SplitHorizontal, true
		}
	}
	return "", 0, false
}
