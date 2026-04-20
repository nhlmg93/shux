package shux

import (
	"time"
)

// PaneState represents the lifecycle state of a pane
type PaneState int

const (
	StateStarting PaneState = iota // Shell process starting
	StateReady                    // Shell running, content clean/cached
	StateDirty                    // New PTY data, needs render
	StateResizing                 // Dimensions changing
	StateExited                   // Process ended
)

func (s PaneState) String() string {
	switch s {
	case StateStarting:
		return "starting"
	case StateReady:
		return "ready"
	case StateDirty:
		return "dirty"
	case StateResizing:
		return "resizing"
	case StateExited:
		return "exited"
	default:
		return "unknown"
	}
}

// UpdateState tracks UI notification state to prevent spam
type UpdateState int

const (
	UpdateIdle UpdateState = iota    // No update in progress
	UpdatePending                       // Timer running, will notify soon
	UpdateSignaled                      // Signal sent, waiting for UI read
)

// paneStateMachine manages pane lifecycle and UI coordination
type paneStateMachine struct {
	state       PaneState
	updateState UpdateState
	timer       *time.Timer
	content     *PaneContent // Cache when in Ready state
}

func newStateMachine() *paneStateMachine {
	return &paneStateMachine{
		state:       StateStarting,
		updateState: UpdateIdle,
	}
}

// transition changes state with validation
func (sm *paneStateMachine) transition(to PaneState) {
	sm.state = to
	if to == StateDirty && sm.content != nil {
		// Invalidate cache when becoming dirty
		sm.content = nil
	}
}

// markDirty transitions to Dirty and queues UI update (throttled)
func (sm *paneStateMachine) markDirty(notify func()) {
	if sm.state == StateExited {
		return
	}
	
	sm.transition(StateDirty)
	
	// Throttle UI notifications
	if sm.updateState == UpdateIdle {
		sm.updateState = UpdatePending
		sm.timer = time.AfterFunc(16*time.Millisecond, func() {
			sm.updateState = UpdateSignaled
			notify()
		})
	}
	// If already Pending or Signaled, coalesce (do nothing)
}

// markClean transitions to Ready with cached content
func (sm *paneStateMachine) markClean(content *PaneContent) {
	sm.state = StateReady
	sm.content = content
	sm.updateState = UpdateIdle
}

// getContent returns cached content if in Ready state
func (sm *paneStateMachine) getContent() (*PaneContent, bool) {
	if sm.state == StateReady && sm.content != nil {
		return sm.content, true
	}
	return nil, false
}

// canRead returns true if pane is in a readable state
func (sm *paneStateMachine) canRead() bool {
	return sm.state == StateReady || sm.state == StateDirty
}
