package shux

import (
	"context"
	"fmt"

	"shux/internal/protocol"
)

func (a *Shux) ListSessions(ctx context.Context) ([]protocol.SessionDescriptor, error) {
	reply := make(chan []protocol.SessionDescriptor, 1)
	if err := a.supervisor.Send(ctx, protocol.CommandListSessions{Reply: reply}); err != nil {
		return nil, err
	}
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case out := <-reply:
		return out, nil
	}
}

func (a *Shux) ResolveSession(ctx context.Context, name string) (protocol.SessionDescriptor, error) {
	sessions, err := a.ListSessions(ctx)
	if err != nil {
		return protocol.SessionDescriptor{}, err
	}
	for _, session := range sessions {
		if session.Name == name {
			return session, nil
		}
	}
	return protocol.SessionDescriptor{}, fmt.Errorf("shux: unknown session %q", name)
}

func (a *Shux) CreateNamedSession(ctx context.Context, name string) (protocol.SessionDescriptor, error) {
	if !protocol.ValidSessionName(name) {
		return protocol.SessionDescriptor{}, fmt.Errorf("shux: invalid session name %q", name)
	}
	events := make(protocol.EventChanAdapter, 16)
	if err := a.hub.Send(ctx, protocol.EventRegisterSubscriber{ClientID: bootstrapClientID, Sink: events}); err != nil {
		return protocol.SessionDescriptor{}, err
	}
	defer a.hub.Send(ctx, protocol.EventUnregisterSubscriber{ClientID: bootstrapClientID})

	reply := make(chan protocol.CommandCreateSessionResult, 1)
	if err := a.supervisor.Send(ctx, protocol.CommandCreateSession{Name: name, Reply: reply}); err != nil {
		return protocol.SessionDescriptor{}, err
	}
	var created protocol.SessionDescriptor
	select {
	case <-ctx.Done():
		return protocol.SessionDescriptor{}, ctx.Err()
	case result := <-reply:
		if result.Err != nil {
			return protocol.SessionDescriptor{}, result.Err
		}
		created = result.Session
	}
	window, err := bootstrapStep[protocol.EventWindowCreated](ctx, a.supervisor, events, protocol.CommandCreateWindow{SessionID: created.SessionID})
	if err != nil {
		return protocol.SessionDescriptor{}, err
	}
	if _, err := bootstrapStep[protocol.EventPaneCreated](ctx, a.supervisor, events, protocol.CommandCreatePane{SessionID: created.SessionID, WindowID: window.WindowID}); err != nil {
		return protocol.SessionDescriptor{}, err
	}
	if a.checkpoints != nil {
		a.checkpoints.schedule()
	}
	return created, nil
}
