package main

import (
	"github.com/nhlmg93/gotor/actor"
)

// SessionActor manages multiple windows
type SessionActor struct {
	id       uint32
	windows  map[uint32]*actor.Ref
	active   uint32
	windowID uint32
	parent   *actor.Ref
	self     *actor.Ref
}

func NewSessionActor(id uint32, parent *actor.Ref) *SessionActor {
	return &SessionActor{
		id:      id,
		windows: make(map[uint32]*actor.Ref),
		parent:  parent,
	}
}

func SpawnSessionActor(id uint32, parent *actor.Ref) *actor.Ref {
	s := NewSessionActor(id, parent)
	ref := actor.Spawn(s, 10)
	s.self = ref
	return ref
}

func (s *SessionActor) Receive(msg any) {
	switch m := msg.(type) {
	case CreateWindow:
		s.createWindow()
	case SwitchWindow:
		s.switchWindow(m.Delta)
	case WindowEmpty:
		s.handleWindowEmpty(m.ID)
	case GetActiveWindow:
		// Cannot handle here - need AskEnvelope
	case GetActivePane:
		// Cannot handle here - need AskEnvelope
	case actor.AskEnvelope:
		s.handleAsk(m)
	}
}

func (s *SessionActor) handleAsk(envelope actor.AskEnvelope) {
	switch envelope.Msg.(type) {
	case GetActiveWindow:
		if s.active != 0 {
			if win, ok := s.windows[s.active]; ok {
				envelope.Reply <- win
				return
			}
		}
		envelope.Reply <- nil
	case GetActivePane:
		if s.active != 0 {
			if win, ok := s.windows[s.active]; ok {
				// Forward to window
				reply := win.Ask(GetActivePane{})
				paneRef := <-reply
				envelope.Reply <- paneRef
				return
			}
		}
		envelope.Reply <- nil
	default:
		envelope.Reply <- nil
	}
}

func (s *SessionActor) createWindow() {
	s.windowID++
	ref := SpawnWindowActor(s.windowID, s.self)
	s.windows[s.windowID] = ref
	ref.Send(CreatePane{Cmd: "/bin/sh", Args: []string{}})
	if s.active == 0 {
		s.active = s.windowID
	}
}

func (s *SessionActor) switchWindow(delta int) {
	if len(s.windows) == 0 {
		return
	}
	// Get ordered list of window IDs
	ids := make([]uint32, 0, len(s.windows))
	for id := range s.windows {
		ids = append(ids, id)
	}
	// Find current index
	currentIdx := 0
	for i, id := range ids {
		if id == s.active {
			currentIdx = i
			break
		}
	}
	// Calculate new index with wrap
	newIdx := (currentIdx + delta + len(ids)) % len(ids)
	s.active = ids[newIdx]
}

func (s *SessionActor) handleWindowEmpty(id uint32) {
	delete(s.windows, id)
	if s.active == id {
		if len(s.windows) > 0 {
			for id := range s.windows {
				s.active = id
				break
			}
		} else if s.parent != nil {
			s.parent.Send(SessionEmpty{ID: s.id})
		}
	}
}

// SessionEmpty is sent when the last window is closed
type SessionEmpty struct {
	ID uint32
}
