package pane

import (
	"context"
	"fmt"

	"shux/internal/actor"
	"shux/internal/protocol"
)

// Actor runs a single pane. Terminal owns the shell process, PTY, VT, render
// state, and screen revision; Actor owns command serialization and hub fanout.
type Actor struct {
	Hub       actor.EventRef
	Terminal  *Terminal
	SessionID protocol.SessionID
	WindowID  protocol.WindowID
	PaneID    protocol.PaneID
}

// NewActor returns a pane actor. Terminal is initialized with dimensions by
// CommandPaneInit.
func NewActor() *Actor {
	return NewActorWithConfig(nil, "", "", "", "/bin/sh")
}

func NewActorWithConfig(hub actor.EventRef, sessionID protocol.SessionID, windowID protocol.WindowID, paneID protocol.PaneID, shellPath string) *Actor {
	if hub != nil && !hub.Valid() {
		panic("pane: NewActor: invalid hub ref")
	}
	return &Actor{
		Hub:       hub,
		Terminal:  NewTerminal(shellPath),
		SessionID: sessionID,
		WindowID:  windowID,
		PaneID:    paneID,
	}
}

func (a *Actor) sendScreen(ctx context.Context, event protocol.EventPaneScreenChanged) {
	if a.Hub == nil {
		return
	}
	event.SessionID = a.SessionID
	event.WindowID = a.WindowID
	event.PaneID = a.PaneID
	_ = a.Hub.Send(ctx, event)
}

func (a *Actor) closeResources() {
	if a.Terminal != nil {
		a.Terminal.Close()
	}
}

func (a *Actor) Run(ctx context.Context, self actor.Ref[protocol.Command], inbox <-chan protocol.Command) {
	defer a.closeResources()
	for {
		select {
		case <-ctx.Done():
			return
		case msg := <-inbox:
			if output, ok := msg.(ptyOutput); ok {
				if event, emit := a.Terminal.FeedOutput(output); emit {
					a.sendScreen(ctx, event)
				}
				continue
			}
			if err := protocol.ValidateCommand(msg); err != nil {
				panic(err)
			}
			switch msg := msg.(type) {
			case protocol.CommandNoop:
			case protocol.CommandPaneInit:
				a.sendScreen(ctx, a.Terminal.Init(ctx, self, msg.Cols, msg.Rows))
			case protocol.CommandPaneResize:
				a.sendScreen(ctx, a.Terminal.Resize(msg.Cols, msg.Rows))
			case protocol.CommandPaneKey:
				a.Terminal.HandleKey(msg)
			case protocol.CommandPaneMouse:
				a.Terminal.HandleMouse(msg)
			case protocol.CommandPaneClose:
				return
			case protocol.CommandPanePaste:
				a.Terminal.HandlePaste(msg)
			default:
				panic(fmt.Sprintf("pane: unhandled command type %T", msg))
			}
		}
	}
}

func Start(ctx context.Context) actor.Ref[protocol.Command] {
	return actor.Start[protocol.Command](ctx, NewActor().Run)
}

func StartWithConfig(ctx context.Context, hub actor.EventRef, sessionID protocol.SessionID, windowID protocol.WindowID, paneID protocol.PaneID, shellPath string) actor.Ref[protocol.Command] {
	return actor.Start[protocol.Command](ctx, NewActorWithConfig(hub, sessionID, windowID, paneID, shellPath).Run)
}
