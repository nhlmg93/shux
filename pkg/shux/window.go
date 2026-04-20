package shux

import "fmt"

type WindowRef struct {
	*loopRef
}

func (r *WindowRef) Send(msg any) bool {
	if r == nil {
		return false
	}
	return r.send(msg)
}

func (r *WindowRef) Ask(msg any) chan any {
	if r == nil {
		return nil
	}
	return r.ask(msg)
}

func (r *WindowRef) Stop() {
	if r != nil {
		r.stopLoop()
	}
}

func (r *WindowRef) Shutdown() {
	if r != nil {
		r.shutdown()
	}
}

type Window struct {
	ref       *WindowRef
	parent    *SessionRef
	id        uint32
	panes     map[uint32]*PaneRef
	paneOrder []uint32
	active    uint32
	paneID    uint32
}

func NewWindow(id uint32) *Window {
	return &Window{
		id:    id,
		panes: make(map[uint32]*PaneRef),
	}
}

func StartWindow(id uint32, parent *SessionRef) *WindowRef {
	w := NewWindow(id)
	w.parent = parent
	ref := &WindowRef{loopRef: newLoopRef(32)}
	w.ref = ref
	go w.run()
	return ref
}

func (w *Window) run() {
	var reason error
	defer func() {
		if r := recover(); r != nil {
			reason = fmt.Errorf("panic: %v", r)
		}
		w.terminate(reason)
		close(w.ref.done)
	}()

	for {
		select {
		case <-w.ref.stop:
			return
		case msg := <-w.ref.inbox:
			w.receive(msg)
		}
	}
}

func (w *Window) terminate(reason error) {
	for _, pane := range w.panes {
		if pane != nil {
			pane.Shutdown()
		}
	}
	if reason != nil {
		Errorf("window: crash id=%d reason=%v", w.id, reason)
		return
	}
	Infof("window: terminate id=%d", w.id)
}

func (w *Window) receive(msg any) {
	switch m := msg.(type) {
	case CreatePane:
		w.createPane(m)
	case SwitchToPane:
		w.switchToPane(m.Index)
	case PaneExited:
		w.handlePaneExited(m.ID)
	case PaneContentUpdated:
		if m.ID == 0 || m.ID == w.active {
			if w.parent != nil {
				w.parent.Send(m)
			}
		}
	case ResizeMsg:
		w.resizeAllPanes(m.Rows, m.Cols)
	case WriteToPane, KeyInput:
		if pane := w.activePane(); pane != nil {
			pane.Send(m)
		}
	case askEnvelope:
		w.handleAsk(m)
	}
}

func (w *Window) handleAsk(envelope askEnvelope) {
	switch envelope.msg.(type) {
	case GetActivePane:
		envelope.reply <- w.activePane()
	case GetPaneContent:
		if pane := w.activePane(); pane != nil {
			result, _ := askValue(pane, envelope.msg)
			envelope.reply <- result
			return
		}
		envelope.reply <- nil
	case GetWindowSnapshotData:
		envelope.reply <- w.gatherSnapshotData()
	default:
		envelope.reply <- nil
	}
}

func (w *Window) gatherSnapshotData() WindowSnapshot {
	snapshot := WindowSnapshot{
		ID:         w.id,
		ActivePane: w.active,
		PaneOrder:  append([]uint32(nil), w.paneOrder...),
		Panes:      make([]PaneSnapshot, 0, len(w.paneOrder)),
	}

	for _, paneID := range w.paneOrder {
		paneRef, ok := w.panes[paneID]
		if !ok {
			continue
		}

		result, ok := askValue(paneRef, GetPaneSnapshotData{})
		paneData, ok := result.(PaneSnapshotData)
		if !ok {
			continue
		}

		snapshot.Panes = append(snapshot.Panes, PaneSnapshot{
			ID:          paneData.ID,
			Shell:       paneData.Shell,
			Rows:        paneData.Rows,
			Cols:        paneData.Cols,
			CWD:         paneData.CWD,
			WindowTitle: paneData.WindowTitle,
		})
	}

	return snapshot
}

func (w *Window) activePane() *PaneRef {
	if w.active == 0 {
		return nil
	}
	return w.panes[w.active]
}

func (w *Window) createPane(cmd CreatePane) {
	paneID := cmd.ID
	if paneID == 0 {
		w.paneID++
		paneID = w.paneID
	} else if paneID > w.paneID {
		w.paneID = paneID
	}

	ref := StartPane(paneID, cmd.Rows, cmd.Cols, cmd.Shell, cmd.CWD, w.ref)
	w.panes[paneID] = ref
	w.paneOrder = append(w.paneOrder, paneID)
	if w.active == 0 {
		w.active = paneID
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
	if w.parent != nil {
		w.parent.Send(PaneContentUpdated{})
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
		if w.parent != nil {
			w.parent.Send(WindowEmpty{ID: w.id})
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
	if w.parent != nil {
		w.parent.Send(PaneContentUpdated{})
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
