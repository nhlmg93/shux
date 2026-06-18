package session

import (
	"context"
	"fmt"
	"strconv"

	"shux/internal/actor"
	"shux/internal/cfg"
	"shux/internal/protocol"
	"shux/internal/window"
)

// Windows is keyed window lifecycle bookkeeping (Init / Delete / Must on command refs).
type Windows = *actor.Lifecycle[protocol.WindowID, protocol.Command]

type Actor struct {
	Windows
	SessionID     protocol.SessionID
	SupervisorRef actor.Ref[protocol.Command]
	Policy        cfg.Config
	WindowNames   map[protocol.WindowID]string
	seq           uint64
	revision      uint64
	windowIDs     []protocol.WindowID
	hub           actor.EventRef
	closing       bool
}

func NewActor(hub actor.EventRef) *Actor {
	return NewActorWithPolicy(actor.Ref[protocol.Command]{}, hub, "", cfg.DefaultConfig())
}

func NewActorWithPolicy(supervisor actor.Ref[protocol.Command], hub actor.EventRef, sessionID protocol.SessionID, policy cfg.Config) *Actor {
	if hub != nil && !hub.Valid() {
		panic("session: NewActor: invalid hub ref")
	}
	policy = policy.WithDefaults()
	if policy.ShellPath == "" {
		panic("session: NewActor: empty shell path")
	}
	return &Actor{
		Windows:       actor.NewLifecycle[protocol.WindowID, protocol.Command]("session", "window", protocol.WindowID.Valid),
		SessionID:     sessionID,
		SupervisorRef: supervisor,
		Policy:        policy,
		WindowNames:   make(map[protocol.WindowID]string),
		hub:           hub,
	}
}

func (a *Actor) Run(ctx context.Context, self actor.Ref[protocol.Command], inbox <-chan protocol.Command) {
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
				a.handleCreateWindow(ctx, self, m)
				continue
			case protocol.CommandWindowClosed:
				a.handleWindowClosed(ctx, m)
				if len(a.windowIDs) == 0 && !a.closing {
					return
				}
				continue
			case protocol.CommandPaneMove:
				a.handlePaneMove(ctx, self, m)
				continue
			case protocol.CommandWindowRename:
				a.WindowNames[m.WindowID] = m.Name
			case protocol.CommandSessionKill:
				err := a.handleSessionKill(ctx, m)
				if m.Reply != nil {
					m.Reply <- err
				}
				return
			}
			if wid, ok := protocol.RouteWindowID(msg); ok {
				if !a.hasWindow(wid) {
					continue
				}
				_ = a.Windows.Must(wid).Send(ctx, msg)
				continue
			}
			panic(fmt.Sprintf("session: unhandled command type %T", msg))
		}
	}
}

func (a *Actor) handleCreateWindow(ctx context.Context, self actor.Ref[protocol.Command], m protocol.CommandCreateWindow) protocol.WindowID {
	a.seq++
	wid := protocol.WindowID("w-" + strconv.FormatUint(a.seq, 10))
	a.windowIDs = append(a.windowIDs, wid)
	a.WindowNames[wid] = ""
	ordinal := len(a.windowIDs)
	a.Init(wid, window.StartWithPolicy(ctx, self, a.hub, m.SessionID, wid, ordinal, a.Policy))
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
	return wid
}

