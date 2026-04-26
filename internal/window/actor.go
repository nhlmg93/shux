package window

import (
	"context"
	"fmt"
	"strconv"

	"shux/internal/actor"
	"shux/internal/pane"
	"shux/internal/protocol"
)

// Panes is keyed pane lifecycle bookkeeping (Init / Delete / Must on command refs).
type Panes = *actor.Lifecycle[protocol.PaneID, protocol.Command]

type Actor struct {
	Panes
	Layout   Layout
	paneIDs  []protocol.PaneID
	hub      actor.EventRef
	seq      uint64
	revision uint64
}

func NewActor(hub actor.EventRef) *Actor {
	if hub != nil && !hub.Valid() {
		panic("window: NewActor: invalid hub ref")
	}
	return &Actor{
		Panes:  actor.NewLifecycle[protocol.PaneID, protocol.Command]("window", "pane", protocol.PaneID.Valid),
		Layout: NewLayout(80, 24),
		hub:    hub,
	}
}

func (a *Actor) emitLayout(ctx context.Context, sid protocol.SessionID, wid protocol.WindowID) {
	if a.hub == nil {
		return
	}
	ids := a.Layout.PaneIDs()
	panes := make([]protocol.EventLayoutPane, 0, len(ids))
	for _, pid := range ids {
		r, rok := a.Layout.Rect(pid)
		if !rok {
			continue
		}
		panes = append(panes, protocol.EventLayoutPane{
			PaneID: pid,
			Col:    int(r.Col), Row: int(r.Row),
			Cols: int(r.Cols), Rows: int(r.Rows),
		})
	}
	_ = a.hub.Send(ctx, protocol.EventWindowLayoutChanged{
		SessionID: sid,
		WindowID:  wid,
		Revision:  a.revision,
		Cols:      int(a.Layout.WindowCols),
		Rows:      int(a.Layout.WindowRows),
		Panes:     panes,
	})
}

func (a *Actor) reject(ctx context.Context, m protocol.CommandPaneSplit, reason string) {
	if a.hub == nil {
		return
	}
	_ = a.hub.Send(ctx, protocol.EventCommandRejected{
		ClientID:  m.Meta.ClientID,
		RequestID: m.Meta.RequestID,
		SessionID: m.SessionID,
		WindowID:  m.WindowID,
		Command:   "pane-split",
		Reason:    reason,
	})
}

func (a *Actor) sendPaneGeometry(ctx context.Context, sid protocol.SessionID, wid protocol.WindowID, pid protocol.PaneID, r Rect, init bool) {
	if init {
		a.Panes.Must(pid).Send(ctx, protocol.CommandPaneInit{Cols: r.Cols, Rows: r.Rows})
		return
	}
	a.Panes.Must(pid).Send(ctx, protocol.CommandPaneResize{
		SessionID: sid,
		WindowID:  wid,
		PaneID:    pid,
		Cols:      r.Cols,
		Rows:      r.Rows,
	})
}

func (a *Actor) Run(ctx context.Context, _ actor.Ref[protocol.Command], inbox <-chan protocol.Command) {
	for {
		select {
		case <-ctx.Done():
			return
		case msg := <-inbox:
			if err := protocol.ValidateCommand(msg); err != nil {
				panic(err)
			}
			switch m := msg.(type) {
			case protocol.CommandNoop:
			case protocol.CommandCreatePane:
				if len(a.paneIDs) > 0 {
					panic("window: second pane: use CommandPaneSplit (layout is single-or-split)")

				}
				a.seq++
				pid := protocol.PaneID("p-" + strconv.FormatUint(a.seq, 10))
				a.paneIDs = append(a.paneIDs, pid)
				a.Init(pid, pane.Start(ctx))
				if a.hub != nil {
					_ = a.hub.Send(ctx, protocol.EventPaneCreated{WindowID: m.WindowID, PaneID: pid})
				}
				a.Layout.SetSinglePane(pid)
				a.revision++
				if r, ok := a.Layout.Rect(pid); !ok {
					panic("window: layout: missing rect after SetSinglePane")
				} else {
					a.sendPaneGeometry(ctx, m.SessionID, m.WindowID, pid, r, true)
				}
				a.emitLayout(ctx, m.SessionID, m.WindowID)
			case protocol.CommandWindowResize:
				a.Layout.SetWindowSize(m.Cols, m.Rows)
				a.revision++
				for _, pid := range a.Layout.PaneIDs() {
					r, ok := a.Layout.Rect(pid)
					if !ok {
						panic("window: layout: missing rect after SetWindowSize")
					}
					a.Panes.Must(pid).Send(ctx, protocol.CommandPaneResize{
						SessionID: m.SessionID,
						WindowID:  m.WindowID,
						PaneID:    pid,
						Cols:      r.Cols,
						Rows:      r.Rows,
					})
				}
				a.emitLayout(ctx, m.SessionID, m.WindowID)
			case protocol.CommandPaneSplit:
				if err := a.Layout.CanSplitPane(m.TargetPaneID, m.Direction); err != nil {
					a.reject(ctx, m, err.Error())
					continue
				}
				a.seq++
				newID := protocol.PaneID("p-" + strconv.FormatUint(a.seq, 10))
				if err := a.Layout.SplitPane(m.TargetPaneID, m.Direction, newID); err != nil {
					a.reject(ctx, m, err.Error())
					continue
				}
				a.revision++
				a.paneIDs = append(a.paneIDs, newID)
				a.Init(newID, pane.Start(ctx))
				if a.hub != nil {
					_ = a.hub.Send(ctx, protocol.EventPaneCreated{WindowID: m.WindowID, PaneID: newID})
				}
				for _, pid := range a.Layout.PaneIDs() {
					r, ok := a.Layout.Rect(pid)
					if !ok {
						panic("window: layout: missing rect after split")
					}
					if pid == newID {
						a.sendPaneGeometry(ctx, m.SessionID, m.WindowID, pid, r, true)
					} else {
						a.sendPaneGeometry(ctx, m.SessionID, m.WindowID, pid, r, false)
					}
				}
				a.emitLayout(ctx, m.SessionID, m.WindowID)
				if a.hub != nil {
					_ = a.hub.Send(ctx, protocol.EventPaneSplitCompleted{
						ClientID:     m.Meta.ClientID,
						RequestID:    m.Meta.RequestID,
						SessionID:    m.SessionID,
						WindowID:     m.WindowID,
						TargetPaneID: m.TargetPaneID,
						NewPaneID:    newID,
						Revision:     a.revision,
					})
				}
			case protocol.CommandPaneResize:
				a.Panes.Must(m.PaneID).Send(ctx, m)
			default:
				panic(fmt.Sprintf("window: unhandled command type %T", msg))
			}
		}
	}
}

func Start(ctx context.Context) actor.Ref[protocol.Command] {
	return StartWithHub(ctx, nil)
}

// StartWithHub is [Start] with optional hub; lifecycle events are best-effort when hub is non-nil.
func StartWithHub(ctx context.Context, hub actor.EventRef) actor.Ref[protocol.Command] {
	return actor.Start[protocol.Command](ctx, NewActor(hub).Run)
}
