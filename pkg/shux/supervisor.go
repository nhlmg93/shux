package shux

import (
	"fmt"
	"sync"
	"time"
)

// Supervisor monitors controllers and handles restarts.
// It maintains the supervision tree inside a session owner process.
type Supervisor struct {
	registry *Registry
	logger   ShuxLogger

	// Configuration
	maxRestarts   int
	restartWindow time.Duration
	panicCooldown time.Duration

	// Restart tracking
	restartCounts map[uint32][]time.Time // entity_id -> restart times
	restartMu     sync.RWMutex

	// Lifecycle
	stop chan struct{}
	wg   sync.WaitGroup

	// Restart callbacks - set by session/window controllers for supervisor to call back
	// These allow the supervisor to recreate controllers with proper parent references
	onSessionRestart func() (*SessionController, error)
	onWindowRestart  func(windowID uint32) (*WindowController, error)
	onPaneRestart    func(runtime *PaneRuntime) (*PaneController, error)
}

// SetRestartCallbacks configures the callbacks for controller restart.
// These are called by the supervisor to recreate controllers during crash recovery.
func (s *Supervisor) SetRestartCallbacks(
	sessionFn func() (*SessionController, error),
	windowFn func(windowID uint32) (*WindowController, error),
	paneFn func(runtime *PaneRuntime) (*PaneController, error),
) {
	s.onSessionRestart = sessionFn
	s.onWindowRestart = windowFn
	s.onPaneRestart = paneFn
}

// SupervisorConfig contains configuration for the supervisor.
type SupervisorConfig struct {
	MaxRestarts   int
	RestartWindow time.Duration
	PanicCooldown time.Duration
	Logger        ShuxLogger
}

// DefaultSupervisorConfig returns sensible defaults.
func DefaultSupervisorConfig() SupervisorConfig {
	return SupervisorConfig{
		MaxRestarts:   5,
		RestartWindow: 60 * time.Second,
		PanicCooldown: 1 * time.Second,
	}
}

// NewSupervisor creates a new supervisor with the given registry.
func NewSupervisor(registry *Registry, cfg SupervisorConfig) *Supervisor {
	if cfg.MaxRestarts == 0 {
		cfg = DefaultSupervisorConfig()
	}

	return &Supervisor{
		registry:      registry,
		logger:        cfg.Logger,
		maxRestarts:   cfg.MaxRestarts,
		restartWindow: cfg.RestartWindow,
		panicCooldown: cfg.PanicCooldown,
		restartCounts: make(map[uint32][]time.Time),
		stop:          make(chan struct{}),
	}
}

// Start begins the supervisor's monitoring.
func (s *Supervisor) Start() {
	s.wg.Add(1)
	go s.monitorLoop()
}

// Stop shuts down the supervisor.
func (s *Supervisor) Stop() {
	close(s.stop)
	s.wg.Wait()
}

// monitorLoop is the main supervision loop.
func (s *Supervisor) monitorLoop() {
	defer s.wg.Done()

	// For now, this is a passive supervisor that handles restarts
	// initiated by controller crashes (detected via panic recovery).
	// In the future, this could actively poll controller health.

	<-s.stop
}

