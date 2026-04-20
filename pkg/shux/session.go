package shux

import (
	"github.com/nhlmg93/gotor/actor"
)

type Session struct {
	id          uint32
	windows     map[uint32]*actor.Ref
	windowOrder []uint32
	active      uint32
	windowID    uint32
	subscribers map[*actor.Ref]struct{}
}

func NewSession(id uint32) *Session {
	return &Session{
		id:          id,
		windows:     make(map[uint32]*actor.Ref),
		subscribers: make(map[*actor.Ref]struct{}),
	}
}

func SpawnSession(id uint32, parent *actor.Ref) *actor.Ref {
	s := NewSession(id)
	return actor.SpawnWithParent(s, 32, parent)
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
	default:
		envelope.Reply <- nil
	}
}

func (s *Session) activeWindow() *actor.Ref {
	if s.active == 0 {
		return nil
	}
	return s.windows[s.active]
}

func (s *Session) resizeActiveWindow(rows, cols int) {
	Infof("session %d: resizing active window to %dx%d", s.id, rows, cols)
	if win := s.activeWindow(); win != nil {
		win.Send(ResizeMsg{Rows: rows, Cols: cols})
	}
}

func (s *Session) createWindow(rows, cols int) {
	s.windowID++
	Infof("session %d: creating window %d with size %dx%d", s.id, s.windowID, rows, cols)
	ref := SpawnWindow(s.windowID, actor.Self())
	s.windows[s.windowID] = ref
	s.windowOrder = append(s.windowOrder, s.windowID)
	ref.Send(CreatePane{Rows: rows, Cols: cols, Shell: "/bin/sh"})
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
	s.active = newActive
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
