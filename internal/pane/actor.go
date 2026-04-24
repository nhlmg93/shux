package pane

import (
	"context"

	"shux-dev/internal/actor"
	"shux-dev/internal/protocol"
)

type LibghosttyVT struct{}

type Actor struct {
	VT *LibghosttyVT
}

func NewActor() *Actor {
	return &Actor{}
}

func (a *Actor) Run(ctx context.Context, self actor.Ref[protocol.Command], inbox <-chan protocol.Command) {
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
