package hub

import (
	"context"
	"fmt"

	"shux-dev/internal/actor"
	"shux-dev/internal/protocol"
)

// Actor fans out events to registered EventSinks. Only Run touches sinks.
type Actor struct {
	sinks map[protocol.ClientID]protocol.EventSink
}

func NewActor() *Actor {
	return &Actor{
		sinks: make(map[protocol.ClientID]protocol.EventSink),
	}
}

func (a *Actor) fanout(ctx context.Context, e protocol.Event) {
	var failed []protocol.ClientID
	for id, sk := range a.sinks {
		if err := sk.DeliverEvent(ctx, e); err != nil {
			failed = append(failed, id)
		}
	}
	for _, id := range failed {
		delete(a.sinks, id)
	}
}

func (a *Actor) Run(ctx context.Context, _ actor.Ref[protocol.Event], inbox <-chan protocol.Event) {
	for {
		select {
		case <-ctx.Done():
			return
		case msg := <-inbox:
			switch m := msg.(type) {
			case protocol.EventNoop:
			case protocol.EventRegisterSubscriber:
				if _, ok := a.sinks[m.ClientID]; ok {
					panic("hub: duplicate EventRegisterSubscriber for client")
				}
				a.sinks[m.ClientID] = m.Sink
			case protocol.EventUnregisterSubscriber:
				delete(a.sinks, m.ClientID)
			case protocol.EventSessionCreated:
				a.fanout(ctx, m)
			case protocol.EventWindowCreated:
				a.fanout(ctx, m)
			case protocol.EventPaneCreated:
				a.fanout(ctx, m)
			default:
				panic(fmt.Sprintf("hub: unhandled event type %T", msg))
			}
		}
	}
}

func Start(ctx context.Context) actor.Ref[protocol.Event] {
	return actor.Start[protocol.Event](ctx, NewActor().Run)
}
