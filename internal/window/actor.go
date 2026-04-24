package window

import (
	"context"
	"fmt"
	"strconv"

	"shux-dev/internal/actor"
	"shux-dev/internal/pane"
	"shux-dev/internal/protocol"
)

type Layout struct{}

// Panes is keyed pane lifecycle bookkeeping (Init / Delete / Must on command refs).
type Panes = *actor.Lifecycle[protocol.PaneID, protocol.Command]

type Actor struct {
	Panes
	Layout Layout
	hub actor.EventRef // optional lifecycle event sink (best-effort publish)
	seq uint64 // next pane id suffix; only touched from Run goroutine
}

func NewActor(hub actor.EventRef) *Actor {
	return &Actor{
		Panes: actor.NewLifecycle[protocol.PaneID, protocol.Command]("window", "pane"),
		hub:   hub,
	}
}

func (a *Actor) Run(ctx context.Context, _ actor.Ref[protocol.Command], inbox <-chan protocol.Command) {
	for {
		select {
		case <-ctx.Done():
			return
		case msg := <-inbox:
			switch m := msg.(type) {
			case protocol.CommandNoop:
			case protocol.CommandCreatePane:
				a.seq++
				pid := protocol.PaneID("p-" + strconv.FormatUint(a.seq, 10))
				a.Init(pid, pane.Start(ctx))
				if a.hub != nil {
					_ = a.hub.Send(ctx, protocol.EventPaneCreated{WindowID: m.WindowID, PaneID: pid})
				}
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
