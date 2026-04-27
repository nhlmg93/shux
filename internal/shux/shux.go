package shux

import (
	"context"
	"fmt"
	"sync"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/colorprofile"
	"shux/internal/actor"
	"shux/internal/hub"
	"shux/internal/protocol"
	"shux/internal/supervisor"
	"shux/internal/ui"
)

type lifecycle uint8

const (
	stateNew lifecycle = iota
	stateStarted
	stateReady
	stateClosed
)

const (
	bootstrapClientID = protocol.ClientID("bootstrap")
	cacheClientID     = protocol.ClientID("state-cache")
)

type Shux struct {
	Logger *Logger
	Config Config

	DefaultSessionID protocol.SessionID
	DefaultWindowID  protocol.WindowID
	DefaultPaneID    protocol.PaneID

	hub         actor.Ref[protocol.Event]
	supervisor  actor.Ref[protocol.Command]
	actorCancel context.CancelFunc
	cache       *stateCache

	stateMu sync.Mutex
	state   lifecycle

	shutdown     chan struct{}
	shutdownOnce sync.Once

	clientsMu sync.Mutex
	clients   map[protocol.ClientID]*tea.Program
}

func NewShux() (*Shux, error) {
	return NewShuxWithConfig(DefaultConfig())
}

func NewShuxWithConfig(config Config) (*Shux, error) {
	logger, err := NewLogger()
	if err != nil {
		return nil, fmt.Errorf("failed to init logger: %w", err)
	}

	return &Shux{
		Logger:   logger,
		Config:   config.WithDefaults(),
		shutdown: make(chan struct{}),
		clients:  make(map[protocol.ClientID]*tea.Program),
	}, nil
}

func (a *Shux) Done() <-chan struct{} {
	return a.shutdown
}

func (a *Shux) RequestShutdown() {
	a.shutdownOnce.Do(func() { close(a.shutdown) })
}

func (a *Shux) DetachAllClients() int {
	a.clientsMu.Lock()
	programs := make([]*tea.Program, 0, len(a.clients))
	for _, p := range a.clients {
		programs = append(programs, p)
	}
	a.clientsMu.Unlock()

	for _, p := range programs {
		p.Quit()
	}
	return len(programs)
}

func (a *Shux) Close() error {
	a.stateMu.Lock()
	if a.state == stateClosed {
		a.stateMu.Unlock()
		return nil
	}
	cancel := a.actorCancel
	a.actorCancel = nil
	a.state = stateClosed
	a.stateMu.Unlock()

	if cancel != nil {
		cancel()
	}
	return a.Logger.Close()
}

