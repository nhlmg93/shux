package cfg

import (
	"context"
	"sync"
)

// AutocmdEvent names hub lifecycle events exposed to Lua plugins.
type AutocmdEvent string

const (
	EventDaemonStarted       AutocmdEvent = "DaemonStarted"
	EventClientAttached      AutocmdEvent = "ClientAttached"
	EventClientDetached      AutocmdEvent = "ClientDetached"
	EventPaneCreated         AutocmdEvent = "PaneCreated"
	EventPaneClosed          AutocmdEvent = "PaneClosed"
	EventPaneRenamed         AutocmdEvent = "PaneRenamed"
	EventWindowCreated       AutocmdEvent = "WindowCreated"
	EventWindowClosed        AutocmdEvent = "WindowClosed"
	EventWindowRenamed       AutocmdEvent = "WindowRenamed"
	EventWindowLayoutChanged AutocmdEvent = "WindowLayoutChanged"
)

// AutocmdCallback is invoked when a subscribed event fires.
type AutocmdCallback func(ctx context.Context, data map[string]any)

// AutocmdRegistry holds lifecycle callbacks registered from Lua or Go.
type AutocmdRegistry struct {
	mu    sync.Mutex
	subs  map[AutocmdEvent][]AutocmdCallback
	queue chan queuedAutocmd
}

type queuedAutocmd struct {
	event AutocmdEvent
	data  map[string]any
}

func NewAutocmdRegistry() *AutocmdRegistry {
	r := &AutocmdRegistry{
		subs:  make(map[AutocmdEvent][]AutocmdCallback),
		queue: make(chan queuedAutocmd, 64),
	}
	go r.run()
	return r
}

func (r *AutocmdRegistry) Subscribe(event AutocmdEvent, cb AutocmdCallback) {
	if cb == nil {
		return
	}
	r.mu.Lock()
	r.subs[event] = append(r.subs[event], cb)
	r.mu.Unlock()
}

func (r *AutocmdRegistry) Emit(_ context.Context, event AutocmdEvent, data map[string]any) {
	select {
	case r.queue <- queuedAutocmd{event: event, data: data}:
	default:
	}
}

func (r *AutocmdRegistry) run() {
	for q := range r.queue {
		r.mu.Lock()
		cbs := append([]AutocmdCallback(nil), r.subs[q.event]...)
		r.mu.Unlock()
		ctx := context.Background()
		for _, cb := range cbs {
			cb(ctx, q.data)
		}
	}
}
