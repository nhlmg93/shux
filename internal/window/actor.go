package window

import (
	"context"
	"fmt"
	"strconv"

	"shux/internal/actor"
	"shux/internal/cfg"
	"shux/internal/pane"
	"shux/internal/protocol"
)

// Panes is keyed pane lifecycle bookkeeping (Init / Delete / Must on command refs).
type Panes = *actor.Lifecycle[protocol.PaneID, protocol.Command]

type Actor struct {
	Panes
	SessionID     protocol.SessionID
	WindowID      protocol.WindowID
	WindowOrdinal int
	Name          string
	Policy        cfg.Config
	SessionRef    actor.Ref[protocol.Command]
	Layout        Layout
	PaneNames     map[protocol.PaneID]string
	paneIDs       []protocol.PaneID
	hub           actor.EventRef
	seq           uint64
	revision      uint64
	zoomedPaneID  protocol.PaneID
	savedLayout   *Layout
}

type PaneTransfer struct {
	PaneID protocol.PaneID
	Ref    actor.Ref[protocol.Command]
}

type DetachPaneResult struct {
	Transfer PaneTransfer
	Err      error
}

// CommandDetachPane asks a window to remove a pane from its layout without
// closing the pane actor, then return ownership to the session actor.
type CommandDetachPane struct {
	SessionID protocol.SessionID
	WindowID  protocol.WindowID
	PaneID    protocol.PaneID
	Reply     chan<- DetachPaneResult
}

// CommandAttachPane asks a window to attach an already-running pane actor.
// If TargetPaneID is empty, the window selects a default target.
type CommandAttachPane struct {
	SessionID    protocol.SessionID
	WindowID     protocol.WindowID
	TargetPaneID protocol.PaneID
	Direction    protocol.SplitDirection
	Transfer     PaneTransfer
	Reply        chan<- error
}

func NewActor(hub actor.EventRef) *Actor {
	return NewActorWithPolicy(actor.Ref[protocol.Command]{}, hub, "", "", 1, cfg.DefaultConfig())
}

func NewActorWithPolicy(sessionRef actor.Ref[protocol.Command], hub actor.EventRef, sessionID protocol.SessionID, windowID protocol.WindowID, windowOrdinal int, policy cfg.Config) *Actor {
	if hub != nil && !hub.Valid() {
		panic("window: NewActor: invalid hub ref")
	}
	if windowOrdinal <= 0 {
		panic("window: NewActor: invalid window ordinal")
	}
	policy = policy.WithDefaults()
	if policy.ShellPath == "" {
		panic("window: NewActor: empty shell path")
	}
	return &Actor{
		Panes:         actor.NewLifecycle[protocol.PaneID, protocol.Command]("window", "pane", protocol.PaneID.Valid),
		SessionID:     sessionID,
		WindowID:      windowID,
		WindowOrdinal: windowOrdinal,
		Policy:        policy,
		SessionRef:    sessionRef,
		Layout:        NewLayout(80, 24),
		PaneNames:     make(map[protocol.PaneID]string),
		hub:           hub,
	}
}

func (a *Actor) Run(ctx context.Context, _ actor.Ref[protocol.Command], inbox <-chan protocol.Command) {
	for {
		select {
		case <-ctx.Done():
			return
		case msg := <-inbox:
			switch m := msg.(type) {
			case CommandDetachPane:
				a.handleDetachPane(ctx, m)
				continue
			case CommandAttachPane:
				a.handleAttachPane(ctx, m)
				continue
			}
			if err := protocol.ValidateCommand(msg); err != nil {
				panic(err)
			}
			switch m := msg.(type) {
			case protocol.CommandNoop:
				continue
			case protocol.CommandCreatePane:
				a.handleCreatePane(ctx, m)
			case protocol.CommandWindowResize:
				a.handleWindowResize(ctx, m)
			case protocol.CommandPaneSplit:
				a.handlePaneSplit(ctx, m)
			case protocol.CommandPaneFocus:
				a.handlePaneFocus(ctx, m)
			case protocol.CommandPaneResizeDelta:
				a.handlePaneResizeDelta(ctx, m)
			case protocol.CommandWindowSelectLayout:
				a.handleWindowSelectLayout(ctx, m)
			case protocol.CommandPaneSwap:
				a.handlePaneSwap(ctx, m)
			case protocol.CommandPaneClose:
				a.handlePaneClose(ctx, m)
			case protocol.CommandPaneZoomToggle:
				a.handlePaneZoomToggle(ctx, m)
			case protocol.CommandWindowRename:
				a.handleWindowRename(ctx, m)
			case protocol.CommandPaneRename:
				a.handlePaneRename(ctx, m)
			default:
				if pid, ok := protocol.RoutePaneID(msg); ok {
					_ = a.Panes.Must(pid).Send(ctx, msg)
					continue
				}
				panic(fmt.Sprintf("window: unhandled command type %T", msg))
			}
		}
	}
}

