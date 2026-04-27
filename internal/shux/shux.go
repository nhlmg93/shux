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
	cacheClientID     = protocol.ClientID("state-cache")
)

type (
	windowLayoutSnapshots map[protocol.WindowID]protocol.EventWindowLayoutChanged
	paneScreenSnapshots   map[protocol.PaneID]protocol.EventPaneScreenChanged

	layoutsBySession map[protocol.SessionID]windowLayoutSnapshots
	screensByWindow  map[protocol.WindowID]paneScreenSnapshots
	screensBySession map[protocol.SessionID]screensByWindow
)

type stateCache struct {
	mu      sync.Mutex
	layouts layoutsBySession
	screens screensBySession
}

func newStateCache() *stateCache {
	return &stateCache{
		layouts: make(layoutsBySession),
		screens: make(screensBySession),
	}
}

func (c *stateCache) DeliverEvent(_ context.Context, e protocol.Event) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	switch event := e.(type) {
	case protocol.EventWindowLayoutChanged:
		windows := c.layouts[event.SessionID]
		if windows == nil {
			windows = make(windowLayoutSnapshots)
			c.layouts[event.SessionID] = windows
		}
		windows[event.WindowID] = event
	case protocol.EventPaneScreenChanged:
		windows := c.screens[event.SessionID]
		if windows == nil {
			windows = make(screensByWindow)
			c.screens[event.SessionID] = windows
		}
		panes := windows[event.WindowID]
		if panes == nil {
			panes = make(paneScreenSnapshots)
			windows[event.WindowID] = panes
		}
		panes[event.PaneID] = event
	}
	return nil
}

func (c *stateCache) LayoutSnapshot(sessionID protocol.SessionID, windowID protocol.WindowID) (protocol.EventWindowLayoutChanged, bool) {
	if c == nil {
		return protocol.EventWindowLayoutChanged{}, false
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	windows := c.layouts[sessionID]
	if windows == nil {
		return protocol.EventWindowLayoutChanged{}, false
	}
	layout, ok := windows[windowID]
	return layout, ok
}

func (c *stateCache) ScreenSnapshots(sessionID protocol.SessionID, windowID protocol.WindowID) []protocol.EventPaneScreenChanged {
	if c == nil {
		return nil
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	windows := c.screens[sessionID]
	if windows == nil {
		return nil
	}
	panes := windows[windowID]
	if panes == nil {
		return nil
	}
	snapshots := make([]protocol.EventPaneScreenChanged, 0, len(panes))
	for _, screen := range panes {
		snapshots = append(snapshots, screen)
	}
	return snapshots
}

type Shux struct {
	Logger *Logger
	Config Config

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
	clientsMu    sync.Mutex
	clients      map[protocol.ClientID]*tea.Program
	cache        *stateCache
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
	exitIntent := ui.ExitDetach
	exitIntentMu := sync.Mutex{}
	setExitIntent := func(intent ui.ExitIntent) {
		exitIntentMu.Lock()
		exitIntent = intent
		exitIntentMu.Unlock()
	}
	model := ui.NewModelWithSupervisorAndExit(clientID, a.DefaultSessionID, a.DefaultWindowID, a.DefaultPaneID, a.supervisor, ctx, setExitIntent)
	if layout, ok := a.cache.LayoutSnapshot(a.DefaultSessionID, a.DefaultWindowID); ok {
		model = model.WithLayoutSnapshot(ui.LayoutSnapshotFromEvent(layout))
	}
	for _, screen := range a.cache.ScreenSnapshots(a.DefaultSessionID, a.DefaultWindowID) {
		model = model.WithPaneScreen(screen)
	}
	p := tea.NewProgram(model, opts...)
	if err := a.hub.Send(ctx, protocol.EventRegisterSubscriber{ClientID: clientID, Sink: &ui.ProgramEventSink{P: p}}); err != nil {
		return nil, nil, fmt.Errorf("shux: register ui hub: %w", err)
	}
	a.clientsMu.Lock()
	if _, exists := a.clients[clientID]; exists {
		a.clientsMu.Unlock()
		_ = a.hub.Send(ctx, protocol.EventUnregisterSubscriber{ClientID: clientID})
		return nil, nil, fmt.Errorf("shux: duplicate client id %q", clientID)
	}
	a.clients[clientID] = p
	a.clientsMu.Unlock()

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
	a.cache = newStateCache()
	if err := a.hub.Send(ctx, protocol.EventRegisterSubscriber{ClientID: cacheClientID, Sink: a.cache}); err != nil {
		cancel()
		return err
	}
	a.supervisor = supervisor.StartWithConfig(actorCtx, &hubRef, a.Config.ShellPath)
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
