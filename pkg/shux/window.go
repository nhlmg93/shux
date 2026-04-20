package shux

import (
	"github.com/nhlmg93/gotor/actor"
)

type Window struct {
	id     uint32
	panes  map[uint32]*actor.Ref
	active uint32
	paneID uint32
}

func NewWindow(id uint32) *Window {
	return &Window{
		id:    id,
		panes: make(map[uint32]*actor.Ref),
	}
}

func SpawnWindow(id uint32, parent *actor.Ref) *actor.Ref {
	w := NewWindow(id)
	return actor.SpawnWithParent(w, 10, parent)
}

func (w *Window) Receive(msg any) {
	switch m := msg.(type) {
	case CreatePane:
		w.createPane(m)
	case SwitchToPane:
		w.switchToPane(m.Index)
	case PaneExited:
		w.handlePaneExited(m.ID)
	case PaneContentUpdated:
		if m.ID == w.active {
			if parent := actor.Parent(); parent != nil {
				parent.Send(m)
			}
		}
	case ResizeMsg:
		w.resizeAllPanes(m.Rows, m.Cols)
	case actor.AskEnvelope:
		w.handleAsk(m)
	}
}

func (w *Window) handleAsk(envelope actor.AskEnvelope) {
	switch envelope.Msg.(type) {
	case GetActivePane:
		if w.active != 0 {
			if pane, ok := w.panes[w.active]; ok {
				envelope.Reply <- pane
				return
			}
		}
		envelope.Reply <- nil
	case GetPaneContent:
		if w.active != 0 {
			if pane, ok := w.panes[w.active]; ok {
				reply := pane.Ask(envelope.Msg)
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

func (w *Window) createPane(cmd CreatePane) {
	w.paneID++
	ref := SpawnPane(w.paneID, cmd.Rows, cmd.Cols, cmd.Shell, actor.Self())
	if ref == nil {
		return
	}
	w.panes[w.paneID] = ref
	if w.active == 0 {
		w.active = w.paneID
	}
}

func (w *Window) killPane(id uint32) {
	if ref, ok := w.panes[id]; ok {
		ref.Send(KillPane{})
	}
}

func (w *Window) switchToPane(index int) {
	i := 0
	for id := range w.panes {
		if i == index {
			w.active = id
			return
		}
		i++
	}
}

func (w *Window) resizeAllPanes(rows, cols int) {
	Infof("window %d: resizing all panes to %dx%d", w.id, rows, cols)
	for _, pane := range w.panes {
		pane.Send(ResizeTerm{Rows: rows, Cols: cols})
	}
}

func (w *Window) handlePaneExited(id uint32) {
	delete(w.panes, id)
	if w.active == id {
		if len(w.panes) > 0 {
			for id := range w.panes {
				w.active = id
				break
			}
		} else if parent := actor.Parent(); parent != nil {
			parent.Send(WindowEmpty{ID: w.id})
		}
	}
}
