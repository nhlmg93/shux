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

type machine uint8

const (
	new machine = iota
	started
	closed
)

type Shux struct {
	Logger *Logger

	SessionID protocol.SessionID
	WindowID  protocol.WindowID
	PaneID    protocol.PaneID

	hub          actor.Ref[protocol.Event]
	supervisor   actor.Ref[protocol.Command]
	actorCancel  context.CancelFunc
	bootstrapSeq uint64
	state        machine
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
	if a.state == closed {
		return nil
	}
	if a.actorCancel != nil {
		a.actorCancel()
		a.actorCancel = nil
	}
	a.state = closed
	return a.Logger.Close()
}

func (a *Shux) Run(opts ...tea.ProgramOption) error {
	if a.state == closed {
		return fmt.Errorf("shux: run after close")
	}
	a.Logger.Info("shux: run starting")
	defer a.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := a.BootstrapDefaultSession(ctx); err != nil {
		a.Logger.Error(fmt.Sprintf("shux: bootstrap failed: %v", err))
		return fmt.Errorf("failed to bootstrap default session: %w", err)
	}

	p := tea.NewProgram(ui.NewModelWithSupervisor(a.SessionID, a.WindowID, a.PaneID, a.supervisor, ctx), opts...)
	uiID := protocol.ClientID("shux-ui")
	if err := a.hub.Send(ctx, protocol.EventRegisterSubscriber{ClientID: uiID, Sink: &ui.ProgramEventSink{P: p}}); err != nil {
		a.Logger.Error(fmt.Sprintf("shux: ui hub register failed: %v", err))
		return fmt.Errorf("shux: register ui hub: %w", err)
	}
	defer func() { _ = a.hub.Send(ctx, protocol.EventUnregisterSubscriber{ClientID: uiID}) }()

	if _, err := p.Run(); err != nil {
		a.Logger.Error(fmt.Sprintf("shux: ui failed: %v", err))
		return fmt.Errorf("failed to run ui: %w", err)
	}

	a.Logger.Info("shux: run stopped")
	return nil
}

func (a *Shux) BootstrapDefaultSession(ctx context.Context) error {
	a.Logger.Info("shux: bootstrap default session starting")
	if a.state == closed {
		return fmt.Errorf("shux: bootstrap after close")
	}
	if a.state == new {
		a.startActors(ctx)
	}

	events := make(eventChanSink, 8)
	a.bootstrapSeq++
	clientID := protocol.ClientID(fmt.Sprintf("bootstrap-%d", a.bootstrapSeq))
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
	a.Logger.Info("shux: bootstrap default session ready")
	return nil
}

func (a *Shux) startActors(ctx context.Context) {
	if a.state != new {
		panic("shux: actors already started")
	}
	a.Logger.Info("shux: starting actors")
	actorCtx, cancel := context.WithCancel(ctx)
	hubRef := hub.Start(actorCtx)
	a.hub = hubRef
	a.supervisor = supervisor.StartWithHub(actorCtx, &hubRef)
	a.actorCancel = cancel
	a.state = started
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
