package gomux

import (
	"github.com/nhlmg93/gotor/actor"
)

// WindowActor manages multiple Terms (panes)
type WindowActor struct {
	id     uint32
	terms  map[uint32]*actor.Ref
	active uint32
	termID uint32
}

func NewWindowActor(id uint32) *WindowActor {
	return &WindowActor{
		id:    id,
		terms: make(map[uint32]*actor.Ref),
	}
}

func SpawnWindowActor(id uint32, parent *actor.Ref) *actor.Ref {
	w := NewWindowActor(id)
	return actor.SpawnWithParent(w, 10, parent)
}

func (w *WindowActor) Receive(msg any) {
	switch m := msg.(type) {
	case CreateTerm:
		w.createTerm(m)
	case SwitchToTerm:
		w.switchToTerm(m.Index)
	case TermExited:
		w.handleTermExited(m.ID)
	case GridUpdated:
		// Forward to parent if from active term
		if m.ID == w.active {
			if parent := actor.Parent(); parent != nil {
				parent.Send(m)
			}
		}
	case ResizeMsg:
		// Resize all terms in this window
		w.resizeAllTerms(m.Rows, m.Cols)
	case actor.AskEnvelope:
		w.handleAsk(m)
	}
}

func (w *WindowActor) handleAsk(envelope actor.AskEnvelope) {
	switch m := envelope.Msg.(type) {
	case GetActiveTerm:
		if w.active != 0 {
			if term, ok := w.terms[w.active]; ok {
				envelope.Reply <- term
				return
			}
		}
		envelope.Reply <- nil
	case GetTermContent:
		if w.active != 0 {
			if term, ok := w.terms[w.active]; ok {
				reply := term.Ask(m)
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

func (w *WindowActor) createTerm(cmd CreateTerm) {
	w.termID++
	ref := Spawn(w.termID, cmd.Rows, cmd.Cols, cmd.Shell, actor.Self())
	if ref == nil {
		return
	}
	w.terms[w.termID] = ref
	if w.active == 0 {
		w.active = w.termID
	}
}

func (w *WindowActor) killTerm(id uint32) {
	if ref, ok := w.terms[id]; ok {
		ref.Send(KillTerm{})
	}
}

func (w *WindowActor) switchToTerm(index int) {
	i := 0
	for id := range w.terms {
		if i == index {
			w.active = id
			return
		}
		i++
	}
}

func (w *WindowActor) resizeAllTerms(rows, cols int) {
	Infof("window %d: resizing all terms to %dx%d", w.id, rows, cols)
	for _, term := range w.terms {
		term.Send(ResizeTerm{Rows: rows, Cols: cols})
	}
}

func (w *WindowActor) handleTermExited(id uint32) {
	delete(w.terms, id)
	if w.active == id {
		if len(w.terms) > 0 {
			for id := range w.terms {
				w.active = id
				break
			}
		} else if parent := actor.Parent(); parent != nil {
			parent.Send(WindowEmpty{ID: w.id})
		}
	}
}
