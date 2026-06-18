package shux

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"shux/internal/actor"
	"shux/internal/persist"
	"shux/internal/protocol"
)

const restoreLayoutWait = 500 * time.Millisecond

func (a *Shux) checkpoint() {
	if !a.Config.Resurrection || a.Config.StateDir == "" || a.cache == nil {
		return
	}
	if a.getState() != stateReady {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	sessions, err := a.ListSessions(ctx)
	if err != nil {
		a.Logger.Printf("shux: checkpoint list sessions failed: %v", err)
		return
	}
	snapshots := make([]persist.SessionManifest, 0, len(sessions))
	for _, session := range sessions {
		windows := a.cache.WindowIDs(session.SessionID)
		if len(windows) == 0 {
			continue
		}
		layouts := make(map[string]persist.LayoutSnapshot, len(windows))
		for _, wid := range windows {
			if lay, ok := a.cache.LayoutSnapshot(session.SessionID, wid); ok {
				layouts[string(wid)] = persist.LayoutFromEvent(lay)
			}
		}
		snapshots = append(snapshots, persist.BuildSessionManifest(session.Name, a.Config.StateDir, windows, layouts))
	}
	if len(snapshots) == 0 {
		return
	}
	defaultName := a.DefaultSession
	if defaultName == "" {
		if name, ok := a.cache.SessionName(a.DefaultSessionID); ok {
			defaultName = name
		}
	}
	if defaultName == "" {
		defaultName = snapshots[0].Name
	}
	m := persist.BuildManifestForSessions(a.Config.ShellPath, defaultName, snapshots)
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

	const defaultSessionName = "main"
	session, err := bootstrapStep[protocol.EventSessionCreated](ctx, a.supervisor, events, protocol.CommandCreateSession{Name: defaultSessionName})
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
	a.DefaultSession = session.Name
	a.DefaultWindowID = window.WindowID
	a.DefaultPaneID = pane.PaneID
	a.setState(stateReady)
	a.Logger.Info("shux: bootstrap default session ready")
	if a.Autocmds != nil {
		a.Autocmds.Emit(ctx, EventDaemonStarted, map[string]any{
			"session_id":   string(a.DefaultSessionID),
			"session_name": a.DefaultSession,
		})
	}
	return nil
}

func (a *Shux) restoreFromManifest(ctx context.Context, m persist.Manifest) error {
	events := make(protocol.EventChanAdapter, 32)
	if err := a.hub.Send(ctx, protocol.EventRegisterSubscriber{ClientID: bootstrapClientID, Sink: events}); err != nil {
		return err
	}
	defer a.hub.Send(ctx, protocol.EventUnregisterSubscriber{ClientID: bootstrapClientID})

	sessionWindows := make(map[string]protocol.WindowID, len(m.Sessions))
	sessionPanes := make(map[string]protocol.PaneID, len(m.Sessions))
	defaultName := m.DefaultSessionName
	if defaultName == "" {
		defaultName = m.Sessions[0].Name
	}
	for _, saved := range m.Sessions {
		session, err := bootstrapStep[protocol.EventSessionCreated](ctx, a.supervisor, events, protocol.CommandCreateSession{Name: saved.Name})
		if err != nil {
			return fmt.Errorf("restore session %q: %w", saved.Name, err)
		}
		for _, wid := range saved.WindowIDs {
			layout, ok := saved.Layouts[string(wid)]
			if !ok || len(layout.Panes) == 0 {
				return fmt.Errorf("restore session %q window %s: missing layout", saved.Name, wid)
			}
			windowID, paneID, err := a.restoreWindow(ctx, a.supervisor, events, session.SessionID, layout)
			if err != nil {
				return fmt.Errorf("restore session %q window %s: %w", saved.Name, wid, err)
			}
			if _, ok := sessionWindows[saved.Name]; !ok {
				sessionWindows[saved.Name] = windowID
				sessionPanes[saved.Name] = paneID
			}
		}
	}
	session, err := a.ResolveSession(ctx, defaultName)
	if err != nil {
		return err
	}
	a.DefaultSessionID = session.SessionID
	a.DefaultSession = session.Name
	a.DefaultWindowID = sessionWindows[session.Name]
	a.DefaultPaneID = sessionPanes[session.Name]
	a.setState(stateReady)
	a.Logger.Info("shux: restored session from manifest")
	if a.Autocmds != nil {
		a.Autocmds.Emit(ctx, EventDaemonStarted, map[string]any{
			"session_id":   string(a.DefaultSessionID),
			"session_name": a.DefaultSession,
		})
	}
	return nil
}

func (a *Shux) restoreWindow(ctx context.Context, super actor.Ref[protocol.Command], events <-chan protocol.Event, sessionID protocol.SessionID, layout persist.LayoutSnapshot) (protocol.WindowID, protocol.PaneID, error) {
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
	if err := a.waitLayoutPanes(ctx, sessionID, window.WindowID, 1, restoreLayoutWait); err != nil {
		return "", "", err
	}

	targetPanes := layout.Panes
	if layout.ZoomedPaneID != "" && len(layout.SavedPanes) > 0 {
		targetPanes = layout.SavedPanes
	}
	targetPanes = sortedLayoutPanes(targetPanes)
	firstPane := protocol.PaneID(targetPanes[0].PaneID)
	currentCount := 1
	for currentCount < len(targetPanes) {
		layoutSnap, ok := a.cache.LayoutSnapshot(sessionID, window.WindowID)
		if !ok {
			return "", "", fmt.Errorf("restore window %s: missing layout", window.WindowID)
		}
		next := targetPanes[currentCount]
		parent, dir, ok := findSplitTarget(layoutSnap, next)
		if !ok {
			return "", "", fmt.Errorf("cannot infer split for pane %s", next.PaneID)
		}
		a.bootstrapReq++
		if err := super.Send(ctx, protocol.CommandPaneSplit{
			Meta:         protocol.CommandMeta{ClientID: bootstrapClientID, RequestID: a.bootstrapReq},
			SessionID:    sessionID,
			WindowID:     window.WindowID,
			TargetPaneID: parent,
			Direction:    dir,
		}); err != nil {
			return "", "", err
		}
		currentCount++
		if err := a.waitLayoutPanes(ctx, sessionID, window.WindowID, currentCount, restoreLayoutWait); err != nil {
			return "", "", err
		}
	}

	if layout.ZoomedPaneID != "" {
		a.bootstrapReq++
		if err := super.Send(ctx, protocol.CommandPaneZoomToggle{
			SessionID: sessionID,
			WindowID:  window.WindowID,
			PaneID:    protocol.PaneID(layout.ZoomedPaneID),
		}); err != nil {
			return "", "", err
		}
		if err := a.waitLayoutPanes(ctx, sessionID, window.WindowID, 1, restoreLayoutWait); err != nil {
			return "", "", err
		}
	}

	return window.WindowID, firstPane, nil
}

func (a *Shux) waitLayoutPanes(ctx context.Context, sessionID protocol.SessionID, windowID protocol.WindowID, minPanes int, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if layout, ok := a.cache.LayoutSnapshot(sessionID, windowID); ok && len(layout.Panes) >= minPanes {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(10 * time.Millisecond):
		}
	}
	return fmt.Errorf("timeout waiting for %d panes in window %s", minPanes, windowID)
}

