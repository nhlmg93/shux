package shux

import (
	"context"
	"sync"

	"shux/internal/protocol"
)

type (
	windowLayoutSnapshots map[protocol.WindowID]protocol.EventWindowLayoutChanged
	paneScreenSnapshots   map[protocol.PaneID]protocol.EventPaneScreenChanged

	layoutsBySession map[protocol.SessionID]windowLayoutSnapshots
	screensByWindow  map[protocol.WindowID]paneScreenSnapshots
	screensBySession map[protocol.SessionID]screensByWindow
	windowsBySession map[protocol.SessionID][]protocol.WindowID
	namesBySession   map[protocol.SessionID]string
)

// stateCache subscribes to hub events and holds the latest layout and screen
// snapshots so newly attaching clients can repaint without waiting for fresh
// activity.
type stateCache struct {
	mu      sync.Mutex
	layouts layoutsBySession
	screens screensBySession
	windows windowsBySession
	names   namesBySession
}

func newStateCache() *stateCache {
	return &stateCache{
		layouts: make(layoutsBySession),
		screens: make(screensBySession),
		windows: make(windowsBySession),
		names:   make(namesBySession),
	}
}

func (c *stateCache) DeliverEvent(_ context.Context, e protocol.Event) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	switch event := e.(type) {
	case protocol.EventSessionCreated:
		c.names[event.SessionID] = event.Name
	case protocol.EventSessionWindowsChanged:
		c.windows[event.SessionID] = append([]protocol.WindowID(nil), event.Windows...)
	case protocol.EventWindowClosed:
		c.removeWindow(event.SessionID, event.WindowID)
	case protocol.EventWindowLayoutChanged:
		windows := c.layouts[event.SessionID]
		if windows == nil {
			windows = make(windowLayoutSnapshots)
			c.layouts[event.SessionID] = windows
		}
		windows[event.WindowID] = event
	case protocol.EventPaneScreenChanged:
		windows := c.screens[event.SessionID]
		if windows == nil {
			windows = make(screensByWindow)
			c.screens[event.SessionID] = windows
		}
		panes := windows[event.WindowID]
		if panes == nil {
			panes = make(paneScreenSnapshots)
			windows[event.WindowID] = panes
		}
		panes[event.PaneID] = event
	}
	return nil
}

func (c *stateCache) removeWindow(sessionID protocol.SessionID, windowID protocol.WindowID) {
	windows := c.windows[sessionID]
	for i, wid := range windows {
		if wid == windowID {
			c.windows[sessionID] = append(windows[:i], windows[i+1:]...)
			break
		}
	}
	if layouts := c.layouts[sessionID]; layouts != nil {
		delete(layouts, windowID)
	}
	if screens := c.screens[sessionID]; screens != nil {
		delete(screens, windowID)
	}
}

func (c *stateCache) WindowIDs(sessionID protocol.SessionID) []protocol.WindowID {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]protocol.WindowID(nil), c.windows[sessionID]...)
}

func (c *stateCache) LayoutSnapshot(sessionID protocol.SessionID, windowID protocol.WindowID) (protocol.EventWindowLayoutChanged, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	windows := c.layouts[sessionID]
	if windows == nil {
		return protocol.EventWindowLayoutChanged{}, false
	}
	layout, ok := windows[windowID]
	return layout, ok
}

func (c *stateCache) ScreenSnapshots(sessionID protocol.SessionID, windowID protocol.WindowID) []protocol.EventPaneScreenChanged {
	c.mu.Lock()
	defer c.mu.Unlock()
	windows := c.screens[sessionID]
	if windows == nil {
		return nil
	}
	panes := windows[windowID]
	if panes == nil {
		return nil
	}
	snapshots := make([]protocol.EventPaneScreenChanged, 0, len(panes))
	for _, screen := range panes {
		snapshots = append(snapshots, screen)
	}
	return snapshots
}

func (c *stateCache) SessionName(sessionID protocol.SessionID) (string, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	name, ok := c.names[sessionID]
	return name, ok
}
