package shux

import (
	"fmt"
	"os"
)

// SessionRef is a reference to a session loop. Methods are promoted from loopRef.
type SessionRef struct {
	*loopRef
	shell       string
	sessionName string
	ownerMode   bool // true if running as owner process (saves on detach, doesn't stop)
}

// GetShell returns the shell used by the session.
func (r *SessionRef) GetShell() string {
	if r == nil {
		return DefaultShell
	}
	return r.shell
}

// GetSessionName returns the name of the session.
func (r *SessionRef) GetSessionName() string {
	if r == nil {
		return DefaultSessionName
	}
	return r.sessionName
}

// SetOwnerMode marks the session as running in owner mode.
// In owner mode, detach saves the session but doesn't stop it.
func (r *SessionRef) SetOwnerMode() {
	if r != nil {
		r.ownerMode = true
	}
}

type Session struct {
	ref         *SessionRef
	logger      ShuxLogger
	notify      func(any)
	supervisor  *Supervisor // For crash recovery
	registry    *Registry   // For rebuild from registry
	id          uint32
	name        string
	windows     map[uint32]*WindowRef
	windowOrder OrderedIDList
	active      uint32
	lastActive  uint32
	windowID    uint32
	subscribers map[chan any]struct{}
	shell       string
	snapshot    *SessionSnapshot
	lastRows    int // last known terminal dimensions for window creation
	lastCols    int
	crashed     bool // true if terminating due to crash (don't kill children)
}

func NewSession(id uint32, logger ShuxLogger) *Session {
	return NewNamedSessionWithShell(id, DefaultSessionName, DefaultShell, logger)
}

func NewSessionWithShell(id uint32, shell string, logger ShuxLogger) *Session {
	return NewNamedSessionWithShell(id, DefaultSessionName, shell, logger)
}

func NewNamedSessionWithShell(id uint32, name, shell string, logger ShuxLogger) *Session {
	name = normalizeSessionName(name)
	return &Session{
		id:          id,
		name:        name,
		logger:      logger,
		windows:     make(map[uint32]*WindowRef),
		subscribers: make(map[chan any]struct{}),
		shell:       normalizeShell(shell),
	}
}

// NewSessionWithSupervisor creates a session with supervisor/ registry for crash recovery.
func NewSessionWithSupervisor(id uint32, name, shell string, logger ShuxLogger, supervisor *Supervisor, registry *Registry) *Session {
	s := NewNamedSessionWithShell(id, name, shell, logger)
	s.supervisor = supervisor
	s.registry = registry
	return s
}

func StartSession(id uint32, notify func(any), logger ShuxLogger) *SessionRef {
	return StartNamedSessionWithShell(id, DefaultSessionName, DefaultShell, notify, logger)
}

func StartSessionWithShell(id uint32, shell string, notify func(any), logger ShuxLogger) *SessionRef {
	return StartNamedSessionWithShell(id, DefaultSessionName, shell, notify, logger)
}

func StartNamedSessionWithShell(id uint32, name, shell string, notify func(any), logger ShuxLogger) *SessionRef {
	s := NewNamedSessionWithShell(id, name, shell, logger)
	s.notify = notify
	return startSessionLoop(s)
}

func StartSessionFromSnapshot(snapshot *SessionSnapshot, notify func(any), logger ShuxLogger) *SessionRef {
	name := normalizeSessionName(snapshot.SessionName)
	s := &Session{
		id:          snapshot.ID,
		name:        name,
		logger:      logger,
		notify:      notify,
		windows:     make(map[uint32]*WindowRef),
		subscribers: make(map[chan any]struct{}),
		shell:       normalizeShell(snapshot.Shell),
		snapshot:    snapshot,
	}
	return startSessionLoop(s)
}

func startSessionLoop(s *Session) *SessionRef {
	ref := &SessionRef{
		loopRef:     newLoopRef(32),
		shell:       s.shell,
		sessionName: s.name,
	}
	s.ref = ref
	go s.runWithSupervisor()
	return ref
}

// runWithSupervisor wraps the session run loop with supervisor panic recovery.
func (s *Session) runWithSupervisor() {
	if s.supervisor != nil {
		SupervisorGuard(s.supervisor, "session", 0, s.run)
	} else {
		s.run()
	}
}

