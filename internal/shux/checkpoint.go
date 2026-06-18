package shux

import (
	"context"
	"sync"
	"time"

	"shux/internal/protocol"
)

const (
	checkpointClientID  = protocol.ClientID("resurrection-checkpoint")
	checkpointDebounce  = 100 * time.Millisecond
)

type checkpointWatcher struct {
	app   *Shux
	mu    sync.Mutex
	timer *time.Timer
}

func newCheckpointWatcher(app *Shux) *checkpointWatcher {
	return &checkpointWatcher{app: app}
}

func (w *checkpointWatcher) DeliverEvent(_ context.Context, e protocol.Event) error {
	switch e.(type) {
	case protocol.EventWindowLayoutChanged,
		protocol.EventSessionWindowsChanged,
		protocol.EventWindowCreated:
		w.schedule()
	}
	return nil
}

func (w *checkpointWatcher) schedule() {
	if w == nil || w.app == nil {
		return
	}
	if w.app.getState() != stateReady {
		return
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.timer != nil {
		w.timer.Stop()
	}
	w.timer = time.AfterFunc(checkpointDebounce, func() {
		w.app.checkpoint()
	})
}

func (w *checkpointWatcher) stop() {
	if w == nil {
		return
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.timer != nil {
		w.timer.Stop()
		w.timer = nil
	}
}
