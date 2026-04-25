package shux

import (
	"context"
	"fmt"

	tea "charm.land/bubbletea/v2"
	"shux/internal/actor"
	"shux/internal/hub"
	"shux/internal/protocol"
	"shux/internal/supervisor"
	"shux/internal/ui"
)

type Shux struct {
	Logger *Logger

	SessionID protocol.SessionID
	WindowID  protocol.WindowID
	PaneID    protocol.PaneID

	hub        actor.Ref[protocol.Event]
	supervisor actor.Ref[protocol.Command]
	started    bool
}

func NewShux() (*Shux, error) {
	logger, err := NewLogger()
	if err != nil {
		return nil, fmt.Errorf("failed to init logger: %w", err)
	}

	return &Shux{
		Logger: logger,
	}, nil
}

func (a *Shux) Close() error {
	return a.Logger.Close()
}

func (a *Shux) Run(opts ...tea.ProgramOption) error {
	defer a.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := a.BootstrapDefaultSession(ctx); err != nil {
		return fmt.Errorf("failed to bootstrap default session: %w", err)
	}

	_, err := tea.NewProgram(ui.NewModel(a.SessionID, a.WindowID, a.PaneID), opts...).Run()
	if err != nil {
		return fmt.Errorf("failed to run ui: %w", err)
	}

	return nil
}

func (a *Shux) BootstrapDefaultSession(ctx context.Context) error {
	if !a.started {
		a.startActors(ctx)
	}

	events := make(eventChanSink, 3)
	clientID := protocol.ClientID("bootstrap")
	if err := a.hub.Send(ctx, protocol.EventRegisterSubscriber{ClientID: clientID, Sink: events}); err != nil {
		return err
	}
	defer a.hub.Send(ctx, protocol.EventUnregisterSubscriber{ClientID: clientID})

	if err := a.supervisor.Send(ctx, protocol.CommandCreateSession{}); err != nil {
		return err
	}
	session, err := waitForEvent[protocol.EventSessionCreated](ctx, events)
	if err != nil {
		return err
	}

	if err := a.supervisor.Send(ctx, protocol.CommandCreateWindow{SessionID: session.SessionID}); err != nil {
		return err
	}
	window, err := waitForEvent[protocol.EventWindowCreated](ctx, events)
	if err != nil {
		return err
	}

	if err := a.supervisor.Send(ctx, protocol.CommandCreatePane{SessionID: session.SessionID, WindowID: window.WindowID}); err != nil {
		return err
	}
	pane, err := waitForEvent[protocol.EventPaneCreated](ctx, events)
	if err != nil {
		return err
	}

	a.SessionID = session.SessionID
	a.WindowID = window.WindowID
	a.PaneID = pane.PaneID
	return nil
}

func (a *Shux) startActors(ctx context.Context) {
	if a.started {
		panic("shux: actors already started")
	}
	hubRef := hub.Start(ctx)
	a.hub = hubRef
	a.supervisor = supervisor.StartWithHub(ctx, &hubRef)
	a.started = true
}

type eventChanSink chan protocol.Event

func (s eventChanSink) DeliverEvent(ctx context.Context, e protocol.Event) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case s <- e:
		return nil
	default:
		return fmt.Errorf("event sink full")
	}
}

func waitForEvent[T protocol.Event](ctx context.Context, events <-chan protocol.Event) (T, error) {
	var zero T
	for {
		select {
		case <-ctx.Done():
			return zero, ctx.Err()
		case event := <-events:
			if typed, ok := event.(T); ok {
				return typed, nil
			}
		}
	}
}
