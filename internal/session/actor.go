package session

import (
	"context"
	"fmt"

	"shux-dev/internal/actor"
	"shux-dev/internal/protocol"
)

// Windows is keyed window lifecycle bookkeeping (Init / Delete / Must on command refs).
type Windows = *actor.Lifecycle[protocol.WindowID, protocol.Command]

type Actor struct {
	Windows
}

func NewActor() *Actor {
	return &Actor{
		Windows: actor.NewLifecycle[protocol.WindowID, protocol.Command]("session", "window"),
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
				panic(fmt.Sprintf("session: unhandled command type %T", msg))
			}
		}
	}
}

func Start(ctx context.Context) actor.Ref[protocol.Command] {
	return actor.Start[protocol.Command](ctx, NewActor().Run)
}