func (a *Actor) handleDetachPane(ctx context.Context, m CommandDetachPane) {
	if m.Reply == nil {
		panic("window: detach pane: nil reply channel")
	}
	ref := a.Panes.Must(m.PaneID)
	if err := a.Layout.RemovePane(m.PaneID); err != nil {
		m.Reply <- DetachPaneResult{Err: fmt.Errorf("window: detach pane: %w", err)}
		return
	}
	a.Panes.Delete(m.PaneID)
	for i, pid := range a.paneIDs {
		if pid == m.PaneID {
			a.paneIDs = append(a.paneIDs[:i], a.paneIDs[i+1:]...)
			break
		}
	}
	a.revision++
	a.emit(ctx, protocol.EventPaneClosed{SessionID: m.SessionID, WindowID: m.WindowID, PaneID: m.PaneID})
	if len(a.Layout.PaneIDs()) == 0 {
		if err := a.SessionRef.Send(ctx, protocol.CommandWindowClosed{
			SessionID: m.SessionID,
			WindowID:  m.WindowID,
		}); err != nil && ctx.Err() == nil {
			panic(fmt.Sprintf("window: detach notify session close %s: %v", m.WindowID, err))
		}
		a.emit(ctx, protocol.EventWindowClosed{SessionID: m.SessionID, WindowID: m.WindowID})
	} else {
		for _, pid := range a.Layout.PaneIDs() {
			a.sendPaneGeometry(ctx, m.SessionID, m.WindowID, pid, a.mustRect(pid, "PaneDetach"), false)
		}
		a.emitLayout(ctx, m.SessionID, m.WindowID)
	}
	m.Reply <- DetachPaneResult{
		Transfer: PaneTransfer{
			PaneID: m.PaneID,
			Ref:    ref,
		},
	}
}

func (a *Actor) handleAttachPane(ctx context.Context, m CommandAttachPane) {
	if m.Reply == nil {
		panic("window: attach pane: nil reply channel")
	}
	if !m.Transfer.PaneID.Valid() || !m.Transfer.Ref.Valid() {
		m.Reply <- fmt.Errorf("window: attach pane: invalid transfer")
		return
	}
	if len(a.Layout.PaneIDs()) == 0 {
		if err := a.Layout.SetSinglePane(m.Transfer.PaneID); err != nil {
			m.Reply <- fmt.Errorf("window: attach pane single: %w", err)
			return
		}
	} else {
		target := m.TargetPaneID
		if !target.Valid() {
			target = a.Layout.PaneIDs()[0]
		}
		dir := m.Direction
		if !dir.Valid() {
			dir = protocol.SplitVertical
		}
		if err := a.Layout.SplitPane(target, dir, m.Transfer.PaneID); err != nil {
			m.Reply <- fmt.Errorf("window: attach pane split: %w", err)
			return
		}
	}
	a.Init(m.Transfer.PaneID, m.Transfer.Ref)
	a.paneIDs = append(a.paneIDs, m.Transfer.PaneID)
	a.revision++
	if err := m.Transfer.Ref.Send(ctx, pane.CommandRehome{
		SessionID: m.SessionID,
		WindowID:  m.WindowID,
	}); err != nil && ctx.Err() == nil {
		panic(fmt.Sprintf("window: rehome pane %s: %v", m.Transfer.PaneID, err))
	}
	for _, pid := range a.Layout.PaneIDs() {
		a.sendPaneGeometry(ctx, m.SessionID, m.WindowID, pid, a.mustRect(pid, "PaneAttach"), false)
	}
	a.emitLayout(ctx, m.SessionID, m.WindowID)
	m.Reply <- nil
}

