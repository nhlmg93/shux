package shux

import (
	"fmt"

	"github.com/nhlmg93/gotor/actor"
)

type Session struct {
	id          uint32
	name        string
	windows     map[uint32]*actor.Ref
	windowOrder []uint32
	active      uint32
	windowID    uint32
	subscribers map[*actor.Ref]struct{}
	shell       string
	snapshot    *SessionSnapshot // For restore-based creation
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
		windows:     make(map[uint32]*actor.Ref),
		subscribers: make(map[*actor.Ref]struct{}),
		shell:       normalizeShell(shell),
	}
}

func SpawnSession(id uint32, parent *actor.Ref) *actor.Ref {
	return SpawnNamedSessionWithShell(id, DefaultSessionName, DefaultShell, parent)
}

func SpawnSessionWithShell(id uint32, shell string, parent *actor.Ref) *actor.Ref {
	return SpawnNamedSessionWithShell(id, DefaultSessionName, shell, parent)
}

func SpawnNamedSessionWithShell(id uint32, name, shell string, parent *actor.Ref) *actor.Ref {
	s := NewNamedSessionWithShell(id, name, shell)
	return actor.SpawnWithParent(s, 32, parent)
}

// SpawnSessionFromSnapshot creates a session restored from a snapshot.
func SpawnSessionFromSnapshot(snapshot *SessionSnapshot, parent *actor.Ref) *actor.Ref {
	name := normalizeSessionName(snapshot.SessionName)
	s := &Session{
		id:          snapshot.ID,
		name:        name,
		windows:     make(map[uint32]*actor.Ref),
		subscribers: make(map[*actor.Ref]struct{}),
		shell:       normalizeShell(snapshot.Shell),
		snapshot:    snapshot,
	}
	return actor.SpawnWithParent(actor.WithLifecycle(s), 32, parent)
}

func (s *Session) Init() error {
	Infof("session: init id=%d name=%s shell=%s restore=%t", s.id, s.name, s.shell, s.snapshot != nil)
	if s.snapshot != nil {
		s.restoreFromSnapshot()
		s.snapshot = nil
	}
	return nil
}

func (s *Session) Terminate(reason error) {
	Infof("session: terminate id=%d name=%s reason=%v", s.id, s.name, reason)
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
		window := NewWindow(winSnap.ID)
		windowRef := actor.SpawnWithParent(window, 32, actor.Self())
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
		if activeIdx := indexOfOrderedID(winSnap.PaneOrder, winSnap.ActivePane); activeIdx > 0 {
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

func (s *Session) Receive(msg any) {
	switch m := msg.(type) {
	case CreateWindow:
		s.createWindow(m.Rows, m.Cols)
	case SwitchWindow:
		s.switchWindow(m.Delta)
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
	case DetachSession:
		if err := s.handleDetach(); err != nil {
			Warnf("detach: session=%s id=%d failed err=%v", s.name, s.id, err)
		}
	case actor.AskEnvelope:
		s.handleAsk(m)
	}
}

func (s *Session) handleAsk(envelope actor.AskEnvelope) {
	switch envelope.Msg.(type) {
	case GetActiveWindow:
		envelope.Reply <- s.activeWindow()
	case GetActivePane:
		if win := s.activeWindow(); win != nil {
			reply := win.Ask(GetActivePane{})
			envelope.Reply <- <-reply
			return
		}
		envelope.Reply <- nil
	case GetPaneContent:
		if win := s.activeWindow(); win != nil {
			reply := win.Ask(envelope.Msg)
			envelope.Reply <- <-reply
			return
		}
		envelope.Reply <- nil
	case GetSessionSnapshotData:
		data := SessionSnapshotData{
			ID:           s.id,
			Shell:        s.shell,
			ActiveWindow: s.active,
			WindowOrder:  append([]uint32(nil), s.windowOrder...),
		}
		envelope.Reply <- data
	case DetachSession:
		envelope.Reply <- s.handleDetach()
	default:
		envelope.Reply <- nil
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

	if parent := actor.Parent(); parent != nil {
		parent.Send(SessionEmpty{ID: s.id})
	}
	if me := actor.Self(); me != nil {
		Infof("detach: session=%s id=%d stopping actor tree", s.name, s.id)
		me.Stop()
	}
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

	// Gather data from each window (windows collect their own pane data)
	for _, winID := range s.windowOrder {
		win := s.windows[winID]
		if win == nil {
			continue
		}

		winReply := win.Ask(GetWindowSnapshotData{})
		winData, ok := (<-winReply).(WindowSnapshot)
		if !ok {
			continue
		}

		snapshot.Windows = append(snapshot.Windows, winData)
	}

	return snapshot
}

func (s *Session) activeWindow() *actor.Ref {
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
	ref := SpawnWindow(s.windowID, actor.Self())
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
		s.notifySubscribers(empty)
		if parent := actor.Parent(); parent != nil {
			parent.Send(empty)
		}
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

func (s *Session) forwardUpdate(msg PaneContentUpdated) {
	if parent := actor.Parent(); parent != nil {
		parent.Send(msg)
	}
	s.notifySubscribers(msg)
}

func (s *Session) notifySubscribers(msg any) {
	for subscriber := range s.subscribers {
		subscriber.Send(msg)
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
