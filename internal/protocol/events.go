package protocol

import (
	"context"
	"fmt"
)

const (
	MaxPaneScreenCols = 512
	MaxPaneScreenRows = 512
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
	case EventPaneClosed:
		if !e.WindowID.Valid() {
			return fmt.Errorf("protocol: EventPaneClosed: invalid WindowID")
		}
		if !e.PaneID.Valid() {
			return fmt.Errorf("protocol: EventPaneClosed: invalid PaneID")
		}
		return nil
	case EventPaneCloseLastRequested:
		if !e.ClientID.Valid() {
			return fmt.Errorf("protocol: EventPaneCloseLastRequested: invalid ClientID")
		}
		if !e.RequestID.Valid() {
			return fmt.Errorf("protocol: EventPaneCloseLastRequested: invalid RequestID")
		}
		if !e.SessionID.Valid() {
			return fmt.Errorf("protocol: EventPaneCloseLastRequested: invalid SessionID")
		}
		if !e.WindowID.Valid() {
			return fmt.Errorf("protocol: EventPaneCloseLastRequested: invalid WindowID")
		}
		if !e.PaneID.Valid() {
			return fmt.Errorf("protocol: EventPaneCloseLastRequested: invalid PaneID")
		}
		return nil
	case EventWindowLayoutChanged:
		if !e.SessionID.Valid() {
			return fmt.Errorf("protocol: EventWindowLayoutChanged: invalid SessionID")
		}
		if !e.WindowID.Valid() {
			return fmt.Errorf("protocol: EventWindowLayoutChanged: invalid WindowID")
		}
		if e.Revision == 0 {
			return fmt.Errorf("protocol: EventWindowLayoutChanged: invalid Revision")
		}
		if e.Cols <= 0 || e.Rows <= 0 {
			return fmt.Errorf("protocol: EventWindowLayoutChanged: invalid size %dx%d", e.Cols, e.Rows)
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
	case EventPaneScreenChanged:
		if !e.SessionID.Valid() {
			return fmt.Errorf("protocol: EventPaneScreenChanged: invalid SessionID")
		}
		if !e.WindowID.Valid() {
			return fmt.Errorf("protocol: EventPaneScreenChanged: invalid WindowID")
		}
		if !e.PaneID.Valid() {
			return fmt.Errorf("protocol: EventPaneScreenChanged: invalid PaneID")
		}
		if e.Revision == 0 {
			return fmt.Errorf("protocol: EventPaneScreenChanged: invalid Revision")
		}
		if e.Cols <= 0 || e.Rows <= 0 || e.Cols > MaxPaneScreenCols || e.Rows > MaxPaneScreenRows {
			return fmt.Errorf("protocol: EventPaneScreenChanged: invalid size %dx%d", e.Cols, e.Rows)
		}
		if err := e.Cursor.Validate(e.Cols, e.Rows); err != nil {
			return fmt.Errorf("protocol: EventPaneScreenChanged: %w", err)
		}
		if len(e.Lines) > e.Rows {
			return fmt.Errorf("protocol: EventPaneScreenChanged: too many lines %d > %d", len(e.Lines), e.Rows)
		}
		for i, line := range e.Lines {
			if len(line.Cells) > e.Cols {
				return fmt.Errorf("protocol: EventPaneScreenChanged: Lines[%d]: too many cells %d > %d", i, len(line.Cells), e.Cols)
			}
		}
		return nil
	case EventPaneSplitCompleted:
		if !e.ClientID.Valid() {
			return fmt.Errorf("protocol: EventPaneSplitCompleted: invalid ClientID")
		}
		if !e.RequestID.Valid() {
			return fmt.Errorf("protocol: EventPaneSplitCompleted: invalid RequestID")
		}
		if !e.SessionID.Valid() {
			return fmt.Errorf("protocol: EventPaneSplitCompleted: invalid SessionID")
		}
		if !e.WindowID.Valid() {
			return fmt.Errorf("protocol: EventPaneSplitCompleted: invalid WindowID")
		}
		if !e.TargetPaneID.Valid() {
			return fmt.Errorf("protocol: EventPaneSplitCompleted: invalid TargetPaneID")
		}
		if !e.NewPaneID.Valid() {
			return fmt.Errorf("protocol: EventPaneSplitCompleted: invalid NewPaneID")
		}
		if e.Revision == 0 {
			return fmt.Errorf("protocol: EventPaneSplitCompleted: invalid Revision")
		}
		return nil
	case EventCommandRejected:
		if !e.ClientID.Valid() {
			return fmt.Errorf("protocol: EventCommandRejected: invalid ClientID")
		}
		if !e.RequestID.Valid() {
			return fmt.Errorf("protocol: EventCommandRejected: invalid RequestID")
		}
		if !e.SessionID.Valid() {
			return fmt.Errorf("protocol: EventCommandRejected: invalid SessionID")
		}
		if !e.WindowID.Valid() {
			return fmt.Errorf("protocol: EventCommandRejected: invalid WindowID")
		}
		if e.Command == "" {
			return fmt.Errorf("protocol: EventCommandRejected: empty Command")
		}
		if e.Reason == "" {
			return fmt.Errorf("protocol: EventCommandRejected: empty Reason")
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

// EventPaneClosed is emitted after a pane is removed from its window.
type EventPaneClosed struct {
	WindowID WindowID
	PaneID   PaneID
}

// EventPaneCloseLastRequested asks the originating client to exit/quit because
// closing the final pane is equivalent to quitting the session.
type EventPaneCloseLastRequested struct {
	ClientID  ClientID
	RequestID RequestID
	SessionID SessionID
	WindowID  WindowID
	PaneID    PaneID
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
	SessionID SessionID
	WindowID  WindowID
	Revision  uint64
	Cols      int
	Rows      int
	Panes     []EventLayoutPane
}

// EventPaneScreenColor is a protocol-owned terminal color description.
// Kind is one of "", "palette", "rgb", or "default". Palette colors use Index;
// RGB colors use R/G/B. The protocol package intentionally does not depend on
// libghostty or UI packages.
type EventPaneScreenColor struct {
	Kind  string
	Index uint8
	R     uint8
	G     uint8
	B     uint8
}

// EventPaneScreenCell is a single styled terminal cell/grapheme in a pane
// viewport snapshot. Text should be bounded by terminal cell semantics at the
// producer; validation bounds the number of cells per line.
type EventPaneScreenCell struct {
	Text          string
	Foreground    EventPaneScreenColor
	Background    EventPaneScreenColor
	Bold          bool
	Italic        bool
	Faint         bool
	Blink         bool
	Inverse       bool
	Invisible     bool
	Underline     bool
	Strikethrough bool
	Overline      bool
}

// EventPaneScreenLine carries plain line text for tests/simple consumers plus
// per-cell style metadata for renderers.
type EventPaneScreenLine struct {
	Text  string
	Cells []EventPaneScreenCell
}

// EventPaneScreenCursor is the terminal cursor state in pane-local cell space.
// Col and Row are zero-based and valid only when Visible is true.
type EventPaneScreenCursor struct {
	Visible bool
	Col     int
	Row     int
	Blink   bool
}

func (c EventPaneScreenCursor) Validate(cols, rows int) error {
	if !c.Visible {
		return nil
	}
	if c.Col < 0 || c.Row < 0 || c.Col >= cols || c.Row >= rows {
		return fmt.Errorf("invalid cursor %d,%d for size %dx%d", c.Col, c.Row, cols, rows)
	}
	return nil
}

func (c EventPaneScreenCursor) WithVisible(visible bool) EventPaneScreenCursor {
	c.Visible = visible
	return c
}

func NewEventPaneScreenCursor(col, row int, blink bool) EventPaneScreenCursor {
	return EventPaneScreenCursor{Visible: true, Col: col, Row: row, Blink: blink}
}

// EventPaneScreenChanged is emitted with the latest bounded viewport snapshot
// for a pane after terminal output or resize.
type EventPaneScreenChanged struct {
	SessionID SessionID
	WindowID  WindowID
	PaneID    PaneID
	Revision  uint64
	Cols      int
	Rows      int
	Lines     []EventPaneScreenLine
	Cursor    EventPaneScreenCursor
}

// EventPaneSplitCompleted correlates an accepted pane split back to the client
// that requested it. Geometry is still published through EventWindowLayoutChanged.
type EventPaneSplitCompleted struct {
	ClientID     ClientID
	RequestID    RequestID
	SessionID    SessionID
	WindowID     WindowID
	TargetPaneID PaneID
	NewPaneID    PaneID
	Revision     uint64
}

// EventCommandRejected reports a bounded command failure without crashing actors.
type EventCommandRejected struct {
	ClientID  ClientID
	RequestID RequestID
	SessionID SessionID
	WindowID  WindowID
	Command   string
	Reason    string
}