func (a *Actor) handleCreatePane(ctx context.Context, m protocol.CommandCreatePane) {
	if len(a.paneIDs) > 0 {
		panic("window: second pane: use CommandPaneSplit (layout is single-or-split)")
	}
	pid := a.nextPaneID()
	a.paneIDs = append(a.paneIDs, pid)
	a.PaneNames[pid] = ""
	a.Init(pid, pane.StartWithPolicy(ctx, a.hub, m.SessionID, m.WindowID, a.WindowOrdinal, pid, a.Policy))
	a.emit(ctx, protocol.EventPaneCreated{SessionID: m.SessionID, WindowID: m.WindowID, PaneID: pid})
	if err := a.Layout.SetSinglePane(pid); err != nil {
		panic(fmt.Sprintf("window: SetSinglePane after CreatePane: %v", err))
	}
	a.revision++
	a.sendPaneGeometry(ctx, m.SessionID, m.WindowID, pid, a.mustRect(pid, "CreatePane"), true)
	a.emitLayout(ctx, m.SessionID, m.WindowID)
}

func (a *Actor) handleWindowResize(ctx context.Context, m protocol.CommandWindowResize) {
	if a.savedLayout != nil {
		saved := a.savedLayout.Clone()
		if err := saved.SetWindowSize(m.Cols, m.Rows); err != nil {
			return
		}
		if err := a.Layout.SetWindowSize(m.Cols, m.Rows); err != nil {
			return
		}
		*a.savedLayout = saved
	} else {
		if err := a.Layout.SetWindowSize(m.Cols, m.Rows); err != nil {
			return
		}
	}
	a.revision++
	for _, pid := range a.Layout.PaneIDs() {
		a.sendPaneGeometry(ctx, m.SessionID, m.WindowID, pid, a.mustRect(pid, "WindowResize"), false)
	}
	a.emitLayout(ctx, m.SessionID, m.WindowID)
}

func (a *Actor) handlePaneSplit(ctx context.Context, m protocol.CommandPaneSplit) {
	if a.restoreZoomLayout(ctx, m.SessionID, m.WindowID) {
		// Split while zoomed deterministically restores first, then applies split.
	}
	if err := a.Layout.CanSplitPane(m.TargetPaneID, m.Direction); err != nil {
		a.rejectSplit(ctx, m, err.Error())
		return
	}
	newID := a.nextPaneID()
	if err := a.Layout.SplitPane(m.TargetPaneID, m.Direction, newID); err != nil {
		a.rejectSplit(ctx, m, err.Error())
		return
	}
	a.revision++
	a.paneIDs = append(a.paneIDs, newID)
	a.PaneNames[newID] = ""
	a.Init(newID, pane.StartWithPolicy(ctx, a.hub, m.SessionID, m.WindowID, a.WindowOrdinal, newID, a.Policy))
	a.emit(ctx, protocol.EventPaneCreated{SessionID: m.SessionID, WindowID: m.WindowID, PaneID: newID})
	for _, pid := range a.Layout.PaneIDs() {
		r := a.mustRect(pid, "PaneSplit")
		a.sendPaneGeometry(ctx, m.SessionID, m.WindowID, pid, r, pid == newID)
	}
	a.emitLayout(ctx, m.SessionID, m.WindowID)
	a.emit(ctx, protocol.EventPaneSplitCompleted{
		ClientID:     m.Meta.ClientID,
		RequestID:    m.Meta.RequestID,
		SessionID:    m.SessionID,
		WindowID:     m.WindowID,
		TargetPaneID: m.TargetPaneID,
		NewPaneID:    newID,
		Revision:     a.revision,
	})
}