func (s *Session) run() {
	var reason error
	defer func() {
		if r := recover(); r != nil {
			reason = fmt.Errorf("panic: %v\n%s", r, recoverWithContext("session", s.id, len(s.windows), int(s.active)))
		}
		s.terminate(reason)
		close(s.ref.done)
	}()

	s.logger.Infof("session: init id=%d name=%s shell=%s restore=%t", s.id, s.name, s.shell, s.snapshot != nil)
	if s.snapshot != nil {
		s.restoreFromSnapshot()
		s.snapshot = nil
	}

	for {
		select {
		case <-s.ref.stop:
			return
		case msg := <-s.ref.inbox:
			s.receive(msg)
		}
	}
}

func (s *Session) terminate(reason error) {
	if reason != nil {
		s.crashed = true
		s.logger.Errorf("session: crash id=%d name=%s reason=%v", s.id, s.name, reason)
		// On crash: Don't kill child windows - they survive via registry
		// The supervisor will rebuild this session controller
		return
	}

	// Graceful shutdown: kill all child windows
	for _, win := range s.windows {
		if win != nil {
			win.Shutdown()
		}
	}
	s.logger.Infof("session: terminate id=%d name=%s", s.id, s.name)
}

// IsCrashed returns true if the session terminated due to a crash.
// Used by supervisor to distinguish crash recovery from graceful shutdown.
func (s *Session) IsCrashed() bool {
	return s.crashed
}

// RebuildFromRegistry reconstructs the session's window list from registry state.
// This is called during crash recovery - windows/panes survive via registry.
func (s *Session) RebuildFromRegistry() error {
	if s.registry == nil {
		return fmt.Errorf("session %d: no registry available for rebuild", s.id)
	}

	s.logger.Infof("session: id=%d name=%s rebuilding from registry", s.id, s.name)

	// Get all windows for this session from registry
	windowIDs := s.registry.GetSessionWindows(s.id)

	// Rebuild window list from registry
	for _, windowID := range windowIDs {
		// Check if window controller exists in registry (it survived)
		windowController := s.registry.GetWindow(windowID)
		if windowController == nil {
			s.logger.Warnf("session: id=%d window %d in registry but no controller found", s.id, windowID)
			continue
		}

		// We need to get the WindowRef from the controller
		// For now, this is a placeholder - the actual implementation depends on
		// how WindowController is refactored to provide a WindowRef
		s.logger.Infof("session: id=%d reconnecting to window %d", s.id, windowID)
		s.windowOrder.Add(windowID)
	}

	// Restore active window from first available
	if firstWindow, ok := s.windowOrder.First(); ok {
		s.active = firstWindow
	}

	s.logger.Infof("session: id=%d name=%s rebuilt with %d windows", s.id, s.name, len(s.windowOrder))
	return nil
}

// assertInvariants validates internal state consistency.
// Panics on invariant violation (tiger style - fail fast on bugs).
func (s *Session) assertInvariants() {
	if len(s.windowOrder) != len(s.windows) {
		panic(fmt.Sprintf("session %d: windowOrder length (%d) != windows count (%d)", s.id, len(s.windowOrder), len(s.windows)))
	}

	for _, winID := range s.windowOrder {
		if _, ok := s.windows[winID]; !ok {
			panic(fmt.Sprintf("session %d: windowOrder contains missing window %d", s.id, winID))
		}
	}

	if len(s.windowOrder) > 0 {
		if s.active == 0 {
			panic(fmt.Sprintf("session %d: activeWindow=0 but %d windows exist", s.id, len(s.windowOrder)))
		}
		if _, ok := s.windows[s.active]; !ok {
			panic(fmt.Sprintf("session %d: activeWindow %d not in windows map", s.id, s.active))
		}
	}
}

func (s *Session) receive(msg any) {
	switch m := msg.(type) {
	case CreateWindow:
		s.createWindow(m.Rows, m.Cols)
	case SwitchWindow:
		s.switchWindow(m.Delta)
	case Split:
		s.forwardToActiveWindow(Split{Dir: m.Dir})
	case NavigatePane:
		s.forwardToActiveWindow(NavigatePane{Dir: m.Dir})
	case ResizePane:
		s.forwardToActiveWindow(ResizePane{Dir: m.Dir, Amount: m.Amount})
	case ActionMsg:
		_ = s.dispatchAction(m)
	case WindowEmpty:
		s.handleWindowEmpty(m.ID)
	case PaneContentUpdated:
		s.forwardUpdate(m)
	case ResizeMsg:
		s.resizeActiveWindow(m.Rows, m.Cols)
	case SubscribeUpdates:
		if m.Subscriber != nil {
			s.subscribers[m.Subscriber] = struct{}{}
		}
	case UnsubscribeUpdates:
		if m.Subscriber != nil {
			delete(s.subscribers, m.Subscriber)
		}
	case WriteToPane, KeyInput:
		s.forwardToActivePane(m)
	case MouseInput:
		s.forwardToActiveWindow(m)
	case DetachSession:
		if err := s.handleDetach(); err != nil {
			s.logger.Warnf("detach: session=%s id=%d failed err=%v", s.name, s.id, err)
		}
	case ExecuteCommandMsg:
		s.handleExecuteCommand(m)
	case askEnvelope:
		s.handleAsk(m)
	}
}