// HandlePaneCrash handles a pane controller crash.
// It attempts to restart the controller around the existing runtime.
func (s *Supervisor) HandlePaneCrash(paneID uint32, panicErr interface{}) error {
	if s.logger != nil {
		s.logger.Errorf("supervisor: pane controller %d crashed: %v", paneID, panicErr)
	}

	// Check restart limits
	if !s.canRestart(paneID) {
		return fmt.Errorf("pane %d: exceeded max restarts (%d in %v)", paneID, s.maxRestarts, s.restartWindow)
	}

	// Record this restart
	s.recordRestart(paneID)

	// Get the existing runtime (must exist for restart)
	runtime := s.registry.GetRuntime(paneID)
	if runtime == nil {
		return fmt.Errorf("pane %d: no runtime found for restart", paneID)
	}

	// Wait a bit before restarting (cooldown)
	time.Sleep(s.panicCooldown)

	// Check if supervisor is stopping
	select {
	case <-s.stop:
		return fmt.Errorf("supervisor stopping, aborting restart")
	default:
	}

	// Find the parent window from registry
	windowID := s.registry.GetPaneWindow(paneID)
	if windowID == 0 {
		return fmt.Errorf("pane %d: no parent window found in registry", paneID)
	}

	// Get the parent window reference
	window := s.registry.GetWindow(windowID)
	if window == nil {
		return fmt.Errorf("pane %d: parent window %d not found", paneID, windowID)
	}

	// Create new controller around existing runtime
	if s.onPaneRestart == nil {
		return fmt.Errorf("pane %d: no restart callback registered", paneID)
	}

	newController, err := s.onPaneRestart(runtime)
	if err != nil {
		return fmt.Errorf("pane %d: restart failed: %w", paneID, err)
	}

	// Register the new controller
	if err := s.registry.RegisterPane(newController); err != nil {
		return fmt.Errorf("pane %d: register failed: %w", paneID, err)
	}

	if s.logger != nil {
		s.logger.Infof("supervisor: pane %d controller restarted successfully", paneID)
	}

	return nil
}

// HandleWindowCrash handles a window controller crash.
// It rebuilds the window from registry + structural state without killing panes.
func (s *Supervisor) HandleWindowCrash(windowID uint32, panicErr interface{}) error {
	if s.logger != nil {
		s.logger.Errorf("supervisor: window controller %d crashed: %v", windowID, panicErr)
	}

	// Check restart limits
	if !s.canRestart(windowID) {
		return fmt.Errorf("window %d: exceeded max restarts", windowID)
	}

	s.recordRestart(windowID)

	// Window restart strategy:
	// 1. Get all pane runtimes for this window (they survive)
	// 2. Rebuild window controller with same layout/active pane
	// 3. Rebind pane controllers to new window

	time.Sleep(s.panicCooldown)

	select {
	case <-s.stop:
		return fmt.Errorf("supervisor stopping")
	default:
	}

	// Get rebuild state from registry
	rebuildState := s.registry.GetRebuildWindowState(windowID)
	if rebuildState == nil {
		return fmt.Errorf("window %d: no rebuild state found in registry", windowID)
	}

	// Create new window controller
	if s.onWindowRestart == nil {
		return fmt.Errorf("window %d: no restart callback registered", windowID)
	}

	newController, err := s.onWindowRestart(windowID)
	if err != nil {
		return fmt.Errorf("window %d: restart failed: %w", windowID, err)
	}

	// Register the new controller
	if err := s.registry.RegisterWindow(newController); err != nil {
		return fmt.Errorf("window %d: register failed: %w", windowID, err)
	}

	if s.logger != nil {
		s.logger.Infof("supervisor: window %d controller restarted successfully", windowID)
	}

	return nil
}

// HandleSessionCrash handles a session controller crash.
// It rebuilds the session from registry + structural state without killing windows.
func (s *Supervisor) HandleSessionCrash(panicErr interface{}) error {
	if s.logger != nil {
		s.logger.Errorf("supervisor: session controller crashed: %v", panicErr)
	}

	// Check restart limits for session (use special ID 0)
	if !s.canRestart(0) {
		return fmt.Errorf("session: exceeded max restarts")
	}

	s.recordRestart(0)

	// Session restart strategy:
	// 1. All window controllers survive (unless they also crashed)
	// 2. All pane runtimes survive
	// 3. Rebuild session controller with same windows/active window

	time.Sleep(s.panicCooldown)

	select {
	case <-s.stop:
		return fmt.Errorf("supervisor stopping")
	default:
	}

	// Create new session controller
	if s.onSessionRestart == nil {
		return fmt.Errorf("session: no restart callback registered")
	}

	newController, err := s.onSessionRestart()
	if err != nil {
		return fmt.Errorf("session: restart failed: %w", err)
	}

	// Register the new controller
	s.registry.SetSession(newController)

	if s.logger != nil {
		s.logger.Infof("supervisor: session controller restarted successfully")
	}

	return nil
}

