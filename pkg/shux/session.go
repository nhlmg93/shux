package shux

import "fmt"

// SessionRef is a reference to a session loop. Methods are promoted from loopRef.
type SessionRef struct {
	*loopRef
}

type Session struct {
	ref         *SessionRef
	logger      ShuxLogger
	notify      func(any)
	id          uint32
	name        string
	windows     map[uint32]*WindowRef
	windowOrder OrderedIDList
	active      uint32
	windowID    uint32
	subscribers map[chan any]struct{}
	shell       string
	snapshot    *SessionSnapshot
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
	ref := &SessionRef{loopRef: newLoopRef(32)}
	s.ref = ref
	go s.run()
	return ref
}

func (s *Session) run() {
	var reason error
	defer func() {
		if r := recover(); r != nil {
			reason = fmt.Errorf("panic: %v", r)
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
	for _, win := range s.windows {
		if win != nil {
			win.Shutdown()
		}
	}
	if reason != nil {
		s.logger.Errorf("session: crash id=%d name=%s reason=%v", s.id, s.name, reason)
		return
	}
	s.logger.Infof("session: terminate id=%d name=%s", s.id, s.name)
}

func (s *Session) restoreFromSnapshot() {
	s.logger.Infof("restore: session=%s id=%d windows=%d activeWindow=%d", s.name, s.id, len(s.snapshot.Windows), s.snapshot.ActiveWindow)

	windowsByID, maxWindowID := indexWindowSnapshots(s.snapshot.Windows)
	for _, winID := range s.snapshot.WindowOrder {
		winSnap, ok := windowsByID[winID]
		if !ok {
			continue
		}
		s.restoreWindow(winSnap)
	}

	s.windowID = maxWindowID
	s.restoreActiveWindow(s.snapshot.ActiveWindow)

	s.logger.Infof("restore: session=%s id=%d complete windows=%d activeWindow=%d nextWindowID=%d", s.name, s.id, len(s.windows), s.active, s.windowID)
}

func indexWindowSnapshots(windows []WindowSnapshot) (map[uint32]WindowSnapshot, uint32) {
	windowsByID := make(map[uint32]WindowSnapshot, len(windows))
	var maxWindowID uint32
	for _, winSnap := range windows {
		windowsByID[winSnap.ID] = winSnap
		if winSnap.ID > maxWindowID {
			maxWindowID = winSnap.ID
		}
	}
	return windowsByID, maxWindowID
}

func (s *Session) restoreWindow(winSnap WindowSnapshot) {
	s.logger.Infof("restore: session=%s window=%d panes=%d activePane=%d", s.name, winSnap.ID, len(winSnap.PaneOrder), winSnap.ActivePane)

	windowRef := StartWindow(winSnap.ID, s.ref, s.logger)
	s.windows[winSnap.ID] = windowRef
	s.windowOrder.Add(winSnap.ID)
	s.restoreWindowPanes(windowRef, winSnap)

	if winSnap.Layout != nil {
		windowRef.Send(RestoreWindowLayout{Root: winSnap.Layout, ActivePane: winSnap.ActivePane})
		return
	}
	if activeIdx := OrderedIDList(winSnap.PaneOrder).IndexOf(winSnap.ActivePane); activeIdx > 0 {
		windowRef.Send(SwitchToPane{Index: activeIdx})
	}
}

func (s *Session) restoreWindowPanes(windowRef *WindowRef, winSnap WindowSnapshot) {
	panesByID := make(map[uint32]PaneSnapshot, len(winSnap.Panes))
	for _, paneSnap := range winSnap.Panes {
		panesByID[paneSnap.ID] = paneSnap
	}

	for _, paneID := range winSnap.PaneOrder {
		paneSnap, ok := panesByID[paneID]
		if !ok {
			continue
		}
		s.logger.Infof("restore: session=%s window=%d pane=%d shell=%s cwd=%s rows=%d cols=%d", s.name, winSnap.ID, paneSnap.ID, paneSnap.Shell, paneSnap.CWD, paneSnap.Rows, paneSnap.Cols)
		windowRef.Send(CreatePane{
			ID:    paneSnap.ID,
			Rows:  paneSnap.Rows,
			Cols:  paneSnap.Cols,
			Shell: paneSnap.Shell,
			CWD:   paneSnap.CWD,
		})
	}
}

func (s *Session) restoreActiveWindow(activeWindow uint32) {
	if activeWindow != 0 {
		if _, ok := s.windows[activeWindow]; ok {
			s.active = activeWindow
			return
		}
	}
	if firstWindow, ok := s.windowOrder.First(); ok {
		s.active = firstWindow
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
	case askEnvelope:
		s.handleAsk(m)
	}
}

func (s *Session) handleAsk(envelope askEnvelope) {
	switch envelope.msg.(type) {
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
			Shell:        s.shell,
			ActiveWindow: s.active,
			WindowOrder:  s.windowOrder.Clone(),
		}
		envelope.reply <- data
	case DetachSession:
		envelope.reply <- s.handleDetach()
	default:
		envelope.reply <- nil
	}
}

func (s *Session) handleDetach() error {
	s.logger.Infof("detach: session=%s id=%d requested windows=%d", s.name, s.id, len(s.windowOrder))
	snapshot := s.buildSnapshot()
	path := SessionSnapshotPath(s.name)
	s.logger.Infof("detach: session=%s id=%d snapshot-built windows=%d activeWindow=%d path=%s", s.name, s.id, len(snapshot.Windows), snapshot.ActiveWindow, path)
	if err := SaveSnapshot(path, snapshot, s.logger); err != nil {
		return fmt.Errorf("save snapshot %q: %w", path, err)
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

func (s *Session) resizeActiveWindow(rows, cols int) {
	s.logger.Infof("session: id=%d name=%s resize-active-window rows=%d cols=%d", s.id, s.name, rows, cols)
	if win := s.activeWindow(); win != nil {
		win.Send(ResizeMsg{Rows: rows, Cols: cols})
	}
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
	if newActive == s.active {
		return
	}
	oldActive := s.active
	s.active = newActive
	s.logger.Infof("session: id=%d name=%s switch-window from=%d to=%d delta=%d", s.id, s.name, oldActive, newActive, delta)
	s.forwardUpdate(PaneContentUpdated{})
}

func (s *Session) handleWindowEmpty(id uint32) {
	currentIdx := s.windowOrder.IndexOf(s.active)
	delete(s.windows, id)
	s.windowOrder.Remove(id)

	if len(s.windowOrder) == 0 {
		s.active = 0
		empty := SessionEmpty{ID: s.id}
		s.notifyEvent(empty)
		return
	}

	if s.active != id {
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
