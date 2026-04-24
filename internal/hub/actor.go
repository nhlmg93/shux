package hub

import (
	"context"

	"shux-dev/internal/actor"
	"shux-dev/internal/protocol"
)

type Subscribers map[protocol.ClientID]actor.Ref[protocol.Event]

type Actor struct {
	Subscribers Subscribers
}

func NewActor() *Actor {
	return &Actor{
		Subscribers: make(Subscribers),
	}
}

func (a *Actor) Run(ctx context.Context, self actor.Ref[protocol.Event], inbox <-chan protocol.Event) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-inbox:
		}
	}
}

func Start(ctx context.Context) actor.Ref[protocol.Event] {
	return actor.Start[protocol.Event](ctx, NewActor().Run)
}
