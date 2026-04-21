package shux

import (
	"fmt"
	"sync"
)

// Registry is a stable storage for runtimes and controllers.
// It survives controller restarts and provides stable identity.
type Registry struct {
	mu       sync.RWMutex
	runtimes map[uint32]*PaneRuntime      // pane_id -> runtime
	panes    map[uint32]*PaneController   // pane_id -> controller
	windows  map[uint32]*WindowController // window_id -> controller
	session  *SessionController           // single session controller

	// Parent tracking for rebuild lookups (survives controller restarts)
	paneToWindow    map[uint32]uint32 // pane_id -> window_id
	windowToSession map[uint32]uint32 // window_id -> session_id (for future multi-session)
}

// NewRegistry creates a new empty registry.
func NewRegistry() *Registry {
	return &Registry{
		runtimes:        make(map[uint32]*PaneRuntime),
		panes:           make(map[uint32]*PaneController),
		windows:         make(map[uint32]*WindowController),
		paneToWindow:    make(map[uint32]uint32),
		windowToSession: make(map[uint32]uint32),
	}
}

// Runtime Management

// RegisterRuntime adds a runtime to the registry.
func (r *Registry) RegisterRuntime(runtime *PaneRuntime) error {
	if runtime == nil {
		return fmt.Errorf("cannot register nil runtime")
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	id := runtime.ID()
	if existing, ok := r.runtimes[id]; ok && existing != nil {
		return fmt.Errorf("runtime with id=%d already registered", id)
	}

	r.runtimes[id] = runtime
	return nil
}

// GetRuntime retrieves a runtime by ID.
func (r *Registry) GetRuntime(id uint32) *PaneRuntime {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.runtimes[id]
}

// UnregisterRuntime removes a runtime from the registry.
func (r *Registry) UnregisterRuntime(id uint32) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.runtimes, id)
}

// ListRuntimes returns all registered runtime IDs.
func (r *Registry) ListRuntimes() []uint32 {
	r.mu.RLock()
	defer r.mu.RUnlock()

	ids := make([]uint32, 0, len(r.runtimes))
	for id := range r.runtimes {
		ids = append(ids, id)
	}
	return ids
}

// RuntimesByWindow returns all runtime IDs for a given window.
// This requires the window to have stored pane membership.
func (r *Registry) RuntimesByWindow(windowID uint32, paneIDs []uint32) []*PaneRuntime {
	r.mu.RLock()
	defer r.mu.RUnlock()

	runtimes := make([]*PaneRuntime, 0, len(paneIDs))
	for _, paneID := range paneIDs {
		if runtime, ok := r.runtimes[paneID]; ok {
			runtimes = append(runtimes, runtime)
		}
	}
	return runtimes
}

// Pane Controller Management

// RegisterPane adds a pane controller to the registry.
func (r *Registry) RegisterPane(pane *PaneController) error {
	if pane == nil {
		return fmt.Errorf("cannot register nil pane")
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	id := pane.id
	r.panes[id] = pane
	return nil
}

// GetPane retrieves a pane controller by ID.
func (r *Registry) GetPane(id uint32) *PaneController {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.panes[id]
}

// UnregisterPane removes a pane controller from the registry.
func (r *Registry) UnregisterPane(id uint32) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.panes, id)
}

// Window Controller Management

// RegisterWindow adds a window controller to the registry.
func (r *Registry) RegisterWindow(window *WindowController) error {
	if window == nil {
		return fmt.Errorf("cannot register nil window")
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	id := window.id
	if existing, ok := r.windows[id]; ok && existing != nil {
		return fmt.Errorf("window with id=%d already registered", id)
	}

	r.windows[id] = window
	return nil
}

// GetWindow retrieves a window controller by ID.
func (r *Registry) GetWindow(id uint32) *WindowController {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.windows[id]
}

// UnregisterWindow removes a window controller from the registry.
func (r *Registry) UnregisterWindow(id uint32) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.windows, id)
}

// ListWindows returns all registered window IDs.
func (r *Registry) ListWindows() []uint32 {
	r.mu.RLock()
	defer r.mu.RUnlock()

	ids := make([]uint32, 0, len(r.windows))
	for id := range r.windows {
		ids = append(ids, id)
	}
	return ids
}

// Session Controller Management

// SetSession sets the session controller.
func (r *Registry) SetSession(session *SessionController) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.session = session
}

// GetSession retrieves the session controller.
func (r *Registry) GetSession() *SessionController {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.session
}

// ClearSession removes the session controller.
func (r *Registry) ClearSession() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.session = nil
}

// Parent Tracking Methods

// SetPaneWindow sets the parent window for a pane.
func (r *Registry) SetPaneWindow(paneID, windowID uint32) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.paneToWindow[paneID] = windowID
}

// GetPaneWindow returns the parent window ID for a pane (0 if not set).
func (r *Registry) GetPaneWindow(paneID uint32) uint32 {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.paneToWindow[paneID]
}

