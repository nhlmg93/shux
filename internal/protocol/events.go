package protocol

import (
	"context"
	"fmt"
)

type Event any

func ValidateEvent(event Event) error {
	switch e := event.(type) {
	case EventNoop:
		return nil
	case EventRegisterSubscriber:
		if !e.ClientID.Valid() {
			return fmt.Errorf("protocol: EventRegisterSubscriber: invalid ClientID")
		}
		if e.Sink == nil {
			return fmt.Errorf("protocol: EventRegisterSubscriber: nil Sink")
		}
		return nil
	case EventUnregisterSubscriber:
		if !e.ClientID.Valid() {
			return fmt.Errorf("protocol: EventUnregisterSubscriber: invalid ClientID")
		}
		return nil
	case EventSessionCreated:
		if !e.SessionID.Valid() {
			return fmt.Errorf("protocol: EventSessionCreated: invalid SessionID")
		}
		return nil
	case EventWindowCreated:
		if !e.SessionID.Valid() {
			return fmt.Errorf("protocol: EventWindowCreated: invalid SessionID")
		}
		if !e.WindowID.Valid() {
			return fmt.Errorf("protocol: EventWindowCreated: invalid WindowID")
		}
		return nil
	case EventPaneCreated:
		if !e.WindowID.Valid() {
			return fmt.Errorf("protocol: EventPaneCreated: invalid WindowID")
		}
		if !e.PaneID.Valid() {
			return fmt.Errorf("protocol: EventPaneCreated: invalid PaneID")
		}
		return nil
	case EventWindowLayoutChanged:
		if !e.SessionID.Valid() {
			return fmt.Errorf("protocol: EventWindowLayoutChanged: invalid SessionID")
		}
		if !e.WindowID.Valid() {
			return fmt.Errorf("protocol: EventWindowLayoutChanged: invalid WindowID")
		}
		if e.Cols <= 0 || e.Rows <= 0 {
			return fmt.Errorf("protocol: EventWindowLayoutChanged: invalid size %dx%d", e.Cols, e.Rows)
		}
		if e.ActivePane != "" && !e.ActivePane.Valid() {
			return fmt.Errorf("protocol: EventWindowLayoutChanged: invalid ActivePane")
		}
		for i, p := range e.Panes {
			if !p.PaneID.Valid() {
				return fmt.Errorf("protocol: EventWindowLayoutChanged: Panes[%d]: invalid PaneID", i)
			}
			if p.Cols <= 0 || p.Rows <= 0 {
				return fmt.Errorf("protocol: EventWindowLayoutChanged: Panes[%d]: invalid size", i)
			}
		}
		return nil
	default:
		return fmt.Errorf("protocol: unknown event type %T", event)
	}
}

// EventSink receives fanout copies of events from the hub (e.g. actor.Ref[Event] in internal/actor).
type EventSink interface {
	DeliverEvent(ctx context.Context, e Event) error
}

// EventRegisterSubscriber registers Sink under ClientID for hub fanout. Duplicate ClientID is an error (hub panics, matching Init).
type EventRegisterSubscriber struct {
	ClientID ClientID
	Sink     EventSink
}

// EventUnregisterSubscriber removes a fanout client registered with EventRegisterSubscriber.
type EventUnregisterSubscriber struct {
	ClientID ClientID
}

type EventNoop struct{}

// EventSessionCreated is emitted after a session actor exists for SessionID.
type EventSessionCreated struct {
	SessionID SessionID
}

// EventWindowCreated is emitted after a window exists under the session.
type EventWindowCreated struct {
	SessionID SessionID
	WindowID  WindowID
}

// EventPaneCreated is emitted after a pane exists under the window.
type EventPaneCreated struct {
	WindowID WindowID
	PaneID   PaneID
}

// EventLayoutPane is one pane’s placement in window cell space for UI snapshots.
type EventLayoutPane struct {
	PaneID PaneID
	Col    int
	Row    int
	Cols   int
	Rows   int
}

// EventWindowLayoutChanged is emitted when a window’s cell geometry changes
// (e.g. after resize or split). Hub fanout and publishers are wired separately.
type EventWindowLayoutChanged struct {
	SessionID  SessionID
	WindowID   WindowID
	Cols       int
	Rows       int
	ActivePane PaneID
	Panes      []EventLayoutPane
}
