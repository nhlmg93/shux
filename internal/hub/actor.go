package hub

import (
	"context"
	"fmt"
	"os"

	"shux/internal/actor"
	"shux/internal/protocol"
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
			if err := protocol.ValidateEvent(msg); err != nil {
				panic(err)
			}
			switch m := msg.(type) {
			case protocol.EventNoop:
			case protocol.EventRegisterSubscriber:
				if _, ok := a.sinks[m.ClientID]; ok {
					fmt.Fprintf(os.Stderr, "hub: duplicate EventRegisterSubscriber for %q (ignored)\n", m.ClientID)
					continue
				}
				a.sinks[m.ClientID] = m.Sink
			case protocol.EventUnregisterSubscriber:
				if _, ok := a.sinks[m.ClientID]; !ok {
					continue
				}
				delete(a.sinks, m.ClientID)
			default:
				a.fanout(ctx, m)
			}
		}
	}
}

func Start(ctx context.Context) actor.Ref[protocol.Event] {
	return actor.Start[protocol.Event](ctx, NewActor().Run)
}
