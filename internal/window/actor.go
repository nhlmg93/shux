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
	paneIDs []protocol.PaneID // order of create; only single-pane layout supported until split
	hub     actor.EventRef  // optional lifecycle event sink (best-effort publish)
	seq     uint64          // next pane id suffix; only touched from Run goroutine
}

func NewActor(hub actor.EventRef) *Actor {
	if hub != nil && !hub.Valid() {
		panic("window: NewActor: invalid hub ref")
	}
	return &Actor{
		Panes:  actor.NewLifecycle[protocol.PaneID, protocol.Command]("window", "pane", protocol.PaneID.Valid),
		Layout: NewLayout(80, 24), // default until window resize / bootstrap thread real size
		hub:    hub,
	}
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
					panic("window: second pane: not implemented (use CommandPaneSplit when layout supports it)")
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
					a.Panes.Must(pid).Send(ctx, protocol.CommandPaneInit{Cols: r.Cols, Rows: r.Rows})
				}
				if a.hub != nil {
					_ = a.hub.Send(ctx, protocol.EventWindowLayoutChanged{
						SessionID: m.SessionID,
						WindowID:  m.WindowID,
						Cols:      int(a.Layout.WindowCols),
						Rows:      int(a.Layout.WindowRows),
					})
				}
			case protocol.CommandWindowResize:
				a.Layout.SetWindowSize(m.Cols, m.Rows)
				if len(a.paneIDs) == 1 {
					p := a.paneIDs[0]
					a.Layout.SetSinglePane(p)
					r, ok := a.Layout.Rect(p)
					if !ok {
						panic("window: layout: missing rect after refit")
					}
					a.Panes.Must(p).Send(ctx, protocol.CommandPaneResize{
						SessionID: m.SessionID,
						WindowID:  m.WindowID,
						PaneID:    p,
						Cols:      r.Cols,
						Rows:      r.Rows,
					})
				} else if len(a.paneIDs) > 1 {
					panic("window: CommandWindowResize: multi-pane not implemented")
				}
				if a.hub != nil {
					_ = a.hub.Send(ctx, protocol.EventWindowLayoutChanged{
						SessionID: m.SessionID,
						WindowID:  m.WindowID,
						Cols:      int(m.Cols),
						Rows:      int(m.Rows),
					})
				}
			case protocol.CommandPaneSplit:
				panic("window: CommandPaneSplit: not implemented")
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
