package shux

import "fmt"

type SessionRef struct {
	*loopRef
}

func (r *SessionRef) Send(msg any) bool {
	if r == nil {
		return false
	}
	return r.send(msg)
}

func (r *SessionRef) Ask(msg any) chan any {
	if r == nil {
		return nil
	}
	return r.ask(msg)
}

func (r *SessionRef) Stop() {
	if r != nil {
		r.stopLoop()
	}
}

func (r *SessionRef) Shutdown() {
	if r != nil {
		r.shutdown()
	}
}

type Session struct {
	ref         *SessionRef
	notify      func(any)
	id          uint32
	name        string
	windows     map[uint32]*WindowRef
	windowOrder []uint32
	active      uint32
	windowID    uint32
	subscribers map[chan any]struct{}
	shell       string
	snapshot    *SessionSnapshot
}

func NewSession(id uint32) *Session {
	return NewNamedSessionWithShell(id, DefaultSessionName, DefaultShell)
}

func NewSessionWithShell(id uint32, shell string) *Session {
	return NewNamedSessionWithShell(id, DefaultSessionName, shell)
}

func NewNamedSessionWithShell(id uint32, name, shell string) *Session {
	name = normalizeSessionName(name)
	return &Session{
		id:          id,
		name:        name,
		windows:     make(map[uint32]*WindowRef),
		subscribers: make(map[chan any]struct{}),
		shell:       normalizeShell(shell),
	}
}

func StartSession(id uint32, notify func(any)) *SessionRef {
	return StartNamedSessionWithShell(id, DefaultSessionName, DefaultShell, notify)
}

func StartSessionWithShell(id uint32, shell string, notify func(any)) *SessionRef {
	return StartNamedSessionWithShell(id, DefaultSessionName, shell, notify)
}

func StartNamedSessionWithShell(id uint32, name, shell string, notify func(any)) *SessionRef {
	s := NewNamedSessionWithShell(id, name, shell)
	s.notify = notify
	return startSessionLoop(s)
}

func StartSessionFromSnapshot(snapshot *SessionSnapshot, notify func(any)) *SessionRef {
	name := normalizeSessionName(snapshot.SessionName)
	s := &Session{
		id:          snapshot.ID,
		name:        name,
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

	Infof("session: init id=%d name=%s shell=%s restore=%t", s.id, s.name, s.shell, s.snapshot != nil)
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
		Errorf("session: crash id=%d name=%s reason=%v", s.id, s.name, reason)
		return
	}
	Infof("session: terminate id=%d name=%s", s.id, s.name)
}

func (s *Session) restoreFromSnapshot() {
	Infof("restore: session=%s id=%d windows=%d activeWindow=%d", s.name, s.id, len(s.snapshot.Windows), s.snapshot.ActiveWindow)

	windowsByID := make(map[uint32]WindowSnapshot, len(s.snapshot.Windows))
	var maxWindowID uint32
	for _, winSnap := range s.snapshot.Windows {
		windowsByID[winSnap.ID] = winSnap
		if winSnap.ID > maxWindowID {
			maxWindowID = winSnap.ID
		}
	}

	for _, winID := range s.snapshot.WindowOrder {
		winSnap, ok := windowsByID[winID]
		if !ok {
			continue
		}

		Infof("restore: session=%s window=%d panes=%d activePane=%d", s.name, winSnap.ID, len(winSnap.PaneOrder), winSnap.ActivePane)
		windowRef := StartWindow(winSnap.ID, s.ref)
		s.windows[winSnap.ID] = windowRef
		s.windowOrder = append(s.windowOrder, winSnap.ID)

		panesByID := make(map[uint32]PaneSnapshot, len(winSnap.Panes))
		for _, paneSnap := range winSnap.Panes {
			panesByID[paneSnap.ID] = paneSnap
		}
		for _, paneID := range winSnap.PaneOrder {
			paneSnap, ok := panesByID[paneID]
			if !ok {
				continue
			}
			Infof("restore: session=%s window=%d pane=%d shell=%s cwd=%s rows=%d cols=%d", s.name, winSnap.ID, paneSnap.ID, paneSnap.Shell, paneSnap.CWD, paneSnap.Rows, paneSnap.Cols)
			windowRef.Send(CreatePane{
				ID:    paneSnap.ID,
				Rows:  paneSnap.Rows,
				Cols:  paneSnap.Cols,
				Shell: paneSnap.Shell,
				CWD:   paneSnap.CWD,
			})
		}
		if winSnap.Layout != nil {
			windowRef.Send(RestoreWindowLayout{Root: winSnap.Layout, ActivePane: winSnap.ActivePane})
		} else if activeIdx := indexOfOrderedID(winSnap.PaneOrder, winSnap.ActivePane); activeIdx > 0 {
			windowRef.Send(SwitchToPane{Index: activeIdx})
		}
	}

	s.windowID = maxWindowID
	if s.snapshot.ActiveWindow != 0 {
		s.active = s.snapshot.ActiveWindow
	} else if len(s.windowOrder) > 0 {
		s.active = s.windowOrder[0]
	}

	Infof("restore: session=%s id=%d complete windows=%d activeWindow=%d nextWindowID=%d", s.name, s.id, len(s.windows), s.active, s.windowID)
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
			Warnf("detach: session=%s id=%d failed err=%v", s.name, s.id, err)
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
			WindowOrder:  append([]uint32(nil), s.windowOrder...),
		}
		envelope.reply <- data
	case DetachSession:
		envelope.reply <- s.handleDetach()
	default:
		envelope.reply <- nil
	}
}

