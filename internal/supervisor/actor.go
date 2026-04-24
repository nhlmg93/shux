package supervisor

import (
	"context"

	"shux-dev/internal/actor"
	"shux-dev/internal/protocol"
)

type Sessions map[protocol.SessionID]actor.Ref[protocol.Command]

type Actor struct {
	Sessions Sessions
}

func NewActor() *Actor {
	return &Actor{
		Sessions: make(Sessions),
	}
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
