package supervisor

import (
	"context"
	"fmt"
	"strconv"

	"shux/internal/actor"
	"shux/internal/cfg"
	"shux/internal/protocol"
	"shux/internal/session"
)

type Sessions = *actor.Lifecycle[protocol.SessionID, protocol.Command]

type Actor struct {
	Sessions
	Policy cfg.Config
	seq    uint64
	hub    actor.EventRef
}

func NewActor(hub actor.EventRef) *Actor {
	return NewActorWithPolicy(hub, cfg.DefaultConfig())
}

func NewActorWithPolicy(hub actor.EventRef, policy cfg.Config) *Actor {
	if hub != nil && !hub.Valid() {
		panic("supervisor: NewActor: invalid hub ref")
	}
	policy = policy.WithDefaults()
	if policy.ShellPath == "" {
		panic("supervisor: NewActor: empty shell path")
	}
	return &Actor{
		Sessions: actor.NewLifecycle[protocol.SessionID, protocol.Command]("supervisor", "session", protocol.SessionID.Valid),
		Policy:   policy,
		hub:      hub,
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
				a.Init(sid, session.StartWithPolicy(ctx, a.hub, sid, a.Policy))
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

func StartWithHub(ctx context.Context, hub actor.EventRef) actor.Ref[protocol.Command] {
	return actor.Start[protocol.Command](ctx, NewActor(hub).Run)
}

func StartWithPolicy(ctx context.Context, hub actor.EventRef, policy cfg.Config) actor.Ref[protocol.Command] {
	return actor.Start[protocol.Command](ctx, NewActorWithPolicy(hub, policy).Run)
}