func (s *Session) handleDetach() error {
	Infof("detach: session=%s id=%d requested windows=%d", s.name, s.id, len(s.windowOrder))
	snapshot := s.buildSnapshot()
	path := SessionSnapshotPath(s.name)
	Infof("detach: session=%s id=%d snapshot-built windows=%d activeWindow=%d path=%s", s.name, s.id, len(snapshot.Windows), snapshot.ActiveWindow, path)
	if err := SaveSnapshot(path, snapshot); err != nil {
		return fmt.Errorf("save snapshot %q: %w", path, err)
	}

	for _, win := range s.windows {
		win.Stop()
	}

	s.notifyEvent(SessionEmpty{ID: s.id})
	Infof("detach: session=%s id=%d stopping loop", s.name, s.id)
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
		WindowOrder:  append([]uint32(nil), s.windowOrder...),
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
	Infof("session: id=%d name=%s resize-active-window rows=%d cols=%d", s.id, s.name, rows, cols)
	if win := s.activeWindow(); win != nil {
		win.Send(ResizeMsg{Rows: rows, Cols: cols})
	}
}

func (s *Session) createWindow(rows, cols int) {
	s.windowID++
	Infof("session: id=%d name=%s create-window window=%d rows=%d cols=%d shell=%s", s.id, s.name, s.windowID, rows, cols, s.shell)
	ref := StartWindow(s.windowID, s.ref)
	s.windows[s.windowID] = ref
	s.windowOrder = append(s.windowOrder, s.windowID)
	ref.Send(CreatePane{Rows: rows, Cols: cols, Shell: s.shell})
	if s.active == 0 {
		s.active = s.windowID
	}
}

func (s *Session) switchWindow(delta int) {
	if len(s.windowOrder) == 0 || delta == 0 {
		return
	}

	currentIdx := s.activeWindowIndex()
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
	Infof("session: id=%d name=%s switch-window from=%d to=%d delta=%d", s.id, s.name, oldActive, newActive, delta)
	s.forwardUpdate(PaneContentUpdated{})
}

func (s *Session) activeWindowIndex() int {
	for i, id := range s.windowOrder {
		if id == s.active {
			return i
		}
	}
	return -1
}

func (s *Session) handleWindowEmpty(id uint32) {
	currentIdx := s.activeWindowIndex()
	delete(s.windows, id)
	s.windowOrder = removeOrderedID(s.windowOrder, id)

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

func removeOrderedID(ids []uint32, target uint32) []uint32 {
	for i, id := range ids {
		if id == target {
			return append(ids[:i], ids[i+1:]...)
		}
	}
	return ids
}

func indexOfOrderedID(ids []uint32, target uint32) int {
	for i, id := range ids {
		if id == target {
			return i
		}
	}
	return -1
}