func (a *Actor) handlePaneMove(ctx context.Context, self actor.Ref[protocol.Command], m protocol.CommandPaneMove) {
	targetWindowID := m.TargetWindowID
	if !targetWindowID.Valid() {
		targetWindowID = a.handleCreateWindow(ctx, self, protocol.CommandCreateWindow{SessionID: m.SessionID})
	}
	if targetWindowID == m.SourceWindowID {
		panic("session: pane move: source and target windows are identical")
	}
	detachReply := make(chan window.DetachPaneResult, 1)
	if err := a.Windows.Must(m.SourceWindowID).Send(ctx, window.CommandDetachPane{
		SessionID: m.SessionID,
		WindowID:  m.SourceWindowID,
		PaneID:    m.PaneID,
		Reply:     detachReply,
	}); err != nil {
		panic(fmt.Sprintf("session: pane move detach send: %v", err))
	}
	var detached window.DetachPaneResult
	select {
	case <-ctx.Done():
		return
	case detached = <-detachReply:
	}
	if detached.Err != nil {
		panic(detached.Err)
	}
	attachReply := make(chan error, 1)
	if err := a.Windows.Must(targetWindowID).Send(ctx, window.CommandAttachPane{
		SessionID: m.SessionID,
		WindowID:  targetWindowID,
		Transfer:  detached.Transfer,
		Reply:     attachReply,
	}); err != nil {
		panic(fmt.Sprintf("session: pane move attach send: %v", err))
	}
	select {
	case <-ctx.Done():
		return
	case err := <-attachReply:
		if err != nil {
			panic(err)
		}
	}
}

func (a *Actor) handleWindowClosed(ctx context.Context, m protocol.CommandWindowClosed) {
	removed := false
	for i, wid := range a.windowIDs {
		if wid == m.WindowID {
			a.windowIDs = append(a.windowIDs[:i], a.windowIDs[i+1:]...)
			removed = true
			break
		}
	}
	if !removed {
		panic("session: close window: missing window id")
	}
	a.Windows.Delete(m.WindowID)
	delete(a.WindowNames, m.WindowID)
	a.revision++
	a.emitWindowsChanged(ctx, m.SessionID)
	if len(a.windowIDs) == 0 && a.SupervisorRef.Valid() {
		_ = a.SupervisorRef.Send(ctx, protocol.CommandSessionEnded{SessionID: a.SessionID})
		return
	}
}

func (a *Actor) handleSessionKill(ctx context.Context, m protocol.CommandSessionKill) error {
	a.closing = true
	wids := append([]protocol.WindowID(nil), a.windowIDs...)
	for _, wid := range wids {
		reply := make(chan struct{}, 1)
		if err := a.Windows.Must(wid).Send(ctx, protocol.CommandKillWindow{
			SessionID: a.SessionID,
			WindowID:  wid,
			Reply:     reply,
		}); err != nil {
			return err
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-reply:
		}
	}
	if a.SupervisorRef.Valid() {
		if err := a.SupervisorRef.Send(ctx, protocol.CommandSessionEnded{SessionID: a.SessionID}); err != nil {
			return err
		}
	}
	return nil
}

func (a *Actor) emitWindowsChanged(ctx context.Context, sessionID protocol.SessionID) {
	if a.hub == nil {
		return
	}
	windows := append([]protocol.WindowID(nil), a.windowIDs...)
	_ = a.hub.Send(ctx, protocol.EventSessionWindowsChanged{SessionID: sessionID, Revision: a.revision, Windows: windows})
}

func (a *Actor) hasWindow(windowID protocol.WindowID) bool {
	for _, wid := range a.windowIDs {
		if wid == windowID {
			return true
		}
	}
	return false
}

func Start(ctx context.Context) actor.Ref[protocol.Command] {
	return StartWithHub(ctx, nil)
}

// StartWithHub is [Start] with optional hub; lifecycle events are best-effort when hub is non-nil.
func StartWithHub(ctx context.Context, hub actor.EventRef) actor.Ref[protocol.Command] {
	return actor.Start[protocol.Command](ctx, NewActor(hub).Run)
}

func StartWithPolicy(ctx context.Context, supervisor actor.Ref[protocol.Command], hub actor.EventRef, sessionID protocol.SessionID, policy cfg.Config) actor.Ref[protocol.Command] {
	return actor.Start[protocol.Command](ctx, NewActorWithPolicy(supervisor, hub, sessionID, policy).Run)
}
