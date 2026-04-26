package shux

import (
	"context"
	"fmt"
	"sync"

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

	bootstrapClientID = protocol.ClientID("bootstrap")
)

type Shux struct {
	Logger *Logger

	DefaultSessionID protocol.SessionID
	DefaultWindowID  protocol.WindowID
	DefaultPaneID    protocol.PaneID

	hub          actor.Ref[protocol.Event]
	supervisor   actor.Ref[protocol.Command]
	actorCancel  context.CancelFunc
	state        machine
	bootstrapped bool
	shutdown     chan struct{}
	shutdownOnce sync.Once
}

func NewShux() (*Shux, error) {
	logger, err := NewLogger()
	if err != nil {
		return nil, fmt.Errorf("failed to init logger: %w", err)
	}

	return &Shux{
		Logger:   logger,
		shutdown: make(chan struct{}),
	}, nil
}

func (a *Shux) Done() <-chan struct{} {
	return a.shutdown
}

func (a *Shux) RequestShutdown() {
	a.shutdownOnce.Do(func() { close(a.shutdown) })
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
	defer a.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := a.BootstrapDefaultSession(ctx); err != nil {
		return fmt.Errorf("failed to bootstrap default session: %w", err)
	}
	return a.AttachClient(ctx, protocol.ClientID("local-ui"), opts...)
}

func (a *Shux) AttachClient(ctx context.Context, clientID protocol.ClientID, opts ...tea.ProgramOption) error {
	p, cleanup, err := a.NewClientProgram(ctx, clientID, opts...)
	if err != nil {
		return err
	}
	defer cleanup()

	go func() {
		select {
		case <-ctx.Done():
		case <-a.Done():
			p.Quit()
		}
	}()

	if _, err := p.Run(); err != nil {
		return fmt.Errorf("failed to run ui: %w", err)
	}
	return nil
}

func (a *Shux) NewClientProgram(ctx context.Context, clientID protocol.ClientID, opts ...tea.ProgramOption) (*tea.Program, func(), error) {
	if clientID == "" {
		return nil, nil, fmt.Errorf("shux: empty client id")
	}
	if a.state == closed {
		return nil, nil, fmt.Errorf("shux: attach after close")
	}
	if a.state != started {
		return nil, nil, fmt.Errorf("shux: attach before start")
	}
	if !a.bootstrapped {
		return nil, nil, fmt.Errorf("shux: attach before bootstrap")
	}

	opts = append([]tea.ProgramOption{tea.WithContext(ctx)}, opts...)
	p := tea.NewProgram(ui.NewModelWithSupervisorAndShutdown(a.DefaultSessionID, a.DefaultWindowID, a.DefaultPaneID, a.supervisor, ctx, a.RequestShutdown), opts...)
	if err := a.hub.Send(ctx, protocol.EventRegisterSubscriber{ClientID: clientID, Sink: &ui.ProgramEventSink{P: p}}); err != nil {
		return nil, nil, fmt.Errorf("shux: register ui hub: %w", err)
	}
	cleanup := func() { _ = a.hub.Send(ctx, protocol.EventUnregisterSubscriber{ClientID: clientID}) }
	return p, cleanup, nil
}

func (a *Shux) BootstrapDefaultSession(ctx context.Context) error {
	if a.state == closed {
		return fmt.Errorf("shux: bootstrap after close")
	}
	if a.bootstrapped {
		return nil
	}
	a.Logger.Info("shux: bootstrap default session starting")
	if err := a.Start(ctx); err != nil {
		return err
	}

	events := make(protocol.EventChanAdapter, 8)
	if err := a.hub.Send(ctx, protocol.EventRegisterSubscriber{ClientID: bootstrapClientID, Sink: events}); err != nil {
		return err
	}
	defer a.hub.Send(ctx, protocol.EventUnregisterSubscriber{ClientID: bootstrapClientID})

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

	a.DefaultSessionID = session.SessionID
	a.DefaultWindowID = window.WindowID
	a.DefaultPaneID = pane.PaneID
	a.bootstrapped = true
	a.Logger.Info("shux: bootstrap default session ready")
	return nil
}

func (a *Shux) Start(ctx context.Context) error {
	if a.state == closed {
		return fmt.Errorf("shux: start after close")
	}
	if a.state == started {
		return nil
	}
	if a.state != new {
		panic("shux: invalid lifecycle state")
	}
	a.Logger.Info("shux: starting actors")
	actorCtx, cancel := context.WithCancel(ctx)
	hubRef := hub.Start(actorCtx)
	a.hub = hubRef
	a.supervisor = supervisor.StartWithHub(actorCtx, &hubRef)
	a.actorCancel = cancel
	a.state = started
	return nil
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
