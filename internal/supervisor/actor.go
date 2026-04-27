package supervisor

import (
	"context"
	"fmt"
	"strconv"

	"shux/internal/actor"
	"shux/internal/protocol"
	"shux/internal/session"
)

type Sessions = *actor.Lifecycle[protocol.SessionID, protocol.Command]

type Actor struct {
	Sessions
	ShellPath string
	seq       uint64
	hub       actor.EventRef // optional lifecycle event sink (best-effort publish)
}

func NewActor(hub actor.EventRef) *Actor {
	return NewActorWithConfig(hub, "/bin/sh")
}

func NewActorWithConfig(hub actor.EventRef, shellPath string) *Actor {
	if hub != nil && !hub.Valid() {
		panic("supervisor: NewActor: invalid hub ref")
	}
	if shellPath == "" {
		panic("supervisor: NewActor: empty shell path")
	}
	return &Actor{
		Sessions:  actor.NewLifecycle[protocol.SessionID, protocol.Command]("supervisor", "session", protocol.SessionID.Valid),
		ShellPath: shellPath,
		hub:       hub,
	}
}

func (a *Actor) Run(ctx context.Context, _ actor.Ref[protocol.Command], inbox <-chan protocol.Command) {
	for {
		select {
		case <-ctx.Done():
			return
		case msg := <-inbox:
			if err := protocol.ValidateCommand(msg); err != nil {
				panic(err)
			}
			switch msg.(type) {
			case protocol.CommandNoop:
				continue
			case protocol.CommandCreateSession:
				a.seq++
				sid := protocol.SessionID("s-" + strconv.FormatUint(a.seq, 10))
				a.Init(sid, session.StartWithConfig(ctx, a.hub, sid, a.ShellPath))
				if a.hub != nil {
					_ = a.hub.Send(ctx, protocol.EventSessionCreated{SessionID: sid})
				}
				continue
			}
			if sid, ok := protocol.RouteSessionID(msg); ok {
				_ = a.Sessions.Must(sid).Send(ctx, msg)
				continue
			}
			panic(fmt.Sprintf("supervisor: unhandled command type %T", msg))
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

func StartWithConfig(ctx context.Context, hub actor.EventRef, shellPath string) actor.Ref[protocol.Command] {
	return actor.Start[protocol.Command](ctx, NewActorWithConfig(hub, shellPath).Run)
}
