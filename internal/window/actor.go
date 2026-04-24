package window

import (
	"context"
	"fmt"

	"shux-dev/internal/actor"
	"shux-dev/internal/protocol"
)

type Layout struct{}

// Panes is keyed pane lifecycle bookkeeping (Init / Delete / Must on command refs).
type Panes = *actor.Lifecycle[protocol.PaneID, protocol.Command]

type Actor struct {
	Panes
	Layout Layout
}

func NewActor() *Actor {
	return &Actor{
		Panes: actor.NewLifecycle[protocol.PaneID, protocol.Command]("window", "pane"),
	}
}

func (a *Actor) Run(ctx context.Context, _ actor.Ref[protocol.Command], inbox <-chan protocol.Command) {
	for {
		select {
		case <-ctx.Done():
			return
		case msg := <-inbox:
			switch msg.(type) {
			case protocol.CommandNoop:
			default:
				panic(fmt.Sprintf("window: unhandled command type %T", msg))
			}
		}
	}
}

func Start(ctx context.Context) actor.Ref[protocol.Command] {
	return actor.Start[protocol.Command](ctx, NewActor().Run)
}