func (a *Actor) handlePaneClose(ctx context.Context, m protocol.CommandPaneClose) {
	if a.restoreZoomLayout(ctx, m.SessionID, m.WindowID) {
		// Close while zoomed deterministically restores first, then closes pane.
	}
	lastPane := len(a.Layout.PaneIDs()) <= 1
	if err := a.Layout.RemovePane(m.PaneID); err != nil {
		panic("window: close pane: " + err.Error())
	}
	_ = a.Panes.Must(m.PaneID).Send(ctx, m)
	a.Panes.Delete(m.PaneID)
	delete(a.PaneNames, m.PaneID)
	for i, pid := range a.paneIDs {
		if pid == m.PaneID {
			a.paneIDs = append(a.paneIDs[:i], a.paneIDs[i+1:]...)
			break
		}
	}
	a.revision++
	a.emit(ctx, protocol.EventPaneClosed{SessionID: m.SessionID, WindowID: m.WindowID, PaneID: m.PaneID})
	if lastPane {
		if err := a.SessionRef.Send(ctx, protocol.CommandWindowClosed{
			SessionID: m.SessionID,
			WindowID:  m.WindowID,
		}); err != nil && ctx.Err() == nil {
			panic(fmt.Sprintf("window: notify session close %s: %v", m.WindowID, err))
		}
		a.emit(ctx, protocol.EventWindowClosed{SessionID: m.SessionID, WindowID: m.WindowID})
		return
	}
	for _, pid := range a.Layout.PaneIDs() {
		a.sendPaneGeometry(ctx, m.SessionID, m.WindowID, pid, a.mustRect(pid, "PaneClose"), false)
	}
	a.emitLayout(ctx, m.SessionID, m.WindowID)
}

func (a *Actor) handlePaneResizeDelta(ctx context.Context, m protocol.CommandPaneResizeDelta) {
	if err := a.Layout.ResizePaneDelta(m.TargetPaneID, m.Edge, m.Delta); err != nil {
		a.rejectCommand(ctx, m.Meta, m.SessionID, m.WindowID, "pane-resize-delta", err.Error())
		return
	}
	a.revision++
	for _, pid := range a.Layout.PaneIDs() {
		a.sendPaneGeometry(ctx, m.SessionID, m.WindowID, pid, a.mustRect(pid, "PaneResizeDelta"), false)
	}
	a.emitLayout(ctx, m.SessionID, m.WindowID)
}

func (a *Actor) handleWindowSelectLayout(ctx context.Context, m protocol.CommandWindowSelectLayout) {
	if err := a.Layout.ApplyPreset(m.ActivePaneID, m.Preset); err != nil {
		a.rejectLayoutPreset(ctx, m, err.Error())
		return
	}
	a.revision++
	for _, pid := range a.Layout.PaneIDs() {
		a.sendPaneGeometry(ctx, m.SessionID, m.WindowID, pid, a.mustRect(pid, "WindowSelectLayout"), false)
	}
	a.emitLayout(ctx, m.SessionID, m.WindowID)
}

func (a *Actor) handlePaneSwap(ctx context.Context, m protocol.CommandPaneSwap) {
	if _, err := a.Layout.SwapPaneByDirection(m.PaneID, m.Direction); err != nil {
		a.rejectPaneSwap(ctx, m, err.Error())
		return
	}
	a.revision++
	for _, pid := range a.Layout.PaneIDs() {
		a.sendPaneGeometry(ctx, m.SessionID, m.WindowID, pid, a.mustRect(pid, "PaneSwap"), false)
	}
	a.emitLayout(ctx, m.SessionID, m.WindowID)
}

func (a *Actor) handlePaneFocus(ctx context.Context, m protocol.CommandPaneFocus) {
	target := m.TargetPaneID
	if m.Direction.Valid() {
		if _, ok := a.Layout.Rect(m.CurrentPaneID); !ok {
			a.rejectFocus(ctx, m, "current pane missing")
			return
		}
		next, ok := a.Layout.FocusTarget(m.CurrentPaneID, m.Direction)
		if !ok {
			target = m.CurrentPaneID
		} else {
			target = next
		}
	}
	if _, ok := a.Layout.Rect(target); !ok {
		a.rejectFocus(ctx, m, "target pane missing")
		return
	}
	a.emit(ctx, protocol.EventPaneFocusResolved{
		ClientID:  m.Meta.ClientID,
		RequestID: m.Meta.RequestID,
		SessionID: m.SessionID,
		WindowID:  m.WindowID,
		PaneID:    target,
	})
}

