package shux

import (
	"fmt"
	"sync"
	"time"
)

// PaneController owns pane coordination and can be restarted around a stable runtime.
// It does NOT own the PTY/process - that belongs to PaneRuntime.
// If the controller panics, it can be restarted without killing the shell.
type PaneController struct {
	ref        *PaneRef
	runtime    *PaneRuntime // The stable runtime (survives controller restarts)
	supervisor *Supervisor  // For panic recovery
	parent     *WindowRef
	id         uint32

	// Controller-local state
	mouseButtons map[MouseButton]bool
	contentCache paneContentCache
	stopped      bool

	// Callback management - prevents callbacks after stop
	callbackMu    sync.RWMutex
	callbacksDone bool

	logger ShuxLogger
}

// paneContentCache caches the rendered content with debounced updates.
type paneContentCache struct {
	dirty         bool
	cached        *PaneContent
	updateTimer   *time.Timer
	updatePending bool
}

func (c *paneContentCache) Stop() {
	if c.updateTimer != nil {
		c.updateTimer.Stop()
	}
}

func (c *paneContentCache) Invalidate() {
	c.dirty = true
	c.cached = nil
}

func (c *paneContentCache) ClearPending() {
	c.updatePending = false
}

func (c *paneContentCache) Current() (*PaneContent, bool) {
	if c.dirty || c.cached == nil {
		return nil, false
	}
	return c.cached, true
}

func (c *paneContentCache) Store(content *PaneContent) *PaneContent {
	c.cached = content
	c.dirty = false
	return content
}

func (c *paneContentCache) Schedule(ref *PaneRef, delay time.Duration) {
	if ref == nil || c.updatePending {
		return
	}
	c.updatePending = true
	c.Stop()
	c.updateTimer = time.AfterFunc(delay, func() {
		if ref != nil {
			ref.Send(paneFlushUpdate{})
		}
	})
}

// PaneRef is a reference to a pane controller loop. Methods are promoted from loopRef.
type PaneRef struct {
	*loopRef
}

// Internal message types for the pane controller.
type (
	paneFlushUpdate   struct{}
	paneProcessExited struct{ Err error }
)

// NewPaneController creates a new controller around an existing runtime.
// This is the primary constructor for pane controllers.
func NewPaneController(id uint32, runtime *PaneRuntime, parent *WindowRef, logger ShuxLogger) *PaneController {
	return &PaneController{
		id:           id,
		runtime:      runtime,
		parent:       parent,
		logger:       logger,
		mouseButtons: make(map[MouseButton]bool),
	}
}

// NewPaneControllerWithSupervisor creates a controller with supervisor for crash recovery.
func NewPaneControllerWithSupervisor(id uint32, runtime *PaneRuntime, parent *WindowRef, logger ShuxLogger, supervisor *Supervisor) *PaneController {
	p := NewPaneController(id, runtime, parent, logger)
	p.supervisor = supervisor
	return p
}

// StartPaneController starts a pane controller loop around an existing runtime.
// This is used when creating new panes or restarting controllers.
func StartPaneController(id uint32, runtime *PaneRuntime, parent *WindowRef, logger ShuxLogger) *PaneRef {
	p := NewPaneController(id, runtime, parent, logger)
	ref := &PaneRef{loopRef: newLoopRef(256)}
	p.ref = ref
	go p.run()
	return ref
}

// StartPaneControllerWithSupervisor starts a pane controller with supervisor support.
func StartPaneControllerWithSupervisor(id uint32, runtime *PaneRuntime, parent *WindowRef, logger ShuxLogger, supervisor *Supervisor) *PaneRef {
	p := NewPaneControllerWithSupervisor(id, runtime, parent, logger, supervisor)
	ref := &PaneRef{loopRef: newLoopRef(256)}
	p.ref = ref
	go p.runWithSupervisor()
	return ref
}

// run is the main event loop for the pane controller.
func (p *PaneController) run() {
	var reason error
	defer func() {
		if r := recover(); r != nil {
			reason = fmt.Errorf("panic: %v\n%s", r, recoverWithContext("pane_controller", p.id, 0, 0))
		}
		p.terminate(reason)
		close(p.ref.done)
	}()

	p.setupRuntimeCallbacks()

	p.contentCache.Invalidate()
	p.contentCache.Schedule(p.ref, 0)

	for {
		select {
		case <-p.ref.stop:
			return
		case msg := <-p.ref.inbox:
			p.receive(msg)
		}
	}
}

// runWithSupervisor wraps the controller run loop with supervisor panic recovery.
func (p *PaneController) runWithSupervisor() {
	if p.supervisor != nil {
		SupervisorGuard(p.supervisor, "pane", p.id, p.run)
	} else {
		p.run()
	}
}