func findSplitTarget(current protocol.EventWindowLayoutChanged, target persist.LayoutPaneSnapshot) (protocol.PaneID, protocol.SplitDirection, bool) {
	parent, dir, ok := findSplitTargetSnaps(persist.LayoutFromEvent(current).Panes, target)
	return protocol.PaneID(parent), dir, ok
}

func findSplitTargetSnaps(current []persist.LayoutPaneSnapshot, target persist.LayoutPaneSnapshot) (string, protocol.SplitDirection, bool) {
	for _, p := range current {
		if p.PaneID == target.PaneID {
			continue
		}
		if target.Col > p.Col && target.Col <= p.Col+p.Cols &&
			target.Row >= p.Row && target.Row+target.Rows <= p.Row+p.Rows {
			return p.PaneID, protocol.SplitVertical, true
		}
		if target.Row > p.Row && target.Row <= p.Row+p.Rows &&
			target.Col >= p.Col && target.Col+target.Cols <= p.Col+p.Cols {
			return p.PaneID, protocol.SplitHorizontal, true
		}
	}
	return "", 0, false
}

func sortedLayoutPanes(panes []persist.LayoutPaneSnapshot) []persist.LayoutPaneSnapshot {
	out := append([]persist.LayoutPaneSnapshot(nil), panes...)
	sort.Slice(out, func(i, j int) bool {
		return paneIDSeq(out[i].PaneID) < paneIDSeq(out[j].PaneID)
	})
	return out
}

func paneIDSeq(id string) int {
	id = strings.TrimPrefix(id, "p-")
	n, _ := strconv.Atoi(id)
	return n
}
