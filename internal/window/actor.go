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
	Layout  Layout
	paneIDs []protocol.PaneID
	hub     actor.EventRef
	seq     uint64
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
	act, ok := a.Layout.ActivePane()
	if !ok {
		act = ""
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
		SessionID:  sid,
		WindowID:   wid,
		Cols:       int(a.Layout.WindowCols),
		Rows:       int(a.Layout.WindowRows),
		ActivePane: act,
		Panes:      panes,
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
				if r, ok := a.Layout.Rect(pid); !ok {
					panic("window: layout: missing rect after SetSinglePane")
				} else {
					a.sendPaneGeometry(ctx, m.SessionID, m.WindowID, pid, r, true)
				}
				a.emitLayout(ctx, m.SessionID, m.WindowID)
			case protocol.CommandWindowResize:
				a.Layout.SetWindowSize(m.Cols, m.Rows)
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
				// Current layout supports one split only. Extra split requests are user input,
				// so bound them as no-ops instead of crashing the multiplexer.
				if len(a.paneIDs) >= 2 {
					continue
				}
				if len(a.paneIDs) != 1 {
					panic("window: CommandPaneSplit: need exactly one pane before split")
				}
				a.seq++
				newID := protocol.PaneID("p-" + strconv.FormatUint(a.seq, 10))
				a.paneIDs = append(a.paneIDs, newID)
				a.Init(newID, pane.Start(ctx))
				if a.hub != nil {
					_ = a.hub.Send(ctx, protocol.EventPaneCreated{WindowID: m.WindowID, PaneID: newID})
				}
				a.Layout.SplitActive(m.Direction, newID)
				for i, pid := range a.Layout.PaneIDs() {
					r, ok := a.Layout.Rect(pid)
					if !ok {
						panic("window: layout: missing rect after split")
					}
					init := i == len(a.Layout.PaneIDs())-1
					if init {
						a.sendPaneGeometry(ctx, m.SessionID, m.WindowID, pid, r, true)
					} else {
						a.sendPaneGeometry(ctx, m.SessionID, m.WindowID, pid, r, false)
					}
				}
				a.emitLayout(ctx, m.SessionID, m.WindowID)
			case protocol.CommandWindowCycleFocus:
				before, _ := a.Layout.ActivePane()
				a.Layout.CycleActive()
				after, _ := a.Layout.ActivePane()
				if before != after {
					a.emitLayout(ctx, m.SessionID, m.WindowID)
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
