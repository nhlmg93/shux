package shux

import (
	"context"
	"sync"

	"shux/internal/protocol"
)

type (
	windowLayoutSnapshots map[protocol.WindowID]protocol.EventWindowLayoutChanged
	paneScreenSnapshots   map[protocol.PaneID]protocol.EventPaneScreenChanged
	windowNamesByID       map[protocol.WindowID]string
	paneNamesByID         map[protocol.PaneID]string

	layoutsBySession map[protocol.SessionID]windowLayoutSnapshots
	screensByWindow  map[protocol.WindowID]paneScreenSnapshots
	screensBySession map[protocol.SessionID]screensByWindow
	windowsBySession map[protocol.SessionID][]protocol.WindowID
	namesBySession   map[protocol.SessionID]string
	windowNames      map[protocol.SessionID]windowNamesByID
	paneNamesByWin   map[protocol.WindowID]paneNamesByID
	paneNames        map[protocol.SessionID]paneNamesByWin
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
	wNames  windowNames
	pNames  paneNames
}

func newStateCache() *stateCache {
	return &stateCache{
		layouts: make(layoutsBySession),
		screens: make(screensBySession),
		windows: make(windowsBySession),
		names:   make(namesBySession),
		wNames:  make(windowNames),
		pNames:  make(paneNames),
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
		if sessionScreens := c.screens[event.SessionID]; sessionScreens != nil {
			if panes := sessionScreens[event.WindowID]; panes != nil {
				alive := make(map[protocol.PaneID]struct{}, len(event.Panes))
				for _, pane := range event.Panes {
					alive[pane.PaneID] = struct{}{}
				}
				for paneID := range panes {
					if _, ok := alive[paneID]; !ok {
						delete(panes, paneID)
					}
				}
			}
		}
	case protocol.EventWindowRenamed:
		windows := c.wNames[event.SessionID]
		if windows == nil {
			windows = make(windowNamesByID)
			c.wNames[event.SessionID] = windows
		}
		windows[event.WindowID] = event.Name
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
	case protocol.EventPaneRenamed:
		windows := c.pNames[event.SessionID]
		if windows == nil {
			windows = make(paneNamesByWin)
			c.pNames[event.SessionID] = windows
		}
		panes := windows[event.WindowID]
		if panes == nil {
			panes = make(paneNamesByID)
			windows[event.WindowID] = panes
		}
		panes[event.PaneID] = event.Name
	case protocol.EventPaneClosed:
		for _, windows := range c.pNames {
			if panes := windows[event.WindowID]; panes != nil {
				delete(panes, event.PaneID)
			}
		}
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
	if names := c.wNames[sessionID]; names != nil {
		delete(names, windowID)
	}
	if screens := c.screens[sessionID]; screens != nil {
		delete(screens, windowID)
	}
	if panes := c.pNames[sessionID]; panes != nil {
		delete(panes, windowID)
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

func (c *stateCache) WindowNames(sessionID protocol.SessionID) map[protocol.WindowID]string {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make(map[protocol.WindowID]string)
	for wid, name := range c.wNames[sessionID] {
		out[wid] = name
	}
	return out
}

func (c *stateCache) PaneNames(sessionID protocol.SessionID, windowID protocol.WindowID) map[protocol.PaneID]string {
	c.mu.Lock()
	defer c.mu.Unlock()
	windows := c.pNames[sessionID]
	if windows == nil {
		return nil
	}
	panes := windows[windowID]
	if panes == nil {
		return nil
	}
	out := make(map[protocol.PaneID]string)
	for pid, name := range panes {
		out[pid] = name
	}
	return out
}
