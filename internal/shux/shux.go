package shux

import (
	"context"
	"fmt"
	"sync"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/colorprofile"
	"shux/internal/actor"
	"shux/internal/hub"
	"shux/internal/luabind"
	"shux/internal/persist"
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

const bootstrapClientID = protocol.ClientID("bootstrap")
const cacheClientID = protocol.ClientID("state-cache")

type Shux struct {
	Logger *Logger
	Config Config

	DefaultSessionID protocol.SessionID
	DefaultSession   string
	DefaultWindowID  protocol.WindowID
	DefaultPaneID    protocol.PaneID

	hub          actor.Ref[protocol.Event]
	supervisor   actor.Ref[protocol.Command]
	actorCancel  context.CancelFunc
	cache        *stateCache
	bootstrapReq protocol.RequestID
	luaRuntime   luabind.Runtime // kept alive for plugin autocmds

	Autocmds *AutocmdRegistry

	stateMu sync.Mutex
	state   lifecycle

	shutdown     chan struct{}
	shutdownOnce sync.Once

	restartShutdown func(context.Context) error

	clientsMu sync.Mutex
	clients   map[protocol.ClientID]*tea.Program

	checkpoints *checkpointWatcher
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

func (a *Shux) SetLuaRuntime(rt luabind.Runtime) {
	a.luaRuntime = rt
}

func (a *Shux) SetAutocmds(r *AutocmdRegistry) {
	a.Autocmds = r
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

func (a *Shux) SetRestartShutdown(fn func(context.Context) error) {
	a.restartShutdown = fn
}

func (a *Shux) Close() error {
	a.checkpoint()
	if a.checkpoints != nil {
		a.checkpoints.stop()
	}
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
	if a.luaRuntime != nil {
		a.luaRuntime.Close()
	}
	a.luaRuntime = nil
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
	return a.NewClientProgramForSession(ctx, clientID, a.DefaultSessionID, opts...)
}

func (a *Shux) NewClientProgramForSession(ctx context.Context, clientID protocol.ClientID, sessionID protocol.SessionID, opts ...tea.ProgramOption) (*tea.Program, func(), error) {
	if clientID == "" {
		return nil, nil, fmt.Errorf("shux: empty client id")
	}
	if !sessionID.Valid() {
		return nil, nil, fmt.Errorf("shux: invalid session id")
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
	windowIDs := a.cache.WindowIDs(sessionID)
	if len(windowIDs) == 0 {
		return nil, nil, fmt.Errorf("shux: session %q has no windows", sessionID)
	}
	windowID := windowIDs[0]
	layout, ok := a.cache.LayoutSnapshot(sessionID, windowID)
	if !ok || len(layout.Panes) == 0 {
		return nil, nil, fmt.Errorf("shux: session %q has no panes", sessionID)
	}
	paneID := layout.Panes[0].PaneID
	model := ui.NewModel(ui.ModelConfig{
		ClientID:               clientID,
		SessionID:              sessionID,
		WindowID:               windowID,
		PaneID:                 paneID,
		Supervisor:             a.supervisor,
		Ctx:                    ctx,
		OnExit:                 setExitIntent,
		MapLeader:              a.Config.MapLeader,
		Keymaps:                a.Config.Keymaps,
		Lua:                    a.luaRuntime,
		PaneQuickSelectTimeout: a.Config.PaneQuickSelectTimeout,
	})
	model = model.WithWindowIDs(windowIDs)
	for _, windowID := range windowIDs {
		if layout, ok := a.cache.LayoutSnapshot(sessionID, windowID); ok {
			model = model.WithLayoutSnapshot(ui.LayoutSnapshotFromEvent(layout))
		}
		for _, screen := range a.cache.ScreenSnapshots(sessionID, windowID) {
			model = model.WithPaneScreen(screen)
		}
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
	if a.Autocmds != nil {
		a.Autocmds.Emit(ctx, EventClientAttached, map[string]any{"client_id": string(clientID)})
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
		if a.Autocmds != nil {
			a.Autocmds.Emit(ctx, EventClientDetached, map[string]any{"client_id": string(clientID)})
		}
		if a.checkpoints != nil {
			a.checkpoints.schedule()
		}
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

	if a.Config.Resurrection && a.Config.StateDir != "" {
		m, ok, err := persist.LoadManifest(a.Config.StateDir)
		if err != nil {
			a.Logger.Printf("shux: manifest load failed: %v", err)
		} else if ok {
			if err := a.restoreFromManifest(ctx, m); err != nil {
				a.Logger.Printf("shux: restore failed, fresh bootstrap: %v", err)
				_ = persist.ClearResurrectionState(a.Config.StateDir)
			} else {
				return nil
			}
		}
	}

	return a.bootstrapFresh(ctx)
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
	watcher := newCheckpointWatcher(a)
	bridge := newAutocmdBridge(a)
	if a.Config.Resurrection && a.Config.StateDir != "" {
		if err := hubRef.Send(ctx, protocol.EventRegisterSubscriber{ClientID: checkpointClientID, Sink: watcher}); err != nil {
			cancel()
			return err
		}
	}
	if err := hubRef.Send(ctx, protocol.EventRegisterSubscriber{ClientID: autocmdBridgeClientID, Sink: bridge}); err != nil {
		cancel()
		return err
	}
	a.checkpoints = watcher
	a.supervisor = supervisor.StartWithPolicy(actorCtx, &hubRef, a.Config)
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
	return bootstrapWait[T](ctx, events)
}

func bootstrapWait[T protocol.Event](ctx context.Context, events <-chan protocol.Event) (T, error) {
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
