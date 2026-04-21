package shux

import (
	"fmt"
	"sync"
)

// Registry is a stable storage for runtimes and controllers.
// It survives controller restarts and provides stable identity.
type Registry struct {
	mu       sync.RWMutex
	runtimes map[uint32]*PaneRuntime       // pane_id -> runtime
	panes    map[uint32]*PaneController    // pane_id -> controller
	windows  map[uint32]*WindowController  // window_id -> controller
	session  *SessionController            // single session controller
}

// NewRegistry creates a new empty registry.
func NewRegistry() *Registry {
	return &Registry{
		runtimes: make(map[uint32]*PaneRuntime),
		panes:    make(map[uint32]*PaneController),
		windows:  make(map[uint32]*WindowController),
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

// Stats returns registry statistics.
func (r *Registry) Stats() RegistryStats {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return RegistryStats{
		RuntimeCount:  len(r.runtimes),
		PaneCount:     len(r.panes),
		WindowCount:   len(r.windows),
		HasSession:    r.session != nil,
	}
}

// RegistryStats contains statistics about the registry.
type RegistryStats struct {
	RuntimeCount  int
	PaneCount     int
	WindowCount   int
	HasSession    bool
}

// Clear removes all entries from the registry.
func (r *Registry) Clear() {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.runtimes = make(map[uint32]*PaneRuntime)
	r.panes = make(map[uint32]*PaneController)
	r.windows = make(map[uint32]*WindowController)
	r.session = nil
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
	id   uint32
	name string
	// TODO(Phase 2): Add session controller fields when implementing
}