// canRestart checks if an entity can be restarted (rate limiting).
func (s *Supervisor) canRestart(id uint32) bool {
	s.restartMu.Lock()
	defer s.restartMu.Unlock()

	restarts := s.restartCounts[id]
	now := time.Now()

	// Remove old restarts outside the window
	validRestarts := make([]time.Time, 0, len(restarts))
	for _, t := range restarts {
		if now.Sub(t) < s.restartWindow {
			validRestarts = append(validRestarts, t)
		}
	}
	s.restartCounts[id] = validRestarts

	return len(validRestarts) < s.maxRestarts
}

// recordRestart records a restart attempt for rate limiting.
func (s *Supervisor) recordRestart(id uint32) {
	s.restartMu.Lock()
	defer s.restartMu.Unlock()

	s.restartCounts[id] = append(s.restartCounts[id], time.Now())
}

// RestartPaneController creates a new controller around an existing runtime.
// This is called during crash recovery to restart a pane controller.
func (s *Supervisor) RestartPaneController(runtime *PaneRuntime) (*PaneController, error) {
	if s.onPaneRestart == nil {
		return nil, fmt.Errorf("no pane restart callback registered")
	}
	return s.onPaneRestart(runtime)
}

// RestartWindowController rebuilds a window controller from registry state.
// This is called during crash recovery to restart a window controller.
func (s *Supervisor) RestartWindowController(windowID uint32) (*WindowController, error) {
	if s.onWindowRestart == nil {
		return nil, fmt.Errorf("no window restart callback registered")
	}
	return s.onWindowRestart(windowID)
}

// RestartSessionController rebuilds the session controller from registry state.
// This is called during crash recovery to restart the session controller.
func (s *Supervisor) RestartSessionController() (*SessionController, error) {
	if s.onSessionRestart == nil {
		return nil, fmt.Errorf("no session restart callback registered")
	}
	return s.onSessionRestart()
}

// RestartStats returns restart statistics for monitoring.
func (s *Supervisor) RestartStats() map[uint32]int {
	s.restartMu.RLock()
	defer s.restartMu.RUnlock()

	stats := make(map[uint32]int)
	for id, restarts := range s.restartCounts {
		// Count only recent restarts
		now := time.Now()
		count := 0
		for _, t := range restarts {
			if now.Sub(t) < s.restartWindow {
				count++
			}
		}
		if count > 0 {
			stats[id] = count
		}
	}
	return stats
}

// SupervisorGuard wraps a function with panic recovery that notifies the supervisor.
// If the supervisor fails to restart the entity, the panic is re-raised.
func SupervisorGuard(supervisor *Supervisor, entityType string, entityID uint32, fn func()) {
	defer func() {
		if r := recover(); r != nil {
			// Log the panic with context
			panicErr := fmt.Errorf("panic in %s %d: %v", entityType, entityID, r)
			if supervisor.logger != nil {
				supervisor.logger.Errorf("supervisor: %v", panicErr)
			}

			// Notify supervisor for restart - re-panic if restart fails
			var restartErr error
			switch entityType {
			case "pane":
				restartErr = supervisor.HandlePaneCrash(entityID, r)
			case "window":
				restartErr = supervisor.HandleWindowCrash(entityID, r)
			case "session":
				restartErr = supervisor.HandleSessionCrash(r)
			}

			// Re-panic if restart failed
			if restartErr != nil {
				panic(fmt.Errorf("panic in %s %d: %v (restart failed: %w)", entityType, entityID, r, restartErr))
			}
		}
	}()

	fn()
}
