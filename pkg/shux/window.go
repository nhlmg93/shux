package shux

import (
	"github.com/nhlmg93/gotor/actor"
)

type Window struct {
	id        uint32
	panes     map[uint32]*actor.Ref
	paneOrder []uint32
	active    uint32
	paneID    uint32
}

func NewWindow(id uint32) *Window {
	return &Window{
		id:    id,
		panes: make(map[uint32]*actor.Ref),
	}
}

func SpawnWindow(id uint32, parent *actor.Ref) *actor.Ref {
	w := NewWindow(id)
	return actor.SpawnWithParent(w, 32, parent)
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
		if m.ID == 0 || m.ID == w.active {
			if parent := actor.Parent(); parent != nil {
				parent.Send(m)
			}
		}
	case ResizeMsg:
		w.resizeAllPanes(m.Rows, m.Cols)
	case WriteToPane, KeyInput:
		if pane := w.activePane(); pane != nil {
			pane.Send(m)
		}
	case actor.AskEnvelope:
		w.handleAsk(m)
	}
}

func (w *Window) handleAsk(envelope actor.AskEnvelope) {
	switch envelope.Msg.(type) {
	case GetActivePane:
		envelope.Reply <- w.activePane()
	case GetPaneContent:
		if pane := w.activePane(); pane != nil {
			reply := pane.Ask(envelope.Msg)
			envelope.Reply <- <-reply
			return
		}
		envelope.Reply <- nil
	default:
		envelope.Reply <- nil
	}
}

func (w *Window) activePane() *actor.Ref {
	if w.active == 0 {
		return nil
	}
	return w.panes[w.active]
}

func (w *Window) createPane(cmd CreatePane) {
	w.paneID++
	ref := SpawnPane(w.paneID, cmd.Rows, cmd.Cols, cmd.Shell, actor.Self())
	if ref == nil {
		return
	}
	w.panes[w.paneID] = ref
	w.paneOrder = append(w.paneOrder, w.paneID)
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
	if index < 0 || index >= len(w.paneOrder) {
		return
	}
	newActive := w.paneOrder[index]
	if newActive == w.active {
		return
	}
	w.active = newActive
	if parent := actor.Parent(); parent != nil {
		parent.Send(PaneContentUpdated{})
	}
}

func (w *Window) resizeAllPanes(rows, cols int) {
	Infof("window %d: resizing all panes to %dx%d", w.id, rows, cols)
	for _, pane := range w.panes {
		pane.Send(ResizeTerm{Rows: rows, Cols: cols})
	}
}

func (w *Window) handlePaneExited(id uint32) {
	currentIdx := w.activePaneIndex()
	delete(w.panes, id)
	w.paneOrder = removeOrderedID(w.paneOrder, id)

	if len(w.paneOrder) == 0 {
		w.active = 0
		if parent := actor.Parent(); parent != nil {
			parent.Send(WindowEmpty{ID: w.id})
		}
		return
	}

	if w.active != id {
		return
	}

	if currentIdx >= len(w.paneOrder) {
		currentIdx = len(w.paneOrder) - 1
	}
	if currentIdx < 0 {
		currentIdx = 0
	}
	w.active = w.paneOrder[currentIdx]
	if parent := actor.Parent(); parent != nil {
		parent.Send(PaneContentUpdated{})
	}
}

func (w *Window) activePaneIndex() int {
	for i, id := range w.paneOrder {
		if id == w.active {
			return i
		}
	}
	return -1
}
