package shux

import (
	"context"
	"time"

	"shux/internal/protocol"
)

const sessionLifecycleClientID = protocol.ClientID("session-lifecycle")

type sessionLifecycleWatcher struct {
	app *Shux
}

func newSessionLifecycleWatcher(app *Shux) *sessionLifecycleWatcher {
	return &sessionLifecycleWatcher{app: app}
}

func (w *sessionLifecycleWatcher) DeliverEvent(_ context.Context, e protocol.Event) error {
	closed, ok := e.(protocol.EventSessionClosed)
	if !ok || w == nil || w.app == nil {
		return nil
	}
	go w.handleSessionClosed(closed)
	return nil
}

func (w *sessionLifecycleWatcher) handleSessionClosed(closed protocol.EventSessionClosed) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	w.app.sessionEnv.RemoveSession(closed.SessionID)

	w.app.stateMu.Lock()
	if w.app.DefaultSessionID == closed.SessionID {
		w.app.DefaultSessionID = ""
		w.app.DefaultSession = ""
		w.app.DefaultWindowID = ""
		w.app.DefaultPaneID = ""
	}
	w.app.stateMu.Unlock()

	sessions, err := w.app.ListSessions(ctx)
	if err != nil {
		w.app.Logger.Printf("shux: session closed list sessions failed: %v", err)
		return
	}
	if len(sessions) > 0 {
		w.app.stateMu.Lock()
		if w.app.DefaultSessionID == "" {
			w.app.DefaultSessionID = sessions[0].SessionID
			w.app.DefaultSession = sessions[0].Name
		}
		w.app.stateMu.Unlock()
		if w.app.checkpoints != nil {
			w.app.checkpoints.schedule()
		}
		return
	}

	w.app.checkpoint()
	w.app.DetachAllClients()
	w.app.RequestShutdown()
}
