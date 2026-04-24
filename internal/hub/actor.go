package hub

import (
	"context"
	"fmt"

	"shux-dev/internal/actor"
	"shux-dev/internal/protocol"
)

// Subscribers is keyed client lifecycle bookkeeping (Init / Delete / Must on event refs).
type Subscribers = *actor.Lifecycle[protocol.ClientID, protocol.Event]

type Actor struct {
	Subscribers
}

func NewActor() *Actor {
	return &Actor{
		Subscribers: actor.NewLifecycle[protocol.ClientID, protocol.Event]("hub", "subscriber"),
	}
}

func (a *Actor) Run(ctx context.Context, _ actor.Ref[protocol.Event], inbox <-chan protocol.Event) {
	for {
		select {
		case <-ctx.Done():
			return
		case msg := <-inbox:
			switch msg.(type) {
			case protocol.EventNoop:
			default:
				panic(fmt.Sprintf("hub: unhandled event type %T", msg))
			}
		}
	}
}

func Start(ctx context.Context) actor.Ref[protocol.Event] {
	return actor.Start[protocol.Event](ctx, NewActor().Run)
}
