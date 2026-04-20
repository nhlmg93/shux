package shux

import (
	"github.com/nhlmg93/gotor/actor"
)

type Session struct {
	id       uint32
	windows  map[uint32]*actor.Ref
	active   uint32
	windowID uint32
}

func NewSession(id uint32) *Session {
	return &Session{
		id:      id,
		windows: make(map[uint32]*actor.Ref),
	}
}

func SpawnSession(id uint32, parent *actor.Ref) *actor.Ref {
	s := NewSession(id)
	return actor.SpawnWithParent(s, 10, parent)
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
		if parent := actor.Parent(); parent != nil {
			parent.Send(m)
		}
	case ResizeMsg:
		s.resizeActiveWindow(m.Rows, m.Cols)
	case actor.AskEnvelope:
		s.handleAsk(m)
	}
}

func (s *Session) handleAsk(envelope actor.AskEnvelope) {
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
				reply := win.Ask(GetActivePane{})
				paneRef := <-reply
				envelope.Reply <- paneRef
				return
			}
		}
		envelope.Reply <- nil
	case GetPaneContent:
		if s.active != 0 {
			if win, ok := s.windows[s.active]; ok {
				reply := win.Ask(envelope.Msg)
				content := <-reply
				envelope.Reply <- content
				return
			}
		}
		envelope.Reply <- nil
	default:
		envelope.Reply <- nil
	}
}

func (s *Session) resizeActiveWindow(rows, cols int) {
	Infof("session %d: resizing active window to %dx%d", s.id, rows, cols)
	if s.active != 0 {
		if win, ok := s.windows[s.active]; ok {
			win.Send(ResizeMsg{Rows: rows, Cols: cols})
		}
	}
}

func (s *Session) createWindow(rows, cols int) {
	s.windowID++
	Infof("session %d: creating window %d with size %dx%d", s.id, s.windowID, rows, cols)
	ref := SpawnWindow(s.windowID, actor.Self())
	s.windows[s.windowID] = ref
	ref.Send(CreatePane{Rows: rows, Cols: cols, Shell: "/bin/sh"})
	if s.active == 0 {
		s.active = s.windowID
	}
}

func (s *Session) switchWindow(delta int) {
	if len(s.windows) == 0 {
		return
	}
	ids := make([]uint32, 0, len(s.windows))
	for id := range s.windows {
		ids = append(ids, id)
	}
	currentIdx := 0
	for i, id := range ids {
		if id == s.active {
			currentIdx = i
			break
		}
	}
	newIdx := (currentIdx + delta + len(ids)) % len(ids)
	s.active = ids[newIdx]
}

func (s *Session) handleWindowEmpty(id uint32) {
	delete(s.windows, id)
	
	// If this was the active window, pick a new one or notify parent we're empty
	if s.active == id {
		if len(s.windows) > 0 {
			for id := range s.windows {
				s.active = id
				break
			}
		} else {
			s.active = 0
		}
	}
	
	// Notify parent when we have no windows (regardless of which was active)
	if len(s.windows) == 0 && s.active == 0 {
		if parent := actor.Parent(); parent != nil {
			parent.Send(SessionEmpty{ID: s.id})
		}
	}
}