// setupRuntimeCallbacks configures the runtime to notify the controller of events.
func (p *PaneController) setupRuntimeCallbacks() {
	p.runtime.onTitleChanged = func(title string) {
		p.callbackMu.RLock()
		done := p.callbacksDone
		p.callbackMu.RUnlock()
		if done {
			return
		}
		p.markDirty()
	}
	p.runtime.onBell = func() {
		p.callbackMu.RLock()
		done := p.callbacksDone
		p.callbackMu.RUnlock()
		if done {
			return
		}
		p.markDirty()
	}
	p.runtime.onOutput = func() {
		p.callbackMu.RLock()
		done := p.callbacksDone
		p.callbackMu.RUnlock()
		if done {
			return
		}
		p.markDirty()
	}
	p.runtime.onProcessExit = func(err error) {
		p.callbackMu.RLock()
		done := p.callbacksDone
		p.callbackMu.RUnlock()
		if done {
			return
		}
		p.ref.Send(paneProcessExited{Err: err})
	}
}

// detachRuntimeCallbacks clears the runtime callbacks to prevent further invocations.
func (p *PaneController) detachRuntimeCallbacks() {
	if p.runtime == nil {
		return
	}
	p.runtime.onTitleChanged = nil
	p.runtime.onBell = nil
	p.runtime.onOutput = nil
	p.runtime.onProcessExit = nil
}

// terminate handles cleanup when the controller exits.
// NOTE: This does NOT close the runtime/PTY - that's separate lifecycle.
func (p *PaneController) terminate(reason error) {
	if reason != nil {
		p.logger.Errorf("pane_controller: crash id=%d reason=%v", p.id, reason)
	} else {
		p.logger.Infof("pane_controller: terminate id=%d", p.id)
	}
	p.contentCache.Stop()

	p.callbackMu.Lock()
	p.callbacksDone = true
	p.callbackMu.Unlock()

	p.detachRuntimeCallbacks()
}

// receive handles incoming messages.
func (p *PaneController) receive(msg any) {
	switch m := msg.(type) {
	case paneFlushUpdate:
		p.contentCache.ClearPending()
		if p.parent != nil {
			p.parent.Send(PaneContentUpdated{ID: p.id})
		}
	case paneProcessExited:
		if p.stopped {
			p.ref.Stop()
			return
		}
		p.logger.Infof("pane_controller: id=%d process-exited", p.id)
		p.stopped = true
		if p.parent != nil {
			p.parent.Send(PaneExited{ID: p.id})
		}
		p.ref.Stop()
	case WriteToPane:
		p.writeToPTY(m.Data)
	case KeyInput:
		p.handleKeyInput(m)
	case MouseInput:
		p.handleMouseInput(m)
	case ResizeTerm:
		oldRows, oldCols := p.runtime.GetSize()
		p.logger.Infof("pane_controller: id=%d resize from=%dx%d to=%dx%d", p.id, oldRows, oldCols, m.Rows, m.Cols)
		if err := p.runtime.Resize(m.Rows, m.Cols); err != nil {
			p.logger.Warnf("pane_controller: id=%d resize failed: %v", p.id, err)
		}
		p.markDirty()
	case KillPane:
		if p.stopped {
			return
		}
		p.logger.Infof("pane_controller: id=%d kill requested", p.id)
		p.stopped = true
		if p.parent != nil {
			p.parent.Send(PaneExited{ID: p.id})
		}
		go func() {
			if err := p.runtime.Kill(); err != nil {
				p.logger.Warnf("pane_controller: id=%d kill failed: %v", p.id, err)
			}
			p.ref.Stop()
		}()
	case askEnvelope:
		p.handleAsk(m)
	}
}

// handleAsk handles synchronous queries.
func (p *PaneController) handleAsk(envelope askEnvelope) {
	switch envelope.msg.(type) {
	case GetPaneMode:
		envelope.reply <- &PaneMode{
			InAltScreen:  p.runtime.IsAltScreen(),
			CursorHidden: !p.runtime.IsCursorVisible(),
		}
	case GetPaneContent:
		if content, ok := p.contentCache.Current(); ok {
			envelope.reply <- content
			return
		}
		content := p.runtime.BuildContent()
		envelope.reply <- p.contentCache.Store(content)
	case GetPaneShell:
		envelope.reply <- p.runtime.shell
	case GetPaneSnapshotData:
		envelope.reply <- p.runtime.GetSnapshotData()
	default:
		envelope.reply <- nil
	}
}

// markDirty marks the content cache dirty and schedules an update.
// Silently does nothing if the controller is stopped.
func (p *PaneController) markDirty() {
	p.callbackMu.RLock()
	stopped := p.callbacksDone || p.stopped
	p.callbackMu.RUnlock()
	if stopped {
		return
	}
	p.contentCache.Invalidate()
	p.contentCache.Schedule(p.ref, 16*time.Millisecond)
}

// Runtime returns the underlying runtime (for access by other components).
func (p *PaneController) Runtime() *PaneRuntime {
	return p.runtime
}

// IsStopped returns true if the controller has been stopped.
func (p *PaneController) IsStopped() bool {
	return p.stopped
}
