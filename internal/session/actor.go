package session

import (
	"context"
	"fmt"
	"strconv"

	"shux/internal/actor"
	"shux/internal/protocol"
	"shux/internal/window"
)

// Windows is keyed window lifecycle bookkeeping (Init / Delete / Must on command refs).
type Windows = *actor.Lifecycle[protocol.WindowID, protocol.Command]

type Actor struct {
	Windows
	seq uint64         // next window id suffix; only touched from Run goroutine
	hub actor.EventRef // optional lifecycle event sink (best-effort publish)
}

func NewActor(hub actor.EventRef) *Actor {
	return &Actor{
		Windows: actor.NewLifecycle[protocol.WindowID, protocol.Command]("session", "window"),
		hub:     hub,
	}
}

func (a *Actor) Run(ctx context.Context, _ actor.Ref[protocol.Command], inbox <-chan protocol.Command) {
	for {
		select {
		case <-ctx.Done():
			return
		case msg := <-inbox:
			switch m := msg.(type) {
			case protocol.CommandNoop:
			case protocol.CommandCreateWindow:
				a.seq++
				wid := protocol.WindowID("w-" + strconv.FormatUint(a.seq, 10))
				a.Init(wid, window.StartWithHub(ctx, a.hub))
				if a.hub != nil {
					_ = a.hub.Send(ctx, protocol.EventWindowCreated{SessionID: m.SessionID, WindowID: wid})
				}
			case protocol.CommandCreatePane:
				a.Windows.Must(m.WindowID).Send(ctx, m)
			default:
				panic(fmt.Sprintf("session: unhandled command type %T", msg))
			}
		}
	}
}

func Start(ctx context.Context) actor.Ref[protocol.Command] {
	return StartWithHub(ctx, nil)
}

// StartWithHub is [Start] with optional hub; lifecycle events are best-effort when hub is non-nil.
func StartWithHub(ctx context.Context, hub actor.EventRef) actor.Ref[protocol.Command] {
	return actor.Start[protocol.Command](ctx, NewActor(hub).Run)
}
