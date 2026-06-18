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
	names  map[string]protocol.SessionID
	ids    map[protocol.SessionID]string
	order  []string
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
		names:    make(map[string]protocol.SessionID),
		ids:      make(map[protocol.SessionID]string),
	}
}

func (a *Actor) listSessionsSnapshot() []protocol.SessionDescriptor {
	out := make([]protocol.SessionDescriptor, 0, len(a.order))
	for _, name := range a.order {
		sid, ok := a.names[name]
		if !ok {
			continue
		}
		out = append(out, protocol.SessionDescriptor{Name: name, SessionID: sid})
	}
	return out
}

func (a *Actor) autoSessionName() string {
	for i := 1; ; i++ {
		name := "session-" + strconv.Itoa(i)
		if _, exists := a.names[name]; !exists {
			return name
		}
	}
}

func (a *Actor) createSession(ctx context.Context, requestedName string) (protocol.SessionDescriptor, error) {
	name := requestedName
	if name == "" {
		name = a.autoSessionName()
	}
	if _, exists := a.names[name]; exists {
		return protocol.SessionDescriptor{}, fmt.Errorf("supervisor: session %q already exists", name)
	}
	if uint(len(a.names)) >= a.Policy.MaxSessions {
		return protocol.SessionDescriptor{}, fmt.Errorf("supervisor: max sessions reached (%d)", a.Policy.MaxSessions)
	}
	a.seq++
	sid := protocol.SessionID("s-" + strconv.FormatUint(a.seq, 10))
	a.Init(sid, session.StartWithPolicy(ctx, a.hub, sid, a.Policy))
	a.names[name] = sid
	a.ids[sid] = name
	a.order = append(a.order, name)
	if a.hub != nil {
		_ = a.hub.Send(ctx, protocol.EventSessionCreated{SessionID: sid, Name: name})
	}
	return protocol.SessionDescriptor{Name: name, SessionID: sid}, nil
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
			switch msg := msg.(type) {
			case protocol.CommandNoop:
				continue
			case protocol.CommandCreateSession:
				created, err := a.createSession(ctx, msg.Name)
				if msg.Reply != nil {
					msg.Reply <- protocol.CommandCreateSessionResult{Session: created, Err: err}
					continue
				}
				if err != nil {
					panic(err)
				}
				continue
			case protocol.CommandListSessions:
				msg.Reply <- a.listSessionsSnapshot()
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
