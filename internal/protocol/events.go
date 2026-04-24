package protocol

import "context"

type Event any

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