// RemovePaneWindow removes the parent mapping for a pane.
func (r *Registry) RemovePaneWindow(paneID uint32) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.paneToWindow, paneID)
}

// SetWindowSession sets the parent session for a window.
func (r *Registry) SetWindowSession(windowID, sessionID uint32) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.windowToSession[windowID] = sessionID
}

// GetWindowSession returns the parent session ID for a window (0 if not set).
func (r *Registry) GetWindowSession(windowID uint32) uint32 {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.windowToSession[windowID]
}

// GetWindowPanes returns all pane IDs for a given window.
func (r *Registry) GetWindowPanes(windowID uint32) []uint32 {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var paneIDs []uint32
	for paneID, winID := range r.paneToWindow {
		if winID == windowID {
			paneIDs = append(paneIDs, paneID)
		}
	}
	return paneIDs
}

// GetSessionWindows returns all window IDs for a given session.
func (r *Registry) GetSessionWindows(sessionID uint32) []uint32 {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var windowIDs []uint32
	for windowID, sessID := range r.windowToSession {
		if sessID == sessionID {
			windowIDs = append(windowIDs, windowID)
		}
	}
	return windowIDs
}

// Stats returns registry statistics.
func (r *Registry) Stats() RegistryStats {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return RegistryStats{
		RuntimeCount: len(r.runtimes),
		PaneCount:    len(r.panes),
		WindowCount:  len(r.windows),
		HasSession:   r.session != nil,
	}
}

// RegistryStats contains statistics about the registry.
type RegistryStats struct {
	RuntimeCount int
	PaneCount    int
	WindowCount  int
	HasSession   bool
}

// Clear removes all entries from the registry.
func (r *Registry) Clear() {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.runtimes = make(map[uint32]*PaneRuntime)
	r.panes = make(map[uint32]*PaneController)
	r.windows = make(map[uint32]*WindowController)
	r.session = nil
	r.paneToWindow = make(map[uint32]uint32)
	r.windowToSession = make(map[uint32]uint32)
}

// Rebuild helpers for crash recovery

// RebuildSessionState holds the data needed to rebuild a session controller.
type RebuildSessionState struct {
	SessionID    uint32
	SessionName  string
	Shell        string
	WindowIDs    []uint32
	ActiveWindow uint32
}

// GetRebuildSessionState returns the state needed to rebuild the session.
// Returns nil if no session is registered.
func (r *Registry) GetRebuildSessionState(sessionID uint32) *RebuildSessionState {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if r.session == nil {
		return nil
	}

	return &RebuildSessionState{
		SessionID:    sessionID,
		WindowIDs:    r.GetSessionWindows(sessionID),
		ActiveWindow: 0, // Will be set from session's internal state
	}
}

// RebuildWindowState holds the data needed to rebuild a window controller.
type RebuildWindowState struct {
	WindowID   uint32
	SessionID  uint32
	PaneIDs    []uint32
	ActivePane uint32
	PaneOrder  []uint32
}

// GetRebuildWindowState returns the state needed to rebuild a window.
// Returns nil if window is not registered.
func (r *Registry) GetRebuildWindowState(windowID uint32) *RebuildWindowState {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if _, ok := r.windows[windowID]; !ok {
		return nil
	}

	return &RebuildWindowState{
		WindowID:  windowID,
		SessionID: r.windowToSession[windowID],
		PaneIDs:   r.GetWindowPanes(windowID),
	}
}

// GetAllRuntimes returns a copy of all registered runtimes.
// Used by supervisor during session/window rebuild.
func (r *Registry) GetAllRuntimes() map[uint32]*PaneRuntime {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make(map[uint32]*PaneRuntime, len(r.runtimes))
	for id, runtime := range r.runtimes {
		result[id] = runtime
	}
	return result
}

// RegisterPaneWithParent registers a pane and sets its parent window atomically.
func (r *Registry) RegisterPaneWithParent(pane *PaneController, windowID uint32) error {
	if err := r.RegisterPane(pane); err != nil {
		return err
	}
	r.SetPaneWindow(pane.id, windowID)
	return nil
}

// RegisterWindowWithParent registers a window and sets its parent session atomically.
func (r *Registry) RegisterWindowWithParent(window *WindowController, sessionID uint32) error {
	if err := r.RegisterWindow(window); err != nil {
		return err
	}
	r.SetWindowSession(window.id, sessionID)
	return nil
}

// WindowController is a placeholder for the new window controller type.
// This will be implemented in window_controller.go
//
// WARNING: This is a Phase 1 placeholder. WindowController functionality
// is incomplete and will be fully implemented in Phase 2.
type WindowController struct {
	id uint32
	// TODO(Phase 2): Add window controller fields when implementing
}

// SessionController is a placeholder for the new session controller type.
// This will be implemented in session_controller.go
//
// WARNING: This is a Phase 1 placeholder. SessionController functionality
// is incomplete and will be fully implemented in Phase 2.
type SessionController struct {
	// TODO(Phase 2): Add session controller fields when implementing
}