func (s *Session) handleAsk(envelope askEnvelope) {
	switch m := envelope.msg.(type) {
	case GetActiveWindow:
		if win := s.activeWindow(); win != nil {
			envelope.reply <- win
			return
		}
		envelope.reply <- nil
	case GetActivePane:
		if win := s.activeWindow(); win != nil {
			result, _ := askValue(win, GetActivePane{})
			envelope.reply <- result
			return
		}
		envelope.reply <- nil
	case GetPaneContent:
		if win := s.activeWindow(); win != nil {
			result, _ := askValue(win, envelope.msg)
			envelope.reply <- result
			return
		}
		envelope.reply <- nil
	case GetWindowView:
		if win := s.activeWindow(); win != nil {
			result, _ := askValue(win, envelope.msg)
			envelope.reply <- result
			return
		}
		envelope.reply <- nil
	case GetSessionSnapshotData:
		data := SessionSnapshotData{
			ID:           s.id,
			SessionName:  s.name,
			Shell:        s.shell,
			ActiveWindow: s.active,
			WindowOrder:  s.windowOrder.Clone(),
		}
		envelope.reply <- data
	case GetFullSessionSnapshot:
		envelope.reply <- s.buildSnapshot()
	case DetachSession:
		envelope.reply <- s.handleDetach()
	case ActionMsg:
		envelope.reply <- s.dispatchAction(m)
	case ExecuteCommandMsg:
		envelope.reply <- s.executeCommandWithResult(m)
	default:
		envelope.reply <- nil
	}
}

func (s *Session) handleDetach() error {
	s.logger.Infof("detach: session=%s id=%d requested windows=%d ownerMode=%t", s.name, s.id, len(s.windowOrder), s.ref.ownerMode)
	snapshot := s.buildSnapshot()

	if s.ref.ownerMode {
		if err := MarkSnapshotLive(snapshot, os.Getpid(), SessionSocketPath(s.name)); err != nil {
			s.logger.Warnf("detach: session=%s failed to mark live: %v", s.name, err)
		}
	}

	path := SessionSnapshotPath(s.name)
	s.logger.Infof("detach: session=%s id=%d snapshot-built windows=%d activeWindow=%d path=%s", s.name, s.id, len(snapshot.Windows), snapshot.ActiveWindow, path)
	if err := SaveSnapshot(path, snapshot, s.logger); err != nil {
		return fmt.Errorf("save snapshot %q: %w", path, err)
	}

	if s.ref.ownerMode {
		s.logger.Infof("detach: session=%s id=%d owner saved and staying alive", s.name, s.id)
		return nil
	}

	for _, win := range s.windows {
		win.Stop()
	}

	s.notifyEvent(SessionEmpty{ID: s.id})
	s.logger.Infof("detach: session=%s id=%d stopping loop", s.name, s.id)
	s.ref.Stop()
	return nil
}

func (s *Session) buildSnapshot() *SessionSnapshot {
	snapshot := &SessionSnapshot{
		Version:      SnapshotVersion,
		SessionName:  s.name,
		ID:           s.id,
		Shell:        s.shell,
		ActiveWindow: s.active,
		WindowOrder:  s.windowOrder.Clone(),
		Windows:      make([]WindowSnapshot, 0, len(s.windowOrder)),
	}

	for _, winID := range s.windowOrder {
		win := s.windows[winID]
		if win == nil {
			continue
		}

		result, _ := askValue(win, GetWindowSnapshotData{})
		winData, ok := result.(WindowSnapshot)
		if !ok {
			continue
		}

		snapshot.Windows = append(snapshot.Windows, winData)
	}

	return snapshot
}

func (s *Session) activeWindow() *WindowRef {
	if s.active == 0 {
		return nil
	}
	return s.windows[s.active]
}

func (s *Session) setActiveWindow(id uint32) bool {
	if id == 0 || id == s.active {
		return false
	}
	if _, ok := s.windows[id]; !ok {
		panic(fmt.Sprintf("session %d: setActiveWindow targets missing window %d", s.id, id))
	}
	prev := s.active
	s.active = id
	if prev != 0 {
		s.lastActive = prev
	}
	s.forwardUpdate(PaneContentUpdated{})
	s.assertInvariants()
	return true
}

