package supervisor

import (
	"context"
	"fmt"

	"shux-dev/internal/actor"
	"shux-dev/internal/protocol"
)

type Sessions = *actor.Lifecycle[protocol.SessionID, protocol.Command]

type Actor struct {
	Sessions
}

func NewActor() *Actor {
	return &Actor{
		Sessions: actor.NewLifecycle[protocol.SessionID, protocol.Command]("supervisor", "session"),
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
				panic(fmt.Sprintf("supervisor: unhandled command type %T", msg))
			}
		}
	}
}

func Start(ctx context.Context) actor.Ref[protocol.Command] {
	return actor.Start[protocol.Command](ctx, NewActor().Run)
}
