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
//
// WARNING: This is a Phase 1 placeholder. The restart logic is not fully
// implemented. The function validates that restart is possible and logs
// the intent, but actual controller recreation requires parent window
// tracking to be completed in Phase 2.
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

	// Find the parent window
	// TODO(Phase 2): Store parent relationship in registry or runtime
	// For now, we need the window to adopt the restarted controller

	// Create new controller around existing runtime
	// The parent window reference needs to be obtained from context
	// This is a simplified version - full implementation needs parent tracking
	// TODO(Phase 2): Actually create the new controller and register it

	if s.logger != nil {
		s.logger.Infof("supervisor: pane %d controller restart prepared (Phase 2: implement actual restart)", paneID)
	}

	return nil
}

// HandleWindowCrash handles a window controller crash.
// It rebuilds the window from registry + structural state without killing panes.
//
// WARNING: This is a Phase 1 placeholder. The rebuild logic is not fully
// implemented and will be completed in Phase 2.
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

	// TODO(Phase 2): Implement full window rebuild logic

	time.Sleep(s.panicCooldown)

	select {
	case <-s.stop:
		return fmt.Errorf("supervisor stopping")
	default:
	}

	if s.logger != nil {
		s.logger.Infof("supervisor: window %d controller restart prepared (Phase 2: implement actual restart)", windowID)
	}

	return nil
}

// HandleSessionCrash handles a session controller crash.
// It rebuilds the session from registry + structural state without killing windows.
//
// WARNING: This is a Phase 1 placeholder. The rebuild logic is not fully
// implemented and will be completed in Phase 2.
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

	// TODO(Phase 2): Implement full session rebuild logic

	time.Sleep(s.panicCooldown)

	select {
	case <-s.stop:
		return fmt.Errorf("supervisor stopping")
	default:
	}

	if s.logger != nil {
		s.logger.Infof("supervisor: session controller restart prepared (Phase 2: implement actual restart)")
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
