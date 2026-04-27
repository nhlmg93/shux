package session

import (
	"context"
	"fmt"
	"strconv"

	"shux/internal/actor"
	"shux/internal/protocol"
	"shux/internal/window"
)

// Windows is keyed window lifecycle bookkeeping (Init / Delete / Must on command refs).
type Windows = *actor.Lifecycle[protocol.WindowID, protocol.Command]

type Actor struct {
	Windows
	SessionID protocol.SessionID
	ShellPath string
	seq       uint64 // next window id suffix; only touched from Run goroutine
	revision  uint64
	windowIDs []protocol.WindowID
	hub       actor.EventRef // optional lifecycle event sink (best-effort publish)
}

func NewActor(hub actor.EventRef) *Actor {
	return NewActorWithConfig(hub, "", "/bin/sh")
}

func NewActorWithConfig(hub actor.EventRef, sessionID protocol.SessionID, shellPath string) *Actor {
	if hub != nil && !hub.Valid() {
		panic("session: NewActor: invalid hub ref")
	}
	if shellPath == "" {
		panic("session: NewActor: empty shell path")
	}
	return &Actor{
		Windows:   actor.NewLifecycle[protocol.WindowID, protocol.Command]("session", "window", protocol.WindowID.Valid),
		SessionID: sessionID,
		ShellPath: shellPath,
		hub:       hub,
	}
}

func (a *Actor) Run(ctx context.Context, _ actor.Ref[protocol.Command], inbox <-chan protocol.Command) {
	for {
		select {
		case <-ctx.Done():
			return
		case msg := <-inbox:
			if err := protocol.ValidateCommand(msg); err != nil {
				panic(err)
			}
			switch m := msg.(type) {
			case protocol.CommandNoop:
				continue
			case protocol.CommandCreateWindow:
				a.handleCreateWindow(ctx, m)
				continue
			}
			if wid, ok := protocol.RouteWindowID(msg); ok {
				_ = a.Windows.Must(wid).Send(ctx, msg)
				continue
			}
			panic(fmt.Sprintf("session: unhandled command type %T", msg))
		}
	}
}

func (a *Actor) handleCreateWindow(ctx context.Context, m protocol.CommandCreateWindow) {
	a.seq++
	wid := protocol.WindowID("w-" + strconv.FormatUint(a.seq, 10))
	a.Init(wid, window.StartWithConfig(ctx, a.hub, m.SessionID, wid, a.ShellPath))
	a.windowIDs = append(a.windowIDs, wid)
	a.revision++
	if a.hub != nil {
		_ = a.hub.Send(ctx, protocol.EventWindowCreated{
			ClientID:  m.Meta.ClientID,
			RequestID: m.Meta.RequestID,
			SessionID: m.SessionID,
			WindowID:  wid,
		})
		a.emitWindowsChanged(ctx, m.SessionID)
	}
	if m.AutoPane {
		cols, rows := m.Cols, m.Rows
		if cols == 0 || rows == 0 {
			cols, rows = 80, 24
		}
		_ = a.Windows.Must(wid).Send(ctx, protocol.CommandWindowResize{SessionID: m.SessionID, WindowID: wid, Cols: cols, Rows: rows})
		_ = a.Windows.Must(wid).Send(ctx, protocol.CommandCreatePane{SessionID: m.SessionID, WindowID: wid})
	}
}

func (a *Actor) emitWindowsChanged(ctx context.Context, sessionID protocol.SessionID) {
	if a.hub == nil {
		return
	}
	windows := append([]protocol.WindowID(nil), a.windowIDs...)
	_ = a.hub.Send(ctx, protocol.EventSessionWindowsChanged{SessionID: sessionID, Revision: a.revision, Windows: windows})
}

func Start(ctx context.Context) actor.Ref[protocol.Command] {
	return StartWithHub(ctx, nil)
}

// StartWithHub is [Start] with optional hub; lifecycle events are best-effort when hub is non-nil.
func StartWithHub(ctx context.Context, hub actor.EventRef) actor.Ref[protocol.Command] {
	return actor.Start[protocol.Command](ctx, NewActor(hub).Run)
}

func StartWithConfig(ctx context.Context, hub actor.EventRef, sessionID protocol.SessionID, shellPath string) actor.Ref[protocol.Command] {
	return actor.Start[protocol.Command](ctx, NewActorWithConfig(hub, sessionID, shellPath).Run)
}
