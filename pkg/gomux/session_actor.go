package gomux

import (
	"github.com/nhlmg93/gotor/actor"
)

// SessionActor manages multiple windows
type SessionActor struct {
	id       uint32
	windows  map[uint32]*actor.Ref
	active   uint32
	windowID uint32
}

func NewSessionActor(id uint32) *SessionActor {
	return &SessionActor{
		id:      id,
		windows: make(map[uint32]*actor.Ref),
	}
}

func SpawnSessionActor(id uint32, parent *actor.Ref) *actor.Ref {
	s := NewSessionActor(id)
	return actor.SpawnWithParent(s, 10, parent)
}

func (s *SessionActor) Receive(msg any) {
	switch m := msg.(type) {
	case CreateWindow:
		s.createWindow(m.Rows, m.Cols)
	case SwitchWindow:
		s.switchWindow(m.Delta)
	case WindowEmpty:
		s.handleWindowEmpty(m.ID)
	case GridUpdated:
		// Forward to parent (Supervisor) to notify UI
		if parent := actor.Parent(); parent != nil {
			parent.Send(m)
		}
	case ResizeMsg:
		// Forward resize to active window
		s.resizeActiveWindow(m.Rows, m.Cols)
	case actor.AskEnvelope:
		s.handleAsk(m)
	}
}

func (s *SessionActor) handleAsk(envelope actor.AskEnvelope) {
	switch m := envelope.Msg.(type) {
	case GetActiveWindow:
		if s.active != 0 {
			if win, ok := s.windows[s.active]; ok {
				envelope.Reply <- win
				return
			}
		}
		envelope.Reply <- nil
	case GetActiveTerm:
		if s.active != 0 {
			if win, ok := s.windows[s.active]; ok {
				reply := win.Ask(GetActiveTerm{})
				termRef := <-reply
				envelope.Reply <- termRef
				return
			}
		}
		envelope.Reply <- nil
	case GetTermContent:
		if s.active != 0 {
			if win, ok := s.windows[s.active]; ok {
				reply := win.Ask(m)
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

func (s *SessionActor) resizeActiveWindow(rows, cols int) {
	Infof("session %d: resizing active window to %dx%d", s.id, rows, cols)
	if s.active != 0 {
		if win, ok := s.windows[s.active]; ok {
			win.Send(ResizeMsg{Rows: rows, Cols: cols})
		}
	}
}

func (s *SessionActor) createWindow(rows, cols int) {
	s.windowID++
	Infof("session %d: creating window %d with size %dx%d", s.id, s.windowID, rows, cols)
	ref := SpawnWindowActor(s.windowID, actor.Self())
	s.windows[s.windowID] = ref
	// Create initial term with actual window size
	ref.Send(CreateTerm{Rows: rows, Cols: cols, Shell: "/bin/sh"})
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
		} else if parent := actor.Parent(); parent != nil {
			parent.Send(SessionEmpty{ID: s.id})
		}
	}
}

// SessionEmpty is sent when the last window is closed
type SessionEmpty struct {
	ID uint32
}
