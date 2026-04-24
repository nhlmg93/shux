package window

import (
	"context"

	"shux-dev/internal/actor"
	"shux-dev/internal/protocol"
)

type Panes map[protocol.PaneID]actor.Ref[protocol.Command]

type Layout struct{}

type Actor struct {
	Panes  Panes
	Layout Layout
}

func NewActor() *Actor {
	return &Actor{
		Panes: make(Panes),
	}
}

func (a *Actor) Run(ctx context.Context, _ actor.Ref[protocol.Command], inbox <-chan protocol.Command) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-inbox:
		}
	}
}

func Start(ctx context.Context) actor.Ref[protocol.Command] {
	return actor.Start[protocol.Command](ctx, NewActor().Run)
}
