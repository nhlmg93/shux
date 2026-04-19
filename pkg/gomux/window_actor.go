package gomux

import (
	"os"
	"os/exec"

	"github.com/nhlmg93/gotor/actor"
)

// WindowActor manages multiple panes
type WindowActor struct {
	id       uint32
	panes    map[uint32]*actor.Ref
	active   uint32
	paneID   uint32
	parent   *actor.Ref
	self     *actor.Ref
}

func NewWindowActor(id uint32, parent *actor.Ref) *WindowActor {
	return &WindowActor{
		id:     id,
		panes:  make(map[uint32]*actor.Ref),
		parent: parent,
	}
}

func SpawnWindowActor(id uint32, parent *actor.Ref) *actor.Ref {
	w := NewWindowActor(id, parent)
	ref := actor.Spawn(w, 10)
	w.self = ref
	return ref
}

func (w *WindowActor) Receive(msg any) {
	switch m := msg.(type) {
	case CreatePane:
		w.createPane(m)
	case SwitchToPane:
		w.switchToPane(m.Index)
	case PaneExited:
		w.handlePaneExited(m.ID)
	case PaneOutput:
		// Write pane output to stdout if this is the active pane
		if m.ID == w.active {
			os.Stdout.Write(m.Data)
		}
	case GridUpdated:
		// Forward to parent if from active pane
		if m.ID == w.active && w.parent != nil {
			w.parent.Send(m)
		}
	case ResizeGrid:
		w.handleResizeGrid(m)
	case actor.AskEnvelope:
		w.handleAsk(m)
	}
}

func (w *WindowActor) handleAsk(envelope actor.AskEnvelope) {
	switch m := envelope.Msg.(type) {
	case GetActivePane:
		if w.active != 0 {
			if pane, ok := w.panes[w.active]; ok {
				envelope.Reply <- pane
				return
			}
		}
		envelope.Reply <- nil
	case GetGrid:
		if w.active != 0 {
			if pane, ok := w.panes[w.active]; ok {
				reply := pane.Ask(m)
				grid := <-reply
				envelope.Reply <- grid
				return
			}
		}
		envelope.Reply <- nil
	default:
		envelope.Reply <- nil
	}
}

func (w *WindowActor) createPane(cmd CreatePane) {
	w.paneID++
	paneCmd := exec.Command(cmd.Cmd, cmd.Args...)
	ref, err := SpawnPaneActor(w.paneID, paneCmd, w.self)
	if err != nil {
		return
	}
	w.panes[w.paneID] = ref
	if w.active == 0 {
		w.active = w.paneID
	}
}

func (w *WindowActor) killPane(id uint32) {
	if ref, ok := w.panes[id]; ok {
		ref.Send(KillPane{})
	}
}

func (w *WindowActor) switchToPane(index int) {
	i := 0
	for id := range w.panes {
		if i == index {
			w.active = id
			return
		}
		i++
	}
}

func (w *WindowActor) handleResizeGrid(r ResizeGrid) {
	if w.active != 0 {
		if pane, ok := w.panes[w.active]; ok {
			pane.Send(r)
		}
	}
}

func (w *WindowActor) handlePaneExited(id uint32) {
	delete(w.panes, id)
	if w.active == id {
		if len(w.panes) > 0 {
			for id := range w.panes {
				w.active = id
				break
			}
		} else if w.parent != nil {
			w.parent.Send(WindowEmpty{ID: w.id})
		}
	}
}
