package shux

import (
	"fmt"
)

// WindowRef is a reference to a window loop. Methods are promoted from loopRef.
type WindowRef struct {
	*loopRef
}

type Window struct {
	ref       *WindowRef
	logger    ShuxLogger
	parent    *SessionRef
	id        uint32
	panes     map[uint32]*PaneRef
	paneOrder OrderedIDList
	active    uint32
	paneID    uint32

	root             *splitNode
	layout           []paneLayout
	splitDir         SplitDir
	rows             int
	cols             int
	dividerDrag      *dividerHit
	mouseCapturePane uint32
}

func NewWindow(id uint32) *Window {
	return &Window{
		id:    id,
		panes: make(map[uint32]*PaneRef),
	}
}

func StartWindow(id uint32, parent *SessionRef, logger ShuxLogger) *WindowRef {
	w := NewWindow(id)
	w.parent = parent
	w.logger = logger
	ref := &WindowRef{loopRef: newLoopRef(32)}
	w.ref = ref
	go w.run()
	return ref
}

func (w *Window) run() {
	var reason error
	defer func() {
		if r := recover(); r != nil {
			reason = fmt.Errorf("panic: %v\n%s", r, recoverWithContext("window", w.id, len(w.panes), int(w.active)))
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
		if pane == nil {
			continue
		}
		pane.Send(KillPane{})
		pane.Shutdown()
	}
	if reason != nil {
		w.logger.Errorf("window: crash id=%d reason=%v", w.id, reason)
		return
	}
	w.logger.Infof("window: terminate id=%d", w.id)
}

// assertInvariants validates internal state consistency.
// Panics on invariant violation (tiger style - fail fast on bugs).
func (w *Window) assertInvariants() {
	if len(w.paneOrder) != len(w.panes) {
		panic(fmt.Sprintf("window %d: paneOrder length (%d) != panes count (%d)", w.id, len(w.paneOrder), len(w.panes)))
	}

	for _, paneID := range w.paneOrder {
		if _, ok := w.panes[paneID]; !ok {
			panic(fmt.Sprintf("window %d: paneOrder contains missing pane %d", w.id, paneID))
		}
	}

	if len(w.paneOrder) > 0 {
		if w.active == 0 {
			panic(fmt.Sprintf("window %d: active=0 but %d panes exist", w.id, len(w.paneOrder)))
		}
		if _, ok := w.panes[w.active]; !ok {
			panic(fmt.Sprintf("window %d: active pane %d not in panes map", w.id, w.active))
		}
	}

	hasPanes := len(w.paneOrder) > 0
	hasRoot := w.root != nil
	if hasPanes != hasRoot {
		panic(fmt.Sprintf("window %d: root nil=%v but has %d panes", w.id, !hasRoot, len(w.paneOrder)))
	}

	if w.root != nil {
		treePaneIDs := make(map[uint32]struct{})
		collectTreePaneIDs(w.root, treePaneIDs)
		for paneID := range treePaneIDs {
			if _, ok := w.panes[paneID]; !ok {
				panic(fmt.Sprintf("window %d: split tree references missing pane %d", w.id, paneID))
			}
		}
	}
}

func (w *Window) receive(msg any) {
	switch m := msg.(type) {
	case CreatePane:
		w.createPane(m)
	case RestoreWindowLayout:
		w.restoreWindowLayout(m.Root, m.ActivePane)
	case Split:
		w.splitPane(m.Dir)
	case NavigatePane:
		w.navigatePane(m.Dir)
	case ResizePane:
		w.resizePane(m.Dir, m.Amount)
	case ActionMsg:
		w.dispatchAction(m)
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
	case MouseInput:
		w.handleMouseInput(m)
	case WriteToPane, KeyInput:
		if pane := w.activePane(); pane != nil {
			pane.Send(m)
		}
	case askEnvelope:
		w.handleAsk(m)
	}
}

// dispatchAction handles pane-scoped actions forwarded from session.
func (w *Window) dispatchAction(msg ActionMsg) {
	switch msg.Action {
	case ActionKillPane:
		if pane := w.activePane(); pane != nil {
			pane.Send(KillPane{})
		}
	case ActionZoomPane:

		w.logger.Infof("window %d: zoom pane requested (not yet implemented)", w.id)
	case ActionSwapPaneUp:

		currentIdx := w.paneOrder.IndexOf(w.active)
		if currentIdx > 0 {
			w.paneOrder[currentIdx], w.paneOrder[currentIdx-1] = w.paneOrder[currentIdx-1], w.paneOrder[currentIdx]
			w.syncLayout()
			if w.parent != nil {
				w.parent.Send(PaneContentUpdated{})
			}
		}
	case ActionSwapPaneDown:

		currentIdx := w.paneOrder.IndexOf(w.active)
		if currentIdx >= 0 && currentIdx < len(w.paneOrder)-1 {
			w.paneOrder[currentIdx], w.paneOrder[currentIdx+1] = w.paneOrder[currentIdx+1], w.paneOrder[currentIdx]
			w.syncLayout()
			if w.parent != nil {
				w.parent.Send(PaneContentUpdated{})
			}
		}
	case ActionRenameWindow:

		w.logger.Infof("window %d: rename requested (not yet implemented)", w.id)
	default:

		w.logger.Warnf("window %d: unknown action %q", w.id, msg.Action)
	}
}

func (w *Window) handleAsk(envelope askEnvelope) {
	switch envelope.msg.(type) {
	case GetActivePane:
		if pane := w.activePane(); pane != nil {
			envelope.reply <- pane
			return
		}
		envelope.reply <- nil
	case GetPaneContent:
		if pane := w.activePane(); pane != nil {
			result, _ := askValue(pane, envelope.msg)
			envelope.reply <- result
			return
		}
		envelope.reply <- nil
	case GetWindowView:
		envelope.reply <- w.buildWindowView()
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
		PaneOrder:  w.paneOrder.Clone(),
		Panes:      make([]PaneSnapshot, 0, len(w.paneOrder)),
		Layout:     snapshotSplitTree(w.root),
	}

	for _, paneID := range w.paneOrder {
		paneRef, ok := w.panes[paneID]
		if !ok {
			continue
		}

		result, _ := askValue(paneRef, GetPaneSnapshotData{})
		paneData, ok := result.(PaneSnapshotData)
		if !ok {
			continue
		}

		snapshot.Panes = append(snapshot.Panes, PaneSnapshot(paneData))
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

	ref := StartPane(paneID, cmd.Rows, cmd.Cols, cmd.Shell, cmd.CWD, w.ref, w.logger)
	w.panes[paneID] = ref
	w.paneOrder.Add(paneID)

	if w.active == 0 {
		w.active = paneID
		w.splitDir = SplitH
		w.rows = cmd.Rows
		w.cols = cmd.Cols
		w.root = leafNode(paneID)
	} else {
		w.root, _ = splitAroundPane(w.root, w.active, w.splitDir, paneID)
		w.active = paneID
	}

	w.syncLayout()
	if w.parent != nil {
		w.parent.Send(PaneContentUpdated{})
	}
}

func (w *Window) splitPane(dir SplitDir) {
	if w.active == 0 || len(w.paneOrder) == 0 {
		return
	}

	w.splitDir = dir

	var (
		shell string
		cwd   string
	)
	if active := w.activePane(); active != nil {
		result, _ := askValue(active, GetPaneSnapshotData{})
		if snap, ok := result.(PaneSnapshotData); ok {
			shell = snap.Shell
			cwd = snap.CWD
		}
	}

	w.paneID++
	newPaneID := w.paneID
	newRef := StartPane(newPaneID, w.rows, w.cols, shell, cwd, w.ref, w.logger)
	w.panes[newPaneID] = newRef
	w.paneOrder.Add(newPaneID)
	w.root, _ = splitAroundPane(w.root, w.active, dir, newPaneID)
	w.active = newPaneID

	w.syncLayout()
	if w.parent != nil {
		w.parent.Send(PaneContentUpdated{})
	}
	w.assertInvariants()
}

func (w *Window) handlePaneExited(id uint32) {
	if _, ok := w.panes[id]; !ok {
		panic(fmt.Sprintf("window %d: handlePaneExited called for missing pane %d", w.id, id))
	}

	currentIdx := w.paneOrder.IndexOf(w.active)
	if w.mouseCapturePane == id {
		w.mouseCapturePane = 0
	}
	w.dividerDrag = nil
	delete(w.panes, id)
	w.paneOrder.Remove(id)

	w.root, _ = removePaneNode(w.root, id)

	if len(w.paneOrder) == 0 {
		w.active = 0
		w.layout = nil
		w.root = nil
		if w.parent != nil {
			w.parent.Send(WindowEmpty{ID: w.id})
		}
		w.assertInvariants()
		return
	}

	if w.active == id {
		if currentIdx >= len(w.paneOrder) {
			currentIdx = len(w.paneOrder) - 1
		}
		if currentIdx < 0 {
			currentIdx = 0
		}
		w.active = w.paneOrder[currentIdx]
	}

	w.syncLayout()
	if w.parent != nil {
		w.parent.Send(PaneContentUpdated{})
	}
	w.assertInvariants()
}