func (a *Actor) handlePaneZoomToggle(ctx context.Context, m protocol.CommandPaneZoomToggle) {
	if a.zoomedPaneID.Valid() && m.PaneID == a.zoomedPaneID {
		_ = a.restoreZoomLayout(ctx, m.SessionID, m.WindowID)
		return
	}
	if a.zoomedPaneID.Valid() {
		if a.savedLayout == nil || !a.savedLayout.hasLeaf(m.PaneID) {
			return
		}
		// Switching zoom target restores from saved layout, then zooms target.
		a.Layout = a.savedLayout.Clone()
	} else {
		if !a.Layout.hasLeaf(m.PaneID) {
			return
		}
		saved := a.Layout.Clone()
		a.savedLayout = &saved
	}
	if err := a.Layout.SetSinglePane(m.PaneID); err != nil {
		panic(fmt.Sprintf("window: zoom pane %s: %v", m.PaneID, err))
	}
	a.zoomedPaneID = m.PaneID
	a.revision++
	a.sendPaneGeometry(ctx, m.SessionID, m.WindowID, m.PaneID, a.mustRect(m.PaneID, "PaneZoomToggle"), false)
	a.emitLayout(ctx, m.SessionID, m.WindowID)
}

func (a *Actor) restoreZoomLayout(ctx context.Context, sid protocol.SessionID, wid protocol.WindowID) bool {
	if !a.zoomedPaneID.Valid() {
		return false
	}
	if a.savedLayout != nil {
		a.Layout = a.savedLayout.Clone()
	}
	a.savedLayout = nil
	a.zoomedPaneID = ""
	a.revision++
	for _, pid := range a.Layout.PaneIDs() {
		a.sendPaneGeometry(ctx, sid, wid, pid, a.mustRect(pid, "PaneZoomRestore"), false)
	}
	a.emitLayout(ctx, sid, wid)
	return true
}

func (a *Actor) handleWindowRename(ctx context.Context, m protocol.CommandWindowRename) {
	a.Name = m.Name
	a.emit(ctx, protocol.EventWindowRenamed{
		SessionID: m.SessionID,
		WindowID:  m.WindowID,
		Name:      m.Name,
	})
}

func (a *Actor) handlePaneRename(ctx context.Context, m protocol.CommandPaneRename) {
	a.PaneNames[m.PaneID] = m.Name
	_ = a.Panes.Must(m.PaneID).Send(ctx, m)
}

func (a *Actor) nextPaneID() protocol.PaneID {
	a.seq++
	return protocol.PaneID("p-" + strconv.FormatUint(a.seq, 10))
}

func (a *Actor) mustRect(pid protocol.PaneID, op string) Rect {
	r, ok := a.Layout.Rect(pid)
	if !ok {
		panic(fmt.Sprintf("window: layout: missing rect after %s", op))
	}
	return r
}

func (a *Actor) emit(ctx context.Context, evt protocol.Event) {
	if a.hub == nil {
		return
	}
	_ = a.hub.Send(ctx, evt)
}

func (a *Actor) emitLayout(ctx context.Context, sid protocol.SessionID, wid protocol.WindowID) {
	if a.hub == nil {
		return
	}
	panes := layoutEventPanes(a.Layout)
	var savedPanes []protocol.EventLayoutPane
	if a.savedLayout != nil {
		savedPanes = layoutEventPanes(*a.savedLayout)
	}
	_ = a.hub.Send(ctx, protocol.EventWindowLayoutChanged{
		SessionID:    sid,
		WindowID:     wid,
		Revision:     a.revision,
		Cols:         int(a.Layout.WindowCols),
		Rows:         int(a.Layout.WindowRows),
		Panes:        panes,
		ZoomedPaneID: a.zoomedPaneID,
		SavedPanes:   savedPanes,
	})
}

func layoutEventPanes(layout Layout) []protocol.EventLayoutPane {
	ids := layout.PaneIDs()
	panes := make([]protocol.EventLayoutPane, 0, len(ids))
	for _, pid := range ids {
		r, rok := layout.Rect(pid)
		if !rok {
			continue
		}
		panes = append(panes, protocol.EventLayoutPane{
			PaneID: pid,
			Col:    int(r.Col),
			Row:    int(r.Row),
			Cols:   int(r.Cols),
			Rows:   int(r.Rows),
		})
	}
	return panes
}