func (a *Shux) Run(opts ...tea.ProgramOption) error {
	if a.getState() == stateClosed {
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
	switch a.getState() {
	case stateClosed:
		return nil, nil, fmt.Errorf("shux: attach after close")
	case stateNew, stateStarted:
		return nil, nil, fmt.Errorf("shux: attach before bootstrap")
	}

	opts = append([]tea.ProgramOption{tea.WithContext(ctx)}, opts...)
	// shux renders nested terminal cells itself. Preserve truecolor cells instead
	// of letting Bubble Tea downsample them based on the SSH PTY environment
	// (which often lacks COLORTERM/RGB and can turn dark grays into ANSI blue).
	opts = append(opts, tea.WithColorProfile(colorprofile.TrueColor))
	exitIntent := ui.ExitDetach
	exitIntentMu := sync.Mutex{}
	setExitIntent := func(intent ui.ExitIntent) {
		exitIntentMu.Lock()
		exitIntent = intent
		exitIntentMu.Unlock()
	}
	model := ui.NewModel(ui.ModelConfig{
		ClientID:   clientID,
		SessionID:  a.DefaultSessionID,
		WindowID:   a.DefaultWindowID,
		PaneID:     a.DefaultPaneID,
		Supervisor: a.supervisor,
		Ctx:        ctx,
		OnExit:     setExitIntent,
	})
	if layout, ok := a.cache.LayoutSnapshot(a.DefaultSessionID, a.DefaultWindowID); ok {
		model = model.WithLayoutSnapshot(ui.LayoutSnapshotFromEvent(layout))
	}
	for _, screen := range a.cache.ScreenSnapshots(a.DefaultSessionID, a.DefaultWindowID) {
		model = model.WithPaneScreen(screen)
	}
	p := tea.NewProgram(model, opts...)

	a.clientsMu.Lock()
	if _, exists := a.clients[clientID]; exists {
		a.clientsMu.Unlock()
		return nil, nil, fmt.Errorf("shux: duplicate client id %q", clientID)
	}
	a.clients[clientID] = p
	a.clientsMu.Unlock()

	if err := a.hub.Send(ctx, protocol.EventRegisterSubscriber{ClientID: clientID, Sink: &ui.ProgramEventSink{P: p}}); err != nil {
		a.clientsMu.Lock()
		delete(a.clients, clientID)
		a.clientsMu.Unlock()
		return nil, nil, fmt.Errorf("shux: register ui hub: %w", err)
	}

	cleanup := func() {
		exitIntentMu.Lock()
		intent := exitIntent
		exitIntentMu.Unlock()

		a.clientsMu.Lock()
		delete(a.clients, clientID)
		remaining := len(a.clients)
		a.clientsMu.Unlock()

		_ = a.hub.Send(ctx, protocol.EventUnregisterSubscriber{ClientID: clientID})
		if intent == ui.ExitQuit && remaining == 0 {
			a.RequestShutdown()
		}
	}
	return p, cleanup, nil
}

func (a *Shux) BootstrapDefaultSession(ctx context.Context) error {
	switch a.getState() {
	case stateClosed:
		return fmt.Errorf("shux: bootstrap after close")
	case stateReady:
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

	session, err := bootstrapStep[protocol.EventSessionCreated](ctx, a.supervisor, events, protocol.CommandCreateSession{})
	if err != nil {
		return err
	}
	window, err := bootstrapStep[protocol.EventWindowCreated](ctx, a.supervisor, events, protocol.CommandCreateWindow{SessionID: session.SessionID})
	if err != nil {
		return err
	}
	pane, err := bootstrapStep[protocol.EventPaneCreated](ctx, a.supervisor, events, protocol.CommandCreatePane{SessionID: session.SessionID, WindowID: window.WindowID})
	if err != nil {
		return err
	}

	a.DefaultSessionID = session.SessionID
	a.DefaultWindowID = window.WindowID
	a.DefaultPaneID = pane.PaneID
	a.setState(stateReady)
	a.Logger.Info("shux: bootstrap default session ready")
	return nil
}

func (a *Shux) Start(ctx context.Context) error {
	a.stateMu.Lock()
	defer a.stateMu.Unlock()
	switch a.state {
	case stateClosed:
		return fmt.Errorf("shux: start after close")
	case stateStarted, stateReady:
		return nil
	case stateNew:
		// fall through
	default:
		panic("shux: invalid lifecycle state")
	}

	a.Logger.Info("shux: starting actors")
	actorCtx, cancel := context.WithCancel(ctx)
	hubRef := hub.Start(actorCtx)
	cache := newStateCache()
	if err := hubRef.Send(ctx, protocol.EventRegisterSubscriber{ClientID: cacheClientID, Sink: cache}); err != nil {
		cancel()
		return err
	}
	a.hub = hubRef
	a.cache = cache
	a.supervisor = supervisor.StartWithConfig(actorCtx, &hubRef, a.Config.ShellPath)
	a.actorCancel = cancel
	a.state = stateStarted
	return nil
}

func (a *Shux) getState() lifecycle {
	a.stateMu.Lock()
	defer a.stateMu.Unlock()
	return a.state
}

func (a *Shux) setState(s lifecycle) {
	a.stateMu.Lock()
	a.state = s
	a.stateMu.Unlock()
}

func bootstrapStep[T protocol.Event](ctx context.Context, super actor.Ref[protocol.Command], events <-chan protocol.Event, cmd protocol.Command) (T, error) {
	var zero T
	if err := super.Send(ctx, cmd); err != nil {
		return zero, err
	}
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
