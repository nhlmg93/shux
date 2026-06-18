package pane

import (
	"context"
	"fmt"
	"os"

	"shux/internal/actor"
	"shux/internal/cfg"
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
	return NewActorWithPolicy(nil, "", "", 1, "", cfg.Config{ShellPath: "/bin/sh"}.WithDefaults())
}

func NewActorWithPolicy(hub actor.EventRef, sessionID protocol.SessionID, windowID protocol.WindowID, windowOrdinal int, paneID protocol.PaneID, policy cfg.Config) *Actor {
	if hub != nil && !hub.Valid() {
		panic("pane: NewActor: invalid hub ref")
	}
	policy = policy.WithDefaults()
	return &Actor{
		Hub:       hub,
		Terminal:  NewTerminal(policy, windowOrdinal, paneID),
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

func (a *Actor) logTerminalErr(err error) {
	fmt.Fprintf(os.Stderr, "pane %s/%s/%s: %v\n", a.SessionID, a.WindowID, a.PaneID, err)
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
			if _, ok := msg.(journalReplay); ok {
				event, err := a.Terminal.ReplayJournalScreen()
				if err != nil {
					a.logTerminalErr(err)
					continue
				}
				a.sendScreen(ctx, event)
				continue
			}
			if output, ok := msg.(ptyOutput); ok {
				event, emit, err := a.Terminal.FeedOutput(output)
				if err != nil {
					a.logTerminalErr(err)
					continue
				}
				if emit {
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
				event, err := a.Terminal.Init(ctx, self, msg.Cols, msg.Rows)
				if err != nil {
					a.logTerminalErr(err)
					return
				}
				a.sendScreen(ctx, event)
			case protocol.CommandPaneResize:
				event, err := a.Terminal.Resize(msg.Cols, msg.Rows)
				if err != nil {
					a.logTerminalErr(err)
					continue
				}
				a.sendScreen(ctx, event)
			case protocol.CommandPaneKey:
				if err := a.Terminal.HandleKey(msg); err != nil {
					a.logTerminalErr(err)
				}
			case protocol.CommandPaneMouse:
				if err := a.Terminal.HandleMouse(msg); err != nil {
					a.logTerminalErr(err)
				}
			case protocol.CommandPaneClose:
				return
			case protocol.CommandPanePaste:
				if err := a.Terminal.HandlePaste(msg); err != nil {
					a.logTerminalErr(err)
				}
			case protocol.CommandPaneScroll:
				event, err := a.Terminal.Scroll(msg.Delta)
				if err != nil {
					a.logTerminalErr(err)
					continue
				}
				a.sendScreen(ctx, event)
			default:
				panic(fmt.Sprintf("pane: unhandled command type %T", msg))
			}
		}
	}
}

func Start(ctx context.Context) actor.Ref[protocol.Command] {
	return actor.Start[protocol.Command](ctx, NewActor().Run)
}

func StartWithPolicy(ctx context.Context, hub actor.EventRef, sessionID protocol.SessionID, windowID protocol.WindowID, windowOrdinal int, paneID protocol.PaneID, policy cfg.Config) actor.Ref[protocol.Command] {
	return actor.Start[protocol.Command](ctx, NewActorWithPolicy(hub, sessionID, windowID, windowOrdinal, paneID, policy).Run)
}