func (a *Actor) rejectSplit(ctx context.Context, m protocol.CommandPaneSplit, reason string) {
	a.rejectCommand(ctx, m.Meta, m.SessionID, m.WindowID, "pane-split", reason)
}

func (a *Actor) rejectCommand(ctx context.Context, meta protocol.CommandMeta, sessionID protocol.SessionID, windowID protocol.WindowID, command, reason string) {
	if meta.Empty() {
		return
	}
	a.emit(ctx, protocol.EventCommandRejected{
		ClientID:  meta.ClientID,
		RequestID: meta.RequestID,
		SessionID: sessionID,
		WindowID:  windowID,
		Command:   command,
		Reason:    reason,
	})
}

func (a *Actor) rejectLayoutPreset(ctx context.Context, m protocol.CommandWindowSelectLayout, reason string) {
	a.emit(ctx, protocol.EventCommandRejected{
		ClientID:  m.Meta.ClientID,
		RequestID: m.Meta.RequestID,
		SessionID: m.SessionID,
		WindowID:  m.WindowID,
		Command:   "select-layout",
		Reason:    reason,
	})
}

func (a *Actor) rejectPaneSwap(ctx context.Context, m protocol.CommandPaneSwap, reason string) {
	a.emit(ctx, protocol.EventCommandRejected{
		ClientID:  m.Meta.ClientID,
		RequestID: m.Meta.RequestID,
		SessionID: m.SessionID,
		WindowID:  m.WindowID,
		Command:   "swap-pane",
		Reason:    reason,
	})
}

func (a *Actor) rejectFocus(ctx context.Context, m protocol.CommandPaneFocus, reason string) {
	a.emit(ctx, protocol.EventCommandRejected{
		ClientID:  m.Meta.ClientID,
		RequestID: m.Meta.RequestID,
		SessionID: m.SessionID,
		WindowID:  m.WindowID,
		Command:   "pane-focus",
		Reason:    reason,
	})
}

func (a *Actor) sendPaneGeometry(ctx context.Context, sid protocol.SessionID, wid protocol.WindowID, pid protocol.PaneID, r Rect, init bool) {
	cols, rows := paneContentSize(r)
	if init {
		if err := a.Panes.Must(pid).Send(ctx, protocol.CommandPaneInit{Cols: cols, Rows: rows}); err != nil {
			if ctx.Err() != nil {
				return
			}
			panic(fmt.Sprintf("window: send pane init %s: %v", pid, err))
		}
		return
	}
	if err := a.Panes.Must(pid).Send(ctx, protocol.CommandPaneResize{
		SessionID: sid,
		WindowID:  wid,
		PaneID:    pid,
		Cols:      cols,
		Rows:      rows,
	}); err != nil {
		if ctx.Err() != nil {
			return
		}
		panic(fmt.Sprintf("window: send pane resize %s: %v", pid, err))
	}
}

func paneContentSize(r Rect) (uint16, uint16) {
	cols, rows := r.Cols, r.Rows
	if cols > 2 {
		cols -= 2
	} else {
		cols = 1
	}
	if rows > 2 {
		rows -= 2
	} else {
		rows = 1
	}
	return cols, rows
}

func Start(ctx context.Context) actor.Ref[protocol.Command] {
	return StartWithHub(ctx, nil)
}

// StartWithHub is [Start] with optional hub; lifecycle events are best-effort when hub is non-nil.
func StartWithHub(ctx context.Context, hub actor.EventRef) actor.Ref[protocol.Command] {
	return actor.Start[protocol.Command](ctx, NewActor(hub).Run)
}

func StartWithPolicy(ctx context.Context, sessionRef actor.Ref[protocol.Command], hub actor.EventRef, sessionID protocol.SessionID, windowID protocol.WindowID, windowOrdinal int, policy cfg.Config) actor.Ref[protocol.Command] {
	return actor.Start[protocol.Command](ctx, NewActorWithPolicy(sessionRef, hub, sessionID, windowID, windowOrdinal, policy).Run)
}