func (s *Session) killActiveWindow() {
	if s.active == 0 {
		return
	}
	id := s.active
	win := s.windows[id]
	if win == nil {
		return
	}
	win.Shutdown()
	s.handleWindowEmpty(id)
}

// killSession kills all windows and stops the session.
// This is the "hard quit" that doesn't save state.
func (s *Session) killSession() ActionResult {
	s.logger.Infof("kill-session: session=%s id=%d windows=%d", s.name, s.id, len(s.windowOrder))

	for _, win := range s.windows {
		if win != nil {
			win.Shutdown()
		}
	}

	s.windows = make(map[uint32]*WindowRef)
	s.windowOrder = OrderedIDList{}
	s.active = 0
	s.lastActive = 0

	s.notifyEvent(SessionEmpty{ID: s.id})

	s.logger.Infof("kill-session: session=%s id=%d stopping loop", s.name, s.id)
	s.ref.Stop()

	return ActionResult{Quit: true}
}

func (s *Session) resizeActiveWindow(rows, cols int) {
	s.logger.Infof("session: id=%d name=%s resize-active-window rows=%d cols=%d", s.id, s.name, rows, cols)
	s.lastRows, s.lastCols = rows, cols
	if win := s.activeWindow(); win != nil {
		win.Send(ResizeMsg{Rows: rows, Cols: cols})
		return
	}
	s.logger.Infof("session: id=%d name=%s no active window; creating initial window on resize", s.id, s.name)
	s.createWindow(rows, cols)
}

func (s *Session) createWindow(rows, cols int) {
	s.windowID++
	s.logger.Infof("session: id=%d name=%s create-window window=%d rows=%d cols=%d shell=%s", s.id, s.name, s.windowID, rows, cols, s.shell)
	ref := StartWindow(s.windowID, s.ref, s.logger)
	s.windows[s.windowID] = ref
	s.windowOrder.Add(s.windowID)
	ref.Send(CreatePane{Rows: rows, Cols: cols, Shell: s.shell})
	if s.active == 0 {
		s.active = s.windowID
	}
	s.assertInvariants()
}

func (s *Session) switchWindow(delta int) {
	if len(s.windowOrder) == 0 || delta == 0 {
		return
	}

	currentIdx := s.windowOrder.IndexOf(s.active)
	if currentIdx < 0 {
		currentIdx = 0
	}

	newIdx := (currentIdx + delta) % len(s.windowOrder)
	if newIdx < 0 {
		newIdx += len(s.windowOrder)
	}
	newActive := s.windowOrder[newIdx]
	oldActive := s.active
	if !s.setActiveWindow(newActive) {
		return
	}
	s.logger.Infof("session: id=%d name=%s switch-window from=%d to=%d delta=%d", s.id, s.name, oldActive, newActive, delta)
}

func (s *Session) handleWindowEmpty(id uint32) {
	if _, ok := s.windows[id]; !ok {
		panic(fmt.Sprintf("session %d: handleWindowEmpty called for missing window %d", s.id, id))
	}

	currentIdx := s.windowOrder.IndexOf(s.active)
	delete(s.windows, id)
	s.windowOrder.Remove(id)
	if s.lastActive == id {
		s.lastActive = 0
	}

	if len(s.windowOrder) == 0 {
		s.active = 0
		s.lastActive = 0
		empty := SessionEmpty{ID: s.id}
		s.notifyEvent(empty)
		s.assertInvariants()
		return
	}

	if s.active != id {
		s.assertInvariants()
		return
	}

	if currentIdx >= len(s.windowOrder) {
		currentIdx = len(s.windowOrder) - 1
	}
	if currentIdx < 0 {
		currentIdx = 0
	}
	s.active = s.windowOrder[currentIdx]
	s.forwardUpdate(PaneContentUpdated{})
	s.assertInvariants()
}

func (s *Session) forwardToActivePane(msg any) {
	if win := s.activeWindow(); win != nil {
		win.Send(msg)
	}
}

func (s *Session) forwardToActiveWindow(msg any) {
	if win := s.activeWindow(); win != nil {
		win.Send(msg)
	}
}

func (s *Session) forwardUpdate(msg PaneContentUpdated) {
	s.notifyEvent(msg)
}

func (s *Session) notifyEvent(msg any) {
	if s.notify != nil {
		s.notify(msg)
	}
	s.notifySubscribers(msg)
}

func (s *Session) notifySubscribers(msg any) {
	for subscriber := range s.subscribers {
		select {
		case subscriber <- msg:
		default:
		}
	}
}
